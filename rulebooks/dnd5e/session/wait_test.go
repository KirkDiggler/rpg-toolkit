// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// wait_test.go is the session's Wait verb (rpg-project#1088, story #1079): a
// seated player outside a fight waits, costing one round on the world clock.
// The price and the refusals are the encounter's; the session adds only the
// downed gate and the open-for-change gate.

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

func (s *HoldStatusSuite) TestAHeldPlayerWaitsAndARoundPasses() {
	s.say("hold")
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: ambushPath()})
	s.requireHeld(err, "You are Held; you cannot move.")
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "precondition: alice is on the world clock")

	before := s.encounters.byID["sess"].Clock.HighWater
	_, err = s.mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "a hold does not refuse a wait")
	s.Equal(before+1, s.encounters.byID["sess"].Clock.HighWater, "one round passed")
}

func (s *SneakSuite) TestAWaitKeepsSneakingAndIsToldAsAWait() {
	s.launch(false, nil)
	s.setSneaking("alice", true)
	s.stream.published = nil

	_, err := s.mgr.Wait(context.Background(), &session.WaitInput{Session: testSession, Member: "alice"})
	s.Require().NoError(err)
	s.True(s.stored("alice").Sneaking, "waiting does not stop sneaking")

	found := false
	for _, a := range s.stream.published {
		if a.Verb != "wait" || a.Family != session.FamilyAct {
			continue
		}
		stay, ok := a.Body.(session.StayAct)
		if !ok {
			continue
		}
		s.Equal("encounter:intent:wait", stay.Cause)
		s.Equal("alice", stay.Member.ID)
		found = true
	}
	s.True(found, "a wait act with a StayAct body reached the stream")
}

func (s *SneakSuite) TestAWaitIsRefusedInAFight() {
	s.launch(true, nil)
	s.setSneaking("alice", true)
	s.move("alice", hexCell(2, 2), hexCell(2, 1))
	s.setSneaking("alice", false)
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: testSession, Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "precondition: a fight")

	before := s.encounters.byID[testSession].Clock.HighWater
	_, err = s.mgr.Wait(context.Background(), &session.WaitInput{Session: testSession, Member: "alice"})
	s.Require().Error(err)
	s.True(errors.Is(err, session.ErrInBubble), "got %v", err)
	s.Equal(before, s.encounters.byID[testSession].Clock.HighWater, "no round passed")
}

func (s *SneakSuite) TestWaitRefusesAStranger() {
	s.launch(false, nil)
	_, err := s.mgr.Wait(context.Background(), &session.WaitInput{Session: testSession, Member: "nobody"})
	s.ErrorIs(err, session.ErrNoMember)
	_, err = s.mgr.Wait(context.Background(), &session.WaitInput{Session: testSession, Member: ""})
	s.ErrorIs(err, session.ErrNoMemberID)
}

func (s *DeathTestSuite) TestADownedMemberCannotWait() {
	launchScene(s.T(), s.mgr, freeRoamDuelWorld())
	s.characters.byID["alice"].HitPoints = 0

	before := s.encounters.byID["sess"].Clock.HighWater
	_, err := s.mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
	s.Require().ErrorIs(err, session.ErrDowned)
	s.Equal(before, s.encounters.byID["sess"].Clock.HighWater, "no round was spent on a downed member")
	s.NotErrorIs(err, session.ErrNoMember, "she is still a member; she is down")

	_, err = s.mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "bob"})
	s.NotErrorIs(err, session.ErrDowned)
}
