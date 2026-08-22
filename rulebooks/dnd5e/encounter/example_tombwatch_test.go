// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// tell prints one member's belief about one subject as narration.
func tell(enc *encounter.Encounter, who, about core.EntityID) {
	view, err := enc.View(&encounter.ViewInput{Member: who})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, h := range view {
		if h.Subject != intel.Subject(about) {
			continue
		}
		var p encounter.SightPayload
		_ = json.Unmarshal(h.Payload, &p)
		if h.Status == intel.Current {
			fmt.Printf("%s sees %s at (%g,%g)\n", who, about, p.X, p.Y)
		} else {
			fmt.Printf("%s holds a GHOST of %s at last-seen (%g,%g)\n", who, about, p.X, p.Y)
		}
		return
	}
	fmt.Printf("%s knows nothing of %s\n", who, about)
}

// Example_theTombWatch plays the free-roam encounter's signature scene
// and PRINTS it — the Output block below is verified by go test, so
// this narration can never drift from what the composition actually
// does. This is the pre-UI loop: the story is the screen.
func Example_theTombWatch() {
	// The crypt: a wall across y=6 from x=5 to x=7; alice enters at (2,6); a goblin
	// stands at (6,10). One way out: the stairs at (11,11).
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{{
				ID: "crypt", Width: 12, Height: 12,
				Props: wallRow(6, 5, 7),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "crypt", Position: spatial.Position{X: 2, Y: 6}},
			{ID: "goblin", Kind: encounter.KindMonster, Room: "crypt", Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Room: "crypt", Position: spatial.Position{X: 11, Y: 11}}},
		},
	})
	if err != nil {
		fmt.Println("setup:", err)
		return
	}

	fmt.Println("-- first light --")
	tell(enc, "alice", "goblin")
	tell(enc, "goblin", "alice")

	// Seeing each other starts a fight (rpg-toolkit#964); the party breaks
	// off, which is what lets alice slip away rather than trade blows.
	if _, err := enc.Dissolve(&encounter.DissolveInput{Member: "alice"}); err != nil {
		fmt.Println("dissolve:", err)
		return
	}

	fmt.Println("-- alice slips behind the wall --")
	if _, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 6, Y: 2}}); err != nil {
		fmt.Println("move:", err)
		return
	}
	tell(enc, "alice", "goblin")
	tell(enc, "goblin", "alice")

	fmt.Println("-- the table saves and comes back tomorrow --")
	data := enc.ToData()
	enc, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data})
	if err != nil {
		fmt.Println("load:", err)
		return
	}
	fmt.Println("one aggregate blob stored and reloaded")
	tell(enc, "alice", "goblin")

	fmt.Println("-- alice finds the stairs --")
	out, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 11, Y: 11}})
	if err != nil {
		fmt.Println("move:", err)
		return
	}
	fmt.Printf("the encounter closes: %q\n", out.Outcome.Ending)
	for _, m := range out.Outcome.Members {
		fmt.Printf("  %s ends at (%g,%g)\n", m.ID, m.Position.X, m.Position.Y)
	}

	// Output:
	// -- first light --
	// alice sees goblin at (6,10)
	// goblin sees alice at (2,6)
	// -- alice slips behind the wall --
	// alice holds a GHOST of goblin at last-seen (6,10)
	// goblin holds a GHOST of alice at last-seen (2,6)
	// -- the table saves and comes back tomorrow --
	// one aggregate blob stored and reloaded
	// alice holds a GHOST of goblin at last-seen (6,10)
	// -- alice finds the stairs --
	// the encounter closes: "stairs"
	//   alice ends at (11,11)
	//   goblin ends at (6,10)
}
