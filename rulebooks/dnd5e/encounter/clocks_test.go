// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
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

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("a bubble of nothing but non-members", func() {
		enc := s.twoMemberEncounter()
		data := enc.ToData()
		data.Bubbles = []clock.TurnData{{
			Order: []core.EntityID{"ghost", "phantom"}, ActiveIdx: 0, Round: 1,
		}}

		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
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

const (
	carl = core.EntityID("carl")
	dana = core.EntityID("dana")
)

// fiveMemberEncounter is the DOS2 split-party cast: four players and a
// monster in one room, plus an external ending so closure is reachable.
func (s *ClocksTestSuite) fiveMemberEncounter() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 3, Y: 2}},
			{ID: carl, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 8, Y: 8}},
			{ID: dana, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 9, Y: 8}},
			{ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 2, Y: 3}},
		},
		Endings: []encounter.EndingInput{
			{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
				Room: room1, Position: spatial.Position{X: 0, Y: 0},
			}},
			{Key: "called", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// assertR6 sweeps the invariant after a transition: every member answers
// ClockOf (on at least one clock — the on-no-clock check would reject), and
// the persisted shape passes the trust boundary, whose load validation is
// the full R6 statement (nobody on two clocks, nobody on a clock who is not
// a member). Asserted after EVERY transition, not only at the end — the
// issue's own requirement, because a transient violation between verbs is
// exactly what load-act-save would persist.
func (s *ClocksTestSuite) assertR6(enc *encounter.Encounter, members ...core.EntityID) {
	s.T().Helper()
	for _, id := range members {
		_, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(err, "member %q must be on exactly one clock", id)
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: enc.ToData()})
	s.Require().NoError(err, "the persisted shape must pass the trust boundary (R6)")
}

