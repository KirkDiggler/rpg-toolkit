// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type composerTraceSuite struct{ suite.Suite }

func TestComposerTraceSuite(t *testing.T) { suite.Run(t, new(composerTraceSuite)) }

func (s *composerTraceSuite) TestLosingProposalIsNotCommitted() {
	high := compositionSpec("investigate", "investigate", 1)
	low := compositionSpec("patrol", "patrol", 100)
	low.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{
		On: composeTick, To: "spent", Do: []machine.Act{{Set: &machine.SetAct{Key: "count", Value: 9}}, {Pick: "work"}},
	}}}
	config := compositionConfig(high, low)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	before := c.ToData()
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	out, err := c.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die, Trace: true})
	s.Require().NoError(err)
	s.Equal(1, die.used)
	s.Equal("investigate", out.Instance)
	s.Require().Len(out.Outcomes, 2)
	winner, loser := out.Outcomes[0], out.Outcomes[1]
	s.Equal("investigate", winner.ID)
	s.True(winner.Selected)
	s.True(winner.Committed)
	s.Equal("ready", winner.Before)
	s.Equal("spent", winner.After)
	s.Equal(behavior.AnswerKey("work"), winner.Requested)
	s.Equal(before.Instances[0].Definition, winner.Definition)
	s.Equal("patrol", loser.ID)
	s.False(loser.Selected)
	s.False(loser.Committed)
	s.Equal("ready", loser.Before)
	s.Equal("ready", loser.After)
	s.Require().NotNil(loser.Step)
	s.Equal("spent", loser.Step.Trace.After, "a preview is a proposal, not a state-change record")
	s.Equal(behavior.AnswerKey("work"), loser.Requested)
	s.Equal(before.Instances[1], c.ToData().Instances[1], "neither proposed Set nor To commits")
	s.Equal([]string{"patrol"}, out.Deferred)
}

func (s *composerTraceSuite) TestSleepingAndAddressedDeliveryAreNotFalseGuards() {
	spec := compositionSpec("sleeping", "work", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{
		{On: composeHeard, Do: []machine.Act{{Set: &machine.SetAct{Key: "observed", Value: true}}}},
		{On: composeTick, Do: []machine.Act{{Pick: "work"}}},
	}}
	config := compositionConfig(spec, compositionSpec("awake", "awake", 1))
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	wake := uint64(8)
	data.Instances[0].Blob.WakeAt = &wake
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
	out, err := c.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Now: 3, Die: die, Trace: true})
	s.Require().NoError(err)
	s.Require().Len(out.Outcomes, 2)
	s.Equal(behavior.TraceSleeping, out.Outcomes[0].Skipped)
	s.Nil(out.Outcomes[0].Step)
	s.False(out.Outcomes[0].Committed)
	s.Empty(out.Deferred, "sleep is not a request losing precedence")
	s.True(out.Outcomes[1].Selected)
	observed, err := c.Observe(&behavior.ObserveInput{On: composeHeard, Instance: "sleeping", Now: 3, Trace: true})
	s.Require().NoError(err)
	s.Require().Len(observed.Outcomes, 2)
	s.Empty(observed.Outcomes[0].Skipped)
	s.True(observed.Outcomes[0].Committed)
	s.False(observed.Outcomes[0].Selected, "data-only delivery selects no work")
	s.Equal(behavior.TraceNotAddressed, observed.Outcomes[1].Skipped)
	s.Nil(observed.Outcomes[1].Step)
	s.Equal(true, c.ToData().Instances[0].Blob.Bag["observed"])
	s.Equal(uint64(8), *c.ToData().Instances[0].Blob.WakeAt)
}

func (s *composerTraceSuite) TestNoCandidateAndExplicitHoldDiffer() {
	for _, explicit := range []bool{false, true} {
		s.Run(map[bool]string{false: "no candidate", true: "explicit hold"}[explicit], func() {
			spec := compositionSpec("work", "unused", 1)
			if explicit {
				spec.Table["work"] = []behavior.Answer{{Weight: 1, Hold: true}}
			} else {
				spec.Table["work"] = []behavior.Answer{{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 4}}}}
			}
			config := compositionConfig(spec, compositionSpec("lower", "must not choose", 1))
			c, err := behavior.NewComposer(&config)
			s.Require().NoError(err)
			die := &exactDie{}
			if explicit {
				die.steps = []rollStep{{sides: 100, face: 1}}
			}
			out, err := c.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die, Trace: true})
			s.Require().NoError(err)
			s.Require().Len(out.Outcomes, 2)
			s.True(out.Outcomes[0].Selected)
			s.Equal(explicit, out.Outcomes[0].Committed)
			s.Equal("spent", out.Outcomes[0].Step.Trace.After)
			if explicit {
				s.Equal("spent", out.Outcomes[0].After)
				s.Equal(1, die.used)
			} else {
				s.Equal("ready", out.Outcomes[0].After)
				s.Zero(die.used)
				s.Equal(-1, out.Pick.Entry)
			}
			s.False(out.Outcomes[1].Committed)
			s.Equal("ready", out.Outcomes[1].After)
		})
	}
}

