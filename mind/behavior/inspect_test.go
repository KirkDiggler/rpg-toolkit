// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

type composerInspectSuite struct{ suite.Suite }

func TestComposerInspectSuite(t *testing.T) { suite.Run(t, new(composerInspectSuite)) }

func (s *composerInspectSuite) TestInspectionPreservesDistinctScopesAndTimedInputs() {
	narrow := compositionSpec("near", "near", 1)
	broad := compositionSpec("far", "far", 1)
	for i, spec := range []*behavior.InstanceSpec{&narrow, &broad} {
		distance := []int{2, 8}[i]
		state := spec.Recipe.States["ready"]
		state.Transitions = append(state.Transitions, machine.Transition{On: composeHeard,
			When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: distance}},
			Do:   []machine.Act{{Set: &machine.SetAct{Key: "observed", Value: true}}}})
		spec.Recipe.States["ready"] = state
	}
	fight := compositionSpec("fighter", "fight", 1)
	fight.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{
		{On: "turn", Do: []machine.Act{{Pick: "work"}}},
		{On: "turn", Do: []machine.Act{{Pick: "again"}}},
	}}
	config := compositionConfig(narrow, broad, fight)
	calls := 0
	config.Vocabulary.Beats[composeHeard] = machine.BeatSpec{InScope: func(any, machine.ScopeLeaf) bool { calls++; return true }}
	config.Vocabulary.Beats["turn"] = machine.BeatSpec{Timed: true}
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	before := c.ToData()
	out, err := c.Inspect(&behavior.InspectComposerInput{Now: 10})
	s.Require().NoError(err)
	s.Require().Len(out.Instances, 3)
	s.Equal("near", out.Instances[0].ID)
	s.Equal("far", out.Instances[1].ID)
	s.Equal("ready", out.Instances[0].State)
	s.Equal(before.Instances[0].Definition, out.Instances[0].Definition)
	s.Equal(2, out.Instances[0].Listens[0].Scope.Within)
	s.Equal(8, out.Instances[1].Listens[0].Scope.Within, "do not deduplicate another instance's scope")
	s.Equal([]machine.Beat{composeTick}, out.Instances[0].Timed)
	s.Equal([]machine.Beat{"turn"}, out.Instances[2].Timed, "each timed beat appears once")
	s.Equal(uint64(11), *out.Instances[0].DueAt)
	s.Zero(calls, "inspection executes no scope/fact callbacks")
	out.Instances[0].Listens[0].Scope.Within = 99
	out.Instances[0].Timed[0] = "changed"
	*out.Instances[0].DueAt = 99
	again, err := c.Inspect(&behavior.InspectComposerInput{Now: 10})
	s.Require().NoError(err)
	s.Equal(2, again.Instances[0].Listens[0].Scope.Within)
	s.Equal([]machine.Beat{composeTick}, again.Instances[0].Timed)
	s.Equal(uint64(11), *again.Instances[0].DueAt)
	s.Equal(before, c.ToData())
}

func (s *composerInspectSuite) TestFutureWakeUntimedAndTerminalAreDistinct() {
	awake := compositionSpec("sleeping", "work", 1)
	untimed := compositionSpec("quiet", "unused", 1)
	untimed.Recipe.States["ready"] = machine.State{}
	terminal := compositionSpec("ended", "unused", 1)
	terminal.Recipe.States["ready"] = machine.State{Terminal: true}
	config := compositionConfig(awake, untimed, terminal)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	at := uint64(8)
	data.Instances[0].Blob.WakeAt = &at
	data.Instances[2].Blob.WakeAt = &at
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	out, err := c.Inspect(&behavior.InspectComposerInput{Now: 3})
	s.Require().NoError(err)
	s.Equal(uint64(8), *out.Instances[0].DueAt)
	s.Equal(uint64(8), *out.Instances[0].WakeAt)
	s.Nil(out.Instances[1].DueAt)
	s.Nil(out.Instances[2].DueAt, "a terminal does not become due merely by holding a wake")
	*out.Instances[0].WakeAt = 99
	s.Equal(uint64(8), *c.ToData().Instances[0].Blob.WakeAt)
	state, err := c.StateOf(&behavior.InstanceStateInput{ID: "quiet"})
	s.Require().NoError(err)
	s.Equal("ready", state.State)
	s.Equal(data.Instances[1].Definition, state.Definition)
}

func (s *composerInspectSuite) TestNilUnknownAndEmptyInspection() {
	c, err := behavior.NewComposer(&behavior.ComposerConfig{})
	s.Require().NoError(err)
	out, err := c.Inspect(&behavior.InspectComposerInput{})
	s.Require().NoError(err)
	s.NotNil(out)
	s.Empty(out.Instances)
	out, err = c.Inspect(nil)
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrNilInput)
	state, err := c.StateOf(nil)
	s.Nil(state)
	s.ErrorIs(err, behavior.ErrNilInput)
	state, err = c.StateOf(&behavior.InstanceStateInput{ID: "missing"})
	s.Nil(state)
	s.ErrorIs(err, behavior.ErrUnknownInstance)
	shifted, err := c.DelayWakes(nil)
	s.Nil(shifted)
	s.ErrorIs(err, behavior.ErrNilInput)
	shifted, err = c.DelayWakes(&behavior.DelayWakesInput{Span: 5})
	s.Require().NoError(err)
	s.Zero(shifted.Shifted)
}

func (s *composerInspectSuite) TestDelayWakesChangesOnlyOwnedWakeTimes() {
	config := compositionConfig(compositionSpec("sleeping", "work", 1), compositionSpec("awake", "work", 1))
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	wake, anchor := uint64(8), uint64(2)
	data.Instances[0].Blob.WakeAt = &wake
	data.Instances[0].Blob.Anchor = &anchor
	data.Instances[0].Blob.Bag = machine.Bag{"observed": true}
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	unchanged := c.ToData()
	out, err := c.DelayWakes(&behavior.DelayWakesInput{})
	s.Require().NoError(err)
	s.Zero(out.Shifted)
	s.Equal(unchanged, c.ToData())
	out, err = c.DelayWakes(&behavior.DelayWakesInput{Span: 5})
	s.Require().NoError(err)
	s.Equal(1, out.Shifted)
	want := unchanged
	at := uint64(13)
	want.Instances[0].Blob.WakeAt = &at
	s.Equal(want, c.ToData())
	s.Equal(uint64(8), *data.Instances[0].Blob.WakeAt, "saved input was not retained as mutable state")
}

func (s *composerInspectSuite) TestWakeOverflowRefusesWithoutPartialCommit() {
	config := compositionConfig(compositionSpec("first", "work", 1), compositionSpec("second", "work", 1))
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	first, second := uint64(8), uint64(math.MaxUint64)
	data.Instances[0].Blob.WakeAt, data.Instances[1].Blob.WakeAt = &first, &second
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	before := c.ToData()
	out, err := c.DelayWakes(&behavior.DelayWakesInput{Span: 1})
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrBadWakeDelay)
	s.ErrorContains(err, "second")
	s.Equal(before, c.ToData(), "the earlier valid wake must not change before the later overflow refusal")
}