// TestDOS2SplitPartyThroughTheComposition is step 4.2's done-when headline:
// the play/clock DOS2 scenario (dos2_test.go — the specification), expressed
// through the composition's own verbs rather than the leaves. Four players
// and a goblin free-roam; a fight pulls three of them into a bubble; the
// distant pair keeps living on the world clock while it runs; one wanders
// too close and falls in mid-round at the requested slot; the round wraps
// with them in the order; the fight dissolves and everyone re-homes.
func (s *ClocksTestSuite) TestDOS2SplitPartyThroughTheComposition() {
	enc := s.fiveMemberEncounter()
	everyone := []core.EntityID{alice, bob, carl, dana, goblin}

	// Everyone free-roams: the world clock is the default, not a mode.
	s.assertR6(enc, everyone...)
	for _, id := range everyone {
		out, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(err)
		s.Require().Equal(encounter.ClockWorld, out.Kind, "member %q", id)
	}

	// Alice and Bob trigger a fight with the goblin. The rulebook has rolled
	// initiative — the order arrives from outside (R7); trigger detection is
	// deliberately not this step's business.
	formOut, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin, bob}})
	s.Require().NoError(err)
	s.NotZero(formOut.Seq, "forming a fight is a story beat")
	s.assertR6(enc, everyone...)

	fighting, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, fighting.Kind)
	s.Equal(alice, fighting.Active, "round 1 opens with the first in the order")
	s.Equal(1, fighting.Round)
	s.Equal([]core.EntityID{alice, goblin, bob}, fighting.Order)

	// The fight is LOCALIZED: the distant pair is not in it and not paused
	// by it.
	for _, id := range []core.EntityID{carl, dana} {
		out, cerr := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(cerr)
		s.Equal(encounter.ClockWorld, out.Kind, "member %q keeps free-roaming", id)
	}

	// The distant pair keeps living while the fight runs: carl moves freely,
	// and when the world thinks, only world-clock members accrue. (Player
	// movement driving Advance directly — the leaf's own accrual model —
	// is not wired into Move yet; Pump is the composition's accrual
	// mechanism today, and the pin here is WHO accrues, not how much.)
	_, err = enc.Move(&encounter.MoveInput{Member: carl, To: spatial.Position{X: 8, Y: 7}})
	s.Require().NoError(err)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	budgets := enc.ToData().Clock.Budgets
	s.Equal(1, budgets[carl], "the distant pair accrues while the fight runs")
	s.Equal(1, budgets[dana])
	s.NotContains(budgets, alice, "fight members are off the tick entirely")
	s.NotContains(budgets, goblin)
	s.NotContains(budgets, bob)

	// A round of combat: alice and the goblin take their turns.
	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(goblin, et.Next)
	s.False(et.RoundWrapped)
	et, err = enc.EndTurn(&encounter.EndTurnInput{Member: goblin})
	s.Require().NoError(err)
	s.Equal(bob, et.Next)
	s.assertR6(enc, everyone...)

	// Carl wanders too close and falls in mid-round, slotted after the
	// goblin. The slot is the caller's choice (R7 again — the rulebook
	// rolled his initiative), and the active member is undisturbed.
	_, err = enc.Transfer(&encounter.TransferInput{Member: carl, To: encounter.ClockTurn, Pos: 2})
	s.Require().NoError(err)
	s.assertR6(enc, everyone...)

	joined, err := enc.ClockOf(&encounter.ClockOfInput{Member: carl})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, joined.Kind)
	s.Equal([]core.EntityID{alice, goblin, carl, bob}, joined.Order, "carl landed at the requested slot")
	s.Equal(bob, joined.Active, "falling in does not steal the active turn")
	s.Equal(1, joined.Round)

	// And having fallen in, carl is the fight's now — free-roam movement is
	// no longer his to make.
	_, err = enc.Move(&encounter.MoveInput{Member: carl, To: spatial.Position{X: 8, Y: 8}})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrInBubble)

	// Bob closes the round: it wraps into round 2 with carl in the order.
	et, err = enc.EndTurn(&encounter.EndTurnInput{Member: bob})
	s.Require().NoError(err)
	s.True(et.RoundWrapped, "the round wraps across the grown order")
	s.Equal(alice, et.Next)
	wrapped, err := enc.ClockOf(&encounter.ClockOfInput{Member: carl})
	s.Require().NoError(err)
	s.Equal(2, wrapped.Round)

	// The fight ends, reached through any of its members: everyone re-homes
	// to the world clock at budget zero — time spent fighting was spent,
	// not banked. Dana never left, so hers is intact.
	dis, err := enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{alice, goblin, carl, bob}, dis.Members, "dissolve reports the bubble order")
	s.assertR6(enc, everyone...)

	for _, id := range everyone {
		out, cerr := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(cerr)
		s.Equal(encounter.ClockWorld, out.Kind, "member %q is home", id)
	}
	after := enc.ToData()
	s.Empty(after.Bubbles, "a bubble exists only while a fight does")
	s.Equal(0, after.Clock.Budgets[carl], "re-homed at zero")
	s.Equal(1, after.Clock.Budgets[dana], "never left, kept her budget")

	// The story heard it all — dana included, from the other side of the
	// map. The clock-tagged transcript is the composition's analogue of the
	// leaf test's milestone transcript: formation, the world thinking
	// (Pump's tick), three turn ends, the fall-in, dissolution.
	story, err := enc.Story(&encounter.StoryInput{Audience: dana})
	s.Require().NoError(err)
	var clockBeats []string
	for _, entry := range story {
		if entry.Tags["tag"] != "clock" {
			continue
		}
		var p struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &p))
		clockBeats = append(clockBeats, p.Beat)
	}
	s.Equal([]string{
		"bubble-formed",
		"tick",
		"turn-ended", "turn-ended",
		"transferred",
		"turn-ended",
		"bubble-dissolved",
	}, clockBeats)
}

