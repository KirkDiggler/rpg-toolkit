// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_cells_test.go is THE FOLD, PLAYED (rpg-project#611): the cells
// and the kitchen's trigger, which were their own entries, as rooms of the
// one castle. The cell lock picked ends the run `freed` with the cells'
// guards standing; the captain's orders carried into his region turn the
// garrison, as castle-kitchen's watch turned.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// kingCellsDoor is the cell block's one door, in the barracks' south wall.
// The file locks it (thieves' tools, DC 13) and shuts it.
const kingCellsDoor = encounter.DoorID("the-mad-king/cells-wall-door")

// The cells' guards, by the ids the file gives them.
const (
	kingSergeant = encounter.MemberID("sergeant")
	kingTurnkey  = encounter.MemberID("turnkey")
)

// kingOrders is the captain's orders, the holdable prop on the kitchen floor.
const kingOrders = encounter.PropID("orders")

// kingCellPick is the cell door's one authored route, as the file writes it.
var kingCellPick = encounter.CheckApproach{Ability: "dex", Tool: "dnd5e:item:thieves-tools", DC: 13}

// TestMadKingPickingTheCellLockEndsTheRunFreed is the cells' route in the
// castle: the hero comes in at the party start, opens the gate, crosses the
// yard into the barracks and walks toward the cells, which leaves the hero
// beside the shut cell door. A pick of 15 against DC 13 opens it, the door
// teaches `cells-opened`, and the run ends `freed` with both of the cells'
// guards still in it: the lock was picked, not fought through.
func TestMadKingPickingTheCellLockEndsTheRunFreed(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))
	cells, ok := s.enc.RegionAt(castleCell(-20, 10))
	require.True(t, ok, "the cell block's floor")

	s.walkTo(castleCell(0, 6))
	s.open(kingGateDoor)
	// Toward the cell floor under the shut door: the walk ends in the
	// barracks, beside it.
	s.walkTo(castleCell(-20, 7))
	here, _ := s.enc.RegionAt(s.at(hero))
	require.NotEqual(t, cells, here, "precondition: the locked door keeps the hero out of the cells")
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: nobody fought on the way")
	require.Empty(t, s.ending())

	_, err := s.enc.Unlock(&encounter.UnlockInput{
		Door: kingCellsDoor, Actor: hero, Beaten: true, Total: 15, Applied: kingCellPick,
	})
	require.NoError(t, err, "the hero stands within reach of the cell door")

	require.Equal(t, "freed", s.ending())
	require.True(t, s.learners("cells-opened")[hero], "the hero who opened it learned it")
	for _, id := range []encounter.MemberID{kingSergeant, kingTurnkey} {
		require.Equal(t, garrison, s.factionOf(id), "%s is the garrison's", id)
		require.Contains(t, memberIDsOf(t, s.enc), id, "%s is still in the run", id)
		require.False(t, s.standing.down[id], "and standing")
	}
}

// TestMadKingTheOrdersTurnTheGarrison is castle-kitchen's trigger in the
// castle: the hero picks the captain's orders up off the kitchen floor and
// carries them out through the hall and the gate. The orders reveal
// `party-recognized`, so the moment their holder stands in the captain's
// region the captain, the garrison's mind, knows it (presence transfer) and
// the garrison's `until` turns it on the party.
func TestMadKingTheOrdersTurnTheGarrison(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)

	s.walkTo(castleCell(-24, -12))
	_, err := s.enc.Hold(&encounter.HoldInput{Member: hero, Target: kingOrders})
	require.NoError(t, err, "the hero stands beside the orders")

	s.outOfTheKitchen()
	s.walkTo(castleCell(0, 6))
	s.open(kingGateDoor)

	t.Run("control: carried as far as the open gate, the orders turn nothing", func(t *testing.T) {
		gatehouse, _ := s.enc.RegionAt(s.at(kingCaptain))
		here, _ := s.enc.RegionAt(s.at(hero))
		require.NotEqual(t, gatehouse, here, "precondition: the hero is not in the captain's region")
		require.False(t, s.learners("party-recognized")[kingCaptain])
		require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty))
	})

	s.walkTo(s.at(kingCaptain))

	t.Run("in the captain's region, the captain knows the party", func(t *testing.T) {
		require.True(t, s.learners("party-recognized")[kingCaptain])
	})

	t.Run("and the garrison is hostile to the party", func(t *testing.T) {
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty))
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "the walk ended in a fight")
	})
}
