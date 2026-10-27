// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_reserves_test.go is THE FAR ROOM COMES IN (rpg-project#661):
// the hero opens the gate with the gate fallen, the two reserves arrive in
// the barracks and see him across the courtyard at the session's sight, and
// he steps back into the gatehouse out of their sight. Each reserve walks in
// at a turn's pace, runs once only a run reaches him (the castle's `dasher`,
// rpg-project#671), and swings.
//
// THE FIXTURE IS CHOSEN TO PROVE THE MECHANIC. The castle runs at the
// session's sight ([sessionSightFeet]), so the reserves see the hero from the
// barracks the moment the gate opens and a fight forms; one step back puts
// him behind the gatehouse wall, so both hold him REMEMBERED: the band
// `answer-the-gate` had no row for, where a reserve held every turn of the
// fight. At sixty feet they never see him from the barracks and walk to the
// yard on `enemy: none`, which is why no route caught it.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingTheReservesComeIntoTheGatehouse opens the gate from beneath it,
// steps the hero back into the gatehouse, and ends his turns: each reserve
// RUNS at least once and stands in reach of him having swung within the
// dasher's own rounds ([dasherRounds]) over the pathfinder's route from where
// it stood to where it REMEMBERS him — the cell it runs to, one short of
// where he stepped (the gate's crossing makes the route longer than the
// straight line) — at a turn's walk at the host's speed.
//
// WHAT THE RUN BUYS HERE (rpg-project#671, measured against the castle
// without the dasher): reserve-1 swings in round 4 where its walk swung in
// round 5; reserve-2 swings in round 4 either way — it runs in round 3 to the
// hero's side instead of walking to three cells off, and both swing the
// round after, the design's middle band, where a run buys no swing sooner.
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
		testimony, ok := encounter.DecodeSightTestimony(s.holding(id, hero).Payload)
		require.True(t, ok, "precondition: %s holds where it saw him", id)
		require.LessOrEqual(t, s.enc.Distance(testimony.Position, inside), float64(pace),
			"precondition: from where %s remembers him, he is a walk away", id)
		toMemory, err := s.enc.Route(encounter.RouteInput{
			Mover: id, Policy: encounter.MoveToward, Anchor: testimony.Position, Budget: 200,
		})
		require.NoError(t, err)
		bound[id] = dasherRounds(len(toMemory.Path), pace)
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
		require.LessOrEqual(t, round, bound[id], "%s comes in at the dasher's pace: within %d rounds", id, bound[id])
		require.True(t, s.ran(id), "%s ran once only a run reached him", id)
	}
}

// dasherRounds is the round a creature on the castle's `dasher` swings in,
// over a route of n cells at pace cells a walk: beyond two walks it walks
// (no dasher row holds; the pursuer's does), within one walk it walks and
// swings, and between the two it runs to his side and swings the next round.
func dasherRounds(n, pace int) int {
	for round := 1; ; round++ {
		switch {
		case n <= pace:
			return round
		case n <= 2*pace:
			return round + 1
		}
		n -= pace
	}
}

// ran reports whether one member's record holds a step of a run.
func (s *kingScene) ran(id encounter.MemberID) bool {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(s.t, err)
	for _, entry := range story {
		var beat map[string]any
		require.NoError(s.t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatMoved && beat["member"] == string(id) && beat["cause"] == "encounter:table:dash" {
			return true
		}
	}

	return false
}
