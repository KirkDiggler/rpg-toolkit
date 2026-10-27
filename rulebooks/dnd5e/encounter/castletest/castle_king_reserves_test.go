// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_reserves_test.go is THE FAR ROOM COMES IN (rpg-project#661):
// the hero opens the gate with the gate fallen, the two reserves arrive in
// the barracks and see him across the courtyard at the session's sight, and
// he steps back into the gatehouse out of their sight. Each reserve walks in
// at a turn's pace and swings.
//
// THE FIXTURE IS CHOSEN TO PROVE THE MECHANIC. The castle runs at the
// session's sight ([sessionSightFeet]), so the reserves see the hero from the
// barracks the moment the gate opens and a fight forms; one step back puts
// him behind the gatehouse wall, so both hold him REMEMBERED: the band
// `answer-the-gate` had no row for, where a reserve held every turn of the
// fight. At sixty feet they never see him from the barracks and walk to the
// yard on `enemy: none`, which is why no route caught it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingTheReservesComeIntoTheGatehouse opens the gate from beneath it,
// steps the hero back into the gatehouse, and ends his turns: each reserve
// stands in reach of him and has swung at him within ceil(walk / pace)
// rounds, where walk is the pathfinder's own route from where it stood when
// he stepped back (the gate's crossing makes it longer than the straight
// line) and pace is a turn's walk at the host's speed.
func TestMadKingTheReservesComeIntoTheGatehouse(t *testing.T) {
	t.Parallel()
	arcs := map[encounter.MemberID]int{}
	for _, m := range loadContent(t, kingFixture).Monsters {
		arcs[encounter.MemberID(m.MemberID)] = 120
	}
	s := openTheCastleAs(t, castleHost{
		at: gatehouseSeat, arcs: arcs, sightFt: sessionSightFeet,
		passive: map[encounter.MemberID]int{kingGate1: gate1Passive, kingGate2: gate2Passive, kingCaptain: captainPassive},
	})
	s.struck(kingGate1, false)
	s.struck(kingGate1, true)
	s.struck(kingGate2, true)
	s.struck(kingCaptain, true)
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: the gate has fallen and the fight is over")

	s.stepTo(beneathTheGate)
	s.open(kingGateDoor)
	reserves := []encounter.MemberID{kingReserve, kingReserve2}
	for _, id := range reserves {
		require.Equal(t, "Barracks", labelOf(t, s.at(id)), "precondition: %s arrived in the barracks", id)
		require.True(t, s.seesLive(id, hero), "precondition: %s sees the hero across the courtyard", id)
	}
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "precondition: the reserves' sighting forms a fight")

	// One step back, inside the gatehouse and behind its wall.
	inside := castleCell(0, 9)
	for _, cell := range []spatial.Position{castleCell(0, 8), inside} {
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: cell == inside})
		require.NoError(t, err)
	}
	require.Equal(t, "Gatehouse", labelOf(t, s.at(hero)), "precondition: the hero stepped back into the gatehouse")

	pace := encounter.CellsFromFeet(castleSpeedFeet)
	bound := map[encounter.MemberID]int{}
	for _, id := range reserves {
		require.NotNil(t, s.holding(id, hero), "precondition: %s still holds the hero", id)
		require.False(t, s.seesLive(id, hero), "precondition: %s remembers him and does not see him", id)
		walk, err := s.enc.Route(encounter.RouteInput{Mover: id, Policy: encounter.MoveToward, Anchor: inside, Budget: 200})
		require.NoError(t, err)
		require.Empty(t, walk.StoppedBy, "precondition: %s has a way to the hero", id)
		bound[id] = (len(walk.Path) + pace - 1) / pace
	}

	swung := func(id encounter.MemberID) bool {
		for _, strike := range *s.strikes {
			if strike.attacker == id && strike.target == hero {
				return true
			}
		}
		return false
	}
	arrived := map[encounter.MemberID]int{}
	for round := 1; len(arrived) < len(reserves) && round <= bound[kingReserve]+bound[kingReserve2]; round++ {
		s.endTurn()
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "the fight is still on")
		for _, id := range reserves {
			if _, ok := arrived[id]; !ok && s.enc.Distance(s.at(id), s.at(hero)) <= 1 && swung(id) {
				arrived[id] = round
			}
		}
	}
	for _, id := range reserves {
		round, ok := arrived[id]
		require.True(t, ok, "%s stands in the hero's reach and swings at him", id)
		require.LessOrEqual(t, round, bound[id], "%s comes in at a turn's pace: within %d rounds", id, bound[id])
	}
}
