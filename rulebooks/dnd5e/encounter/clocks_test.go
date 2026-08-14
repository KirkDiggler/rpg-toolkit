// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ClocksTestSuite covers which clock a member is on — the query the
// composition exists to answer once more than one clock can run at a time.
type ClocksTestSuite struct {
	suite.Suite
}

// twoMemberEncounter builds the smallest encounter with a player and a
// monster in one room.
func (s *ClocksTestSuite) twoMemberEncounter() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 7, Y: 7}},
		},
		Endings: []encounter.EndingInput{
			{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
				Room: room1, Position: spatial.Position{X: 0, Y: 0},
			}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// TestEveryMemberStartsOnTheWorldClock pins the absence of a mode: there is no
// "free roam" state to enter, because being on the world clock IS the default
// and the only thing that changes it is a fight pulling you off it.
func (s *ClocksTestSuite) TestEveryMemberStartsOnTheWorldClock() {
	enc := s.twoMemberEncounter()

	for _, id := range []core.EntityID{alice, goblin} {
		out, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(err, "member %q", id)
		s.Equal(encounter.ClockWorld, out.Kind, "member %q", id)

		// The world clock has no turn order, and saying so explicitly is the
		// point: a caller must not be able to read a meaningful "active" out
		// of a clock where everyone acts.
		s.Empty(out.Active)
		s.Zero(out.Round)
		s.Nil(out.Order)
	}
}

// TestAJoinerLandsOnTheWorldClockNotInAFight pins that joining an encounter and
// being pulled into a fight are different decisions. Even with a bubble
// running, Join puts you on the world clock; Transfer is what moves you.
func (s *ClocksTestSuite) TestAJoinerLandsOnTheWorldClockNotInAFight() {
	enc := s.twoMemberEncounter()

	_, err := enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
		ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 4, Y: 4},
	}})
	s.Require().NoError(err)

	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: bob})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, out.Kind)
}

// TestClockOfRejectsANonMember pins that "which clock" is only askable about
// somebody in the encounter — never answered with a plausible-looking default.
func (s *ClocksTestSuite) TestClockOfRejectsANonMember() {
	enc := s.twoMemberEncounter()

	_, err := enc.ClockOf(&encounter.ClockOfInput{Member: core.EntityID("nobody")})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNotMember)
}

// TestExitTakesTheMemberOffTheWorldClock pins that leaving the encounter leaves
// the clock too. A member who exits but keeps a budget entry would keep
// accruing time in a world they are not in.
func (s *ClocksTestSuite) TestExitTakesTheMemberOffTheWorldClock() {
	enc := s.twoMemberEncounter()

	before := enc.ToData()
	s.Require().Contains(before.Clock.Budgets, goblin, "precondition: goblin is on the world clock")

	_, err := enc.Exit(&encounter.ExitInput{Member: goblin})
	s.Require().NoError(err)

	after := enc.ToData()
	s.NotContains(after.Clock.Budgets, goblin)
	s.Contains(after.Clock.Budgets, alice, "the member who stayed is untouched")
}

// TestABubbleRoundTripsAndIsReachedThroughItsMembers pins the whole point of
// storing bubbles without identity: after a reload, the way you find a fight is
// by asking about somebody in it.
func (s *ClocksTestSuite) TestABubbleRoundTripsAndIsReachedThroughItsMembers() {
	enc := s.twoMemberEncounter()
	data := enc.ToData()

	// Move alice and the goblin off the world clock and into a bubble, which is
	// what forming a fight will do. Written by hand here because forming is the
	// next step's verb — this step only has to persist and reload the shape.
	delete(data.Clock.Budgets, alice)
	delete(data.Clock.Budgets, goblin)
	data.Bubbles = []clock.TurnData{{
		Order:     []core.EntityID{goblin, alice},
		ActiveIdx: 1,
		Round:     3,
	}}

	reloaded, err := encounter.LoadEncounter(data, nil)
	s.Require().NoError(err)

	out, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, out.Kind)
	s.Equal(alice, out.Active, "ActiveIdx 1 of [goblin alice] is alice")
	s.Equal(3, out.Round)
	s.Equal([]core.EntityID{goblin, alice}, out.Order)

	// The goblin is in the same fight, so it reports the same clock — asking
	// through a different member reaches the same bubble.
	fromGoblin, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: goblin})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, fromGoblin.Kind)
	s.Equal(out.Active, fromGoblin.Active)
	s.Equal(out.Round, fromGoblin.Round)

	// And the round trip is stable: saving the reloaded encounter reproduces
	// the bubble rather than quietly dropping it.
	s.Equal(data.Bubbles, reloaded.ToData().Bubbles)
}

