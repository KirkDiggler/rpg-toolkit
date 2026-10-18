// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_kitchen_test.go is THE KITCHEN DOOR (rpg-project#606, sneaking
// S1; the mad king use case §3, "Kitchen door"): the party declares sneaking
// in the kitchen, picks the locked kitchen door, and crosses the great hall
// past the hall guard's patrol to the throne door, unseen.
//
// THE HALL GUARD IS THE GARRISON'S, AND THE GARRISON IS CIVIL to a party it
// has not recognized. So being seen by him forms no fight: what sneaking buys
// here is that NO SIGHTING LANDS — the use case's own assertion — and the
// variant without sneaking proves the sighting lands. A watcher that forms a
// fight on sight cannot stand in the hall without breaking every route that
// leaves the kitchen through it (the troupe, the scout, the orders); that is
// stated on the PR, and the throne room's knights are the S-walk's.
//
// THE FORCED DOOR'S CRASH (TestMadKingKitchenDoorHeard in the plan) IS NOT
// HERE. Measured on this file: forced from the kitchen, the crash is DC 5 in
// the kitchen and DC 10 in the great hall (one hop, the door now open); the
// throne room is three hops away (the hall, then its shut door, which counts
// twice) at DC 20, out of every knight's passive 10. The only listener that
// hears it outside the kitchen is the hall guard, whose table has no `heard`
// row. TestMadKingTheKitchenDoorsCrashReachesOnlyTheHall pins that
// measurement; the content fix belongs to the operator.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// kingFighter is the party's second player in the kitchen routes.
const kingFighter = encounter.MemberID("fighter")

// besideTheHero is the kitchen floor beside the hero's start, where the
// fighter begins (offset [col, row], for MemberInput.Position).
var besideTheHero = spatial.Position{X: -18, Y: -12}

// pastTheThroneDoor is the throne room's cell past its shut door: a walk
// toward it ends in the great hall, beside the door.
var pastTheThroneDoor = castleCell(0, -17)

// pastTheKitchenDoor is the great hall's cell past the kitchen door.
var pastTheKitchenDoor = castleCell(-14, -12)

// partyStealth is each party member's scripted Stealth: a modifier and one
// d20 per roll, decided against the DC handed in, as the session's check
// seam does. An unscripted roll fails the route.
type partyStealth struct {
	modifier map[encounter.MemberID]int
	d20s     map[encounter.MemberID][]int
	asked    []encounter.ResolveCheckInput
}

func (r *partyStealth) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("the kitchen route makes no explicit check")
}

func (r *partyStealth) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	r.asked = append(r.asked, *in)
	faces := r.d20s[in.Member]
	if len(faces) == 0 {
		return nil, fmt.Errorf("roll %d for %s was not scripted", len(r.asked), in.Member)
	}
	r.d20s[in.Member] = faces[1:]
	total := faces[0] + r.modifier[in.Member]
	applied := in.Approaches[0]

	return &encounter.ResolveCheckOutput{Beaten: total >= applied.DC, Applied: applied, Total: total}, nil
}

// kitchenParty opens the castle with the hero and the fighter on the kitchen
// floor and their Stealth scripted: a +7 rogue and a +1 fighter, each with a
// 15 for every roll the walk asks.
func kitchenParty(t *testing.T) (*kingScene, *partyStealth) {
	t.Helper()
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7, kingFighter: 1},
		d20s:     map[encounter.MemberID][]int{hero: {15, 15}, kingFighter: {15, 15}},
	}
	s := openTheCastleAs(t, castleHost{
		at:       besideTheCook,
		players:  []encounter.MemberInput{{ID: kingFighter, Kind: encounter.KindPlayer, Position: besideTheHero}},
		resolver: stealth,
	})

	return s, stealth
}

// walkMember walks one party member along the pathfinder's route toward a
// cell, a step at a time, checking after every step that the hall guard holds
// no party member, until it stands beside the cell or a fight takes the walk.
func (s *kingScene) walkMember(who encounter.MemberID, to spatial.Position, unseen bool) {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: who, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(s.t, err)
	require.Empty(s.t, route.StoppedBy, "%s has a way to %v", who, to)
	for i, cell := range route.Path {
		_, err := s.enc.Step(&encounter.StepInput{Member: who, To: cell, EndWalk: i == len(route.Path)-1})
		require.NoError(s.t, err)
		if unseen {
			s.hallGuardHoldsNobody()
		}
		if clockKind(s.t, s.enc, who) == encounter.ClockTurn {
			return
		}
	}
}

// hallGuardHolding is the hall guard's sight holding of a member, or nil.
func (s *kingScene) hallGuardHolding(who encounter.MemberID) *perception.Holding {
	s.t.Helper()
	held, err := s.enc.View(&encounter.ViewInput{Member: kingHall1})
	require.NoError(s.t, err)
	for i := range held {
		if held[i].Channel == perception.Sight && encounter.MemberID(held[i].Subject) == who {
			return &held[i]
		}
	}

	return nil
}

