// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_pause_facing_test.go is THE GATE GUARDS WATCH THE GATE, THEN
// THE INCOMERS (rpg-project#624, reshaped by the QA castle's step 4,
// rpg-project#639): each gate guard walks its own short loop, standing one
// round at a door post turned to the cell under the gate, then one at a beat
// post turned inward to the party start (Kirk, 2026-10-19: a pause is a
// dwell, and its facing holds only while he stands there).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingGateGuardsDwellWatchingTheApproach paces a sneaking hero at the
// party start, with an arc of 120 scripted for both gate guards, until a gate
// guard has stood out a dwell at each of the four posts; on every round of it
// he faces the post's word and his zone holds the cell the post watches.
func TestMadKingGateGuardsDwellWatchingTheApproach(t *testing.T) {
	t.Parallel()
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: make([]int, 400)},
	}
	for i := range stealth.d20s[hero] {
		stealth.d20s[hero][i] = 15
	}
	s := openTheCastleAs(t, castleHost{
		at: partyStart(t), resolver: stealth, arcs: map[encounter.MemberID]int{kingGate1: 120, kingGate2: 120},
	})
	s.declare(hero)
	start := s.at(hero)
	beside := spatial.Position{X: start.X - 1, Y: start.Y}

	// The posts as the file writes them, the bearing each states, and the
	// cell it watches: the door posts the cell under the gate, the beat posts
	// the party start, where the incomers arrive.
	type post struct {
		facing  float64
		watches spatial.Position
	}
	posts := map[spatial.Position]post{
		westDoorPost: {45, beneathTheGate},  // ne
		eastDoorPost: {315, beneathTheGate}, // nw
		westBeatPost: {135, start},          // se
		eastBeatPost: {225, start},          // sw
	}
	for at, p := range posts {
		require.LessOrEqual(t, offArc(p.facing, coneBearing(at, p.watches)), 60.0,
			"the post at %v watches %v inside its front third", at, p.watches)
	}

	stoodOut := map[spatial.Position]bool{}
	done := func() bool { return len(stoodOut) == len(posts) }
	for i := 0; i < 240 && !done(); i++ {
		to := map[bool]spatial.Position{true: beside, false: start}[i%2 == 0]
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "the civil garrison forms no fight")
		for _, guard := range []encounter.MemberID{kingGate1, kingGate2} {
			dwell := dwellOf(t, s.enc, guard)
			if dwell == nil {
				continue
			}
			at := s.at(guard)
			p, onPost := posts[at]
			require.True(t, onPost, "%s dwells on a post, not %v", guard, at)
			require.Equal(t, at, spatial.Position{X: dwell.Cell.X, Y: dwell.Cell.Y})
			if dwell.Rounds == 0 {
				stoodOut[at] = true
			}
			require.InDelta(t, p.facing, facingOf(t, s.enc, guard), 1e-9, "%s stands facing the post's word", guard)
			zone := map[spatial.Position]bool{}
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == guard {
					zone = cellSet(z.Cells)
				}
			}
			require.True(t, zone[p.watches], "%s's zone, standing at %v, holds %v", guard, at, p.watches)
		}
	}
	require.True(t, done(), "a gate guard stood out a dwell at each post")
}