// TestAMemberOutsideTheFightKeepsFreeRoamingWhileItRuns is the point of the
// whole design, and the one case a single-bubble fixture cannot show: a fight
// is LOCALIZED. Somebody on the other side of the map is not paused by it and
// is not in it — they are still on the world clock while the bubble takes turns.
//
// Added because mutation testing found the gap: making bubbleFor return the
// first bubble unconditionally, ignoring membership entirely, passed every
// other test in this file. Every fixture had all its members inside the one
// bubble, so "the bubble holding this member" and "the bubble" were
// indistinguishable.
func (s *ClocksTestSuite) TestAMemberOutsideTheFightKeepsFreeRoamingWhileItRuns() {
	enc := s.twoMemberEncounter()

	// bob is far away, doing something else entirely.
	_, err := enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
		ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 9, Y: 9},
	}})
	s.Require().NoError(err)

	data := enc.ToData()
	// alice and the goblin are fighting. bob is deliberately left on the world
	// clock.
	delete(data.Clock.Budgets, alice)
	delete(data.Clock.Budgets, goblin)
	data.Bubbles = []clock.TurnData{{
		Order: []core.EntityID{alice, goblin}, ActiveIdx: 0, Round: 1,
	}}

	reloaded, err := encounter.LoadEncounter(data, nil)
	s.Require().NoError(err)

	fighting, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, fighting.Kind)

	roaming, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: bob})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, roaming.Kind, "bob is not in the fight and must not be reported as in it")
	s.Empty(roaming.Active)
	s.Zero(roaming.Round)
	s.Nil(roaming.Order)
}

// TestMutatingTheReturnedOrderCannotCorruptTheEncounter pins the OBSERVABLE
// guarantee — ClockOf is a read, and a read whose result can be edited into the
// encounter is not one.
//
// It deliberately does not claim where the guarantee comes from. It comes from
// clock.Turn.Order, which copies before returning; this composition adds
// nothing. An earlier version of this test copied again here and described
// itself as the reason the encounter was safe, which mutation testing
// disproved: deleting the copy changed no behaviour and failed no test. The
// test is still worth keeping, because it is what notices if the leaf's promise
// ever changes underneath us.
func (s *ClocksTestSuite) TestMutatingTheReturnedOrderCannotCorruptTheEncounter() {
	enc := s.twoMemberEncounter()
	data := enc.ToData()
	delete(data.Clock.Budgets, alice)
	delete(data.Clock.Budgets, goblin)
	data.Bubbles = []clock.TurnData{{Order: []core.EntityID{goblin, alice}, ActiveIdx: 0, Round: 1}}

	reloaded, err := encounter.LoadEncounter(data, nil)
	s.Require().NoError(err)

	out, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	out.Order[0] = core.EntityID("vandal")

	again, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{goblin, alice}, again.Order)
}

