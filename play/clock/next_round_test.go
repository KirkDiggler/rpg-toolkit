// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

type NextRoundSuite struct {
	suite.Suite
	turn *clock.Turn
}

func (s *NextRoundSuite) SetupTest() { s.turn = &clock.Turn{} }

func TestNextRoundSuite(t *testing.T) { suite.Run(t, new(NextRoundSuite)) }

func ids(xs ...string) []core.EntityID {
	out := make([]core.EntityID, len(xs))
	for i, x := range xs {
		out[i] = core.EntityID(x)
	}
	return out
}

func (s *NextRoundSuite) order(xs ...string) {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: ids(xs...)})
	s.Require().NoError(err)
}

func (s *NextRoundSuite) end(actor string) *clock.EndOutput {
	out, err := s.turn.End(&clock.EndInput{Actor: core.EntityID(actor)})
	s.Require().NoError(err)
	return out
}

func (s *NextRoundSuite) insert(id string, pos int, nextRound bool) {
	_, err := s.turn.Insert(&clock.InsertInput{ID: core.EntityID(id), Pos: pos, NextRound: nextRound})
	s.Require().NoError(err)
}

func (s *NextRoundSuite) active() core.EntityID {
	a, err := s.turn.Active()
	s.Require().NoError(err)
	return a
}

func (s *NextRoundSuite) round() int {
	r, err := s.turn.Round()
	s.Require().NoError(err)
	return r
}

func startedFor(ms []clock.Milestone, id core.EntityID) bool {
	for _, m := range ms {
		if m.Kind == clock.TurnStarted && m.Subject == id {
			return true
		}
	}
	return false
}

func countKind(ms []clock.Milestone, k clock.MilestoneKind) int {
	n := 0
	for _, m := range ms {
		if m.Kind == k {
			n++
		}
	}
	return n
}

func (s *NextRoundSuite) TestAMemberForTheNextRoundIsPassedOver() {
	s.order("a", "b", "c")
	s.end("a")
	s.insert("j", 2, true) // after b, the active one
	out := s.end("b")
	s.Equal(core.EntityID("c"), out.Next)
	s.False(startedFor(out.Milestones, "j"))
	out = s.end("c")
	s.Equal(2, s.round())
	s.Equal(core.EntityID("a"), out.Next)
	s.Equal(core.EntityID("b"), s.end("a").Next)
	s.Equal(core.EntityID("j"), s.end("b").Next)
}

func (s *NextRoundSuite) TestAMemberForTheNextRoundPlacedBeforeTheActiveIsNotMarked() {
	s.order("a", "b", "c")
	s.end("a") // b active
	s.insert("j", 0, true)
	s.Empty(s.turn.ToData().NextRound)
	s.end("b")
	out := s.end("c")
	s.Equal(2, s.round())
	s.Equal(core.EntityID("j"), out.Next)
}

func (s *NextRoundSuite) TestAMemberForTheNextRoundAtTheEndWaitsForTheWrap() {
	s.order("a", "b")
	s.end("a") // b active
	s.insert("j", 2, true)
	out := s.end("b")
	s.Equal(2, s.round())
	s.Equal(core.EntityID("a"), out.Next)
	s.Equal(1, countKind(out.Milestones, clock.RoundStarted))
	s.False(startedFor(out.Milestones, "j"))
}

func (s *NextRoundSuite) TestTwoMembersForTheNextRoundInARowArePassedOverTogether() {
	s.order("a", "b", "c")
	s.end("a")
	s.insert("j1", 2, true)
	s.insert("j2", 3, true)
	s.Equal(core.EntityID("c"), s.end("b").Next)
	s.Equal(core.EntityID("a"), s.end("c").Next)
	s.Equal(core.EntityID("b"), s.end("a").Next)
	s.Equal(core.EntityID("j1"), s.end("b").Next)
	s.Equal(core.EntityID("j2"), s.end("j1").Next)
}