// TestFormRejections pins Form's refusals, including the issue's named
// requirement: a member already in a bubble is rejected, never silently
// merged. The disjoint-second-fight case is the one-bubble policy — when
// that policy lifts, the per-member overlap check is what remains.
func (s *ClocksTestSuite) TestFormRejections() {
	s.Run("an empty order", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: nil})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("a duplicated entry", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin, alice}})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("a non-member in the order", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, "stranger"}})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNotMember)
	})

	s.Run("a member who is already fighting", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, bob}})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInBubble)
	})

	s.Run("a disjoint second fight while one runs", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Form(&encounter.FormInput{Order: []core.EntityID{carl, dana}})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInBubble)
	})

	s.Run("a rejected form touches nothing", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin, "stranger"}})
		s.Require().Error(err)

		// alice and the goblin were named BEFORE the defect: R5 means they
		// were never pulled off the world clock.
		for _, id := range []core.EntityID{alice, goblin} {
			out, cerr := enc.ClockOf(&encounter.ClockOfInput{Member: id})
			s.Require().NoError(cerr)
			s.Equal(encounter.ClockWorld, out.Kind, "member %q", id)
		}
		s.Empty(enc.ToData().Bubbles)
	})
}

// TestTransferRejections pins Transfer's refusals, and the one guarantee a
// refusal must keep: both clocks unchanged (the leaf compensates, R6).
func (s *ClocksTestSuite) TestTransferRejections() {
	s.Run("into a fight that is not running", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockTurn})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoBubble)
	})

	s.Run("into a fight the member is already in", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockTurn})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrInBubble)
	})

	s.Run("out of a fight the member is not in", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Transfer(&encounter.TransferInput{Member: bob, To: encounter.ClockWorld})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoBubble)
	})

	s.Run("to an unknown clock kind", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockKind("lunch")})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrBadClock)
	})

	s.Run("an out-of-range slot leaves both clocks unchanged", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Transfer(&encounter.TransferInput{Member: bob, To: encounter.ClockTurn, Pos: 7})
		s.Require().Error(err)
		s.ErrorIs(err, clock.ErrBadPosition, "the leaf's own rejection propagates")

		// The failed transfer compensated: bob is still free-roaming, the
		// fight is still two, and the whole shape still loads (R6).
		out, cerr := enc.ClockOf(&encounter.ClockOfInput{Member: bob})
		s.Require().NoError(cerr)
		s.Equal(encounter.ClockWorld, out.Kind)
		s.assertR6(enc, alice, bob, carl, dana, goblin)
	})
}

// TestEndTurnRejections pins that turn discipline is the bubble's, surfaced
// through the composition: no fight, no turn to end; not your turn, no
// state change.
func (s *ClocksTestSuite) TestEndTurnRejections() {
	s.Run("a member not in a fight", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoBubble)
	})

	s.Run("not their turn", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.EndTurn(&encounter.EndTurnInput{Member: goblin})
		s.Require().Error(err)
		s.ErrorIs(err, clock.ErrNotActive)

		still, cerr := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
		s.Require().NoError(cerr)
		s.Equal(alice, still.Active, "a rejected end changes nothing")
	})
}

// TestDissolveRejectsAMemberNotInAFight pins that dissolving is reached
// through a fight member — a free-roamer names no bubble.
func (s *ClocksTestSuite) TestDissolveRejectsAMemberNotInAFight() {
	enc := s.fiveMemberEncounter()
	_, err := enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoBubble)
}

// TestClockVerbsRejectAClosedEncounter pins ErrClosed uniformly across the
// four clock verbs: a closed encounter's clocks are history, not state.
func (s *ClocksTestSuite) TestClockVerbsRejectAClosedEncounter() {
	enc := s.fiveMemberEncounter()
	_, err := enc.End(&encounter.EndInput{Ending: "called"})
	s.Require().NoError(err)

	_, err = enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
	s.ErrorIs(err, encounter.ErrClosed)
	_, err = enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockTurn})
	s.ErrorIs(err, encounter.ErrClosed)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.ErrorIs(err, encounter.ErrClosed)
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.ErrorIs(err, encounter.ErrClosed)
}

// TestAFightMemberCannotFreeRoam pins the coexistence rule from the verb
// side: Move and Traverse are world-clock verbs. A fight member acts through
// the bubble — there is no in-fight movement verb yet, and until one
// arrives a fight member moving AT ALL would be moving outside initiative.
func (s *ClocksTestSuite) TestAFightMemberCannotFreeRoam() {
	enc := s.fiveMemberEncounter()
	_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
	s.Require().NoError(err)

	_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 2, Y: 1}})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrInBubble)

	// The gate outranks connection resolution: this fixture has no
	// connections at all, and the answer is still "you are fighting",
	// not "no such connection".
	_, err = enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrInBubble)

	// Everyone else's world is still running.
	_, err = enc.Move(&encounter.MoveInput{Member: carl, To: spatial.Position{X: 8, Y: 7}})
	s.Require().NoError(err)
}

