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
	// Two rooms, one gate. DELIBERATELY asymmetric endpoints (T1 review
	// lesson): FromPosition {9,5} in the corridor, ToPosition {0,5} in
	// the vault — a from/to mix-up would be observable.
	const (
		corridorRoom    = "corridor"
		vaultRoom       = "vault"
		gateConnection  = "gate"
		sanctuaryEnding = "sanctuary"
	)
	gate := encounter.ConnectionInput{
		ID: gateConnection, From: corridorRoom, To: vaultRoom,
		FromPosition: spatial.Position{X: 9, Y: 5},
		ToPosition:   spatial.Position{X: 0, Y: 5},
	}

	// ---- Beat 1: sight -------------------------------------------------
	// Alice and the goblin share the corridor, in the open — first light
	// finds them mutually visible. The goblin's pursuitDecider knows the
	// gate (static topology, given at construction) and its target
	// (alice); it never reads encounter state directly — Snapshot +
	// Holdings + its own construction-time config is all it gets (C2).
	pursuit := &pursuitDecider{connections: []encounter.ConnectionInput{gate}, target: alice}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: corridorRoom, Width: 10, Height: 10},
				// Anchored immediately east of the corridor (#929 T1): the
				// gate's endpoints (9,5) and (0,5)+(10,0)=(10,5) are
				// Chebyshev-adjacent (W3), and the rooms' absolute
				// footprints (x:[0,9] vs x:[10,19]) stay disjoint (W2).
				{ID: vaultRoom, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{gate},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: corridorRoom, Position: spatial.Position{X: 2, Y: 5}},
			{ID: goblin, Kind: encounter.KindMonster, Room: corridorRoom,
				Position: spatial.Position{X: 2, Y: 8}, Decider: pursuit},
		},
		Endings: []encounter.EndingInput{
			{Key: sanctuaryEnding, Trigger: encounter.TriggerReachedPosition{
				Room: vaultRoom, Position: spatial.Position{X: 8, Y: 8}}},
		},
	})
	require.NoError(t, err, "beat 1: the corridor assembles")

	st, _ := seen(t, enc, alice, goblin)
	require.Equal(t, intel.Current, st, "beat 1: alice sees the goblin across the open corridor")
	st, _ = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 1: and the goblin sees her back — intel is symmetric")

	// ---- Beat 2: to the threshold, and through it ----------------------
	// Alice crosses to the gate, then slips through it into the vault.
	_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 9, Y: 5}})
	require.NoError(t, err, "beat 2: alice crosses to the gate")

	travOut, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: gateConnection})
	require.NoError(t, err, "beat 2: she slips through the gate")
	require.Equal(t, vaultRoom, travOut.Traversed.ToRoom, "beat 2: she arrives in the vault")
	require.Equal(t, spatial.Position{X: 0, Y: 5}, travOut.Traversed.To)

	// The ghost forms — for BOTH of them, symmetrically. Note WHERE:
	// the threshold in the corridor, her last-seen position — not the
	// vault, which the goblin has never seen at all.
	st, p := seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 2: the goblin's sight of alice fades — she left the room")
	require.Equal(t, corridorRoom, p.Room, "beat 2: its ghost holds her LAST-SEEN room")
	require.Equal(t, 9.0, p.X, "beat 2: at the threshold, not the far side it never saw")
	require.Equal(t, 5.0, p.Y)
	st, _ = seen(t, enc, alice, goblin)
	require.Equal(t, intel.Held, st, "beat 2: alice loses the goblin too — symmetric")

	// She doesn't linger at the arrival cell — she moves deeper into the
	// vault, toward (but not yet at) sanctuary.
	_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 4, Y: 5}})
	require.NoError(t, err, "beat 2: she moves deeper into the vault")

	// ---- Beat 3: the pause (pause is free) ------------------------------
	// The table closes the Discord activity mid-chase. The host persists
	// ONE aggregate and rehydrates it later. Deciders are behavior, not
	// state: the campaign re-attaches a FRESH pursuitDecider at load —
	// same topology and target, no memory of its own carried over (its
	// INTEL does — beliefs are state and traveled in the aggregate).
	data := enc.ToData()
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
		goblin: &pursuitDecider{connections: []encounter.ConnectionInput{gate}, target: alice},
	}})
	require.NoError(t, err, "beat 3: the suspended chase crosses a process boundary")
	enc = enc2 // the reload IS the encounter now

	st, p = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 3: the ghost survived the reload")
	require.Equal(t, corridorRoom, p.Room, "beat 3: still at the threshold — loading never re-derives sight")
	require.Equal(t, 9.0, p.X)

	// ---- Beat 4: the pursuit traverse ------------------------------------
	// First pump: the goblin, still in the corridor, walks toward the
	// ghost's last-seen position — the threshold.
	pumpOut1, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the pursuit resumes")
	require.Len(t, pumpOut1.MonsterMoves, 1, "beat 4: the goblin walks toward the threshold")
	require.Equal(t, spatial.Position{X: 9, Y: 5}, pumpOut1.MonsterMoves[0].To)
	require.Empty(t, pumpOut1.MonsterTraverses, "beat 4: not standing at the threshold yet on this tick's decision")

	// Second pump: now standing exactly on the connection's endpoint, the
	// goblin's OWN decision crosses it — the same mechanics the Traverse
	// verb used for alice, reused, not duplicated.
	pumpOut2, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the goblin follows her through")
	require.Len(t, pumpOut2.MonsterTraverses, 1, "beat 4: the goblin traverses the gate")
	require.Equal(t, goblin, pumpOut2.MonsterTraverses[0].Member)
	require.Equal(t, corridorRoom, pumpOut2.MonsterTraverses[0].FromRoom)
	require.Equal(t, vaultRoom, pumpOut2.MonsterTraverses[0].ToRoom)
	require.Equal(t, spatial.Position{X: 0, Y: 5}, pumpOut2.MonsterTraverses[0].To)

	// It's in her room now — this pump's own refreshSight already shows
	// her Current again, at (4,5), where she stopped in beat 2.
	st, p = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 4: the goblin holds alice Current again, having crossed the threshold")
	require.Equal(t, vaultRoom, p.Room)
	require.Equal(t, 4.0, p.X)
	require.Equal(t, 5.0, p.Y)

	// ---- Beat 5: sanctuary, in the far room ------------------------------
	// Alice reaches the shrine before the goblin closes the distance. The
	// declared ending fires in the VAULT — a room the corridor-side setup
	// never mentioned directly; the ending only knows the room by name.
	moveOut, err := enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 8, Y: 8}})
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
	require.Equal(t, vaultRoom, aliceOutcome.Room)
	require.Equal(t, spatial.Position{X: 8, Y: 8}, aliceOutcome.Position)
	// The goblin made it across the threshold too — the outcome reflects
	// where it TRULY stands, in the vault, not its pre-chase corridor spot.
	require.Equal(t, vaultRoom, goblinOutcome.Room, "beat 5: the outcome reflects the goblin's post-traverse room")
	require.Equal(t, spatial.Position{X: 0, Y: 5}, goblinOutcome.Position)

	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open, "beat 5: closed = has an Outcome")

	// ---- Beat 6: the archive sweep, now including Traverse ---------------
	// A closed encounter rejects every mutating verb — tomb watch pinned
	// this for Move/Pump/Join/Exit/End; the connections wave adds Traverse
	// to the same sweep.
	for name, verb := range map[string]func() error{
		"Move": func() error {
			_, e := enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 1, Y: 1}})
			return e
		},
		"Pump": func() error { _, e := enc.Pump(&encounter.PumpInput{}); return e },
		"Join": func() error {
			_, e := enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
				ID: "late", Kind: encounter.KindPlayer, Room: vaultRoom, Position: spatial.Position{X: 1, Y: 1}}})
			return e
		},
		"Exit": func() error { _, e := enc.Exit(&encounter.ExitInput{Member: alice}); return e },
		"End":  func() error { _, e := enc.End(&encounter.EndInput{Ending: sanctuaryEnding}); return e },
		// The goblin, not alice: it ends the scene standing exactly on
		// the gate's vault-side endpoint (0,5), so this attempt is
		// valid-if-open — the rejection below is attributable to
		// ErrClosed alone, not also to alice's position (she is deeper
		// in the vault, at (8,8), nowhere near either threshold).
		"Traverse": func() error {
			_, e := enc.Traverse(&encounter.TraverseInput{Member: goblin, Connection: gateConnection})
			return e
		},
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
		"scene-opened",  // beat 1
		"moved",         // beat 2: alice crosses to the gate
		"traversed",     // beat 2: she slips through
		"moved",         // beat 2: she moves deeper into the vault
		"tick", "moved", // beat 4: pump 1, the goblin walks to the threshold
		"tick", "traversed", // beat 4: pump 2, the goblin follows her through
		"moved", // beat 5: alice reaches sanctuary
	}, kinds, "beat 7: the story IS the chase, told in order")
}
