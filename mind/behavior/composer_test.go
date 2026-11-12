// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	composeTick  machine.Beat = "tick"
	composeHeard machine.Beat = "heard"
)

type rollStep struct {
	sides int
	face  int
	err   error
}

// exactDie is the only selection double. Unexpected sizes/calls are errors,
// so a hidden lower-priority roll cannot consume a plausible spare face.
type exactDie struct {
	steps []rollStep
	used  int
}

func (d *exactDie) Roll(_ context.Context, sides int) (int, error) {
	if d.used >= len(d.steps) {
		return 0, fmt.Errorf("unexpected d%d roll", sides)
	}
	step := d.steps[d.used]
	d.used++
	if sides != step.sides {
		return 0, fmt.Errorf("wanted d%d, asked d%d", step.sides, sides)
	}
	if step.err != nil {
		return 0, step.err
	}
	if step.face < 1 || step.face > sides {
		return 0, fmt.Errorf("scripted face %d outside d%d", step.face, sides)
	}
	return step.face, nil
}

func compositionVocabulary() machine.Vocabulary {
	return machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{
		composeTick:  {Timed: true, Round: func(ev any) (uint64, bool) { at, ok := ev.(uint64); return at, ok }},
		composeHeard: {},
	}}
}

func compositionSpec(id, line string, weight int) behavior.InstanceSpec {
	return behavior.InstanceSpec{
		ID: id,
		Recipe: machine.Recipe{
			Ref: core.Ref{Module: "test", Type: "behaviors", ID: id}, Initial: "ready",
			Bag: machine.Bag{"observed": false, "count": 0},
			States: map[string]machine.State{
				"ready": {Transitions: []machine.Transition{{On: composeTick,
					To: "spent", Do: []machine.Act{{Pick: "work"}}, Reason: "selected work"}}},
				"spent": {Transitions: []machine.Transition{{On: composeTick, Do: []machine.Act{{Pick: "again"}}}}},
			},
		},
		Table: behavior.Table{
			"work":  {{Weight: weight, Say: line}},
			"again": {{Weight: 1, Say: "already progressed"}},
		},
	}
}

func compositionConfig(specs ...behavior.InstanceSpec) behavior.ComposerConfig {
	return behavior.ComposerConfig{Instances: specs, Vocabulary: compositionVocabulary()}
}

type composerSuite struct{ suite.Suite }

// TestComposerSuite exercises the real machine/picker composition and its
// state ownership, with only the die replaced by an exact script.
func TestComposerSuite(t *testing.T) { suite.Run(t, new(composerSuite)) }

func (s *composerSuite) TestPriorityDoesNotBecomeWeightAndDeferredProgressStaysPut() {
	high := compositionSpec("investigate", "investigate", 1)
	high.Recipe.States["spent"] = machine.State{Terminal: true}
	config := compositionConfig(high, compositionSpec("patrol", "pending patrol", 100))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 10000, face: 1}}}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{
		On: composeTick, Event: uint64(1), Now: 1, Die: die,
	})
	s.Require().NoError(err)
	s.Equal("investigate", out.Instance)
	s.Equal("investigate", out.Pick.Answer.Say)
	s.Equal([]string{"patrol"}, out.Deferred)
	s.Equal("spent", composer.ToData().Instances[0].Blob.State)
	s.Equal("ready", composer.ToData().Instances[1].Blob.State)

	out, err = composer.Decide(context.Background(), &behavior.DecideInput{
		On: composeTick, Event: uint64(2), Now: 2, Die: die,
	})
	s.Require().NoError(err)
	s.Equal("patrol", out.Instance)
	s.Equal("pending patrol", out.Pick.Answer.Say, "not the row after a silently advanced continuation")
	s.Equal(2, die.used)
}

func (s *composerSuite) TestDataOnlyUpdatesCommitAlongsideOneSelectedInstance() {
	passive := compositionSpec("listener", "unused", 1)
	passive.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{
		On: composeTick, Do: []machine.Act{{Set: &machine.SetAct{Key: "observed", Value: true}}},
	}}}
	config := compositionConfig(passive, compositionSpec("worker", "work", 1))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die})
	s.Require().NoError(err)
	s.Equal("worker", out.Instance)
	s.Equal(true, composer.ToData().Instances[0].Blob.Bag["observed"])
	s.Len(out.Steps, 2, "both committed transitions are observable")
	s.Equal(1, die.used)
}

