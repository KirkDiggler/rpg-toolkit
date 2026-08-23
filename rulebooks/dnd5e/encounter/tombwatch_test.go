// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Scene cast and set (alice and endingStairs come from encounter_test.go).
const (
	bella          = core.EntityID("bella")
	cormac         = core.EntityID("cormac")
	cryptRoom      = "crypt"
	withdrawEnding = "withdrew"
)

// seen decodes a member's holding of a subject, requiring it to exist.
func seen(t *testing.T, enc *encounter.Encounter, observer core.EntityID, subject core.EntityID) (intel.Status, encounter.SightPayload) {
	t.Helper()
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(t, err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			var p encounter.SightPayload
			require.NoError(t, json.Unmarshal(h.Payload, &p))
			return h.Status, p
		}
	}
	t.Fatalf("%s holds nothing on %s", observer, subject)
	return "", encounter.SightPayload{}
}

// TestTombWatch is AC1: ONE continuous scene from first light to the
// sequel seed. It is deliberately a single flowing story — the ghost
// formed mid-scene must survive the mid-scene reload, the story must
// accumulate into a true transcript, and the outcome must seed the
// campaign's sequel. This test doubles as the composition's usage
// documentation: a host wires the free-roam encounter exactly like this.
func TestTombWatch(t *testing.T) {
	// ---- Beat 1: the crypt lights up -------------------------------
	// A 12x12 crypt with a wall across the middle, x=5..7 at y=6. Alice
	// (2,2) and Bella (3,2) enter; a goblin patrols the far end between
	// (7,10) and (6,10). The wall is three cells wide rather than the one
	// this scene used to have: spatial v0.9.1 leans around a lone pillar
	// (see testwalls_test.go), and this crypt needs a sightline that really
	// cannot get through.
	goblinPatrol := &patrolDecider{positions: []spatial.Position{cellAt(7, 10), cellAt(6, 10)}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(cryptRoom, 0, 0, 12, 12)}, Props: wallRow(6, 5, 7),
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: bella, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}, Decider: goblinPatrol},
		},
		Endings: []encounter.EndingInput{
			{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 11, Y: 11}}},
			{Key: withdrawEnding, Trigger: encounter.TriggerExternal{}},
		},
	})
	require.NoError(t, err, "beat 1: the crypt assembles")

	st, _ := seen(t, enc, alice, goblin)
	require.Equal(t, intel.Current, st, "beat 1: alice sees the goblin across the open crypt")
	st, _ = seen(t, enc, bella, goblin)
	require.Equal(t, intel.Current, st, "beat 1: bella sees it too")
	st, _ = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 1: and the goblin sees them back — intel is symmetric, nobody wall-hacks")

	// Seeing each other started the fight (rpg-toolkit#964). The party breaks
	// off to keep watching rather than trading blows — the watch is the scene.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	require.NoError(t, err, "beat 1: the party breaks off to watch")

	// ---- Beat 2: the watch -----------------------------------------
	// Alice advances to (2,6) to watch; the world moves on her action:
	// the pump ticks and the goblin takes its first patrol step to (7,10).
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 6)})
	require.NoError(t, err, "beat 2: alice advances")
	pumpOut, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 2: her activity pumps the clock")
	require.Equal(t, uint64(1), pumpOut.Tick, "beat 2: the exploration clock ticks once")
	_, p := seen(t, enc, alice, goblin)
	require.Equal(t, cellAt(7, 10), spatial.Position{X: p.X, Y: p.Y}, "beat 2: alice's view tracks the patrol step to (7,10)")

	// A second pump brings the goblin back to (6,10) — directly behind
	// the wall, setting up the ghost.
	_, err = enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 2: the watch continues")
	_, p = seen(t, enc, alice, goblin)
	require.Equal(t, cellAt(6, 10), spatial.Position{X: p.X, Y: p.Y}, "beat 2: the goblin returns to (6,10), still in alice's sight from (2,6)")

	// ---- Beat 3: the ghost forms -----------------------------------
	// Alice slips to (6,2): the wall now sits square on the
	// vertical line between her and the goblin at (6,10). Both lose
	// sight of each other — and both keep a ghost at last-seen.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	require.NoError(t, err, "beat 3: alice slips behind the wall")

	st, p = seen(t, enc, alice, goblin)
	require.Equal(t, intel.Held, st, "beat 3: alice's sight of the goblin fades — the ghost forms")
	require.Equal(t, cellAt(6, 10), spatial.Position{X: p.X, Y: p.Y},
		"beat 3: her ghost holds the goblin at last-seen (6,10)")
	st, p = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 3: the goblin loses her too — symmetric")
	require.Equal(t, cellAt(2, 6), spatial.Position{X: p.X, Y: p.Y}, "beat 3: its ghost of alice is at (2,6) — it never saw her arrive at (6,2)")
	st, _ = seen(t, enc, bella, goblin)
	require.Equal(t, intel.Current, st, "beat 3: bella, off the blocked file, still sees the goblin plainly")

	// ---- Beat 4: the pause (pause is free) -------------------------
	// The table closes the Discord activity. The host persists ONE
	// aggregate and rehydrates it later — same encounter, mid-scene.
	// Deciders are behavior, not state: the campaign re-attaches the
	// goblin's patrol at load (its route position restarts; its INTEL
	// does not — beliefs are state and traveled in the aggregate).
	data := enc.ToData()
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
			goblin: &patrolDecider{positions: []spatial.Position{cellAt(7, 10), cellAt(6, 10)}},
		}})
	require.NoError(t, err, "beat 4: the suspended scene crosses a process boundary")
	enc = enc2 // the reload IS the encounter now

	st, p = seen(t, enc, alice, goblin)
	require.Equal(t, intel.Held, st, "beat 4: the ghost survived the reload")
	require.Equal(t, cellAt(6, 10), spatial.Position{X: p.X, Y: p.Y}, "beat 4: still at last-seen (6,10) — loading never re-derives sight")
	st, _ = seen(t, enc, bella, goblin)
	require.Equal(t, intel.Current, st, "beat 4: bella's live sight survived too")

	// ---- Beat 5: the reinforcement ---------------------------------
	// Cormac connects late — the ambient is always there to join.
	joinOut, err := enc.Join(&encounter.JoinInput{
		Member: cormac,
		Kind:   encounter.KindPlayer,
		Cell:   cellAt(10, 2),
	})
	require.NoError(t, err, "beat 5: cormac joins the delve")
	require.NotZero(t, joinOut.Seq, "beat 5: his arrival is a story beat")
	st, _ = seen(t, enc, cormac, goblin)
	require.Equal(t, intel.Current, st, "beat 5: from (10,2) the wall doesn't block him — first light lands")
	st, _ = seen(t, enc, goblin, cormac)
	require.Equal(t, intel.Current, st, "beat 5: the goblin notices the newcomer")
	require.NotNil(t, joinOut.Formed, "beat 5: noticing each other IS the fight starting")
	require.Greater(t, joinOut.Formed.Seq, joinOut.Seq,
		"beat 5: he arrives, THEN the fight starts — the story never runs backwards")
	st, _ = seen(t, enc, alice, goblin)
	require.Equal(t, intel.Held, st, "beat 5: the join's refresh does not disturb alice's ghost")

	// ---- Beat 6: the departure -------------------------------------
	// Bella heads back to town. Members exit; encounters close — her
	// exit is not an ending, and her carry-forward is the sequel seed
	// the campaign holds for her return.
	exitOut, err := enc.Exit(&encounter.ExitInput{Member: bella})
	require.NoError(t, err, "beat 6: bella departs")
	require.Equal(t, cellAt(3, 2),
		spatial.Position{X: exitOut.Outcome.Position.X, Y: exitOut.Outcome.Position.Y},
		"beat 6: her carry-forward records where she left")
	require.NotEmpty(t, exitOut.Carry, "beat 6: and what she believed — her holdings travel with her")

	st, p = seen(t, enc, goblin, bella)
	require.Equal(t, intel.Held, st, "beat 6: the goblin's sight of bella fades to a ghost")
	require.Equal(t, cellAt(3, 2), spatial.Position{X: p.X, Y: p.Y}, "beat 6: at the door where it last saw her")

	story, err := enc.Story(&encounter.StoryInput{Audience: bella})
	require.NoError(t, err, "beat 6: the departed can still read the story (everMembers)")
	require.NotEmpty(t, story)

	// ---- Beat 7: the stairs ----------------------------------------
	// Alice crosses the crypt and finds the stairs down. The declared
	// ending fires; the encounter closes with the Outcome the campaign
	// consumes.
	moveOut, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(11, 11)})
	require.NoError(t, err, "beat 7: alice reaches the stairs")
	require.NotNil(t, moveOut.Outcome, "beat 7: the ending fires in the Move's own output")
	require.Equal(t, endingStairs, moveOut.Outcome.Ending)
	require.Len(t, moveOut.Outcome.Members, 3, "beat 7: alice, cormac, and the goblin remain")
	require.Equal(t, alice, moveOut.Outcome.Members[0].ID, "beat 7: outcome members in sorted order")

	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open, "beat 7: closed = has an Outcome")

	for name, verb := range map[string]func() error{
		"Move": func() error {
			_, e := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 1)})
			return e
		},
		"Pump": func() error { _, e := enc.Pump(&encounter.PumpInput{}); return e },
		"Join": func() error {
			_, e := enc.Join(&encounter.JoinInput{
				Member: "late",
				Kind:   encounter.KindPlayer,
				Cell:   cellAt(1, 1),
			})
			return e
		},
		"Exit": func() error { _, e := enc.Exit(&encounter.ExitInput{Member: alice}); return e },
		"End":  func() error { _, e := enc.End(&encounter.EndInput{Ending: withdrawEnding}); return e },
	} {
		require.ErrorIs(t, verb(), encounter.ErrClosed, "beat 7: %s on a closed encounter", name)
	}

	// ---- Beat 8: the archive ---------------------------------------
	// Closed encounters answer queries forever. The story is the
	// scene's own narration — assert the FULL transcript of beat kinds.
	view, err := enc.View(&encounter.ViewInput{Member: alice})
	require.NoError(t, err, "beat 8: the archive answers View")
	require.NotEmpty(t, view)

	story, err = enc.Story(&encounter.StoryInput{Audience: alice})
	require.NoError(t, err, "beat 8: the archive answers Story")
	kinds := make([]string, 0, len(story))
	for _, e := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	require.Equal(t, []string{
		"scene-opened", // beat 1
		// beat 1: they see each other, the fight starts, the party breaks off
		// to watch instead
		"bubble-formed", "bubble-dissolved",
		"moved",         // beat 2: alice advances
		"tick", "moved", // beat 2: pump 1, goblin steps out
		"tick", "moved", // beat 2: pump 2, goblin steps back
		"moved",  // beat 3: alice slips behind the wall
		"joined", // beat 5: cormac (the pause leaves no beat — pause is free)
		// beat 5: and the goblin sees him arrive — his fight starts AFTER the
		// arrival that caused it. Cause before effect: the reverse order is
		// what this transcript caught when trigger detection moved inside
		// refreshSight (see refreshSight's law).
		"bubble-formed",
		"exited", // beat 6: bella (alice and the goblin's watchers free-roam
		"moved",  // beat 7: on — cormac's fight is localized to cormac)
	}, kinds, "beat 8: the story IS the scene, told in order")

	// ---- Beat 9: the sequel seed -----------------------------------
	// The campaign holds the Outcome. A return to this crypt is a NEW
	// encounter seeded from it — members at their outcome positions.
	// (Bella's separate carry from beat 6 would seed her return the
	// same way; intel transfer is the campaign's wave-two work.)
	sequelMembers := make([]encounter.MemberInput, 0, len(status.Outcome.Members))
	for _, mo := range status.Outcome.Members {
		kind := encounter.KindPlayer
		if mo.ID == goblin {
			kind = encounter.KindMonster
		}
		sequelMembers = append(sequelMembers, encounter.MemberInput{
			ID: mo.ID, Kind: kind, Position: mo.Position,
		})
	}
	sequel, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(cryptRoom, 0, 0, 12, 12)}, Props: wallRow(6, 5, 7),
		},
		Members: sequelMembers,
		Endings: []encounter.EndingInput{{Key: withdrawEnding, Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err, "beat 9: the outcome alone seeds the sequel — encounters end, places persist")
	seqStatus, err := sequel.Status()
	require.NoError(t, err)
	require.True(t, seqStatus.Open, "beat 9: the next scene begins")
}
