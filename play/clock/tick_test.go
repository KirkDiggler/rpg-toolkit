// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/suite"
)

type TickSuite struct {
	suite.Suite
	tick *clock.Tick
}

func (s *TickSuite) SetupTest() {
	var err error
	s.tick, err = clock.NewTick()
	s.Require().NoError(err)
}

func TestTickSuite(t *testing.T) { suite.Run(t, new(TickSuite)) }

//nolint:goconst // goconst exemption for test string
func (s *TickSuite) TestJoinLeave() {
	out, err := s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{{Kind: clock.MemberJoined, Subject: "goblin"}}, out.Milestones)
	b, err := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Zero(b)

	_, err = s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)

	lout, err := s.tick.Leave(&clock.LeaveInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "goblin"}}, lout.Milestones)
	_, err = s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().ErrorIs(err, clock.ErrNotMember)
	_, err = s.tick.Leave(&clock.LeaveInput{ID: "goblin"})
	s.Require().ErrorIs(err, clock.ErrNotMember)
}

func (s *TickSuite) TestMembersEmptySliceConvention() {
	members, err := s.tick.Members()
	s.Require().NoError(err)
	s.Empty(members)
}
