// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Example_theDoorway plays the canvas wave's signature scene — a member
// crossing a threshold, a wall doing the work a room boundary used to, and a
// monster's pursuit following through the same opening — and PRINTS it. The
// Output block below is verified by go test, so this narration can never drift
// from what the composition actually does. Sibling to Example_theTombWatch
// (sight, the ghost, save/reload, the ending); this one shows the map itself.
//
// It used to be Example_theTraverse, and its point was the opposite of this
// one: sight stopped at a room boundary, so alice crossing the gate vanished
// from the goblin's eyes the instant she stepped through it, however wide open
// the gate was. That was never a rule anybody wrote down — it was the room
// label standing in for walls the composition could not express
// (rpg-toolkit#1105/#1106).
//
// So the hall WALLS ITS OWN EDGE here, leaving one cell open, and what happens
// is what a player would expect: she is visible while she is in the opening,
// and she disappears when she steps out of its line — not when she crosses an
// invisible boundary between two chambers.
//
// Every position printed here is DUNGEON-ABSOLUTE (rpg-toolkit#1044): the
// vault is anchored at (8,0), so its own (0,4) reads as (8,4) on the map.
func Example_theDoorway() {
	gate := encounter.ConnectionInput{
		ID: "gate", From: "hall", To: "vault",
		FromPosition: spatial.Position{X: 7, Y: 4},
		ToPosition:   spatial.Position{X: 0, Y: 4},
	}

	// The hall's east wall: an edge between its own last column and the first
	// column of the vault, every row but the gate's. Both endpoints are
	// absolute cells on the canvas, which is the only reason this can be said
	// at all — registered on a room's own grid, the far endpoint is not a cell
	// of that room and spatial refuses it.
	wall := squareSeamWall(7, 8, 4)

	field := encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
		Rooms: []encounter.RoomInput{
			{ID: "hall", Width: 8, Height: 8, Boundaries: wall},
			// Anchored immediately east of the hall (#929 T1): the gate's
			// endpoints (7,4) and (0,4)+(8,0)=(8,4) are Chebyshev-adjacent
			// (W3), and the rooms' absolute footprints (x:[0,7] vs x:[8,15])
			// stay disjoint (W2).
			{ID: "vault", Width: 8, Height: 8, Origin: spatial.Position{X: 8, Y: 0}},
		},
		Connections: []encounter.ConnectionInput{gate},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 7, Y: 4}},
			{ID: "goblin", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 2, Y: 4},
				Decider: &pursuitDecider{doorways: doorwaysFrom(field), target: "alice"}},
		},
		Endings: []encounter.EndingInput{
			{Key: "done", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		fmt.Println("setup:", err)
		return
	}

	fmt.Println("-- first light --")
	tell(enc, "alice", "goblin")
	tell(enc, "goblin", "alice")

	// Seeing each other starts a fight (rpg-toolkit#964); alice breaks off
	// before she runs, which is what makes the rest of this a chase.
	if _, err := enc.Dissolve(&encounter.DissolveInput{Member: "alice"}); err != nil {
		fmt.Println("dissolve:", err)
		return
	}

	// One step, to the cell on the other side. Nothing about it is a crossing
	// except the doorway's name on the way out.
	fmt.Println("-- alice steps through the gate, and is still standing in it --")
	out, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 8, Y: 4}})
	if err != nil {
		fmt.Println("step:", err)
		return
	}
	fmt.Println("she went through:", out.Crossing)
	tell(enc, "goblin", "alice")

	fmt.Println("-- she moves out of the opening, and the wall takes her --")
	if _, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 10, Y: 7}}); err != nil {
		fmt.Println("step:", err)
		return
	}
	tell(enc, "goblin", "alice")

	// The goblin closes on the ghost, then reaches the doorway and looks
	// through it — decided from nothing but its own snapshot and what it holds.
	fmt.Println("-- the goblin gives chase, and follows her through --")
	for i := 0; i < 4; i++ {
		if _, err := enc.Pump(&encounter.PumpInput{}); err != nil {
			fmt.Println("pump:", err)
			return
		}
	}
	tell(enc, "goblin", "alice")

	// Output:
	// -- first light --
	// alice sees goblin at (2,4)
	// goblin sees alice at (7,4)
	// -- alice steps through the gate, and is still standing in it --
	// she went through: gate
	// goblin sees alice at (8,4)
	// -- she moves out of the opening, and the wall takes her --
	// goblin holds a GHOST of alice at last-seen (8,4)
	// -- the goblin gives chase, and follows her through --
	// goblin sees alice at (10,7)
}
