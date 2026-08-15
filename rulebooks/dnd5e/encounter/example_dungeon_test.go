// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Example_theDungeon is the anchoring wave's signature scene (#929 T5) —
// sibling to Example_theTraverse, which shows the SAME kind of doorway
// crossing through percepts (what a MEMBER believes). This one shows what
// a HOST renders: every position projected into dungeon-absolute space
// via Absolute, so the room boundary that Example_theTraverse's story
// crosses becomes visible as exactly what it is — one continuous space,
// not two local coordinate systems glued together. The Output block
// below is verified by go test, so this narration can never drift from
// what Absolute actually returns.
func Example_theDungeon() {
	gate := encounter.ConnectionInput{
		ID: "gate", From: "hall", To: "vault",
		FromPosition: spatial.Position{X: 7, Y: 4},
		ToPosition:   spatial.Position{X: 0, Y: 4},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
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

	// tellAbsolute is what a host does with a member's room-local
	// position before handing it to a renderer: project it, once,
	// through the bridge — never re-derive world coordinates by hand.
	tellAbsolute := func(room string, pos spatial.Position) {
		out, err := enc.Absolute(&encounter.AbsoluteInput{Room: room, Position: pos})
		if err != nil {
			fmt.Println("absolute:", err)
			return
		}
		fmt.Printf("alice: %s local(%g,%g) -- dungeon-absolute(%g,%g)\n", room, pos.X, pos.Y, out.Position.X, out.Position.Y)
	}

	fmt.Println("-- alice, at the threshold --")
	tellAbsolute("hall", spatial.Position{X: 7, Y: 4})

	fmt.Println("-- she crosses the gate --")
	travOut, err := enc.Traverse(&encounter.TraverseInput{Member: "alice", Connection: "gate"})
	if err != nil {
		fmt.Println("traverse:", err)
		return
	}
	tellAbsolute(travOut.Traversed.ToRoom, travOut.Traversed.To)

	// Output:
	// -- alice, at the threshold --
	// alice: hall local(7,4) -- dungeon-absolute(7,4)
	// -- she crosses the gate --
	// alice: vault local(0,4) -- dungeon-absolute(8,4)
}
