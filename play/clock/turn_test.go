// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"encoding/json"
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

// TestRemoveLastMemberRoundTrips pins that attrition-ended fights persist
// as canonical idle — without Remove's round reset, the R9 guard would
// reject the snapshot and the save would be unloadable.
func (s *TurnSuite) TestRemoveLastMemberRoundTrips() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	_, err = s.turn.Remove(&clock.RemoveInput{ID: "a"})
	s.Require().NoError(err)
	s.Equal(clock.TurnData{}, s.turn.ToData(), "post-attrition state is canonical idle")
	loaded, err := clock.LoadTurn(s.turn.ToData())
	s.Require().NoError(err)
	s.Equal(clock.TurnData{}, loaded.ToData())
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

//nolint:goconst // repeated test fixture ID; _test.go exclusions inert until toolkit#904
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
	orderAfter, err := s.turn.Order()
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a"}, orderAfter)
	roundAfter, err := s.turn.Round()
	s.Require().NoError(err)
	s.Equal(1, roundAfter)

	// self-merge must refuse, not silently destroy the receiver
	_, err = s.turn.Merge(&clock.MergeInput{Other: s.turn, Order: []core.EntityID{"a"}})
	s.Require().ErrorIs(err, clock.ErrBadOrder)
	activeSelf, err := s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), activeSelf)

	// same length, wrong member: exercises the "in neither clock" branch
	_, err = s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"ghost", "x"}})
	s.Require().ErrorIs(err, clock.ErrBadOrder)

	// same length, duplicate: exercises the "appears twice" branch
	_, err = s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"x", "x"}})
	s.Require().ErrorIs(err, clock.ErrBadOrder)
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

func (s *TurnSuite) TestToDataSnapshotIsImmuneToLaterVerbs() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
	s.Require().NoError(err)
	snap := s.turn.ToData()
	_, err = s.turn.Remove(&clock.RemoveInput{ID: "b"})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b", "c"}, snap.Order, "snapshot must not observe later mutations")
}

func (s *TurnSuite) TestTurnRoundTrip() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
	s.Require().NoError(err)
	_, err = s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().NoError(err)
	data := s.turn.ToData()
	loaded, err := clock.LoadTurn(data)
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), loaded.ToData())
	// behavior-identical: same next actor on End
	out, err := loaded.End(&clock.EndInput{Actor: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("c"), out.Next)
}

func (s *TurnSuite) TestRoundTripAfterMergeAndDissolve() {
	// design AC3: post-merge and post-dissolve are named round-trip states
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)
	other := &clock.Turn{}
	_, err = other.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x"}})
	s.Require().NoError(err)
	_, err = s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"a", "x", "b"}})
	s.Require().NoError(err)
	merged, err := clock.LoadTurn(s.turn.ToData())
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), merged.ToData())
	drained, err := clock.LoadTurn(other.ToData()) // drained Other is canonical idle
	s.Require().NoError(err)
	s.Equal(other.ToData(), drained.ToData())

	_, err = s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().NoError(err)
	idle, err := clock.LoadTurn(s.turn.ToData()) // post-dissolve is canonical idle
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), idle.ToData())
}

func (s *TurnSuite) TestLoadTurnRejectsInvalid() {
	cases := []struct {
		name string
		data clock.TurnData
	}{
		{"duplicate members", clock.TurnData{Order: []core.EntityID{"a", "a"}, Round: 1}},
		{"active idx out of range", clock.TurnData{Order: []core.EntityID{"a"}, ActiveIdx: 1, Round: 1}},
		{"negative active idx", clock.TurnData{Order: []core.EntityID{"a"}, ActiveIdx: -1, Round: 1}},
		{"idle with nonzero active idx", clock.TurnData{ActiveIdx: 2}},
		{"negative round with order", clock.TurnData{Order: []core.EntityID{"a"}, Round: -3}},
		{"zero round with order", clock.TurnData{Order: []core.EntityID{"a"}}},
		{"idle with nonzero round", clock.TurnData{Round: 5}},
		{"idle with negative active idx", clock.TurnData{ActiveIdx: -1}},
		{"idle with negative round", clock.TurnData{Round: -3}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := clock.LoadTurn(tc.data)
			s.Require().ErrorIs(err, clock.ErrInvalidData)
		})
	}
}

func (s *TurnSuite) TestLoadTurnAcceptsCanonicalIdle() {
	loaded, err := clock.LoadTurn(clock.TurnData{})
	s.Require().NoError(err)
	_, err = loaded.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = loaded.Round()
	s.Require().ErrorIs(err, clock.ErrIdle)
}

// TestTurnDataWireShape pins the JSON persistence contract — tag renames
// are invisible to gorelease and would orphan every stored snapshot.
func (s *TurnSuite) TestTurnDataWireShape() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)
	_, err = s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().NoError(err)
	raw, err := json.Marshal(s.turn.ToData())
	s.Require().NoError(err)
	s.JSONEq(`{"order":["a","b"],"active_idx":1,"round":1}`, string(raw))
	var back clock.TurnData
	s.Require().NoError(json.Unmarshal(raw, &back))
	loaded, err := clock.LoadTurn(back)
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), loaded.ToData())
	// idle snapshot marshals to {}
	idleRaw, err := json.Marshal(clock.TurnData{})
	s.Require().NoError(err)
	s.JSONEq(`{}`, string(idleRaw))
}
