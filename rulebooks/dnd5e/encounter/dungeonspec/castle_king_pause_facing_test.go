// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_pause_facing_test.go is THE GATE GUARDS WATCH THE INCOMERS
// (rpg-project#624): the gate patrol's gate-side corner states `wait: 2,
// facing: s`, so a gate guard whose walk ends there stands two rounds facing
// south (Kirk, 2026-10-19: a pause is a dwell, and its facing holds only
// while he stands there).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingGateGuardsDwellWatchingTheApproach paces a sneaking hero at the
// party start, with an arc of 120 scripted for both gate guards, until a gate
// guard has stood out his dwell at the gate-side corner; on every round of
// it he faces south and his zone holds the approach cell.
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

	// The gate-side corner as the file writes it, and the approach: a cell
	// of the gatehouse floor two rows south of it, inside the hero's sight.
	corner := spatial.Position{X: -7, Y: 9}
	approach := spatial.Position{X: -8, Y: 11}
	require.InDelta(t, 180, coneBearing(corner, approach), 1e-6, "the approach lies due south of the corner")

	stood := map[encounter.MemberID]map[int]bool{kingGate1: {}, kingGate2: {}}
	done := func() bool {
		for _, rounds := range stood {
			if rounds[0] {
				return true
			}
		}
		return false
	}
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
			require.Equal(t, corner, s.at(guard), "%s dwells on the gate-side corner", guard)
			stood[guard][dwell.Rounds] = true
			require.InDelta(t, 180, facingOf(t, s.enc, guard), 1e-9, "%s stands facing the stated south", guard)
			zone := map[spatial.Position]bool{}
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == guard {
					zone = cellSet(z.Cells)
				}
			}
			require.True(t, zone[approach], "%s's zone, standing at the corner, holds the approach cell", guard)
		}
	}
	require.True(t, done(), "a gate guard stood out his dwell at the gate-side corner")
}
