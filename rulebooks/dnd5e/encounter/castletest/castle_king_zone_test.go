// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

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
// the cell it stands on: the entry rolled and the hero won it, every zone
// sent is a watcher the hero sees, and the hall guard's holds the hero's
// cell. Declared off, the hero is sent none.
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

	// THE ENTRY ROLLED, AND THE HERO WON IT. A beaten watcher keeps its zone
	// (R10, R23): the won roll hides the hero from the hall guard, never the
	// floor he watches, so the zone is still sent while the hero stands in it.
	rolledHall := false
	for _, in := range stealth.asked {
		rolledHall = rolledHall || (in.Member == hero && in.Approaches[0].DC == 10)
	}
	require.True(t, rolledHall, "entering the hall guard's zone rolled the hero's Stealth")
	verdict, judged := s.enc.ToData().SneakVerdicts[hero][kingHall1]
	require.True(t, judged, "the hero holds a verdict against the hall guard")
	require.True(t, verdict.Beaten, "a 15 and +7 beat the hall guard's passive 10")

	// EVERY ZONE IS A WATCHER THE HERO SEES, and the hall guard's holds the
	// hero's own cell.
	var hall []encounter.WatchZone
	for _, zone := range zones {
		h := s.holding(hero, zone.Watcher)
		require.NotNil(t, h, "the hero holds %s", zone.Watcher)
		require.NotEmpty(t, h.CurrentVia, "the hero sees %s now", zone.Watcher)
		if zone.Watcher == kingHall1 {
			hall = append(hall, zone)
		}
	}
	require.NotEmpty(t, hall, "the hall guard's zone is sent")
	require.True(t, cellSet(hall[0].Cells)[s.at(hero)], "the hero's own cell is in the hall guard's zone")

	_, err = s.enc.SetSneaking(&encounter.SetSneakingInput{Member: hero, Sneaking: false})
	require.NoError(t, err)
	require.Empty(t, zonesOf(t, s.enc, hero), "not sneaking: no red floor")
}
