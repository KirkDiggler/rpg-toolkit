// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
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

//nolint:goconst // repeated test fixture ID; _test.go exclusions inert until toolkit#904
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

// TestContainsAndMembersDistinguishZeroBudgetFromAbsent pins that a member
// at budget 0 is still a member — the distinction Ready (budget > 0) will
// tempt callers to blur once Advance lands.
func (s *TickSuite) TestContainsAndMembersDistinguishZeroBudgetFromAbsent() {
	_, err := s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().NoError(err)
	got, err := s.tick.Contains(&clock.ContainsInput{ID: "goblin"})
	s.Require().NoError(err)
	s.True(got, "zero-budget member is still a member")
	got, err = s.tick.Contains(&clock.ContainsInput{ID: "absent"})
	s.Require().NoError(err)
	s.False(got)
}

// TestMembersSortedOrder pins deterministic ordering against map iteration.
func (s *TickSuite) TestMembersSortedOrder() {
	for _, id := range []core.EntityID{"c", "a", "b"} {
		_, err := s.tick.Join(&clock.JoinInput{ID: id})
		s.Require().NoError(err)
	}
	members, err := s.tick.Members()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b", "c"}, members)
}

func (s *TickSuite) TestAdvanceHighWaterMaxNotSum() {
	_, err := s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().NoError(err)
	// four players each report displacement 3: members accrue 3, not 12
	for _, driver := range []core.EntityID{"p1", "p2", "p3", "p4"} {
		out, aerr := s.tick.Advance(&clock.AdvanceInput{Driver: driver, Displacement: 3})
		s.Require().NoError(aerr)
		s.Equal([]clock.Milestone{{Kind: clock.Ticked, Subject: driver}}, out.Milestones)
	}
	b, err := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal(3, b)

	// p1 pulls ahead by 2 more (cumulative 5): delta 2 granted
	out, err := s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 2})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"goblin"}, out.Ready)
	b, err = s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal(5, b)
}

func (s *TickSuite) TestAdvanceEdges() {
	_, err := s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().NoError(err)
	// zero displacement: legal, Ticked emitted, no grant
	out, err := s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 0})
	s.Require().NoError(err)
	s.Empty(out.Ready)
	// negative rejected, nothing changed (R5)
	_, err = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: -1})
	s.Require().ErrorIs(err, clock.ErrBadAmount)
	b, err := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Zero(b)
	// a member joining late starts at 0 and accrues only future deltas
	_, err = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 4})
	s.Require().NoError(err)
	_, err = s.tick.Join(&clock.JoinInput{ID: "late"})
	s.Require().NoError(err)
	_, err = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 1})
	s.Require().NoError(err)
	late, err := s.tick.Budget(&clock.BudgetInput{ID: "late"})
	s.Require().NoError(err)
	s.Equal(1, late)
}

func (s *TickSuite) TestSpend() {
	_, err := s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().NoError(err)
	_, err = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 3})
	s.Require().NoError(err)
	out, err := s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 2})
	s.Require().NoError(err)
	s.Empty(out.Milestones) // Spend emits no milestones (closed kind set)
	b, err := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal(1, b)

	_, err = s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 2})
	s.Require().ErrorIs(err, clock.ErrInsufficientBudget)
	_, err = s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 0})
	s.Require().ErrorIs(err, clock.ErrBadAmount)
	_, err = s.tick.Spend(&clock.SpendInput{ID: "zz", Amount: 1})
	s.Require().ErrorIs(err, clock.ErrNotMember)
}
