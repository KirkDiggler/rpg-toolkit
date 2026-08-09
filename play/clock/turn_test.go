// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/suite"
)

type TurnSuite struct {
	suite.Suite
	turn *clock.Turn
}

func (s *TurnSuite) SetupTest() { s.turn = &clock.Turn{} }

func TestTurnSuite(t *testing.T) { suite.Run(t, new(TurnSuite)) }

func (s *TurnSuite) TestZeroValueIsIdle() {
	_, err := s.turn.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = s.turn.Round()
	s.Require().ErrorIs(err, clock.ErrIdle)
	order, err := s.turn.Order()
	s.Require().NoError(err) // empty list is an answer
	s.Empty(order)
	got, err := s.turn.Contains(&clock.ContainsInput{ID: "a"})
	s.Require().NoError(err)
	s.False(got) // false is an answer, idle included
}

func (s *TurnSuite) TestSetOrderStartsRoundOne() {
	out, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{
		{Kind: clock.RoundStarted, Round: 1},
		{Kind: clock.TurnStarted, Subject: "a", Round: 1},
	}, out.Milestones)
	active, err := s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), active)
	round, err := s.turn.Round()
	s.Require().NoError(err)
	s.Equal(1, round)
}

func (s *TurnSuite) TestSetOrderRejectsEmptyAndDuplicates() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: nil})
	s.Require().ErrorIs(err, clock.ErrBadOrder)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "a"}})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)
	_, err = s.turn.Active() // still idle after both rejections (R5 atomicity)
	s.Require().ErrorIs(err, clock.ErrIdle)
}

func (s *TurnSuite) TestContains() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	got, err := s.turn.Contains(&clock.ContainsInput{ID: "a"})
	s.Require().NoError(err)
	s.True(got)
	got, err = s.turn.Contains(&clock.ContainsInput{ID: "zz"})
	s.Require().NoError(err)
	s.False(got)
}

// TestSetOrderReplacesAndFailedSetOrderChangesNothing pins R5 from a
// populated clock and the design's replacement semantics.
func (s *TurnSuite) TestSetOrderReplacesAndFailedSetOrderChangesNothing() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
	s.Require().NoError(err)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x", "x"}})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)
	active, err := s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), active)
	order, err := s.turn.Order()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b", "c"}, order)
	out, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x", "y"}})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{
		{Kind: clock.RoundStarted, Round: 1},
		{Kind: clock.TurnStarted, Subject: "x", Round: 1},
	}, out.Milestones)
	active, err = s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("x"), active)
}

// TestNoAliasingThroughSetOrderOrOrder pins the defensive-copy invariants.
func (s *TurnSuite) TestNoAliasingThroughSetOrderOrOrder() {
	in := []core.EntityID{"a", "b"}
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: in})
	s.Require().NoError(err)
	in[0] = "mutated"
	order, err := s.turn.Order()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b"}, order)
	order[1] = "mutated"
	again, err := s.turn.Order()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b"}, again)
}

func (s *TurnSuite) TestEndAdvancesAndWraps() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)

	out, err := s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("b"), out.Next)
	s.False(out.RoundWrapped)
	s.Equal([]clock.Milestone{
		{Kind: clock.TurnEnded, Subject: "a", Round: 1},
		{Kind: clock.TurnStarted, Subject: "b", Round: 1},
	}, out.Milestones)

	out, err = s.turn.End(&clock.EndInput{Actor: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), out.Next)
	s.True(out.RoundWrapped)
	// Wrap: TurnEnded carries the OLD round; RoundStarted/TurnStarted the NEW (design: Milestone).
	s.Equal([]clock.Milestone{
		{Kind: clock.TurnEnded, Subject: "b", Round: 1},
		{Kind: clock.RoundStarted, Round: 2},
		{Kind: clock.TurnStarted, Subject: "a", Round: 2},
	}, out.Milestones)
}

func (s *TurnSuite) TestEndErrors() {
	_, err := s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)
	_, err = s.turn.End(&clock.EndInput{Actor: "b"})
	s.Require().ErrorIs(err, clock.ErrNotActive)
	active, err := s.turn.Active() // unchanged (R5)
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), active)
}
