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
