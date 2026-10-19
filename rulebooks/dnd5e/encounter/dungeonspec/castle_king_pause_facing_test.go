// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_pause_facing_test.go is THE GATE GUARDS WATCH THE ROAD
// (rpg-project#624): the gate patrol's two gate-side waypoints state the
// facing taken at the pause there, se from the west corner and sw from the
// east, so a gate guard whose walk ends on one of them turns inward to the
// party start, where the road comes in. By Kirk's ruling of 2026-10-19 the
// facing holds only while he is paused there; walking, he faces his step
// (sneaking design R17–R20).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingGateGuardsPauseFacingTheRoad paces a sneaking hero at the party
// start, with an arc of 120 scripted for both gate guards, until a gate
// guard's round ends on a gate-side waypoint; there he faces the stated
// word, and his zone holds the approach cell — the party start.
func TestMadKingGateGuardsPauseFacingTheRoad(t *testing.T) {
	t.Parallel()
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: make([]int, 200)},
	}
	for i := range stealth.d20s[hero] {
		stealth.d20s[hero][i] = 15
	}
	s := openTheCastleAs(t, castleHost{
		at: partyStart(t), resolver: stealth, arcs: map[encounter.MemberID]int{kingGate1: 120, kingGate2: 120},
	})
	s.declare(hero)
	approach := s.at(hero)

	// The gate-side waypoints, as the file writes them, and the word each
	// states.
	gateSide := map[spatial.Position]float64{
		{X: -7, Y: 9}: 135, // se
		{X: -1, Y: 9}: 225, // sw
	}
	beside := spatial.Position{X: approach.X - 1, Y: approach.Y}
	last := map[encounter.MemberID]spatial.Position{
		kingGate1: s.at(kingGate1), kingGate2: s.at(kingGate2),
	}
	paused := map[spatial.Position]bool{}
	for i := 0; i < 120 && len(paused) < len(gateSide); i++ {
		to := map[bool]spatial.Position{true: beside, false: approach}[i%2 == 0]
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "the civil garrison forms no fight")
		for _, guard := range []encounter.MemberID{kingGate1, kingGate2} {
			at := s.at(guard)
			if at == last[guard] {
				continue
			}
			last[guard] = at
			stated, onGateSide := gateSide[at]
			if !onGateSide {
				continue
			}
			paused[at] = true
			require.InDelta(t, stated, facingOf(t, s.enc, guard), 1e-9,
				"%s, paused on the gate-side waypoint %v, faces the stated word", guard, at)
			require.LessOrEqual(t, offArc(stated, coneBearing(at, approach)), 60.0,
				"the approach lies inside his front third")
			zone := map[spatial.Position]bool{}
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == guard {
					zone = cellSet(z.Cells)
				}
			}
			require.True(t, zone[approach], "%s's zone, paused at %v, holds the approach cell", guard, at)
		}
	}
	require.Len(t, paused, len(gateSide), "a gate guard's round ended on each gate-side waypoint")
}
