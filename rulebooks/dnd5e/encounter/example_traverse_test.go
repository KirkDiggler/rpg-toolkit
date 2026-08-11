// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Example_theTraverse plays the connections wave's signature scene — a
// member crossing a threshold into another room, and a monster's own
// pursuit following through the same doorway — and PRINTS it. The
// Output block below is verified by go test, so this narration can
// never drift from what the composition actually does. Sibling to
// Example_theTombWatch (sight, the ghost, save/reload, the ending);
// this one shows what tomb watch predates: the room boundary itself.
// Reuses tell() from example_tombwatch_test.go (same package).
func Example_theTraverse() {
	// Two rooms, one gate: alice starts exactly at the hall's threshold;
	// the goblin, across the hall, can see her but not yet follow.
	gate := encounter.ConnectionInput{
		ID: "gate", From: "hall", To: "vault",
		FromPosition: spatial.Position{X: 7, Y: 4},
		ToPosition:   spatial.Position{X: 0, Y: 4},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hall", Width: 8, Height: 8},
				{ID: "vault", Width: 8, Height: 8},
			},
			Connections: []encounter.ConnectionInput{gate},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 7, Y: 4}},
			{ID: "goblin", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 2, Y: 4},
				Decider: &pursuitDecider{connections: []encounter.ConnectionInput{gate}, target: "alice"}},
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

	fmt.Println("-- alice slips through the gate --")
	if _, err := enc.Traverse(&encounter.TraverseInput{Member: "alice", Connection: "gate"}); err != nil {
		fmt.Println("traverse:", err)
		return
	}
	tell(enc, "goblin", "alice")

	// Two ticks: the goblin closes the distance to the threshold, then
	// crosses it — both decided from nothing but its own snapshot and
	// the ghost it is holding. Sight stays room-scoped the whole way
	// (T3): the goblin learns nothing new until it is standing in the
	// vault itself.
	fmt.Println("-- the goblin gives chase, and follows her through --")
	if _, err := enc.Pump(&encounter.PumpInput{}); err != nil {
		fmt.Println("pump:", err)
		return
	}
	if _, err := enc.Pump(&encounter.PumpInput{}); err != nil {
		fmt.Println("pump:", err)
		return
	}
	tell(enc, "goblin", "alice")

	// Output:
	// -- first light --
	// alice sees goblin at (2,4)
	// goblin sees alice at (7,4)
	// -- alice slips through the gate --
	// goblin holds a GHOST of alice at last-seen (7,4)
	// -- the goblin gives chase, and follows her through --
	// goblin sees alice at (0,4)
}
