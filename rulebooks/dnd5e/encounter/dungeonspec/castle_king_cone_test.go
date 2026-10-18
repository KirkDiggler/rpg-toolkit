// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_cone_test.go is THE HALL GUARD'S BACK (rpg-project#621,
// sneaking K1, design R14): with an arc of 120 scripted for the hall guard,
// a sneaking hero who comes through the kitchen door while his patrol walks
// away from it is in his sight and out of his zone — nothing rolls and he
// does not see the hero — and the roll fires once his patrol turns back
// toward the door.

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

// TestMadKingTheHallGuardsBackIsNotWatched paces the sneaking hero inside
// the kitchen door, out of the hall guard's sight, until his patrol has its
// back to the door with the hero in his ordinary sight; walks the hero into
// the hall behind him; and paces there until his patrol turns back.
func TestMadKingTheHallGuardsBackIsNotWatched(t *testing.T) {
	t.Parallel()
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: {15, 15, 15, 15}},
	}
	s := openTheCastleAs(t, castleHost{
		at: besideTheCook, resolver: stealth, arcs: map[encounter.MemberID]int{kingHall1: 120},
	})
	seer := &coneSite{sneakSite: &sneakSite{t: t, enc: s.enc}}
	s.declare(hero)
	s.walkTo(pastTheKitchenDoor)
	s.pickTheKitchenDoor()
	door := s.at(hero)

	// behindHim reports whether the hall guard's ordinary sight reaches the
	// hero's cell — his own fog — with the hero outside his front third.
	behindHim := func() bool {
		guard := positionOf(t, s.enc, kingHall1)
		bearing := coneBearing(guard, s.at(hero))
		return seer.seenNow(kingHall1)[s.at(hero)] && offArc(facingOf(t, s.enc, kingHall1), bearing) > 60
	}
	step := func(to spatial.Position) {
		t.Helper()
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: true})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
	}
	// requireUnwatched pins the hero unrolled, unseen by the hall guard, and
	// off his red floor.
	requireUnwatched := func(where string) {
		t.Helper()
		require.Empty(t, stealth.asked, "%s: nothing rolled", where)
		require.False(t, seer.seesLive(kingHall1, hero), "%s: the hall guard does not see the hero", where)
		for _, zone := range zonesOf(t, s.enc, hero) {
			if zone.Watcher == kingHall1 {
				require.False(t, cellSet(zone.Cells)[s.at(hero)], "%s: the hero's cell is not in his zone", where)
			}
		}
	}

	// Inside the kitchen, two and three cells back from the door: out of
	// the hall's sight while the patrol walks toward the door.
	back, further := door, door
	back.X -= 2
	further.X -= 3
	found := false
	for i := 0; i < 40 && !found; i++ {
		step(map[bool]spatial.Position{true: further, false: back}[i%2 == 0])
		require.Empty(t, stealth.asked, "pacing inside the door rolls nothing")
		found = behindHim()
	}
	require.True(t, found, "the patrol turns its back on the door with the hero in its sight")
	requireUnwatched("in the kitchen, in his sight, behind him")

	// Into the hall behind his back, a step at a time.
	inHall := door
	inHall.X += 2
	checkedInHall := false
	for x := door.X - 1; x <= inHall.X; x++ {
		step(spatial.Position{X: x, Y: door.Y})
		if !behindHim() {
			break
		}
		requireUnwatched("walking into the hall behind him")
		checkedInHall = checkedInHall || x > door.X
	}
	require.True(t, checkedInHall, "at least one hall cell was walked behind his back and checked")

	// THE PATROL TURNS BACK: the roll fires against him, from his front.
	inside := s.at(hero)
	beside := inside
	beside.X--
	for i := 0; i < 40 && len(stealth.asked) == 0; i++ {
		step(map[bool]spatial.Position{true: beside, false: inside}[i%2 == 0])
	}
	require.NotEmpty(t, stealth.asked, "the patrol's turn toward the hero rolls")
	require.Equal(t, []encounter.CheckApproach{{Ability: "stealth", DC: 10}}, stealth.asked[0].Approaches,
		"against the hall guard's passive")
	guard := positionOf(t, s.enc, kingHall1)
	require.LessOrEqual(t, offArc(facingOf(t, s.enc, kingHall1), coneBearing(guard, s.at(hero))), 60.0,
		"the roll fired with the hero inside his front third")
	verdict, judged := s.enc.ToData().SneakVerdicts[hero][kingHall1]
	require.True(t, judged)
	require.True(t, verdict.Beaten, "a 15 and +7 beat his passive 10")
}
