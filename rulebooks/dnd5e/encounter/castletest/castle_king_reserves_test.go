// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_reserves_test.go is THE FAR ROOM COMES IN (rpg-project#661):
// the hero opens the gate with the gate fallen, the two reserves arrive in
// the barracks and see him across the courtyard at the session's sight, and
// he steps back into the gatehouse out of their sight. Each reserve walks in
// in, running while it is more than a run from him and walking inside one
// (the castle's `dasher`, rpg-project#671), and swings.
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
// RUNS in round 1 on every roll — the run is the only candidate at every
// level of its pick ([kingScene.ranOnEveryRoll]) — and stands in reach of him having swung sooner than a
// walk would have brought it, and within the dasher's own rounds
// ([dasherRounds]) over the pathfinder's route to where it REMEMBERS him,
// plus one round for the step from that memory to where he stands — he
// stepped back after they saw him, and a walk that ends on the memory can
// have nothing left to take it. Routes are from where each stood when he
// stepped back (the gate's crossing makes them longer than the straight
// line), at a turn's walk at the host's speed.
//
// WHAT THE RUN BUYS HERE (rpg-project#671, measured against the castle
// without the dasher): both reserves start more than a run away and run in
// round 1, then walk. reserve-2 swings in round 3 (a walk: round 4).
// reserve-1's walk in round 3 ends on its memory of the hero with nothing
// left, two cells short of him, and it swings in round 4 (a walk: round 5).
// Inside one run of reach the castle walks: a run there lands beside the hero
// with no swing and swings the round after anyway, handing him a free swing
// (the design's middle band).
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
	walked := map[encounter.MemberID]int{}
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
		bound[id] = dasherRounds(len(toMemory.Path), pace) + 1
		walked[id] = (len(walk.Path) + pace - 1) / pace
	}

	swung := func(id encounter.MemberID) bool {
		for _, strike := range *s.strikes {
			if strike.attacker == id && strike.target == hero {
				return true
			}
		}
		return false
	}
	before, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(t, err)
	arrived := map[encounter.MemberID]int{}
	for round := 1; len(arrived) < len(reserves) && round <= bound[kingReserve]+bound[kingReserve2]; round++ {
		s.endTurn()
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "the fight is still on")
		if round == 1 {
			for _, id := range reserves {
				s.ranOnEveryRoll(id, len(before))
			}
		}
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
		require.Less(t, round, walked[id], "%s swings sooner than a walk's %d rounds", id, walked[id])
		require.True(t, s.ran(id), "%s ran once only a run reached him", id)
	}
}

// dasherRounds is the round a creature on the castle's `dasher` swings in,
// over a route of n cells at pace cells a walk: within one walk it walks and
// swings; within two (one run) it walks; beyond that it runs two walks' worth.
func dasherRounds(n, pace int) int {
	for round := 1; ; round++ {
		switch {
		case n <= pace:
			return round
		case n <= 2*pace:
			n -= pace
		default:
			n -= 2 * pace
		}
	}
}

// ranOnEveryRoll asserts a creature's first pick after the story's first
// `from` entries was the run, and that it was the ONLY candidate at every
// level of the roll — so whatever the die says, it runs. The harness's die
// is 1; this reads the candidates instead, so it holds for every face.
func (s *kingScene) ranOnEveryRoll(id encounter.MemberID, from int) {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(s.t, err)
	for _, entry := range story[from:] {
		var pick struct {
			Beat       string           `json:"beat"`
			Creature   string           `json:"creature"`
			Word       string           `json:"word"`
			Path       []int            `json:"path"`
			Candidates []map[string]any `json:"candidates"`
			Rows       []struct {
				Held       bool `json:"held"`
				Affordable bool `json:"affordable"`
				Rows       []struct {
					Held       bool `json:"held"`
					Affordable bool `json:"affordable"`
				} `json:"rows"`
			} `json:"rows"`
		}
		require.NoError(s.t, json.Unmarshal(entry.Payload, &pick))
		if pick.Beat != encounter.BeatAnswered || pick.Creature != string(id) {
			continue
		}
		require.Equal(s.t, "dash", pick.Word, "%s's first pick is the run", id)
		require.Len(s.t, pick.Candidates, 1, "%s: the dasher is the only row on the table, so every roll lands on it", id)
		inner := 0
		for _, row := range pick.Rows[pick.Path[0]].Rows {
			if row.Held && row.Affordable {
				inner++
			}
		}
		require.Equal(s.t, 1, inner, "%s: and the run is the only row inside it", id)

		return
	}
	require.Failf(s.t, "no pick", "%s picked nothing in round 1", id)
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