// TestLoadRejectsAMemberOnTwoClocks pins R6 at the trust boundary. A blob
// claiming someone is both free-roaming and in a fight has no correct reading,
// and picking one silently would make ClockOf depend on iteration order.
func (s *ClocksTestSuite) TestLoadRejectsAMemberOnTwoClocks() {
	s.Run("in a bubble while still on the world clock", func() {
		enc := s.twoMemberEncounter()
		data := enc.ToData()
		// alice deliberately LEFT in Clock.Budgets while also in the bubble.
		delete(data.Clock.Budgets, goblin)
		data.Bubbles = []clock.TurnData{{Order: []core.EntityID{goblin, alice}, ActiveIdx: 0, Round: 1}}

		_, err := encounter.LoadEncounter(data, nil)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("in two bubbles at once", func() {
		enc := s.twoMemberEncounter()
		data := enc.ToData()
		delete(data.Clock.Budgets, alice)
		delete(data.Clock.Budgets, goblin)
		data.Bubbles = []clock.TurnData{
			{Order: []core.EntityID{alice, goblin}, ActiveIdx: 0, Round: 1},
			{Order: []core.EntityID{alice}, ActiveIdx: 0, Round: 1},
		}

		_, err := encounter.LoadEncounter(data, nil)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})
}

// TestLoadRejectsANonMemberOnAClock pins that only members may be on a clock.
//
// Raised in review on PR #960 and confirmed by probe before being believed. A
// non-member on the world clock accrues budget on every Advance forever; a
// non-member in a bubble order can be reported as Active, so ClockOf would
// answer a real member's question by naming somebody who is not in the
// encounter. Neither announces itself.
//
// The world-clock case deserves the explicit budget-0 fixture: LoadTick
// independently rejects a budget above the high-water mark, so a ghost carrying
// a non-zero budget was already failing for an unrelated reason. That is
// accidental coverage, and accidental coverage reads as a guarantee — with
// budget 0 the same ghost loaded clean.
func (s *ClocksTestSuite) TestLoadRejectsANonMemberOnAClock() {
	s.Run("on the world clock, carrying no budget", func() {
		enc := s.twoMemberEncounter()
		data := enc.ToData()
		data.Clock.Budgets["ghost"] = 0

		_, err := encounter.LoadEncounter(data, nil)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("in a bubble order", func() {
		enc := s.twoMemberEncounter()
		data := enc.ToData()
		delete(data.Clock.Budgets, alice)
		data.Bubbles = []clock.TurnData{{
			Order: []core.EntityID{alice, core.EntityID("ghost")}, ActiveIdx: 0, Round: 1,
		}}

		_, err := encounter.LoadEncounter(data, nil)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("a bubble of nothing but non-members", func() {
		enc := s.twoMemberEncounter()
		data := enc.ToData()
		data.Bubbles = []clock.TurnData{{
			Order: []core.EntityID{"ghost", "phantom"}, ActiveIdx: 0, Round: 1,
		}}

		_, err := encounter.LoadEncounter(data, nil)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})
}

// TestABlobFromBeforeClockMembershipLoadsEveryoneOntoTheWorldClock pins the
// retrofit. Encounters persisted before members were tracked on the world clock
// carry an empty budget map; they meant "everyone is free-roaming", and they
// must still mean that after this change — without a migration anybody has to
// remember to run.
func (s *ClocksTestSuite) TestABlobFromBeforeClockMembershipLoadsEveryoneOntoTheWorldClock() {
	enc := s.twoMemberEncounter()
	data := enc.ToData()

	// Exactly what an older blob looks like: members present, nobody on a clock.
	data.Clock.Budgets = nil
	data.Bubbles = nil

	reloaded, err := encounter.LoadEncounter(data, nil)
	s.Require().NoError(err)

	for _, id := range []core.EntityID{alice, goblin} {
		out, cerr := reloaded.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(cerr, "member %q", id)
		s.Equal(encounter.ClockWorld, out.Kind, "member %q", id)
	}

	// And they are really on it, not merely defaulting to the world answer
	// because no bubble held them — the difference matters, because only real
	// membership accrues budget when the world advances.
	saved := reloaded.ToData()
	s.Contains(saved.Clock.Budgets, alice)
	s.Contains(saved.Clock.Budgets, goblin)
}

func TestClocksSuite(t *testing.T) {
	suite.Run(t, new(ClocksTestSuite))
}