func (s *NextRoundSuite) TestRemovingTheActiveSkipsAMemberForTheNextRound() {
	s.order("a", "b", "c", "d")
	s.end("a") // b active
	s.insert("j", 2, true)
	out, err := s.turn.Remove(&clock.RemoveInput{ID: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("c"), s.active())
	s.True(startedFor(out.Milestones, "c"))
	s.False(startedFor(out.Milestones, "j"))
	s.Equal(1, s.round())
}

func (s *NextRoundSuite) TestRemovingTheActiveWrapsPastMembersForTheNextRound() {
	s.order("a", "b")
	s.end("a") // b active
	s.insert("j", 2, true)
	out, err := s.turn.Remove(&clock.RemoveInput{ID: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), s.active())
	s.True(startedFor(out.Milestones, "a"))
	s.Equal(1, s.round()) // Remove keeps the round on a wrap
	s.Empty(s.turn.ToData().NextRound)
}

func (s *NextRoundSuite) TestRemovingAMemberForTheNextRoundForgetsIt() {
	s.order("a", "b", "c")
	s.end("a")
	s.insert("j", 2, true)
	_, err := s.turn.Remove(&clock.RemoveInput{ID: "j"})
	s.Require().NoError(err)
	s.Empty(s.turn.ToData().NextRound)
	s.insert("j", 2, false) // comes back with a turn this round
	s.Equal(core.EntityID("j"), s.end("b").Next)
}

func (s *NextRoundSuite) TestSetOrderAndDissolveForgetMembersForTheNextRound() {
	s.order("a", "b")
	s.insert("j", 1, true)
	s.Equal([]core.EntityID{"j"}, s.turn.ToData().NextRound)
	s.order("a", "b", "j")
	s.Empty(s.turn.ToData().NextRound)
	s.Equal(core.EntityID("b"), s.end("a").Next)
	s.Equal(core.EntityID("j"), s.end("b").Next)

	s.order("a", "b")
	s.insert("k", 1, true)
	_, err := s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().NoError(err)
	s.Empty(s.turn.ToData().NextRound)
	s.order("a", "k")
	s.Equal(core.EntityID("k"), s.end("a").Next)
}

func (s *NextRoundSuite) TestMergeKeepsOnlyMembersStillAhead() {
	build := func() (*clock.Turn, *clock.Turn) {
		recv := &clock.Turn{}
		_, err := recv.SetOrder(&clock.SetOrderInput{Order: ids("a", "b", "c")})
		s.Require().NoError(err)
		_, err = recv.End(&clock.EndInput{Actor: "a"}) // b active
		s.Require().NoError(err)
		_, err = recv.Insert(&clock.InsertInput{ID: "j", Pos: 2, NextRound: true})
		s.Require().NoError(err)
		other := &clock.Turn{}
		_, err = other.SetOrder(&clock.SetOrderInput{Order: ids("x", "y")})
		s.Require().NoError(err)
		_, err = other.Insert(&clock.InsertInput{ID: "z", Pos: 1, NextRound: true})
		s.Require().NoError(err)
		return recv, other
	}
	s.Run("still ahead of the active member", func() {
		recv, other := build()
		_, err := recv.Merge(&clock.MergeInput{Other: other, Order: ids("a", "b", "j", "x", "z", "y", "c")})
		s.Require().NoError(err)
		s.Equal([]core.EntityID{"j"}, recv.ToData().NextRound) // z, Other's, is dropped
	})
	s.Run("merged to before the active member", func() {
		recv, other := build()
		_, err := recv.Merge(&clock.MergeInput{Other: other, Order: ids("j", "a", "x", "b", "z", "y", "c")})
		s.Require().NoError(err)
		s.Empty(recv.ToData().NextRound)
	})
}

func (s *NextRoundSuite) TestNextRoundSurvivesToDataAndLoadTurn() {
	s.order("a", "b", "c")
	s.end("a")
	s.insert("j2", 3, true)
	s.insert("j", 2, true)
	data := s.turn.ToData()
	s.Equal([]core.EntityID{"j", "j2"}, data.NextRound) // in order position
	loaded, err := clock.LoadTurn(data)
	s.Require().NoError(err)
	s.Equal(data, loaded.ToData())
	out, err := loaded.End(&clock.EndInput{Actor: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("c"), out.Next)
	out, err = loaded.End(&clock.EndInput{Actor: "c"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), out.Next)
}

func (s *NextRoundSuite) TestLoadTurnRefusesABadNextRound() {
	base := clock.TurnData{Order: ids("a", "b", "c"), ActiveIdx: 1, Round: 1}
	cases := map[string]clock.TurnData{
		"absent from order": {Order: base.Order, ActiveIdx: 1, Round: 1, NextRound: ids("zz")},
		"listed twice":      {Order: base.Order, ActiveIdx: 1, Round: 1, NextRound: ids("c", "c")},
		"at the active":     {Order: base.Order, ActiveIdx: 1, Round: 1, NextRound: ids("b")},
		"before the active": {Order: base.Order, ActiveIdx: 1, Round: 1, NextRound: ids("a")},
		"on an idle clock":  {NextRound: ids("a")},
	}
	for name, data := range cases {
		s.Run(name, func() {
			_, err := clock.LoadTurn(data)
			s.Require().ErrorIs(err, clock.ErrInvalidData)
			if name == "absent from order" {
				s.Contains(err.Error(), "is not in the order")
			}
		})
	}
}

func (s *NextRoundSuite) TestTransferCarriesNextRound() {
	world, err := clock.NewTick()
	s.Require().NoError(err)
	_, err = world.Join(&clock.JoinInput{ID: "j"})
	s.Require().NoError(err)
	s.order("a", "b")
	_, err = clock.Transfer(&clock.TransferInput{From: world, To: s.turn, ID: "j", Pos: 2, NextRound: true})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"j"}, s.turn.ToData().NextRound)

	_, err = clock.Transfer(&clock.TransferInput{From: s.turn, To: world, ID: "j", NextRound: true})
	s.Require().NoError(err) // a Tick ignores it
	in, err := world.Contains(&clock.ContainsInput{ID: "j"})
	s.Require().NoError(err)
	s.True(in)
}