func (s *composerSuite) TestObserveCannotSilentlyExecuteOrDropARequestedPick() {
	spec := compositionSpec("worker", "work", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{
		{On: composeHeard, Do: []machine.Act{{Set: &machine.SetAct{Key: "observed", Value: true}}}},
		{On: composeTick, To: "spent", Do: []machine.Act{{Pick: "work"}}},
	}}
	config := compositionConfig(spec)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	observed, err := composer.Observe(&behavior.ObserveInput{On: composeHeard})
	s.Require().NoError(err)
	s.Len(observed.Steps, 1)
	s.Equal(true, composer.ToData().Instances[0].Blob.Bag["observed"])
	before := composer.ToData()
	observed, err = composer.Observe(&behavior.ObserveInput{On: composeTick})
	s.ErrorIs(err, behavior.ErrSelectionRequired)
	s.Nil(observed)
	s.Equal(before, composer.ToData())
}

func (s *composerSuite) TestObservationRefusalCommitsNoneOfTheBroadcast() {
	passive := compositionSpec("listener", "unused", 1)
	passive.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{
		On: composeHeard, Do: []machine.Act{{Set: &machine.SetAct{Key: "count", Value: 2}}},
	}}}
	active := compositionSpec("worker", "work", 1)
	active.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{
		On: composeHeard, To: "spent", Do: []machine.Act{{Pick: "work"}},
	}}}
	config := compositionConfig(passive, active)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	before := composer.ToData()
	out, err := composer.Observe(&behavior.ObserveInput{On: composeHeard})
	s.ErrorIs(err, behavior.ErrSelectionRequired)
	s.Nil(out)
	s.Equal(before, composer.ToData(), "the earlier data-only update must not leak through the later refusal")
}

func (s *composerSuite) TestCompletionCanAddressOneOfTwoInstancesOfTheSameDefinition() {
	first := compositionSpec("east", "unused", 1)
	first.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{On: composeHeard, To: "spent"}}}
	second := first
	second.ID = "west"
	config := compositionConfig(first, second)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	out, err := composer.Observe(&behavior.ObserveInput{Instance: "east", On: composeHeard})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1)
	s.Equal("east", out.Steps[0].ID)
	s.Equal("spent", composer.ToData().Instances[0].Blob.State)
	s.Equal("ready", composer.ToData().Instances[1].Blob.State, "completion must not advance an unrelated attachment")
	before := composer.ToData()
	out, err = composer.Observe(&behavior.ObserveInput{Instance: "missing", On: composeHeard})
	s.ErrorIs(err, behavior.ErrUnknownInstance)
	s.Nil(out)
	s.Equal(before, composer.ToData())
}

func (s *composerSuite) TestRollFailureCommitsNoPrivateProgress() {
	passive := compositionSpec("listener", "unused", 1)
	passive.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{
		On: composeTick, Do: []machine.Act{{Set: &machine.SetAct{Key: "count", Value: 2}}},
	}}}
	config := compositionConfig(passive, compositionSpec("worker", "work", 1))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	before := composer.ToData()
	boom := errors.New("die unavailable")
	die := &exactDie{steps: []rollStep{{sides: 100, err: boom}}}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die})
	s.ErrorIs(err, boom)
	s.Nil(out)
	s.Equal(before, composer.ToData())
	s.Equal(1, die.used, "the external die call is not rolled back")
}

func (s *composerSuite) TestNoEligibleRowHoldsWithoutProgressOrAnotherRolesRoll() {
	high := compositionSpec("blocked", "unused", 1)
	high.Table["work"] = []behavior.Answer{{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 4}}}}
	config := compositionConfig(high, compositionSpec("other", "must not run", 1))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	before := composer.ToData()
	die := &exactDie{}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die})
	s.Require().NoError(err)
	s.Equal("blocked", out.Instance)
	s.True(out.Pick.Answer.Hold)
	s.Equal(-1, out.Pick.Entry)
	s.Zero(die.used)
	s.Equal(before, composer.ToData(), "no eligible work must not consume the pending step")
	s.Empty(out.Steps)
}

func (s *composerSuite) TestExplicitHoldIsAnEligibleSelection() {
	spec := compositionSpec("wait", "unused", 1)
	spec.Table["work"] = []behavior.Answer{{Weight: 1, Hold: true}}
	config := compositionConfig(spec)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die})
	s.Require().NoError(err)
	s.True(out.Pick.Answer.Hold)
	s.Equal(0, out.Pick.Entry)
	s.Equal("spent", composer.ToData().Instances[0].Blob.State)
}

func (s *composerSuite) TestEmptyNamedTableIsAHoldNotAMissingTable() {
	spec := compositionSpec("empty", "unused", 1)
	spec.Table["work"] = nil
	config := compositionConfig(spec)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick})
	s.Require().NoError(err)
	s.True(out.Pick.Answer.Hold)
	s.Equal(behavior.AnswerKey("work"), out.Pick.Key)
	s.Equal("ready", composer.ToData().Instances[0].Blob.State)
}

