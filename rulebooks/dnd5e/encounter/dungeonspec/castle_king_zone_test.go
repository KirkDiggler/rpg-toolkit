// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_zone_test.go is THE RED FLOOR IN THE GREAT HALL (rpg-project#619,
// sneaking Z1, design R15): the rogue sneaks out of the kitchen into the hall
// and, once it stands inside the hall guard's sight, is sent his zone — his
// alone — with its own cell in it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestMadKingTheHallGuardsRedFloor walks the sneaking hero from the kitchen
// toward the throne door, a step at a time, until the hall guard's zone holds
// the cell it stands on. There his zone is the only one the hero is sent;
// declared off, the hero is sent none.
//
// THE KITCHEN IS RED TOO. The cook and the scullion are watchers (R9: any
// placed member that is not a player), so in the kitchen the hero is sent
// their zones; the walk stops on the hall guard's, not on the first zone.
func TestMadKingTheHallGuardsRedFloor(t *testing.T) {
	t.Parallel()
	stealth := &partyStealth{
		modifier: map[encounter.MemberID]int{hero: 7},
		d20s:     map[encounter.MemberID][]int{hero: {15, 15}},
	}
	s := openTheCastleAs(t, castleHost{at: besideTheCook, resolver: stealth})
	s.declare(hero)
	s.walkTo(pastTheKitchenDoor)
	s.pickTheKitchenDoor()

	route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: pastTheThroneDoor, Budget: 200})
	require.NoError(t, err)
	require.Empty(t, route.StoppedBy, "the hero has a way across the hall")
	var zones []encounter.WatchZone
	inZone := false
	for i, cell := range route.Path {
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: i == len(route.Path)-1})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "no fight formed")
		zones = zonesOf(t, s.enc, hero)
		for _, zone := range zones {
			inZone = inZone || (zone.Watcher == kingHall1 && cellSet(zone.Cells)[s.at(hero)])
		}
		if inZone {
			break
		}
	}
	require.True(t, inZone, "the hero's walk across the hall stands inside the hall guard's zone")
	require.Len(t, zones, 1, "one zone: the hall guard's")
	require.Equal(t, kingHall1, zones[0].Watcher)
	require.True(t, cellSet(zones[0].Cells)[s.at(hero)], "the hero's own cell is in the hall guard's zone")

	_, err = s.enc.SetSneaking(&encounter.SetSneakingInput{Member: hero, Sneaking: false})
	require.NoError(t, err)
	require.Empty(t, zonesOf(t, s.enc, hero), "not sneaking: no red floor")
}
