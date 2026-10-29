// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

const holder = "barbarian-1"

// rageRig is one Rage machine on one bus with its listeners recorded.
type rageRig struct {
	s       *suite.Suite
	ctx     context.Context
	bus     events.EventBus
	m       *machine.Machine
	seen    []machine.Transitioned
	changed int
}

func newRageRig(s *suite.Suite, r machine.Recipe) *rageRig {
	p, err := r.Compile(rageVocabulary())
	s.Require().NoError(err)
	rig := &rageRig{s: s, ctx: context.Background(), bus: events.NewEventBus()}
	rig.m = machine.New(p, holder, rageSource(), nil, "")
	rig.m.OnTransition(func(_ context.Context, t machine.Transitioned) error {
		rig.seen = append(rig.seen, t)
		return nil
	})
	rig.m.OnChanged(func(context.Context) error {
		rig.changed++
		return nil
	})
	s.Require().NoError(rig.m.Apply(rig.ctx, rig.bus))
	return rig
}

func (r *rageRig) turnEnd(round int) {
	r.s.Require().NoError(turnEndTopic.On(r.bus).Publish(r.ctx, turnEnd{SubjectID: holder, Round: round}))
}

func (r *rageRig) attack() {
	_, err := postAttackRollChain.On(r.bus).PublishWithChain(r.ctx, postAttackRoll{AttackerID: holder},
		events.NewStagedChain[postAttackRoll](nil))
	r.s.Require().NoError(err)
}

func (r *rageRig) hit() {
	r.s.Require().NoError(damageReceivedTopic.On(r.bus).Publish(r.ctx, damageReceived{TargetID: holder}))
}

// expectOneMore asserts the listener saw exactly one new transition, want.
func (r *rageRig) expectOneMore(before int, want machine.Transitioned) {
	r.s.Require().Len(r.seen, before+1, "exactly one transition")
	r.s.Equal(want, r.seen[before])
	r.s.Equal(want.To, r.m.State())
}

// graced starts the rage on the activation turn end at round.
func (r *rageRig) graced(round int) {
	r.turnEnd(round)
	r.expectOneMore(0, machine.Transitioned{From: stFresh, To: stRaging})
}

type rageWalkSuite struct {
	suite.Suite
	rig *rageRig
}

func (s *rageWalkSuite) SetupTest() { s.rig = newRageRig(&s.Suite, rageRecipe()) }

func (s *rageWalkSuite) TestFirstTurnEndIsGracedAndAnchors() {
	s.Equal(stFresh, s.rig.m.State())
	s.rig.graced(1) // no flags set: no sustain check on the activation turn

	bag := s.rig.m.Bag()
	s.Equal(1, bag[machine.KeyRoundActivated])
	s.Equal(true, bag[machine.KeySawTurnEnd])
	s.Equal(false, bag[keyWasHit])
	s.Equal(false, bag[keyDidAttack])
	s.Equal(0, bag[machine.KeyTurnsActive], "the design's fresh transition derives nothing")
}

func (s *rageWalkSuite) TestAnAttackSustains() {
	s.rig.graced(1)
	s.rig.attack()
	s.Equal(true, s.rig.m.Bag()[keyDidAttack])
	s.rig.turnEnd(2)

	s.Len(s.rig.seen, 1, "staying raging fires no transition")
	s.Equal(stRaging, s.rig.m.State())
	bag := s.rig.m.Bag()
	s.Equal(false, bag[keyDidAttack], "flags cleared")
	s.Equal(false, bag[keyWasHit])
	s.Equal(2, bag[machine.KeyTurnsActive])
}

func (s *rageWalkSuite) TestBeingHitSustains() {
	s.rig.graced(1)
	s.rig.hit()
	s.rig.turnEnd(2)
	s.Equal(stRaging, s.rig.m.State())
}

func (s *rageWalkSuite) TestATurnWithNoActivityEnds() {
	s.rig.graced(1)
	s.rig.turnEnd(2)
	s.rig.expectOneMore(1, machine.Transitioned{From: stRaging, To: stEnded, Reason: reasonNoActivity})
	s.False(s.rig.m.IsApplied(), "entering the terminal state detaches everything")

	s.rig.attack()
	s.rig.turnEnd(3)
	s.Len(s.rig.seen, 2, "a detached machine hears nothing")
}

func (s *rageWalkSuite) TestTenRoundsExpire() {
	s.rig.graced(1)
	for round := 2; round <= 9; round++ {
		s.rig.attack()
		s.rig.turnEnd(round)
		s.Equal(stRaging, s.rig.m.State(), "round %d", round)
	}
	s.Equal(9, s.rig.m.Bag()[machine.KeyTurnsActive])
	s.rig.attack()
	s.rig.turnEnd(10)
	s.rig.expectOneMore(1, machine.Transitioned{From: stRaging, To: stEnded, Reason: reasonExpired})
}

