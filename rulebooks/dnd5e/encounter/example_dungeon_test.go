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
// gone (rpg-toolkit#1106). Rooms are how the dungeon is AUTHORED, and they are
// compiled into one canvas at construction — so by the time a host asks
// anything, every answer is already a cell on one map. The Atlas reports where
// the authored chambers landed; the roster reports where people stand; nothing
// needs projecting, because nothing was ever projected apart.
func Example_theDungeon() {
	gate := encounter.ConnectionInput{
		ID: "gate", From: "hall", To: "vault",
		FromPosition: spatial.Position{X: 7, Y: 4},
		ToPosition:   spatial.Position{X: 0, Y: 4},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hall", Width: 8, Height: 8},
				// Anchored immediately east of the hall (#929 T1): the
				// gate's endpoints (7,4) and (0,4)+(8,0)=(8,4) are
				// Chebyshev-adjacent (W3), and the rooms' absolute
				// footprints (x:[0,7] vs x:[8,15]) stay disjoint (W2).
				{ID: "vault", Width: 8, Height: 8, Origin: spatial.Position{X: 8, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{gate},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 7, Y: 4}},
		},
		Endings: []encounter.EndingInput{
			{Key: "done", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		fmt.Println("setup:", err)
		return
	}

	// The static map, as construction truth: where each authored chamber landed
	// as a region, and where the doorway between them sits. A region says what
	// it IS — anchor and span — rather than listing its cells; asking which
	// region holds a particular cell is a question, not a list
	// (rpg-toolkit#1108).
	atlas, err := enc.Atlas()
	if err != nil {
		fmt.Println("atlas:", err)
		return
	}
	fmt.Println("-- the map --")
	for _, region := range atlas.Regions {
		fmt.Printf("%s: %dx%d, anchored at (%g,%g)\n",
			region.ID, region.Width, region.Height, region.Origin.X, region.Origin.Y)
	}
	for _, d := range atlas.Doorways {
		fmt.Printf("doorway %s: (%g,%g) -- (%g,%g), one cell apart\n",
			d.Connection, d.FromCell.X, d.FromCell.Y, d.ToCell.X, d.ToCell.Y)
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
	if _, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 8, Y: 4}}); err != nil {
		fmt.Println("step:", err)
		return
	}
	tell()

	// Output:
	// -- the map --
	// hall: 8x8, anchored at (0,0)
	// vault: 8x8, anchored at (8,0)
	// doorway gate: (7,4) -- (8,4), one cell apart
	// -- alice, at the threshold --
	// alice stands at (7,4), in the hall
	// -- she steps through the gate --
	// alice stands at (8,4), in the vault
}