// TestPumpDoesNotThinkForAFightMonster pins the other half of coexistence:
// the world thinks on the tick, and a fight thinks in turns. A monster in a
// bubble is not consulted at all — and the skip is fight-scoped, not
// permanent: dissolve the fight and the same decider wakes back up.
func (s *ClocksTestSuite) TestPumpDoesNotThinkForAFightMonster() {
	wanderer := &patrolDecider{positions: []spatial.Position{{X: 5, Y: 5}, {X: 6, Y: 5}}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 7, Y: 7}, Decider: wanderer},
		},
		Endings: []encounter.EndingInput{
			{Key: "called", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)

	_, err = enc.Form(&encounter.FormInput{Order: []core.EntityID{goblin, alice}})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Zero(wanderer.callCount, "a fight monster is not the world's to think for")

	_, err = enc.Dissolve(&encounter.DissolveInput{Member: goblin})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Equal(1, wanderer.callCount, "the skip is fight-scoped: dissolved means free to wander again")
}

// TestADrainedBubbleIsPruned pins the husk rule: a bubble exists only while
// a fight does. Exit and Transfer can each drain a fight one member at a
// time, and the moment the last member is gone the bubble must be too —
// otherwise ToData writes a fight that is not happening into every blob,
// which Load rejects.
func (s *ClocksTestSuite) TestADrainedBubbleIsPruned() {
	s.Run("drained by Exit", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)
		s.Len(enc.ToData().Bubbles, 1, "a fight of one is still a fight")

		_, err = enc.Exit(&encounter.ExitInput{Member: goblin})
		s.Require().NoError(err)
		s.Empty(enc.ToData().Bubbles, "the last member gone takes the bubble with them")
	})

	s.Run("drained by Transfer", func() {
		enc := s.fiveMemberEncounter()
		_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin}})
		s.Require().NoError(err)

		_, err = enc.Transfer(&encounter.TransferInput{Member: goblin, To: encounter.ClockWorld})
		s.Require().NoError(err)
		s.Len(enc.ToData().Bubbles, 1)

		_, err = enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockWorld})
		s.Require().NoError(err)
		s.Empty(enc.ToData().Bubbles)
		s.assertR6(enc, alice, bob, carl, dana, goblin)
	})
}

// TestLoadRejectsAnIdleBubble pins the other end of the husk rule at the
// trust boundary: this module never persists an empty bubble (every verb
// that can empty one prunes it in the same call), so reading one means the
// blob was edited — rejected, not silently carried.
func (s *ClocksTestSuite) TestLoadRejectsAnIdleBubble() {
	enc := s.fiveMemberEncounter()
	data := enc.ToData()
	data.Bubbles = []clock.TurnData{{}}

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrInvalidData)
}

// TestAMidFightBlobRoundTrips pins that a fight survives load-act-save: the
// composition's own Form (not a hand-written blob) produces the persisted
// shape, and the reloaded encounter continues the same round.
func (s *ClocksTestSuite) TestAMidFightBlobRoundTrips() {
	enc := s.fiveMemberEncounter()
	_, err := enc.Form(&encounter.FormInput{Order: []core.EntityID{alice, goblin, bob}})
	s.Require().NoError(err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: enc.ToData()})
	s.Require().NoError(err)

	out, err := reloaded.ClockOf(&encounter.ClockOfInput{Member: bob})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, out.Kind)
	s.Equal(goblin, out.Active, "the reloaded fight is mid-round, exactly where it was")
	s.Equal(1, out.Round)

	// And it keeps playing: the goblin's turn ends in the reloaded world.
	et, err := reloaded.EndTurn(&encounter.EndTurnInput{Member: goblin})
	s.Require().NoError(err)
	s.Equal(bob, et.Next)
}

func TestClocksSuite(t *testing.T) {
	suite.Run(t, new(ClocksTestSuite))
}
