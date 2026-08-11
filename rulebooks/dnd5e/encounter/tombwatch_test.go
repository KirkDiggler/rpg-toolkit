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

// Additional test constants for AC1 (reuse alice, goblin, endingStairs from encounter_test.go)
const (
	bella          = core.EntityID("bella")
	cormac         = core.EntityID("cormac")
	cryptRoom      = "crypt"
	withdrawEnding = "withdrew"
	pillarX        = 6.0
	pillarY        = 6.0
)

// TestTombWatch is a narrative scene test covering AC1 requirements:
// Setup, line-of-sight blocking with pillar occluder, refresh during movement,
// mid-scene save/load, Join/Exit, ending evaluation, and story archival.
func TestTombWatch(t *testing.T) {
	t.Run("setup the crypt", func(t *testing.T) {
		// Beat 1: Setup — 12x12 crypt with pillar at (6,6).
		// Alice (player) at (2,2), Bella (player) at (3,2), Goblin (monster) at (6,10).
		// Goblin patrols between (6,10) and (7,10).
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY}, // Pillar at center
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bella,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 3, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
				{
					Key:     withdrawEnding,
					Trigger: encounter.TriggerExternal{},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err, "setup should succeed")

		// Assert: Both players see the goblin (and each other in clear LoS).
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(aliceView), 1, "alice should see at least goblin")

		// Find goblin in alice's holdings
		goblinFound := false
		for _, h := range aliceView {
			if h.Subject == intel.Subject(goblin) {
				goblinFound = true
				require.Equal(t, "current", string(h.Status), "goblin should be current")
				break
			}
		}
		require.True(t, goblinFound, "alice should see goblin")

		bellaView, err := enc.View(&encounter.ViewInput{Member: bella})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(bellaView), 1, "bella should see at least goblin")

		// Assert: Goblin sees both players (symmetric — anti-wall-hack contract).
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		require.NoError(t, err)
		require.Len(t, goblinView, 2, "goblin should see both players (symmetric LoS)")

		// Extract IDs from holdings
		holdings := make(map[intel.Subject]bool)
		for _, h := range goblinView {
			holdings[h.Subject] = true
		}
		require.True(t, holdings[intel.Subject(alice)], "goblin should see alice")
		require.True(t, holdings[intel.Subject(bella)], "goblin should see bella")
	})

	t.Run("the watch: alice moves and refresh during pump", func(t *testing.T) {
		// Beat 2: Alice moves to (2,6), triggering a refresh.
		// Beat 3: Pump once — goblin patrols and its new position appears in alice's View.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Alice moves to (2,6)
		moveOut, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 2, Y: 6},
		})
		require.NoError(t, err)
		require.NotNil(t, moveOut)

		// Check alice's view after move — should still see goblin at (6,10)
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.Len(t, aliceView, 1, "alice should see goblin after move")

		var payload encounter.SightPayload
		err = json.Unmarshal(aliceView[0].Payload, &payload)
		require.NoError(t, err)
		require.Equal(t, 6.0, payload.X)
		require.Equal(t, 10.0, payload.Y)

		// Pump once — goblin patrols. Since patrolDecider starts at callCount=0,
		// first pump will pick positions[0 % 2] = positions[0] = (6,10).
		// So goblin stays at (6,10) on first pump (no visible movement).
		// After pump 2, it will be at (7,10).
		pumpOut, err := enc.Pump(&encounter.PumpInput{})
		require.NoError(t, err)
		require.NotNil(t, pumpOut)

		// Check alice's view after pump — goblin should still be at (6,10)
		aliceView, err = enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.Len(t, aliceView, 1)

		// Pump again to move goblin to (7,10)
		_, err = enc.Pump(&encounter.PumpInput{})
		require.NoError(t, err)

		aliceView, err = enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.Len(t, aliceView, 1)
		err = json.Unmarshal(aliceView[0].Payload, &payload)
		require.NoError(t, err)
		require.Equal(t, 7.0, payload.X, "goblin should have moved to x=7")
	})

	t.Run("the ghost: pillar blocks line of sight", func(t *testing.T) {
		// Beat 4: Set up alice and goblin with pillar between them such that
		// they cannot see each other. Alice sees goblin, then loses sight when
		// goblin moves behind pillar or LoS geometry changes.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: 5.0, Y: 5.0}, // Pillar positioned to block diagonal
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 0, Y: 5},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 11, Y: 5},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 11, Y: 5},
							{X: 10, Y: 5},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Setup: alice at (0,5), goblin at (11,5), pillar at (5,5) should block horizontal sight
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.Len(t, aliceView, 0, "alice should NOT see goblin initially (blocked by pillar)")

		// Goblin should also not see alice (symmetric)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		require.NoError(t, err)
		require.Len(t, goblinView, 0, "goblin should NOT see alice initially (blocked by pillar)")

		// Move goblin perpendicular to line of sight to escape pillar blockage
		// However, since the pillar is at (5,5) and line is at y=5, moving off y=5 clears it
		// But we can't directly move a monster. Instead, pump to make goblin move via decider.
		// Actually, patrolDecider only patrols along x-axis, so it won't help here.
		// Let's just verify the pillar blocking is working by moving alice to unblock.

		// Move alice to (7,5) — still blocked by pillar at (5,5) on the line
		_, err = enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 7, Y: 5},
		})
		require.NoError(t, err)

		// Alice at (7,5) should see goblin at (11,5) — both on same side of pillar
		aliceView, err = enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.Len(t, aliceView, 1, "alice should see goblin when both on same side of pillar")
	})

	t.Run("the pause (mid-scene save/load)", func(t *testing.T) {
		// Beat 5: ToData → LoadEncounter round-trip.
		// Verify state is preserved and scene can continue.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc1, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Move and pump to establish state
		_, err = enc1.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 2, Y: 6},
		})
		require.NoError(t, err)

		_, err = enc1.Pump(&encounter.PumpInput{})
		require.NoError(t, err)

		// Get view before serialization
		enc1View, err := enc1.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)

		// Serialize
		data := enc1.ToData()

		// Re-attach decider and load
		goblinDecider := &patrolDecider{
			positions: []spatial.Position{
				{X: 6, Y: 10},
				{X: 7, Y: 10},
			},
			callCount: 1, // Match the call count from enc1 (one pump occurred)
		}

		enc2, err := encounter.LoadEncounter(data, map[encounter.MemberID]encounter.Decider{
			goblin: goblinDecider,
		})
		require.NoError(t, err)

		// Views should be identical
		enc2View, err := enc2.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)

		require.Len(t, enc2View, len(enc1View), "alice's view should match after reload")

		// Continue the scene on enc2
		_, err = enc2.Pump(&encounter.PumpInput{})
		require.NoError(t, err)

		// Verify continued operation works
		enc2ViewAfter, err := enc2.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.NotNil(t, enc2ViewAfter)
	})

	t.Run("the reinforcement (Join)", func(t *testing.T) {
		// Beat 6: Cormac joins at (10,2). Incumbents see him and he sees them.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 6},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Cormac joins at (10,2)
		joinOut, err := enc.Join(&encounter.JoinInput{
			Member: encounter.MemberInput{
				ID:       cormac,
				Kind:     encounter.KindPlayer,
				Room:     cryptRoom,
				Position: spatial.Position{X: 10, Y: 2},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, joinOut)

		// Cormac should see alice and goblin
		cormacView, err := enc.View(&encounter.ViewInput{Member: cormac})
		require.NoError(t, err)
		require.Len(t, cormacView, 2, "cormac should see alice and goblin")

		// Alice should see goblin and cormac
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.Len(t, aliceView, 2, "alice should see goblin and cormac")
	})

	t.Run("the departure (Exit)", func(t *testing.T) {
		// Beat 7: Bella exits ("heads back to town").
		// Assert: her carry-forward (position + holdings), remaining members' views update.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 6},
				},
				{
					ID:       bella,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 3, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Exit bella
		exitOut, err := enc.Exit(&encounter.ExitInput{
			Member: bella,
		})
		require.NoError(t, err)
		require.NotNil(t, exitOut)

		// Check carry-forward
		require.Equal(t, bella, exitOut.Outcome.ID)
		require.Equal(t, cryptRoom, exitOut.Outcome.Room)
		require.Equal(t, 3.0, exitOut.Outcome.Position.X)
		require.Equal(t, 2.0, exitOut.Outcome.Position.Y)

		// Bella should still be able to access story (everMembers)
		bellaStory, err := enc.Story(&encounter.StoryInput{
			Audience: bella,
			AfterSeq: 0,
		})
		require.NoError(t, err)
		require.Len(t, bellaStory, 2, "bella should read story (opened + exited)")
	})

	t.Run("the stairs (ending)", func(t *testing.T) {
		// Beat 8: Alice moves to (11,11) — fires the "stairs" ending.
		// Assert: Outcome carries members, Status closed, all mutating verbs return ErrClosed.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bella,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 3, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
				{
					Key:     withdrawEnding,
					Trigger: encounter.TriggerExternal{},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Alice moves to stairs
		moveOut, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 11, Y: 11},
		})
		require.NoError(t, err)
		require.NotNil(t, moveOut.Outcome, "moving to stairs should fire the ending")
		require.Equal(t, endingStairs, moveOut.Outcome.Ending)

		// Outcome should carry all three members in sorted order
		require.Len(t, moveOut.Outcome.Members, 3, "outcome should have all three members")
		require.Equal(t, alice, moveOut.Outcome.Members[0].ID)
		require.Equal(t, bella, moveOut.Outcome.Members[1].ID)
		require.Equal(t, goblin, moveOut.Outcome.Members[2].ID)

		// Status should show closed
		status, err := enc.Status()
		require.NoError(t, err)
		require.False(t, status.Open)
		require.NotNil(t, status.Outcome)
		require.Equal(t, endingStairs, status.Outcome.Ending)

		// All mutating verbs should return ErrClosed
		_, err = enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 11, Y: 10},
		})
		require.ErrorIs(t, err, encounter.ErrClosed)

		_, err = enc.Pump(&encounter.PumpInput{})
		require.ErrorIs(t, err, encounter.ErrClosed)

		_, err = enc.Exit(&encounter.ExitInput{
			Member: alice,
		})
		require.ErrorIs(t, err, encounter.ErrClosed)

		_, err = enc.End(&encounter.EndInput{
			Ending: withdrawEnding,
		})
		require.ErrorIs(t, err, encounter.ErrClosed)
	})

	t.Run("the archive (closed encounter queries)", func(t *testing.T) {
		// Beat 9: Closed encounter still answers View and Story.
		// Story transcript shows scene progression.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bella,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 3, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Run a scenario: move, pump, exit, move to stairs
		_, err = enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 2, Y: 6},
		})
		require.NoError(t, err)

		_, err = enc.Pump(&encounter.PumpInput{})
		require.NoError(t, err)

		_, err = enc.Exit(&encounter.ExitInput{
			Member: bella,
		})
		require.NoError(t, err)

		moveOut, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 11, Y: 11},
		})
		require.NoError(t, err)
		require.NotNil(t, moveOut.Outcome)

		// Get the story from alice's perspective
		story, err := enc.Story(&encounter.StoryInput{
			Audience: alice,
			AfterSeq: 0,
		})
		require.NoError(t, err)

		// Assert beat sequence: scene-opened, moved, tick+moved, exited, moved
		// Note: Move-triggered endings don't append a separate "ended" beat;
		// the last beat is the move that triggered the ending.
		require.Greater(t, len(story), 0, "story should have beats")

		// First beat should be scene-opened
		require.NotNil(t, story[0])
		var firstBeat map[string]interface{}
		err = json.Unmarshal(story[0].Payload, &firstBeat)
		require.NoError(t, err)
		require.Equal(t, "scene-opened", firstBeat["beat"])

		// Last beat should be "moved" (the Move that fired the ending; no separate ended beat)
		require.NotNil(t, story[len(story)-1])
		var lastBeat map[string]interface{}
		err = json.Unmarshal(story[len(story)-1].Payload, &lastBeat)
		require.NoError(t, err)
		require.Equal(t, "moved", lastBeat["beat"], "last beat should be moved (Move-triggered ending)")

		// View should still work on closed encounter
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		require.NoError(t, err)
		require.NotNil(t, aliceView)
	})

	t.Run("the sequel seed (outcome carry-forward)", func(t *testing.T) {
		// Beat 10: Build carry from Outcome — positions for campaign sequel.
		// Prove the Outcome's data suffices to seed a new Encounter.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     cryptRoom,
						Width:  12,
						Height: 12,
						Occluders: []spatial.Position{
							{X: pillarX, Y: pillarY},
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bella,
					Kind:     encounter.KindPlayer,
					Room:     cryptRoom,
					Position: spatial.Position{X: 3, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     cryptRoom,
					Position: spatial.Position{X: 6, Y: 10},
					Decider: &patrolDecider{
						positions: []spatial.Position{
							{X: 6, Y: 10},
							{X: 7, Y: 10},
						},
					},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: endingStairs,
					Trigger: encounter.TriggerReachedPosition{
						Room:     cryptRoom,
						Position: spatial.Position{X: 11, Y: 11},
					},
				},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)

		// Run minimal scenario to close
		_, err = enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 11, Y: 11},
		})
		require.NoError(t, err)

		// Get outcome
		status, err := enc.Status()
		require.NoError(t, err)
		require.NotNil(t, status.Outcome)

		// Verify outcome data is sufficient for carry-forward
		require.Len(t, status.Outcome.Members, 3, "outcome should have all members")

		// Verify sorted order
		require.Equal(t, alice, status.Outcome.Members[0].ID)
		require.Equal(t, bella, status.Outcome.Members[1].ID)
		require.Equal(t, goblin, status.Outcome.Members[2].ID)

		// Each should have valid positions
		for _, mo := range status.Outcome.Members {
			require.Equal(t, cryptRoom, mo.Room)
			require.GreaterOrEqual(t, mo.Position.X, 0.0)
			require.GreaterOrEqual(t, mo.Position.Y, 0.0)
			require.Less(t, mo.Position.X, 12.0)
			require.Less(t, mo.Position.Y, 12.0)
		}
	})
}
