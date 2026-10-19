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

	// gateDoorInside is the gatehouse's cell beneath the gate door, the one
	// of its two nearest the captain's post.
	gateDoorInside = spatial.Position{X: -4, Y: 7}

	// cellsDoorOutside is the barracks' cell nearest the cells door.
	cellsDoorOutside = spatial.Position{X: -23, Y: 6}

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
		{kingCaptain, partyStart(t), gateDoorInside, "the gate", false},
		{kingTurnkey, inTheBarracks, cellsDoorOutside, "the cells door", false},
		{kingSergeant, inTheBarracks, cellsDoorOutside, "the cells door", false},
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
