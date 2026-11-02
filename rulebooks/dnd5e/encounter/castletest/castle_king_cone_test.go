// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_cone_test.go is THE HALL GUARD'S BACK (rpg-project#621,
// sneaking K1, design R14): with an arc of 120 scripted for the hall guard,
// a sneaking hero who comes through the kitchen door while his patrol walks
// away from it is in his sight and out of his zone — nothing rolls and he
// does not see the hero — and the roll fires once his patrol turns back
// toward the door.
//
// THE HERO DECLARES IN THE HALL'S CORNER, then goes back through the door:
// nobody begins to sneak in the kitchen, where the cook and the scullion see
// it (sneaking R10 as amended by R11, rpg-project#768). Walking back in, the
// hero is rolled against the cook and wins; the hall guard's passive is 11
// here, the kitchen's 10, so the DC a roll was asked at names whose it was.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// offArc is how far a bearing lies from a facing, 0 to 180.
func offArc(facing, bearing float64) float64 {
	off := bearing - facing
	for off > 180 {
		off -= 360
	}
	for off < -180 {
		off += 360
	}
	if off < 0 {
		off = -off
	}

	return off
}

// waitInside is how many steps the hero paces inside the kitchen, not
// sneaking, before the patrol stands at the throne door's end of its loop:
// the corner is then out of the hall guard's sight to declare in, and the
// way back through the door is too. Measured on this castle; a change to
// his loop moves it, and the declaration's refusal names him.
const waitInside = 12

// TestMadKingTheHallGuardsBackIsNotWatched has the hero declare in the
// hall's corner and come back through the kitchen door; paces the sneaking
// hero inside, off the door's line and out of the hall guard's sight, until
// his patrol turns its back on the door; walks the hero out along the door's
// line into the hall behind him, in his sight; and paces there until his
// patrol turns back.
func TestMadKingTheHallGuardsBackIsNotWatched(t *testing.T) {
	t.Parallel()
	theHallGuardsBack(t, waitInside)
}

// theHallGuardsBack is the test, the hero waiting inside for a number of
// steps before it declares.
func theHallGuardsBack(t *testing.T, wait int) {
	const hallPassive = 11
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: {15, 15, 15, 15, 15, 15}},
	}
	s := openTheCastleAs(t, castleHost{
		at: besideTheCook, resolver: stealth, arcs: map[encounter.MemberID]int{kingHall1: 120},
		passive: map[encounter.MemberID]int{kingHall1: hallPassive},
	})
	s.walkTo(pastTheKitchenDoor)
	s.pickTheKitchenDoor()
	door := s.at(hero)
	// back and further are two and three cells inside the door on its own
	// row, where the hero waits not sneaking; aside and beyond are the same
	// two on the row north of it, off the door's line, where it paces
	// sneaking.
	back, further := door, door
	back.X -= 2
	further.X -= 3
	aside, beyond := back, further
	aside.Y--
	beyond.Y--
	step := func(to spatial.Position) {
		t.Helper()
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
	}

	// Not sneaking, the hero waits inside the kitchen, out of the hall's
	// sight, while the patrol walks to the throne door's end of its loop;
	// then declares in the corner and comes back in before it turns.
	step(spatial.Position{X: door.X - 1, Y: door.Y})
	step(back)
	for i := 0; i < wait; i++ {
		step(map[bool]spatial.Position{true: further, false: back}[i%2 == 0])
	}
	s.walkTo(theHallCorner)
	s.declare(hero)
	s.walkTo(aside)
	if s.at(hero) != aside {
		step(aside)
	}

	// rolledAgainstHim is every roll asked at the hall guard's passive.
	rolledAgainstHim := func() []encounter.ResolveCheckInput {
		var out []encounter.ResolveCheckInput
		for _, in := range stealth.asked {
			if in.Approaches[0].DC == hallPassive {
				out = append(out, in)
			}
		}
		return out
	}

	// behindHim reports whether the hall guard's ordinary sight reaches the
	// hero's cell — his own fog — with the hero outside his front third.
	behindHim := func() bool {
		guard := positionOf(t, s.enc, kingHall1)
		bearing := coneBearing(guard, s.at(hero))
		return s.seenNow(kingHall1)[s.at(hero)] && offArc(facingOf(t, s.enc, kingHall1), bearing) > 60
	}
	// requireUnwatched pins the hero unrolled, unseen by the hall guard, and
	// off his red floor.
	requireUnwatched := func(where string) {
		t.Helper()
		require.Empty(t, rolledAgainstHim(), "%s: nothing rolled against him", where)
		require.False(t, s.seesLive(kingHall1, hero), "%s: the hall guard does not see the hero", where)
		for _, zone := range zonesOf(t, s.enc, hero) {
			if zone.Watcher == kingHall1 {
				require.False(t, cellSet(zone.Cells)[s.at(hero)], "%s: the hero's cell is not in his zone", where)
			}
		}
	}

	// Inside the kitchen, two and three cells back from the door and off its
	// line: out of the hall's sight while the patrol walks toward the door.
	require.Empty(t, rolledAgainstHim(), "coming back in rolls nothing against him")
	away := false
	for i := 0; i < 40 && !away; i++ {
		step(map[bool]spatial.Position{true: beyond, false: aside}[i%2 == 0])
		require.Empty(t, rolledAgainstHim(), "pacing inside the door rolls nothing against him")
		away = offArc(facingOf(t, s.enc, kingHall1), 270) > 90
	}
	require.True(t, away, "the patrol turns its back on the door")

	// Out along the door's line, through the door and into the hall behind
	// his back, a step at a time: in his sight, out of his zone.
	found, checkedInHall := false, false
	for x := back.X; x <= door.X+3; x++ {
		step(spatial.Position{X: x, Y: door.Y})
		if !behindHim() {
			if found {
				break // he turned, or walked out of sight
			}
			require.Empty(t, rolledAgainstHim(), "out of his sight: nothing rolled against him")
			continue
		}
		found = true
		requireUnwatched("walking behind him")
		checkedInHall = checkedInHall || x > door.X
	}
	require.True(t, found, "the hero walked into his sight behind him")
	require.True(t, checkedInHall, "at least one hall cell was walked behind his back and checked")

	// THE PATROL TURNS BACK: the roll fires against him, from his front.
	inside := s.at(hero)
	beside := inside
	beside.X--
	for i := 0; i < 40 && len(rolledAgainstHim()) == 0; i++ {
		step(map[bool]spatial.Position{true: beside, false: inside}[i%2 == 0])
	}
	require.NotEmpty(t, rolledAgainstHim(), "the patrol's turn toward the hero rolls against him")
	require.Equal(t, []encounter.CheckApproach{{Ability: "stealth", DC: hallPassive}}, rolledAgainstHim()[0].Approaches)
	guard := positionOf(t, s.enc, kingHall1)
	require.LessOrEqual(t, offArc(facingOf(t, s.enc, kingHall1), coneBearing(guard, s.at(hero))), 60.0,
		"the roll fired with the hero inside his front third")
	verdict, judged := s.enc.ToData().SneakVerdicts[hero][kingHall1]
	require.True(t, judged)
	require.True(t, verdict.Beaten, "a 15 and +7 beat his passive 11")
}
