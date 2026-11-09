// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// The reserves' fight-round notification prepares their authored marching state.
// Actual movement waits for a priced world opportunity: catching up to an existing
// high-water grants no free leg. R6 separates news from permission.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// marchBeat is the two beats this test reads: a pick (its creature and key)
// and a state change (its member and states).
type marchBeat struct {
	Beat     string `json:"beat"`
	Creature string `json:"creature"`
	Key      string `json:"key"`
	Member   string `json:"member"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// marchBeats is the hero's story from seq on, as those beats.
func marchBeats(t *testing.T, s *kingScene, from int) []marchBeat {
	t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(t, err)
	var out []marchBeat
	for _, e := range story[from:] {
		var b marchBeat
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		out = append(out, b)
	}

	return out
}

// TestReserves_MarchOnFightRounds opens the gate in the middle of the gate
// fight: the reserves arrive in the barracks out of sight, on the world
// clock, and the round the fight runs each of them picks `march` once and
// nothing else, closing on the gate; the state beat says they left waiting
// once. (A second round brings them into the hero's sight and the fight,
// which is the march arriving.) When the fight is over and the hero walks, the world's first round
// puts them back to waiting, and the round after they roll their `time`.
func TestReserves_NotificationsPrepareAPricedMarch(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, gatehouseSeat, nil)
	// Beside the crossing, not on it: gate-1 holds the cell beneath the gate
	// (rpg-project#836), and the door is in reach from the cell beside it.
	s.stepTo(besideTheCrossing)
	s.struck(kingGate1, false)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "precondition: the gate fight is on")
	// The gate door is held while gate-1 stands his post (rpg-project#835
	// R7): he falls first, and the fight goes on.
	s.standing.down[kingGate1] = true
	s.open(kingGateDoor)
	reserves := []encounter.MemberID{kingReserve, kingReserve2}
	for _, id := range reserves {
		require.Contains(t, memberIDsOf(t, s.enc), id, "precondition: %s arrived", id)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, id), "precondition: %s is not in the fight", id)
	}

	gate := castleCell(-2, 4)
	start := map[encounter.MemberID]float64{}
	for _, id := range reserves {
		start[id] = s.enc.Distance(s.at(id), gate)
	}

	// The first fight round catches up to the existing high-water: it is
	// news, not a world budget. The second raises time and pays one march.
	const rounds = 2
	for round := 1; round <= rounds; round++ {
		mark, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
		require.NoError(t, err)
		beforeClock := s.enc.ToData().Clock
		s.endTurn()
		for _, id := range reserves {
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, id), "one leg does not bring %s into the fight", id)
		}
		picks := map[encounter.MemberID][]string{}
		for _, b := range marchBeats(t, s, len(mark)) {
			if b.Beat == encounter.BeatAnswered && b.Creature != "" {
				picks[encounter.MemberID(b.Creature)] = append(picks[encounter.MemberID(b.Creature)], b.Key)
			}
		}
		for _, id := range reserves {
			if round == 1 {
				require.Equal(t, beforeClock.HighWater, s.enc.ToData().Clock.HighWater)
				require.Empty(t, picks[id], "clock notification grants no free action")
				require.Equal(t, start[id], s.enc.Distance(s.at(id), gate), "unpaid news moves nobody")
				require.Equal(t, "marching", machineState(t, s.enc, id), "news prepares the authored state")
			} else {
				require.Equal(t, beforeClock.HighWater+1, s.enc.ToData().Clock.HighWater)
				require.Equal(t, []string{"behavior/6:policy/march"}, picks[id], "the priced world opportunity moves one leg")
			}
		}
	}
	for _, id := range reserves {
		require.Less(t, s.enc.Distance(s.at(id), gate), start[id], "%s closed on the gate", id)
	}

	left := map[string]int{}
	for _, b := range marchBeats(t, s, 0) {
		if b.Beat == encounter.BeatState && b.From == "waiting" && b.To == "marching" {
			left[b.Member]++
		}
	}
	for _, id := range reserves {
		require.Equal(t, 1, left[string(id)], "%s left waiting once, on the fight's first round", id)
	}

	// The fight ends; the world's first round sends them back to waiting.
	s.struck(kingGate1, true)
	s.struck(kingGate2, true)
	s.struck(kingCaptain, true)
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: the fight is over")
	mark, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(t, err)
	from := s.enc.ToData().Clock.HighWater
	require.True(t, s.pace(besideTheCrossing, castleCell(-2, 7), 24, func() bool {
		return s.enc.ToData().Clock.HighWater >= from+2
	}), "precondition: two world rounds passed")
	back := map[string]bool{}
	timed := map[string]bool{}
	for _, b := range marchBeats(t, s, len(mark)) {
		if b.Beat == encounter.BeatState && b.From == "marching" && b.To == "waiting" {
			back[b.Member] = true
		}
		if b.Beat == encounter.BeatAnswered && b.Key == "behavior/6:policy/time" {
			timed[b.Creature] = true
		}
	}
	for _, id := range reserves {
		require.True(t, back[string(id)], "%s waits again once the world's clock moves", id)
		require.True(t, timed[string(id)], "%s rolls its time again", id)
	}
}
