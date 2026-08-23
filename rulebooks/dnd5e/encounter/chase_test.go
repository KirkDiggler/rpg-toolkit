// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestVaultChase is the wave's second continuous scene (AC2, alongside
// TestTombWatch — tombwatch itself untouched): ONE flowing story that
// exercises the connection primitives Waves 1's tomb watch predates —
// sight across an open room, slipping through a doorway, the ghost that
// forms at the threshold (not the far side never seen), a mid-chase save/
// reload, a decider's OWN pursuit traverse, an ending declared in the far
// room, and an archive sweep that now includes Traverse. Uses seen() and
// pursuitDecider, shared with tombwatch_test.go and pump_test.go
// respectively (same package).
func TestVaultChase(t *testing.T) {
	// ---- The set -----------------------------------------------------
	// Two regions, one gate: an open door standing in the crossing from the
	// corridor's [9,5] to the vault's [10,5] (rpg-project#256 — a doorway is
	// the door standing in it).
	const (
		corridorRoom    = "corridor"
		vaultRoom       = "vault"
		gateConnection  = "gate"
		sanctuaryEnding = "sanctuary"
	)
	gate := openDoorway(gateConnection, 9, 5, 10, 5)

	// The corridor's east wall, open at the gate's row. Without it the two
	// chambers share an open edge ten cells wide: the field is one canvas
	// (rpg-toolkit#1106), so a chase that needs somewhere to disappear to has
	// to say where the walls are rather than let a room boundary imply them.
	corridorWall := squareSeamWall(9, 10, 5)

	// ---- Beat 1: sight -------------------------------------------------
	// Alice and the goblin share the corridor, in the open — first light
	// finds them mutually visible. The goblin's pursuitDecider knows the
	// gate (static topology, given at construction) and its target
	// (alice); it never reads encounter state directly — Snapshot +
	// Holdings + its own construction-time config is all it gets (C2).
	// The decider is built empty and handed the map once there is one to
	// hand it: the doorways it needs are the Atlas's, and the Atlas comes
	// from the encounter it is about to be part of. That ordering IS the
	// point — a decider takes the same absolute map a host renders, not the
	// composition's room-shaped topology (rpg-toolkit#1044).
	pursuit := &pursuitDecider{target: alice}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(corridorRoom, 0, 0, 10, 10), rectRegion(vaultRoom, 10, 0, 10, 10)}, Walls: corridorWall,
			Doors: []encounter.DoorInput{gate},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 5}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 8}, Decider: pursuit},
		},
		Endings: []encounter.EndingInput{
			{Key: sanctuaryEnding, Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 18, Y: 8}}}, // the vault's own [8,8], anchored at [10,0]
		},
	})
	require.NoError(t, err, "beat 1: the corridor assembles")

	atlas, err := enc.Atlas()
	require.NoError(t, err, "beat 1: the map exists")
	pursuit.doorways = atlas.Doorways

	st, _ := seen(t, enc, alice, goblin)
	require.Equal(t, intel.Current, st, "beat 1: alice sees the goblin across the open corridor")
	st, _ = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 1: and the goblin sees her back — intel is symmetric")

	// Seeing each other started the fight (rpg-toolkit#964). She breaks off
	// to run — which is what a chase IS, and what this scene always showed
	// without having to say so.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	require.NoError(t, err, "beat 1: alice breaks off to run")

	// ---- Beat 2: to the threshold, and through it ----------------------
	// Alice crosses to the gate, then slips through it into the vault.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(9, 5)})
	require.NoError(t, err, "beat 2: alice crosses to the gate")

	travOut, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	require.NoError(t, err, "beat 2: she slips through the gate")
	require.Len(t, travOut.Doors, 1, "beat 2: the door is named, and decides nothing")
	require.Equal(t, encounter.DoorID(gateConnection), travOut.Doors[0].ID)
	require.Equal(t, cellAt(10, 5), travOut.Stepped.To, "beat 2: vault-local (0,5) on the map")

	// She doesn't linger in the opening — she moves deeper into the vault,
	// toward (but not yet at) sanctuary, and out of the gate's line.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(14, 8)})
	require.NoError(t, err, "beat 2: she moves deeper into the vault")

	// The ghost forms — for BOTH of them, symmetrically. Note WHERE: the last
	// cell each actually saw the other at, which the wall beside the gate is
	// what makes possible. Standing IN the opening she was still visible; it
	// takes the wall to hide her (rpg-toolkit#1106).
	st, p := seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 2: the goblin's sight of alice fades — the wall took her")
	require.Equal(t, cellAt(10, 5), spatial.Position{X: p.X, Y: p.Y}, "beat 2: at the gate's far cell, the last place it saw her")
	st, _ = seen(t, enc, alice, goblin)
	require.Equal(t, intel.Held, st, "beat 2: alice loses the goblin too — symmetric")

	// ---- Beat 3: the pause (pause is free) ------------------------------
	// The table closes the Discord activity mid-chase. The host persists
	// ONE aggregate and rehydrates it later. Deciders are behavior, not
	// state: the campaign re-attaches a FRESH pursuitDecider at load —
	// same topology and target, no memory of its own carried over (its
	// INTEL does — beliefs are state and traveled in the aggregate).
	data := enc.ToData()
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
			goblin: &pursuitDecider{doorways: atlas.Doorways, target: alice},
		}})
	require.NoError(t, err, "beat 3: the suspended chase crosses a process boundary")
	enc = enc2 // the reload IS the encounter now

	st, p = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 3: the ghost survived the reload")
	require.Equal(t, cellAt(10, 5), spatial.Position{X: p.X, Y: p.Y}, "beat 3: still at the gate's far cell — loading never re-derives sight")

	// ---- Beat 4: the pursuit through the gate ---------------------------
	// One pump: the goblin walks to the ghost's last-seen cell, which is on
	// the far side of the gate. It is ONE step in the composition's terms and
	// one entry in the output — there is no second list for crossings, because
	// there is no second mechanism (rpg-toolkit#1106).
	pumpOut1, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the pursuit resumes")
	require.Len(t, pumpOut1.MonsterMoves, 1, "beat 4: the goblin walks toward the last place it saw her")
	require.Equal(t, goblin, pumpOut1.MonsterMoves[0].Member)
	// The arrival cell on the MAP: vault-local (0,5) through the vault's
	// (10,0) anchor. The same cell the movement beat carries — one movement
	// cannot be reported in two frames (rpg-toolkit#1062).
	require.Equal(t, cellAt(10, 5), pumpOut1.MonsterMoves[0].To)

	// It's in her chamber now — this pump's own refreshSight already shows
	// her Current again, where she stopped in beat 2.
	st, p = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 4: the goblin holds alice Current again, having come through the gate")
	require.Equal(t, cellAt(14, 8), spatial.Position{X: p.X, Y: p.Y}, "beat 4: vault-local (4,8), anchored at (10,0) — one map")

	// ---- Beat 5: sanctuary, in the far room ------------------------------
	// Alice reaches the shrine before the goblin closes the distance. The
	// declared ending fires in the VAULT — a room the corridor-side setup
	// never mentioned directly; the ending only knows the room by name.
	// The vault is anchored at (10,0), so its own (8,8) is (18,8) on the map.
	moveOut, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(18, 8)})
	require.NoError(t, err, "beat 5: alice reaches sanctuary")
	require.NotNil(t, moveOut.Outcome, "beat 5: the ending fires in the Move's own output")
	require.Equal(t, sanctuaryEnding, moveOut.Outcome.Ending)
	require.Len(t, moveOut.Outcome.Members, 2, "beat 5: alice and the goblin remain")

	var aliceOutcome, goblinOutcome encounter.MemberOutcome
	for _, m := range moveOut.Outcome.Members {
		switch m.ID {
		case alice:
			aliceOutcome = m
		case goblin:
			goblinOutcome = m
		}
	}
	require.Equal(t, encounter.RegionID(vaultRoom), aliceOutcome.Region)
	// Dungeon-absolute (#1068): the vault is anchored at (10,0), so her
	// vault-local (8,8) is (18,8) on the map — the same frame every other
	// cell in this scene is reported in.
	require.Equal(t, cellAt(18, 8), aliceOutcome.Position)
	// The goblin made it across the threshold too — the outcome reflects
	// where it TRULY stands, in the vault, not its pre-chase corridor spot.
	require.Equal(t, encounter.RegionID(vaultRoom), goblinOutcome.Region, "beat 5: the outcome reflects the region the goblin finished in")
	require.Equal(t, cellAt(10, 5), goblinOutcome.Position, "vault-local (0,5) anchored at (10,0)")

	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open, "beat 5: closed = has an Outcome")

	// ---- Beat 6: the archive sweep --------------------------------------
	// A closed encounter rejects every mutating verb.
	for name, verb := range map[string]func() error{
		"Step": func() error {
			_, e := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 1)})
			return e
		},
		"Pump": func() error { _, e := enc.Pump(&encounter.PumpInput{}); return e },
		"Join": func() error {
			_, e := enc.Join(&encounter.JoinInput{
				Member: "late",
				Kind:   encounter.KindPlayer,
				Cell:   cellAt(11, 1), // the vault, anchored at (10,0)
			})
			return e
		},
		"Exit": func() error { _, e := enc.Exit(&encounter.ExitInput{Member: alice}); return e },
		"End":  func() error { _, e := enc.End(&encounter.EndInput{Ending: sanctuaryEnding}); return e },
	} {
		require.ErrorIs(t, verb(), encounter.ErrClosed, "beat 6: %s on a closed encounter", name)
	}

	// ---- Beat 7: the archive answers ------------------------------------
	// Closed encounters answer queries forever. Assert the FULL transcript
	// of beat kinds, in order — the story IS the scene.
	view, err := enc.View(&encounter.ViewInput{Member: alice})
	require.NoError(t, err, "beat 7: the archive answers View")
	require.NotEmpty(t, view)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	require.NoError(t, err, "beat 7: the archive answers Story")
	kinds := make([]string, 0, len(story))
	for _, e := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	require.Equal(t, []string{
		"scene-opened", // beat 1
		// beat 1: they see each other, so the fight starts; she breaks off
		// to run, which is what makes the rest of this a chase
		"bubble-formed", "bubble-dissolved",
		// Beat 2 is three steps and every one of them is a "moved": crossing
		// the gate stopped being a second kind of movement when the field
		// stopped being a set of rooms (rpg-toolkit#1106). The doorway's name
		// rides on the middle beat's own payload, not on its kind.
		"moved",         // beat 2: alice crosses to the gate
		"moved",         // beat 2: she slips through it
		"moved",         // beat 2: she moves deeper into the vault
		"tick", "moved", // beat 4: pump 1, the goblin comes through after her
		"moved", // beat 5: alice reaches sanctuary
	}, kinds, "beat 7: the story IS the chase, told in order")
}
