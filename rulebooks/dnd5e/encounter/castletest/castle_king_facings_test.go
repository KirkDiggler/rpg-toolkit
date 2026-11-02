// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

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

	// northOfTheCellsDoor is the barracks floor directly north of the cells
	// door, whose own cell (-23,6) the door stands on. It lies at 90 from the
	// sergeant and 270 from the turnkey: off the south either would face
	// unstated, so only their stated facings cover it.
	northOfTheCellsDoor = spatial.Position{X: -23, Y: 5}

	// outsideTheGate is the courtyard's cell at the shut gate door, behind the
	// captain (offset [col, row], for MemberInput.Position).
	outsideTheGate = spatial.Position{X: 0, Y: 6}

	// throneDoorInside is the throne room's cell beside its door, on the
	// king's row.
	throneDoorInside = spatial.Position{X: 8, Y: -17}

	// inTheBarracks is barracks floor in sight of the cells door's keepers
	// (offset [col, row], for MemberInput.Position).
	inTheBarracks = spatial.Position{X: -20, Y: 3}
)

// inTheCourtyard is courtyard floor out of every watcher's sight, east of
// the barracks (offset [col, row], for MemberInput.Position): where a hero
// declares before walking into the barracks.
var inTheCourtyard = spatial.Position{X: -6, Y: 3}

// TestMadKingStillWatchersFaceWhatTheyKeep brings the hero, sneaking, to
// where it sees each stationary watcher, and asks the watcher's red floor
// for the cell in front of what it keeps.
//
// NOBODY BEGINS TO SNEAK IN A WATCHER'S SIGHT (sneaking R10 as amended by
// R11, rpg-project#768), so the hero declares where nobody sees it and walks
// in, winning every roll the walk asks: into the kitchen from the hall's
// corner, into the barracks from the courtyard, and to the king through the
// hall and the shut throne door, which it opens — from there it sees him
// with no fight formed (THE KING IS HOSTILE).
func TestMadKingStillWatchersFaceWhatTheyKeep(t *testing.T) {
	t.Parallel()
	// intoTheKitchen picks the kitchen door, declares in the hall's corner
	// and walks back to the hero's start beside the cook.
	intoTheKitchen := func(s *kingScene) {
		start := s.at(hero)
		s.walkTo(pastTheKitchenDoor)
		s.pickTheKitchenDoor()
		s.declareInTheHallCorner(hero)
		s.walkTo(start)
	}
	// intoTheBarracks declares in the courtyard and walks to the barracks
	// floor in sight of the cells door's keepers.
	intoTheBarracks := func(s *kingScene) {
		s.declare(hero)
		s.walkTo(castleCell(int(inTheBarracks.X), int(inTheBarracks.Y)))
	}
	// toTheKing declares in the hall's corner, crosses the hall to the shut
	// throne door and opens it.
	toTheKing := func(s *kingScene) {
		s.walkTo(pastTheKitchenDoor)
		s.pickTheKitchenDoor()
		s.declareInTheHallCorner(hero)
		s.walkTo(pastTheThroneDoor)
		s.open(kingThroneDoor)
	}
	cases := []struct {
		watcher encounter.MemberID
		hero    spatial.Position
		keeps   spatial.Position
		what    string
		reach   func(s *kingScene)
	}{
		{kingCook, besideTheCook, kitchenDoorInside, "the kitchen door", intoTheKitchen},
		{kingScullion, besideTheCook, kitchenDoorInside, "the kitchen door", intoTheKitchen},
		{kingTurnkey, inTheCourtyard, northOfTheCellsDoor, "the cells door", intoTheBarracks},
		{kingSergeant, inTheCourtyard, northOfTheCellsDoor, "the cells door", intoTheBarracks},
		{theKing, besideTheCook, throneDoorInside, "the throne door", toTheKing},
	}
	for _, c := range cases {
		t.Run(string(c.watcher), func(t *testing.T) {
			t.Parallel()
			stealth := &partyStealth{
				modifier: map[encounter.MemberID]int{hero: 7},
				d20s:     map[encounter.MemberID][]int{hero: make([]int, 20)},
			}
			for i := range stealth.d20s[hero] {
				stealth.d20s[hero][i] = 15
			}
			s := openTheCastleAs(t, castleHost{
				at: c.hero, resolver: stealth, arcs: map[encounter.MemberID]int{c.watcher: 120},
			})
			c.reach(s)
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
			require.True(t, s.enc.Sneaking(hero), "the hero is still sneaking")

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
	require.True(t, s.seenNow(kingCaptain)[behind], "the captain's sight reaches the hero behind him")
	require.False(t, rolled(), "behind him nothing rolls against the captain")
	require.False(t, s.seesLive(kingCaptain, hero), "the captain does not see the hero behind him")
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