func (s *composerTraceSuite) TestTraceDoesNotChangeChoicesOrReadCounts() {
	var saved behavior.ComposerData
	var picked *behavior.PickOutput
	for _, trace := range []bool{false, true} {
		config := compositionConfig(compositionSpec("high", "high", 1), compositionSpec("low", "low", 1))
		reads := 0
		config.Vocabulary.Facts = map[machine.FactName]machine.FactSpec{"available": {Kind: machine.KindBool, Read: func(any) (any, bool) { reads++; return true, true }}}
		for i := range config.Instances {
			state := config.Instances[i].Recipe.States["ready"]
			state.Transitions[0].When = &machine.Guard{Fact: &machine.FactLeaf{Name: "available", Op: machine.OpEq, Value: true}}
			config.Instances[i].Recipe.States["ready"] = state
		}
		c, err := behavior.NewComposer(&config)
		s.Require().NoError(err)
		die := &exactDie{steps: []rollStep{{sides: 100, face: 73}}}
		out, err := c.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Die: die, Trace: trace})
		s.Require().NoError(err)
		s.Equal(2, reads, "one read per machine, not an explanation rerun")
		s.Equal(1, die.used, "no roll for a deferred table")
		if !trace {
			s.Nil(out.Outcomes)
			saved, picked = c.ToData(), out.Pick
		} else {
			s.Equal(saved, c.ToData())
			s.Equal(picked, out.Pick)
			s.Require().Len(out.Outcomes, 2)
			s.Equal(true, out.Outcomes[0].Step.Trace.Transitions[0].Guard.Value)
		}
	}
}

func (s *composerTraceSuite) TestReturnedTraceCannotMutateProgressOrConfiguration() {
	spec := compositionSpec("worker", "work", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{On: composeHeard, To: "spent", Do: []machine.Act{{Set: &machine.SetAct{Key: "count", Value: 2}}}}}}
	config := compositionConfig(spec)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	out, err := c.Observe(&behavior.ObserveInput{On: composeHeard, Trace: true})
	s.Require().NoError(err)
	saved := c.ToData()
	s.Require().Len(out.Outcomes, 1)
	out.Outcomes[0].Step.Step.Acts[0].Set.Value = 99
	out.Outcomes[0].Step.Trace.Transitions[0].On = "changed"
	out.Outcomes[0].After = "changed"
	s.Equal(saved, c.ToData())
	loaded, err := behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: saved})
	s.Require().NoError(err)
	s.Equal(saved, loaded.ToData())
}

func (s *composerTraceSuite) TestObservationPickRefusalStillAtomic() {
	passive := compositionSpec("listener", "unused", 1)
	passive.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{On: composeHeard, Do: []machine.Act{{Set: &machine.SetAct{Key: "count", Value: 2}}}}}}
	active := compositionSpec("worker", "work", 1)
	active.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{On: composeHeard, To: "spent", Do: []machine.Act{{Pick: "work"}}}}}
	config := compositionConfig(passive, active)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	before := c.ToData()
	out, err := c.Observe(&behavior.ObserveInput{On: composeHeard, Trace: true})
	s.ErrorIs(err, behavior.ErrSelectionRequired)
	s.Nil(out)
	s.Equal(before, c.ToData())
}

func (s *composerTraceSuite) TestAConsumedWakeCommitsWithoutAFiredTransition() {
	spec := compositionSpec("timer", "unused", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{On: composeHeard, To: "spent"}}}
	config := compositionConfig(spec)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	wake := uint64(8)
	data.Instances[0].Blob.WakeAt = &wake
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	out, err := c.Observe(&behavior.ObserveInput{On: composeTick, Now: 8, Trace: true})
	s.Require().NoError(err)
	s.Empty(out.Steps)
	s.Require().Len(out.Outcomes, 1)
	s.True(out.Outcomes[0].Committed)
	s.False(out.Outcomes[0].Step.Step.Fired)
	s.Nil(c.ToData().Instances[0].Blob.WakeAt)
}
