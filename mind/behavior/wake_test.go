// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

type composerWakeSuite struct{ suite.Suite }

func TestComposerWakeSuite(t *testing.T) { suite.Run(t, new(composerWakeSuite)) }

func (s *composerWakeSuite) TestTimedInputDoesNotStepASleepingInstance() {
	config := compositionConfig(compositionSpec("sleeping", "sleeper", 1), compositionSpec("awake", "awake", 1))
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	at := uint64(8)
	data.Instances[0].Blob.WakeAt = &at
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	die := &exactDie{steps: []rollStep{{sides: 100, face: 1}, {sides: 100, face: 1}}}
	out, err := c.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Event: uint64(3), Now: 3, Die: die})
	s.Require().NoError(err)
	s.Equal("awake", out.Instance)
	s.Equal("awake", out.Pick.Answer.Say)
	s.Equal(data.Instances[0], c.ToData().Instances[0], "another role's opportunity must not spend the sleeper's progress")
	s.Empty(out.Deferred, "sleep is not a losing action request")
	s.Equal(1, die.used)
	out, err = c.Decide(context.Background(), &behavior.DecideInput{On: composeTick, Event: uint64(8), Now: 8, Die: die})
	s.Require().NoError(err)
	s.Equal("sleeping", out.Instance)
	s.Equal("sleeper", out.Pick.Answer.Say)
	s.Nil(c.ToData().Instances[0].Blob.WakeAt)
	s.Equal(2, die.used)
}

func (s *composerWakeSuite) TestTimedObservationKeepsOtherDataProgress() {
	passive := compositionSpec("passive", "unused", 1)
	passive.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{{
		On: composeTick, Do: []machine.Act{{Set: &machine.SetAct{Key: "observed", Value: true}}},
	}}}
	config := compositionConfig(compositionSpec("sleeping", "sleeper", 1), passive)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	at := uint64(8)
	data.Instances[0].Blob.WakeAt = &at
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	out, err := c.Observe(&behavior.ObserveInput{On: composeTick, Now: 3})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1)
	s.Equal("passive", out.Steps[0].ID)
	s.Equal(true, c.ToData().Instances[1].Blob.Bag["observed"])
	s.Equal(data.Instances[0], c.ToData().Instances[0])
}

func (s *composerWakeSuite) TestNonTimedInputStillReachesSleepingInstance() {
	spec := compositionSpec("sleeping", "work", 1)
	spec.Recipe.States["ready"] = machine.State{Transitions: []machine.Transition{
		{On: composeHeard, Do: []machine.Act{{WakeIn: 1}, {Set: &machine.SetAct{Key: "observed", Value: true}}}},
		{On: composeTick, Do: []machine.Act{{Pick: "work"}}},
	}}
	config := compositionConfig(spec)
	c, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	data := c.ToData()
	at := uint64(8)
	data.Instances[0].Blob.WakeAt = &at
	c, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
	s.Require().NoError(err)
	out, err := c.Observe(&behavior.ObserveInput{Instance: "sleeping", On: composeHeard, Now: 3})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1)
	s.Equal(true, c.ToData().Instances[0].Blob.Bag["observed"])
	s.Equal(uint64(4), *c.ToData().Instances[0].Blob.WakeAt, "the authored observation may replace its wake")
}
