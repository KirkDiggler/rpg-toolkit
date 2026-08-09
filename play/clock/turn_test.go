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

	// wrap into round 2, then a further replacement must reset to round 1
	_, err = s.turn.End(&clock.EndInput{Actor: "x"})
	s.Require().NoError(err)
	_, err = s.turn.End(&clock.EndInput{Actor: "y"})
	s.Require().NoError(err)
	round, err := s.turn.Round()
	s.Require().NoError(err)
	s.Equal(2, round)
	out, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"z"}})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{
		{Kind: clock.RoundStarted, Round: 1},
		{Kind: clock.TurnStarted, Subject: "z", Round: 1},
	}, out.Milestones)
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

// TestEndSingleMemberWraps pins Next == Actor on a one-member clock —
// every End wraps, and a composition loop waiting for Active to change
// would hang; this pin documents the contract.
func (s *TurnSuite) TestEndSingleMemberWraps() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	out, err := s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), out.Next)
	s.True(out.RoundWrapped)
	s.Equal([]clock.Milestone{
		{Kind: clock.TurnEnded, Subject: "a", Round: 1},
		{Kind: clock.RoundStarted, Round: 2},
		{Kind: clock.TurnStarted, Subject: "a", Round: 2},
	}, out.Milestones)
}

func (s *TurnSuite) TestInsertKeepsActiveEntityActive() {
	cases := []struct {
		name string
		pos  int
		want []core.EntityID
	}{
		{"before active", 0, []core.EntityID{"x", "a", "b", "c"}},
		{"at active", 1, []core.EntityID{"a", "x", "b", "c"}},
		{"after active", 2, []core.EntityID{"a", "b", "x", "c"}},
		{"at end", 3, []core.EntityID{"a", "b", "c", "x"}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			// fresh clock per subtest; advance so "b" (idx 1) is active
			s.turn = &clock.Turn{}
			_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
			s.Require().NoError(err)
			_, err = s.turn.End(&clock.EndInput{Actor: "a"})
			s.Require().NoError(err)

			out, err := s.turn.Insert(&clock.InsertInput{ID: "x", Pos: tc.pos})
			s.Require().NoError(err)
			s.Equal([]clock.Milestone{{Kind: clock.MemberJoined, Subject: "x", Round: 1}}, out.Milestones)
			active, err := s.turn.Active()
			s.Require().NoError(err)
			s.Equal(core.EntityID("b"), active, "active entity must survive insert at pos %d", tc.pos)
			order, err := s.turn.Order()
			s.Require().NoError(err)
			s.Equal(tc.want, order)
		})
	}
}

func (s *TurnSuite) TestInsertErrors() {
	_, err := s.turn.Insert(&clock.InsertInput{ID: "x", Pos: 0})
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	_, err = s.turn.Insert(&clock.InsertInput{ID: "a", Pos: 0})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)
	_, err = s.turn.Insert(&clock.InsertInput{ID: "x", Pos: 2})
	s.Require().ErrorIs(err, clock.ErrBadPosition)
	_, err = s.turn.Insert(&clock.InsertInput{ID: "x", Pos: -1})
	s.Require().ErrorIs(err, clock.ErrBadPosition)
}