// hallGuardHoldsNobody pins that the hall guard's holdings never gained a
// party member — not live, not as a ghost.
func (s *kingScene) hallGuardHoldsNobody() {
	s.t.Helper()
	for _, who := range []encounter.MemberID{hero, kingFighter} {
		require.Nil(s.t, s.hallGuardHolding(who), "the hall guard never sighted %s", who)
	}
}

// declare sets a party member sneaking.
func (s *kingScene) declare(who encounter.MemberID) {
	s.t.Helper()
	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: who, Sneaking: true})
	require.NoError(s.t, err)
	require.True(s.t, out.Sneaking)
}

// TestMadKingKitchenDoorSneaksPastTheHall is the kitchen-door route: both
// party members declare sneaking in the kitchen, the rogue picks the locked
// door, and the party crosses the great hall to the throne door while the
// hall guard walks his loop. Every entry into his zone rolls once per party
// member; the 15s beat his passive 10, so no sighting of either lands, no
// fight forms, and the party stands at the throne door.
//
// THE MUTATION IT KILLS: the same walk with nobody sneaking — the subtest
// below — lands the hall guard's sighting of the party.
func TestMadKingKitchenDoorSneaksPastTheHall(t *testing.T) {
	t.Parallel()
	t.Run("sneaking", func(t *testing.T) {
		t.Parallel()
		s, stealth := kitchenParty(t)
		s.declare(hero)
		s.declare(kingFighter)
		require.Empty(t, stealth.asked, "the kitchen door is shut: no watcher's zone holds the party yet")

		s.walkTo(pastTheKitchenDoor)
		s.pickTheKitchenDoor()
		s.hallGuardHoldsNobody()
		s.walkMember(hero, pastTheThroneDoor, true)
		s.walkMember(kingFighter, pastTheThroneDoor, true)

		for _, who := range []encounter.MemberID{hero, kingFighter} {
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, who), "no fight formed")
			require.True(t, s.enc.Sneaking(who), "%s is still sneaking", who)
			require.LessOrEqual(t, s.enc.Distance(s.at(who), castleCell(0, -16)), 2.0,
				"%s stands at the throne door", who)
		}
		rolled := map[encounter.MemberID]int{}
		for _, in := range stealth.asked {
			require.Equal(t, []encounter.CheckApproach{{Ability: "stealth", DC: 10}}, in.Approaches,
				"every roll is against the hall guard's passive 10")
			rolled[in.Member]++
		}
		require.Equal(t, 1, rolled[hero], "the rogue entered the hall guard's zone once")
		require.Equal(t, 1, rolled[kingFighter], "and so did the fighter")
	})

	t.Run("without sneaking the hall guard sees the party", func(t *testing.T) {
		t.Parallel()
		s, stealth := kitchenParty(t)
		s.walkTo(pastTheKitchenDoor)
		s.pickTheKitchenDoor()
		s.walkMember(hero, pastTheThroneDoor, false)
		s.walkMember(kingFighter, pastTheThroneDoor, false)

		require.Empty(t, stealth.asked, "nobody sneaked, so nothing rolled")
		seen := s.hallGuardHolding(hero) != nil || s.hallGuardHolding(kingFighter) != nil
		require.True(t, seen, "the hall guard's sighting of the party lands")
	})
}

// TestMadKingTheKitchenDoorsCrashReachesOnlyTheHall is the plan's measurement
// of the forced door (rpg-project#606, Open items): forcing the kitchen door
// crashes, and at the shipped numbers the crash reaches the great hall at
// DC 10 and the throne room at DC 20. Each listener's passive is set one
// either side of those DCs: the hall guard hears at 10 and not at 9, a
// knight hears at 20 and not at 19. With the host's passive 10 for everyone,
// the hall guard is the only listener outside the kitchen who hears it, and
// his table carries no `heard` row — so the route's heard half is content
// the operator has to add, and no heard test is written.
func TestMadKingTheKitchenDoorsCrashReachesOnlyTheHall(t *testing.T) {
	cases := []struct {
		name    string
		who     encounter.MemberID
		passive int
		hears   bool
	}{
		{name: "the hall at 10", who: kingHall1, passive: 10, hears: true},
		{name: "the hall at 9", who: kingHall1, passive: 9, hears: false},
		{name: "the throne room at 20", who: kingKeep1, passive: 20, hears: true},
		{name: "the throne room at 19", who: kingKeep1, passive: 19, hears: false},
	}
	t.Parallel()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := openTheCastleAs(t, castleHost{at: besideTheCook, passive: map[encounter.MemberID]int{c.who: c.passive}})
			s.walkTo(pastTheKitchenDoor)
			_, err := s.enc.Unlock(&encounter.UnlockInput{
				Door: kingKitchenDoor, Actor: hero, Beaten: true, Total: 18, Applied: kitchenDoorForce,
			})
			require.NoError(t, err, "the hero forces the kitchen door")
			require.Equal(t, c.hears, s.heardKind(c.who, encounter.SoundCrash), "%s at passive %d", c.who, c.passive)
		})
	}
}
