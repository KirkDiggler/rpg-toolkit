// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Example_theDungeon is what a HOST renders — sibling to Example_theDoorway,
// which shows the same doorway through a MEMBER's percepts. The Output block
// below is verified by go test, so this narration can never drift from what
// the composition actually reports.
//
// It used to be the anchoring wave's signature scene (#929 T5), and its whole
// subject was a bridge: a host held a room and a room-local cell, and called
// Absolute to turn the pair into a coordinate it could draw. That bridge is
// gone (rpg-toolkit#1106). Regions are how the dungeon is AUTHORED — named
// sets of [col,row] cells (rpg-project#256) — and they are compiled into one
// canvas at construction, so by the time a host asks anything, every answer
// is already a cell on one map. The Atlas reports the floor; the roster
// reports where people stand; nothing needs projecting, because nothing was
// ever projected apart.
func Example_theDungeon() {
	gate := openDoorway("gate", 7, 4, 8, 4)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8), rectRegion("vault", 8, 0, 8, 8)},
			Doors:   []encounter.DoorInput{gate},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 7, Y: 4}},
		},
		Endings: []encounter.EndingInput{
			{Key: "done", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		fmt.Println("setup:", err)
		return
	}

	// The static map, as construction truth: every region with the cells it
	// owns, and where the doorway between them sits (rpg-project#256). Cells
	// are absolute axial — the authored [col,row] pairs, converted once.
	atlas, err := enc.Atlas()
	if err != nil {
		fmt.Println("atlas:", err)
		return
	}
	fmt.Println("-- the map --")
	for _, region := range atlas.Regions {
		fmt.Printf("%s: %d cells, %s, lit %g, first cell (%g,%g)\n",
			region.ID, len(region.Cells), region.Archetype, region.Lighting.Intensity,
			region.Cells[0].X, region.Cells[0].Y)
	}
	for _, d := range atlas.Doorways {
		fmt.Printf("doorway %s: (%g,%g) -- (%g,%g), one cell apart\n",
			d.Door, d.From.X, d.From.Y, d.To.X, d.To.Y)
	}

	// tell is what a host does with a member: read the roster and draw. No
	// projection, no room-and-cell pair to reassemble.
	tell := func() {
		members, merr := enc.Members()
		if merr != nil {
			fmt.Println("members:", merr)
			return
		}
		for _, m := range members {
			fmt.Printf("alice stands at (%g,%g), in the %s\n", m.Position.X, m.Position.Y, m.Region)
		}
	}

	fmt.Println("-- alice, at the threshold --")
	tell()

	fmt.Println("-- she steps through the gate --")
	if _, err := enc.Step(&encounter.StepInput{Member: "alice", To: cellAt(8, 4)}); err != nil {
		fmt.Println("step:", err)
		return
	}
	tell()

	// Output:
	// -- the map --
	// hall: 64 cells, crypt, lit 1, first cell (-3,6)
	// vault: 64 cells, crypt, lit 1, first cell (5,6)
	// doorway gate: (5,4) -- (6,4), one cell apart
	// -- alice, at the threshold --
	// alice stands at (5,4), in the hall
	// -- she steps through the gate --
	// alice stands at (6,4), in the vault
}