func (s *rageWalkSuite) TestRoundZeroSkipsTheCap() {
	s.rig.graced(1)
	for round := 2; round <= 9; round++ {
		s.rig.attack()
		s.rig.turnEnd(round)
	}
	s.rig.attack()
	s.rig.turnEnd(0)
	s.Len(s.rig.seen, 1, "no cap evaluation and no clock reset on a round-less turn end")
	s.Equal(stRaging, s.rig.m.State())
	bag := s.rig.m.Bag()
	s.Equal(9, bag[machine.KeyTurnsActive], "nothing to derive from round 0")
	s.Equal(false, bag[keyDidAttack], "flags still cleared")
}

func (s *rageWalkSuite) TestRoundBelowTheAnchorResets() {
	s.rig.graced(5)
	s.rig.attack()
	s.rig.turnEnd(2)
	s.rig.expectOneMore(1, machine.Transitioned{From: stRaging, To: stEnded, Reason: "clock_reset"})
}

func (s *rageWalkSuite) TestEndings() {
	cases := []struct {
		reason  string
		publish func(r *rageRig)
	}{
		{"unconscious", func(r *rageRig) {
			r.s.Require().NoError(droppedToZeroTopic.On(r.bus).Publish(r.ctx, droppedToZero{MemberID: holder}))
		}},
		{reasonRest, func(r *rageRig) {
			r.s.Require().NoError(restTopic.On(r.bus).Publish(r.ctx, rest{CharacterID: holder}))
		}},
		{"combat_ended", func(r *rageRig) {
			r.s.Require().NoError(combatEndTopic.On(r.bus).Publish(r.ctx, combatEnd{SubjectID: holder}))
		}},
	}
	for _, tc := range cases {
		s.Run(tc.reason+" from raging", func() {
			rig := newRageRig(&s.Suite, rageRecipe())
			rig.graced(1)
			tc.publish(rig)
			rig.expectOneMore(1, machine.Transitioned{From: stRaging, To: stEnded, Reason: tc.reason})
			s.False(rig.m.IsApplied())
		})
		s.Run(tc.reason+" from fresh", func() {
			rig := newRageRig(&s.Suite, rageRecipe())
			tc.publish(rig)
			rig.expectOneMore(0, machine.Transitioned{From: stFresh, To: stEnded, Reason: tc.reason})
		})
	}
}

func (s *rageWalkSuite) TestEventsAboutSomeoneElseAreIgnored() {
	s.Require().NoError(turnEndTopic.On(s.rig.bus).Publish(s.rig.ctx, turnEnd{SubjectID: otherHolder, Round: 1}))
	s.Equal(stFresh, s.rig.m.State())
	s.Require().NoError(restTopic.On(s.rig.bus).Publish(s.rig.ctx, rest{CharacterID: otherHolder}))
	s.Equal(stFresh, s.rig.m.State())

	s.rig.graced(1)
	s.Require().NoError(damageReceivedTopic.On(s.rig.bus).Publish(s.rig.ctx, damageReceived{TargetID: otherHolder}))
	s.Equal(false, s.rig.m.Bag()[keyWasHit])
}

func TestRageWalk(t *testing.T) {
	suite.Run(t, new(rageWalkSuite))
}

type orderingSuite struct {
	suite.Suite
}

// At round 10 with no activity, the sustain check and the cap would both
// fire; recipe order decides, and the first that applies wins.
func (s *orderingSuite) TestSustainCheckPrecedesTheCap() {
	rig := newRageRig(&s.Suite, rageRecipe())
	rig.graced(1)
	rig.turnEnd(10)
	rig.expectOneMore(1, machine.Transitioned{From: stRaging, To: stEnded, Reason: reasonNoActivity})
}

func (s *orderingSuite) TestRecipeOrderIsTheOrder() {
	r := rageRecipe()
	edit(&r, stRaging, func(st *machine.State) {
		st.Transitions[0], st.Transitions[1] = st.Transitions[1], st.Transitions[0]
	})
	rig := newRageRig(&s.Suite, r)
	rig.graced(1)
	rig.turnEnd(10)
	rig.expectOneMore(1, machine.Transitioned{From: stRaging, To: stEnded, Reason: reasonExpired})
}

func TestOrderingOnOneBeat(t *testing.T) {
	suite.Run(t, new(orderingSuite))
}

type trackerSuite struct {
	suite.Suite
}

func (s *trackerSuite) TestTwoHitsInOneTurnChangeOnce() {
	rig := newRageRig(&s.Suite, rageRecipe())
	rig.graced(1)
	before := rig.changed
	rig.hit()
	rig.hit()
	s.Equal(before+1, rig.changed, "the second hit finds the flag set")
	s.Equal(true, rig.m.Bag()[keyWasHit])
}

func (s *trackerSuite) TestTheFlagReArmsAfterTheTurn() {
	rig := newRageRig(&s.Suite, rageRecipe())
	rig.graced(1)
	rig.attack()
	rig.turnEnd(2)
	before := rig.changed
	rig.attack()
	s.Equal(before+1, rig.changed)
}

func TestTrackerIdempotent(t *testing.T) {
	suite.Run(t, new(trackerSuite))
}
