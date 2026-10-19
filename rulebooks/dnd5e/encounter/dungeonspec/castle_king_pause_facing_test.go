// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_pause_facing_test.go is THE GATE GUARDS WATCH THE INCOMERS
// (rpg-project#624): the gate patrol's two gate-side corners state `wait: 2`
// with `facing: se` and `facing: sw`, so a gate guard whose walk ends on one
// stands two rounds there turned inward to the party start (Kirk,
// 2026-10-19: a pause is a dwell, and its facing holds only while he stands
// there).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingGateGuardsDwellWatchingTheApproach paces a sneaking hero at the
// party start, with an arc of 120 scripted for both gate guards, until a gate
// guard has stood out a dwell at each gate-side corner; on every round of it
// he faces the corner's word and his zone holds the approach cell, the party
// start.
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

	// The gate-side corners as the file writes them and the bearing each
	// states; the approach is the party start, where the incomers arrive.
	corners := map[spatial.Position]float64{
		{X: -7, Y: 9}: 135, // se
		{X: -1, Y: 9}: 225, // sw
	}
	approach := start
	for corner, facing := range corners {
		require.LessOrEqual(t, offArc(facing, coneBearing(corner, approach)), 60.0,
			"the approach lies inside the front third of the corner at %v", corner)
	}

	stoodOut := map[spatial.Position]bool{}
	done := func() bool { return len(stoodOut) == len(corners) }
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
			corner := s.at(guard)
			stated, onCorner := corners[corner]
			require.True(t, onCorner, "%s dwells on a gate-side corner, not %v", guard, corner)
			require.Equal(t, corner, spatial.Position{X: dwell.Cell.X, Y: dwell.Cell.Y})
			if dwell.Rounds == 0 {
				stoodOut[corner] = true
			}
			require.InDelta(t, stated, facingOf(t, s.enc, guard), 1e-9, "%s stands facing the corner's word", guard)
			zone := map[spatial.Position]bool{}
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == guard {
					zone = cellSet(z.Cells)
				}
			}
			require.True(t, zone[approach], "%s's zone, standing at the corner, holds the approach cell", guard)
		}
	}
	require.True(t, done(), "a gate guard stood out a dwell at each gate-side corner")
}