func (s *composerSuite) TestConfigurationAndReturnedAnswersAreDetached() {
	point := &spatial.Position{X: 4, Y: 2}
	spec := compositionSpec("walk", "unused", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{
		{On: composeTick, Do: []machine.Act{{Pick: "work"}}},
	}}
	spec.Table["work"] = []behavior.Answer{{Weight: 1, Toward: &behavior.Selector{At: point}}}
	config := compositionConfig(spec)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	point.X = 99
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}}}
	input := &behavior.DecideInput{On: composeTick, Facts: behavior.Facts{CanMove: true}, Die: die}
	out, err := composer.Decide(context.Background(), input)
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4, Y: 2}, *out.Pick.Answer.Toward.At)
	out.Pick.Answer.Toward.At.X = 88
	out, err = composer.Decide(context.Background(), input)
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4, Y: 2}, *out.Pick.Answer.Toward.At)
	s.Equal(2, die.used)
}

func (s *composerSuite) TestFactProjectionCannotMutateTheStoredTable() {
	spec := compositionSpec("walk", "unused", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{
		{On: composeTick, Do: []machine.Act{{Pick: "work"}}},
	}}
	spec.Table["work"] = []behavior.Answer{{Weight: 1,
		When:   []behavior.When{{Threat: behavior.ThreatNone}},
		Toward: &behavior.Selector{At: &spatial.Position{X: 4}},
	}}
	config := compositionConfig(spec)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}}}
	_, err = composer.Decide(context.Background(), &behavior.DecideInput{
		On: composeTick, Die: die,
		Facts: behavior.Facts{CanMove: true, Threat: func(row behavior.Answer) behavior.ThreatWord {
			row.Toward.At.X = 99
			return behavior.ThreatNone
		}},
	})
	s.Require().NoError(err)
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{
		On: composeTick, Die: die, Facts: behavior.Facts{CanMove: true,
			Threat: func(behavior.Answer) behavior.ThreatWord { return behavior.ThreatNone }},
	})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4}, *out.Pick.Answer.Toward.At,
		"a fact reader must not gain a writer to the stored definition")
}

func (s *composerSuite) TestSnapshotRoundTripAndCopyOut() {
	config := compositionConfig(compositionSpec("worker", "work", 1))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	_, err = composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die})
	s.Require().NoError(err)
	data := composer.ToData()
	data.Instances[0].Blob.Bag = machine.Bag{"count": 2}
	stamp := uint64(7)
	data.Instances[0].Blob.Anchor = &stamp
	wake := uint64(9)
	data.Instances[0].Blob.WakeAt = &wake
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	var decoded behavior.ComposerData
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	loaded, err := behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: decoded})
	s.Require().NoError(err)
	decoded.Instances[0].Blob.Bag["count"] = 99
	*decoded.Instances[0].Blob.Anchor = 99
	*decoded.Instances[0].Blob.WakeAt = 99
	got := loaded.ToData()
	s.Equal(2, got.Instances[0].Blob.Bag["count"])
	s.Equal(uint64(7), *got.Instances[0].Blob.Anchor)
	s.Equal(uint64(9), *got.Instances[0].Blob.WakeAt)
	*got.Instances[0].Blob.WakeAt = 88
	s.Equal(uint64(9), *loaded.ToData().Instances[0].Blob.WakeAt)
	got.Instances[0].Blob.Bag["count"] = 88
	s.Equal(2, loaded.ToData().Instances[0].Blob.Bag["count"])
	die = &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	out, err := loaded.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Now: 8, Die: die})
	s.Require().NoError(err)
	s.Nil(out.Pick, "a restored future wake still blocks early timed selection")
	s.Zero(die.used)
	out, err = loaded.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Now: 9, Die: die})
	s.Require().NoError(err)
	s.Require().NotNil(out.Pick)
	s.Equal("already progressed", out.Pick.Answer.Say)
}

func (s *composerSuite) TestLoadRefusesChangedIdentityOrderAndMalformedProgress() {
	config := compositionConfig(compositionSpec("a", "a", 1), compositionSpec("b", "b", 1))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	for _, tc := range []struct {
		name  string
		spoil func(*behavior.ComposerData)
	}{
		{"count", func(d *behavior.ComposerData) { d.Instances = d.Instances[:1] }},
		{"order", func(d *behavior.ComposerData) { d.Instances[0], d.Instances[1] = d.Instances[1], d.Instances[0] }},
		{"digest", func(d *behavior.ComposerData) { d.Instances[0].Definition = "changed" }},
		{"unknown state", func(d *behavior.ComposerData) { d.Instances[0].Blob.State = "missing" }},
		{"empty state", func(d *behavior.ComposerData) { d.Instances[0].Blob.State = "" }},
		{"unknown bag key", func(d *behavior.ComposerData) { d.Instances[0].Blob.Bag = machine.Bag{"extra": 1} }},
		{"wrong bag kind", func(d *behavior.ComposerData) { d.Instances[0].Blob.Bag = machine.Bag{"count": "2"} }},
	} {
		s.Run(tc.name, func() {
			data := composer.ToData()
			tc.spoil(&data)
			loaded, err := behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
			s.ErrorIs(err, behavior.ErrBadComposerData)
			s.Nil(loaded)
		})
	}
	config.Instances[0].Table["work"][0].Say = "new configuration"
	_, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: composer.ToData()})
	s.ErrorIs(err, behavior.ErrBadComposerData)
}

