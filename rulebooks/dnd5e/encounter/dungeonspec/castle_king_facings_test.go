// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_facings_test.go is EVERY STILL WATCHER LOOKS AT WHAT IT KEEPS
// (rpg-project#623, after sneaking C-cone): with an arc of 120 scripted for
// each, the castle's stationary watchers cone over the cell in front of the
// thing they guard, because the file states their facings. Unstated, each
// would face south (design R13) and cone at a wall.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// kingScullion is the kitchen's second hand.
const kingScullion = encounter.MemberID("scullion")

// The cells in front of what the castle's stationary watchers keep, on the
// keeper's side of the door, dungeon-absolute.
var (
	// kitchenDoorInside is the kitchen's cell nearest its door into the hall.
	kitchenDoorInside = spatial.Position{X: -9, Y: -11}

	// cellsDoorOutside is the barracks' cell nearest the cells door.
	cellsDoorOutside = spatial.Position{X: -23, Y: 6}

	// beforeTheCellsDoor is one step from cellsDoorOutside toward the
	// sergeant, east of him: 90 degrees off the south he would face unstated,
	// so only his stated facing covers it.
	beforeTheCellsDoor = spatial.Position{X: -23, Y: 5}

	// outsideTheGate is the courtyard's cell at the shut gate door, behind the
	// captain (offset [col, row], for MemberInput.Position).
	outsideTheGate = spatial.Position{X: 0, Y: 6}

	// throneDoorInside is the throne room's cell beside its door, on the
	// king's row.
	throneDoorInside = spatial.Position{X: 8, Y: -17}

	// inTheBarracks is barracks floor in sight of the cells door's keepers
	// (offset [col, row], for MemberInput.Position).
	inTheBarracks = spatial.Position{X: -20, Y: 3}

	// atTheThroneDoor is the great hall's cell at the shut throne door
	// (offset [col, row], for MemberInput.Position).
	atTheThroneDoor = spatial.Position{X: 0, Y: -16}
)

// TestMadKingStillWatchersFaceWhatTheyKeep opens the castle with the hero
// sneaking where it sees each stationary watcher, and asks the watcher's red
// floor for the cell in front of what it keeps.
//
// THE KING IS HOSTILE, so the hero cannot simply stand beside him: it
// declares in the hall at the shut throne door and opens it, winning every
// roll the opening asks, as the sneaking walk did — and from there it sees
// him with no fight formed.
func TestMadKingStillWatchersFaceWhatTheyKeep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		watcher encounter.MemberID
		hero    spatial.Position
		keeps   spatial.Position
		what    string
		enter   bool
	}{
		{kingCook, besideTheCook, kitchenDoorInside, "the kitchen door", false},
		{kingScullion, besideTheCook, kitchenDoorInside, "the kitchen door", false},
		{kingTurnkey, inTheBarracks, cellsDoorOutside, "the cells door", false},
		{kingSergeant, inTheBarracks, beforeTheCellsDoor, "the cells door", false},
		{theKing, atTheThroneDoor, throneDoorInside, "the throne door", true},
	}
	for _, c := range cases {
		t.Run(string(c.watcher), func(t *testing.T) {
			t.Parallel()
			stealth := &partyStealth{
				modifier: map[encounter.MemberID]int{hero: 7},
				d20s:     map[encounter.MemberID][]int{hero: {15, 15, 15, 15, 15}},
			}
			s := openTheCastleAs(t, castleHost{
				at: c.hero, resolver: stealth, arcs: map[encounter.MemberID]int{c.watcher: 120},
			})
			s.declare(hero)
			if c.enter {
				s.open(kingThroneDoor)
				require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
			}

			var zone map[spatial.Position]bool
			for _, z := range zonesOf(t, s.enc, hero) {
				if z.Watcher == c.watcher {
					zone = cellSet(z.Cells)
				}
			}
			require.NotNil(t, zone, "the hero, sneaking in sight of %s, is sent its zone", c.watcher)
			require.True(t, zone[c.keeps], "%s's zone holds the cell in front of %s", c.watcher, c.what)
		})
	}
}

// TestMadKingTheCaptainWatchesWhoComesToTheGate is GETTING BEHIND THE CONE
// (rpg-project#623): the captain faces down the gatehouse at whoever comes to
// the gate, arc 120 scripted. A hero who sneaks in through the gate door
// stands behind him, in his sight and out of his zone, and nothing rolls;
// circling round to his front rolls against him.
func TestMadKingTheCaptainWatchesWhoComesToTheGate(t *testing.T) {
	const captainPassive = 13
	t.Parallel()
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: {15, 15, 15, 15, 15, 15, 15, 15}},
	}
	s := openTheCastleAs(t, castleHost{
		at: outsideTheGate, resolver: stealth, arcs: map[encounter.MemberID]int{kingCaptain: 120},
		passive: map[encounter.MemberID]int{kingCaptain: captainPassive},
	})
	seer := &coneSite{sneakSite: &sneakSite{t: t, enc: s.enc}}
	s.declare(hero)
	s.open(kingGateDoor)

	// rolled reports whether the hero's Stealth was rolled against the
	// captain: his passive is the DC only he answers. The gate guards, who
	// watch all round, roll at the host's 10 as the hero comes through.
	rolled := func() bool {
		for _, in := range stealth.asked {
			if in.Member == hero && in.Approaches[0].DC == captainPassive {
				return true
			}
		}
		return false
	}
	captainZone := func() map[spatial.Position]bool {
		for _, z := range zonesOf(t, s.enc, hero) {
			if z.Watcher == kingCaptain {
				return cellSet(z.Cells)
			}
		}
		return nil
	}
	step := func(to spatial.Position, endWalk bool) {
		t.Helper()
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to, EndWalk: endWalk})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
	}

	// BEHIND HIM: through the gate door to the gatehouse cell beneath it,
	// north of the captain, in his sight.
	step(spatial.Position{X: -3, Y: 7}, true)
	behind := s.at(hero)
	captain := positionOf(t, s.enc, kingCaptain)
	require.Greater(t, offArc(facingOf(t, s.enc, kingCaptain), coneBearing(captain, behind)), 60.0,
		"the cell beneath the gate is behind the captain")
	require.True(t, seer.seenNow(kingCaptain)[behind], "the captain's sight reaches the hero behind him")
	require.False(t, rolled(), "behind him nothing rolls against the captain")
	require.False(t, seer.seesLive(kingCaptain, hero), "the captain does not see the hero behind him")
	require.False(t, captainZone()[behind], "the captain's zone does not hold the cell behind him")

	// ROUND TO HIS FRONT: east of him, then down the gatehouse to his south,
	// a step at a time, until the roll fires.
	round := []spatial.Position{{X: -3, Y: 8}, {X: -4, Y: 9}, {X: -4, Y: 10}, {X: -4, Y: 11}}
	for i, cell := range round {
		step(cell, i == len(round)-1)
		if rolled() {
			break
		}
	}
	require.True(t, rolled(), "in front of him the roll fires against the captain")
	require.LessOrEqual(t, offArc(facingOf(t, s.enc, kingCaptain), coneBearing(captain, s.at(hero))), 60.0,
		"the roll fired with the hero inside his front third")
	require.True(t, captainZone()[s.at(hero)], "the captain's zone holds the cell in front of him")
	verdict := s.enc.ToData().SneakVerdicts[hero][kingCaptain]
	require.True(t, verdict.Beaten, "a 15 and +7 beat his passive")
}