func (s *TurnSuite) TestRemoveSemantics() {
	// design Remove row: non-active adjusts index; active advances
	// (MemberLeft, TurnStarted{next}, round unchanged even from last slot);
	// last member empties the clock (MemberLeft only).
	//nolint:dupl // different test case: removes before active, not after
	s.Run("non-active before active", func() {
		s.turn = &clock.Turn{}
		_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
		s.Require().NoError(err)
		_, err = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
		s.Require().NoError(err)
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "a"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "a", Round: 1}}, out.Milestones)
		active, err := s.turn.Active()
		s.Require().NoError(err)
		s.Equal(core.EntityID("b"), active)
		order, err := s.turn.Order()
		s.Require().NoError(err)
		s.Equal([]core.EntityID{"b", "c"}, order)
	})
	//nolint:dupl // different test case: removes after active, not before
	s.Run("non-active after active", func() {
		s.turn = &clock.Turn{}
		_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
		s.Require().NoError(err)
		_, err = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
		s.Require().NoError(err)
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "c"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "c", Round: 1}}, out.Milestones)
		active, err := s.turn.Active()
		s.Require().NoError(err)
		s.Equal(core.EntityID("b"), active)
		order, err := s.turn.Order()
		s.Require().NoError(err)
		s.Equal([]core.EntityID{"a", "b"}, order)
	})
	s.Run("active mid-order", func() {
		s.turn = &clock.Turn{}
		_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
		s.Require().NoError(err)
		_, err = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
		s.Require().NoError(err)
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "b"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{
			{Kind: clock.MemberLeft, Subject: "b", Round: 1},
			{Kind: clock.TurnStarted, Subject: "c", Round: 1},
		}, out.Milestones)
		order, err := s.turn.Order()
		s.Require().NoError(err)
		s.Equal([]core.EntityID{"a", "c"}, order)
	})
	s.Run("active in last slot: next is first, round unchanged", func() {
		s.turn = &clock.Turn{}
		_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
		s.Require().NoError(err)
		_, err = s.turn.End(&clock.EndInput{Actor: "a"}) // b active, last slot
		s.Require().NoError(err)
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "b"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{
			{Kind: clock.MemberLeft, Subject: "b", Round: 1},
			{Kind: clock.TurnStarted, Subject: "a", Round: 1},
		}, out.Milestones)
		round, err := s.turn.Round()
		s.Require().NoError(err)
		s.Equal(1, round)
		order, err := s.turn.Order()
		s.Require().NoError(err)
		s.Equal([]core.EntityID{"a"}, order)
	})
	s.Run("last member empties", func() {
		s.turn = &clock.Turn{}
		_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
		s.Require().NoError(err)
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "a"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "a", Round: 1}}, out.Milestones)
		_, err = s.turn.Active()
		s.Require().ErrorIs(err, clock.ErrIdle)
		order, err := s.turn.Order()
		s.Require().NoError(err)
		s.Empty(order)
	})
	s.Run("absent errors", func() {
		s.turn = &clock.Turn{}
		_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
		s.Require().NoError(err)
		_, err = s.turn.Remove(&clock.RemoveInput{ID: "zz"})
		s.Require().ErrorIs(err, clock.ErrNotMember)
	})
}

func (s *TurnSuite) TestMergeCombinesBubbles() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)
	_, err = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
	s.Require().NoError(err)
	other := &clock.Turn{}
	_, err = other.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x", "y"}})
	s.Require().NoError(err)

	out, err := s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"x", "b", "y", "a"}})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{{Kind: clock.Merged, Round: 1}}, out.Milestones)
	active, err := s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("b"), active, "receiving clock's active entity remains active")
	round, err := s.turn.Round()
	s.Require().NoError(err)
	s.Equal(1, round, "receiver's round retained")
	order, err := s.turn.Order()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"x", "b", "y", "a"}, order)
	// Other reset to zero/idle state
	_, err = other.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
	otherOrder, err := other.Order()
	s.Require().NoError(err)
	s.Empty(otherOrder)
}

func (s *TurnSuite) TestMergeErrors() {
	other := &clock.Turn{}
	_, err := other.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x"}})
	s.Require().NoError(err)
	_, err = s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"x"}})
	s.Require().ErrorIs(err, clock.ErrIdle, "idle receiver refuses merge")

	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	_, err = s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"a", "x", "ghost"}})
	s.Require().ErrorIs(err, clock.ErrBadOrder, "order must be an exact permutation of the union")
	activeAfter, err := s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), activeAfter, "failed merge changes nothing (R5)")
	otherOrder, err := other.Order()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"x"}, otherOrder, "failed merge leaves Other intact (R5)")
}

func (s *TurnSuite) TestDissolve() {
	_, err := s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().ErrorIs(err, clock.ErrIdle, "dissolving an empty clock errors")

	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)
	out, err := s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b"}, out.Members)
	s.Equal([]clock.Milestone{{Kind: clock.Dissolved, Round: 1}}, out.Milestones)
	_, err = s.turn.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
}