func (s *composerSuite) TestConstructionRefusesUnusableDefinitions() {
	for _, tc := range []struct {
		name  string
		spoil func(*behavior.ComposerConfig)
	}{
		{"missing ID", func(c *behavior.ComposerConfig) { c.Instances[0].ID = "" }},
		{"duplicate ID", func(c *behavior.ComposerConfig) { c.Instances = append(c.Instances, c.Instances[0]) }},
		{"missing table", func(c *behavior.ComposerConfig) { delete(c.Instances[0].Table, "work") }},
		{"bad weight", func(c *behavior.ComposerConfig) { c.Instances[0].Table["work"][0].Weight = 0 }},
		{"two words", func(c *behavior.ComposerConfig) {
			c.Instances[0].Table["work"][0] = behavior.Answer{Weight: 1, Hold: true, Flee: true}
		}},
		{"multiple picks", func(c *behavior.ComposerConfig) {
			st := c.Instances[0].Recipe.States["ready"]
			st.Transitions[0].Do = append(st.Transitions[0].Do, machine.Act{Pick: "again"})
			c.Instances[0].Recipe.States["ready"] = st
		}},
		{"missing reader", func(c *behavior.ComposerConfig) {
			c.Vocabulary.Facts = map[machine.FactName]machine.FactSpec{"x": {Kind: machine.KindBool}}
		}},
	} {
		s.Run(tc.name, func() {
			config := compositionConfig(compositionSpec("worker", "work", 1))
			tc.spoil(&config)
			composer, err := behavior.NewComposer(&config)
			s.ErrorIs(err, behavior.ErrBadComposition)
			s.Nil(composer)
		})
	}
}

func (s *composerSuite) TestNilUnknownAndEmptyHaveExplicitOutcomes() {
	composer, err := behavior.NewComposer(nil)
	s.ErrorIs(err, behavior.ErrNilInput)
	s.Nil(composer)
	_, err = behavior.LoadComposer(nil)
	s.ErrorIs(err, behavior.ErrNilInput)
	config := compositionConfig()
	composer, err = behavior.NewComposer(&config)
	s.Require().NoError(err)
	_, err = composer.Observe(nil)
	s.ErrorIs(err, behavior.ErrNilInput)
	_, err = composer.Decide(context.Background(), nil)
	s.ErrorIs(err, behavior.ErrNilInput)
	_, err = composer.Observe(&behavior.ObserveInput{On: "unknown"})
	s.ErrorIs(err, machine.ErrUnknownBeat)
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick})
	s.Require().NoError(err)
	s.Empty(out.Instance)
	s.Nil(out.Pick)
	s.Empty(out.Steps)
}

// TestALineCarriesItsLoudnessUnread: a row's loudness comes back on the pick
// exactly as authored, through the composer's owned copy, and is never read.
func (s *composerSuite) TestALineCarriesItsLoudnessUnread() {
	spec := compositionSpec("crier", "To arms!", 1)
	spec.Table["work"][0].Loudness = "loud"
	config := compositionConfig(spec)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die})
	s.Require().NoError(err)
	s.Require().NotNil(out.Pick)
	s.Equal("To arms!", out.Pick.Answer.Say)
	s.Equal("loud", out.Pick.Answer.Loudness, "carried as authored")
}

// TestATableWithoutLoudnessKeepsItsDigest: a definition authored before the
// field existed digests exactly as it did (the value pinned from main), and a
// loudness, once authored, is part of the definition.
func (s *composerSuite) TestATableWithoutLoudnessKeepsItsDigest() {
	config := compositionConfig(compositionSpec("worker", "work", 1))
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	s.Equal("38ebab5ccf0b366cdf68a03f9efeb796c86eded4c45d485e06f10820526b2784",
		composer.ToData().Instances[0].Definition, "an existing table is unchanged byte for byte")

	loud := compositionSpec("worker", "work", 1)
	loud.Table["work"][0].Loudness = "loud"
	loudConfig := compositionConfig(loud)
	louder, err := behavior.NewComposer(&loudConfig)
	s.Require().NoError(err)
	s.NotEqual(composer.ToData().Instances[0].Definition, louder.ToData().Instances[0].Definition,
		"an authored loudness is part of the definition")
}
