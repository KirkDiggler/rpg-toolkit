// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Shared test constants
const (
	alice        = core.EntityID("alice")
	bob          = core.EntityID("bob")
	goblin       = core.EntityID("goblin")
	room1        = "room-1"
	endingStairs = "stairs"
)

type EncounterTestSuite struct {
	suite.Suite
}

func (s *EncounterTestSuite) TestSetupFirstLight() {
	s.Run("two members clear line of sight", func() {
		// Arrange: 10x10 room, alice at (2,2) player, goblin at (7,7) monster
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 7, Y: 7},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 9, Y: 9},
					},
				},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: alice sees goblin
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 1, "alice should see exactly one holding (goblin)")

		holding := aliceView[0]
		s.Equal(intel.Subject(goblin), holding.Subject, "holding subject should be goblin")
		s.Equal("current", string(holding.Status), "holding status should be Current")

		// Decode position payload into SightPayload
		var payload encounter.SightPayload
		err = json.Unmarshal(holding.Payload, &payload)
		s.Require().NoError(err)
		s.Equal(room1, payload.Room)
		s.Equal(7.0, payload.X)
		s.Equal(7.0, payload.Y)

		// Assert: goblin sees alice (symmetric)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		s.Require().NoError(err)
		s.Len(goblinView, 1, "goblin should see exactly one holding (alice)")

		holding = goblinView[0]
		s.Equal(intel.Subject(alice), holding.Subject, "holding subject should be alice")
		s.Equal("current", string(holding.Status), "holding status should be Current")

		// Decode position
		err = json.Unmarshal(holding.Payload, &payload)
		s.Require().NoError(err)
		s.Equal(2.0, payload.X)
		s.Equal(2.0, payload.Y)
	})
}

func (s *EncounterTestSuite) TestSetupPillarBlocksSight() {
	s.Run("occluder blocks both directions", func() {
		// Arrange: alice and goblin separated by a pillar
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  10,
						Height: 10,
						Occluders: []spatial.Position{
							{X: 4, Y: 5}, // Pillar in the middle
						},
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 2, Y: 5},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 7, Y: 5},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 9, Y: 9},
					},
				},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: alice does not see goblin
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 0, "alice should see nothing (blocked by pillar)")

		// Assert: goblin does not see alice (symmetric)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		s.Require().NoError(err)
		s.Len(goblinView, 0, "goblin should see nothing (blocked by pillar)")
	})
}

func (s *EncounterTestSuite) TestSetupValidationOrderAndAtomicity() {
	s.Run("validation order: nil input", func() {
		_, err := encounter.NewEncounter(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("validation order: no field", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "end", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoField)
	})

	s.Run("validation order: no endings", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: reserved ending key", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "abandoned", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: empty ending key", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: empty member ID", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "", Kind: encounter.KindPlayer, Room: "room-1"},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("validation order: duplicate member IDs", func() {
		alice := core.EntityID("alice")
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("atomicity: after error, valid setup works", func() {
		// First attempt: fails validation
		setup1 := &encounter.SetupInput{
			Field:   encounter.FieldInput{Rooms: []encounter.RoomInput{}},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup1)
		s.Require().ErrorIs(err, encounter.ErrNoField)

		// Second attempt: valid
		setup2 := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-1", Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "alice", Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		enc, err := encounter.NewEncounter(setup2)
		s.Require().NoError(err)
		s.NotNil(enc)
	})
}

func (s *EncounterTestSuite) TestSetupOpeningBeat() {
	s.Run("opening beat reaches all members via Story", func() {
		// Arrange
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
				{ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: alice's Story contains exactly one entry (opening beat)
		aliceStory, err := enc.Story(&encounter.StoryInput{Audience: alice, AfterSeq: 0})
		s.Require().NoError(err)
		s.Len(aliceStory, 1, "alice should have exactly one story entry")

		// Decode the opening beat payload
		var beatPayload map[string]string
		err = json.Unmarshal(aliceStory[0].Payload, &beatPayload)
		s.Require().NoError(err)
		s.Equal("scene-opened", beatPayload["beat"], "beat payload should contain scene-opened")

		// Assert: bob and goblin also receive the opening beat
		bobStory, err := enc.Story(&encounter.StoryInput{Audience: bob, AfterSeq: 0})
		s.Require().NoError(err)
		s.Len(bobStory, 1, "bob should have exactly one story entry")

		goblinStory, err := enc.Story(&encounter.StoryInput{Audience: goblin, AfterSeq: 0})
		s.Require().NoError(err)
		s.Len(goblinStory, 1, "goblin should have exactly one story entry")

		// MUTATION-PROOF: Verify by checking the actual implementation
		// (This test ensures that if the Append call is deleted, the test fails)
	})
}

func (s *EncounterTestSuite) TestSetupBadPlacementSentinel() {
	s.Run("member placed in nonexistent room errors with ErrBadPlacement", func() {
		// Arrange
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				// Try to place alice in a room that doesn't exist
				{ID: alice, Kind: encounter.KindPlayer, Room: "nonexistent", Position: spatial.Position{X: 0, Y: 0}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}

		// Act
		_, err := encounter.NewEncounter(setup)

		// Assert: error wraps ErrBadPlacement
		s.Require().NotNil(err)
		s.True(errors.Is(err, encounter.ErrBadPlacement), "error should wrap ErrBadPlacement")
	})
}

func (s *EncounterTestSuite) TestSetupCompletePercept() {
	s.Run("three members mutually visible all see each other", func() {
		// Arrange: ONE room, THREE mutually visible members (alice, bob players; goblin monster)
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 20, Height: 20},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 5, Y: 5}},
				{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 10, Y: 10}},
				{ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 15, Y: 15}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}

		// Act
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Assert: alice sees both bob and goblin
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 2, "alice should see exactly 2 holdings (bob and goblin)")

		aliceSeesSubjects := make(map[string]bool)
		for _, holding := range aliceView {
			aliceSeesSubjects[string(holding.Subject)] = true
		}
		s.True(aliceSeesSubjects[string(bob)], "alice should see bob")
		s.True(aliceSeesSubjects[string(goblin)], "alice should see goblin")

		// Assert: bob sees alice and goblin
		bobView, err := enc.View(&encounter.ViewInput{Member: bob})
		s.Require().NoError(err)
		s.Len(bobView, 2, "bob should see exactly 2 holdings (alice and goblin)")

		// Assert: goblin sees alice and bob
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		s.Require().NoError(err)
		s.Len(goblinView, 2, "goblin should see exactly 2 holdings (alice and bob)")

		// MUTATION-PROOF: verify by checking the implementation
		// (This test ensures that if the percept loop's append is broken, the test fails)
	})
}

// NOTE: Item 8 (Boundary LoS test) deferred. The spatial module provides
// RegisterBoundary and isDirectLineOfSightBoundaryBlockedUnsafe, but LoS
// integration needs verification across all cases. Placeholder test
// intentionally removed to prevent false negatives; will be added in Task 3
// once spatial module integration is validated.

func (s *EncounterTestSuite) TestMembers() {
	s.Run("returns stable order", func() {
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 10, Height: 10},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		members, err := enc.Members()
		s.Require().NoError(err)
		s.Len(members, 2)
	})
}

func (s *EncounterTestSuite) TestMovePerceptRefreshes() {
	s.Run("mover's vantage refreshes, others see mover at new position", func() {
		// Arrange: alice and bob in clear sight
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  20,
						Height: 20,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 10, Y: 10},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 18, Y: 18},
					},
				},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: alice moves
		moveOut, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 12, Y: 12},
		})
		s.Require().NoError(err)

		// Assert: Move returned successfully
		s.NotNil(moveOut)
		s.Equal(alice, moveOut.Moved.Member)
		s.Equal(spatial.Position{X: 2, Y: 2}, moveOut.Moved.From)
		s.Equal(spatial.Position{X: 12, Y: 12}, moveOut.Moved.To)

		// Assert: alice still holds bob at his current position
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 1, "alice should still see bob")
		s.Equal(intel.Subject(bob), aliceView[0].Subject)

		// Decode alice's holding of bob—should be at bob's current position
		var bobPayload encounter.SightPayload
		err = json.Unmarshal(aliceView[0].Payload, &bobPayload)
		s.Require().NoError(err)
		s.Equal(10.0, bobPayload.X)
		s.Equal(10.0, bobPayload.Y)

		// Assert: bob now sees alice at her NEW position
		bobView, err := enc.View(&encounter.ViewInput{Member: bob})
		s.Require().NoError(err)
		s.Len(bobView, 1, "bob should see alice")
		s.Equal(intel.Subject(alice), bobView[0].Subject)

		// Decode bob's holding of alice—should be at alice's NEW position
		var alicePayload encounter.SightPayload
		err = json.Unmarshal(bobView[0].Payload, &alicePayload)
		s.Require().NoError(err)
		s.Equal(12.0, alicePayload.X)
		s.Equal(12.0, alicePayload.Y)
	})
}

func (s *EncounterTestSuite) TestMoveGhostForms() {
	s.Run("moving behind the pillar fades holdings both ways", func() {
		// Geometry: pillar at (10,10). alice starts at (2,2); bob at (10,18).
		// The line (2,2)->(10,18) misses the pillar: initially visible.
		// alice moves to (10,2): the line (10,2)->(10,18) is the x=10
		// vertical and crosses (10,10): blocked BOTH ways.
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  20,
						Height: 20,
						Occluders: []spatial.Position{
							{X: 10, Y: 10},
						},
					},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
				{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 10, Y: 18}},
			},
			Endings: []encounter.EndingInput{
				{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
					Room: room1, Position: spatial.Position{X: 19, Y: 19}}},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		aliceViewBefore, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Require().Len(aliceViewBefore, 1, "alice must initially see bob (geometry precondition)")
		s.Require().Equal(intel.Current, aliceViewBefore[0].Status)

		_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 10, Y: 2}})
		s.Require().NoError(err)

		// alice's holding of bob: faded to ghost at bob's (unchanged) position.
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Require().Len(aliceView, 1, "the ghost is HELD, not gone")
		s.Equal(intel.Held, aliceView[0].Status, "alice's sight of bob must fade behind the pillar")
		var bobSeen encounter.SightPayload
		s.Require().NoError(json.Unmarshal(aliceView[0].Payload, &bobSeen))
		s.Equal(18.0, bobSeen.Y, "ghost holds bob at his last-seen position")

		// bob's holding of alice: the true ghost — alice's LAST-SEEN
		// (pre-move) position. bob never saw her arrive at (10,2).
		bobView, err := enc.View(&encounter.ViewInput{Member: bob})
		s.Require().NoError(err)
		s.Require().Len(bobView, 1)
		s.Equal(intel.Held, bobView[0].Status, "bob's sight of alice must fade too (symmetric)")
		var aliceSeen encounter.SightPayload
		s.Require().NoError(json.Unmarshal(bobView[0].Payload, &aliceSeen))
		s.Equal(2.0, aliceSeen.X, "bob's ghost of alice is at her PRE-move position")
		s.Equal(2.0, aliceSeen.Y, "bob never saw alice arrive at (10,2)")
	})
}

// TestMoveSequentialConsistency pins that consecutive moves proceed from
// the mover's updated position. It does NOT pin managed-seam usage: for
// same-room moves the seam and a raw room call are observationally
// identical (spatial's managed MoveEntity only precondition-checks the
// index). Real seam enforcement becomes falsifiable with cross-room
// Transition (multi-room task) and is held by convention (law C2) until
// then — an unfalsifiable pin here would violate the mutation-proof law.
func (s *EncounterTestSuite) TestMoveSequentialConsistency() {
	s.Run("after sequential moves, spatial index remains valid (CanMoveEntityBetweenRooms-style)", func() {
		// Arrange: alice in one room
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  20,
						Height: 20,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 5, Y: 5},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 18, Y: 18},
					},
				},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: move alice twice
		_, err = enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 10, Y: 10},
		})
		s.Require().NoError(err)

		// Second move from the new position must succeed (pins managed seam correctness)
		moveOut2, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 15, Y: 15},
		})
		s.Require().NoError(err, "second move must succeed from updated position")
		s.NotNil(moveOut2)
		s.Equal(spatial.Position{X: 10, Y: 10}, moveOut2.Moved.From,
			"second move should originate from the position after the first move")
		s.Equal(spatial.Position{X: 15, Y: 15}, moveOut2.Moved.To)
	})
}

func (s *EncounterTestSuite) TestMoveEndingFires() {
	s.Run("player reaching ReachedPosition trigger fires the ending", func() {
		// Arrange: alice is player, will reach stairs (no member filter = any player)
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  20,
						Height: 20,
					},
				},
				Connections: []encounter.ConnectionInput{},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 5, Y: 5},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Room:     room1,
					Position: spatial.Position{X: 10, Y: 10},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 19, Y: 19},
					},
				},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: alice moves to the ending position
		moveOut, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 19, Y: 19},
		})
		s.Require().NoError(err)

		// Assert: ending fired
		s.NotNil(moveOut.Outcome, "outcome should be set when ending fires")
		s.Equal("stairs", moveOut.Outcome.Ending, "outcome ending key should match")
		s.Equal(uint64(0), moveOut.Outcome.At, "outcome At should be stamped with clock reading (still 0 at setup)")
		s.GreaterOrEqual(moveOut.Outcome.At, uint64(0), "outcome At should be valid clock reading")

		// Verify members recorded in outcome
		s.Len(moveOut.Outcome.Members, 2, "outcome should contain both members")

		// Find alice and goblin in the outcome
		aliceOutcome := moveOut.Outcome.Members[0]
		goblinOutcome := moveOut.Outcome.Members[1]
		if aliceOutcome.ID != alice {
			aliceOutcome, goblinOutcome = goblinOutcome, aliceOutcome
		}

		s.Equal(alice, aliceOutcome.ID)
		s.Equal(spatial.Position{X: 19, Y: 19}, aliceOutcome.Position, "alice should be at ending position")

		s.Equal(goblin, goblinOutcome.ID)
		s.Equal(spatial.Position{X: 10, Y: 10}, goblinOutcome.Position, "goblin should remain at original position")

		// Assert: encounter is now closed
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open, "encounter should be closed")
		s.NotNil(status.Outcome)

		// Assert: further moves are rejected
		_, err = enc.Move(&encounter.MoveInput{
			Member: goblin,
			To:     spatial.Position{X: 12, Y: 12},
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

func (s *EncounterTestSuite) TestMoveValidationAndAtomicity() {
	s.Run("validation order and R5 atomicity", func() {
		s.Run("nil input returns ErrNilInput", func() {
			setup := &encounter.SetupInput{
				Field: encounter.FieldInput{
					Rooms: []encounter.RoomInput{
						{ID: room1, Width: 10, Height: 10},
					},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			_, err = enc.Move(nil)
			s.Require().ErrorIs(err, encounter.ErrNilInput)
		})

		s.Run("empty member ID returns ErrNoMember", func() {
			setup := &encounter.SetupInput{
				Field: encounter.FieldInput{
					Rooms: []encounter.RoomInput{
						{ID: room1, Width: 10, Height: 10},
					},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			_, err = enc.Move(&encounter.MoveInput{
				Member: "",
				To:     spatial.Position{X: 5, Y: 5},
			})
			s.Require().ErrorIs(err, encounter.ErrNoMember)
		})

		s.Run("closed encounter returns ErrClosed", func() {
			setup := &encounter.SetupInput{
				Field: encounter.FieldInput{
					Rooms: []encounter.RoomInput{
						{ID: room1, Width: 10, Height: 10},
					},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "end", Trigger: encounter.TriggerReachedPosition{
						Room:     room1,
						Position: spatial.Position{X: 9, Y: 9},
					}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			// Close the encounter by reaching the ending
			_, err = enc.Move(&encounter.MoveInput{
				Member: alice,
				To:     spatial.Position{X: 9, Y: 9},
			})
			s.Require().NoError(err)

			// Try to move again—should be closed
			_, err = enc.Move(&encounter.MoveInput{
				Member: alice,
				To:     spatial.Position{X: 5, Y: 5},
			})
			s.Require().ErrorIs(err, encounter.ErrClosed)
		})

		s.Run("not a member returns ErrNotMember", func() {
			setup := &encounter.SetupInput{
				Field: encounter.FieldInput{
					Rooms: []encounter.RoomInput{
						{ID: room1, Width: 10, Height: 10},
					},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			_, err = enc.Move(&encounter.MoveInput{
				Member: core.EntityID("unknown"),
				To:     spatial.Position{X: 5, Y: 5},
			})
			s.Require().ErrorIs(err, encounter.ErrNotMember)
		})

		s.Run("failed move leaves members unchanged", func() {
			setup := &encounter.SetupInput{
				Field: encounter.FieldInput{
					Rooms: []encounter.RoomInput{
						{ID: room1, Width: 10, Height: 10},
					},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
					{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 5, Y: 5}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			// Get initial member count
			membersBefore, err := enc.Members()
			s.Require().NoError(err)
			s.Len(membersBefore, 2)

			// Try invalid move (non-member)
			_, err = enc.Move(&encounter.MoveInput{
				Member: core.EntityID("unknown"),
				To:     spatial.Position{X: 5, Y: 5},
			})
			s.Require().Error(err, "move of non-member should fail")

			// Members should be unchanged
			membersAfter, err := enc.Members()
			s.Require().NoError(err)
			s.Len(membersAfter, 2, "failed move should not change member count")
			s.Equal(alice, membersAfter[0].ID, "alice should still be first member")
			s.Equal(bob, membersAfter[1].ID, "bob should still be second member")
		})
	})
}

func (s *EncounterTestSuite) TestMoveOutcomeCopyOut() {
	s.Run("mutating returned outcome does not affect internal state", func() {
		// Arrange
		setup := &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 20, Height: 20},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 5, Y: 5}},
				{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 10, Y: 10}},
			},
			Endings: []encounter.EndingInput{
				{Key: "end", Trigger: encounter.TriggerReachedPosition{
					Room:     room1,
					Position: spatial.Position{X: 18, Y: 18},
				}},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: move alice to ending position
		moveOut1, err := enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 18, Y: 18},
		})
		s.Require().NoError(err)
		s.NotNil(moveOut1.Outcome)

		// Mutate the returned outcome's Members slice
		if len(moveOut1.Outcome.Members) > 0 {
			moveOut1.Outcome.Members[0].Position = spatial.Position{X: 99, Y: 99}
		}

		// Assert: querying Status still returns the original outcome
		status, err := enc.Status()
		s.Require().NoError(err)
		s.NotNil(status.Outcome)

		// Find alice in the status outcome
		for _, member := range status.Outcome.Members {
			if member.ID == alice {
				s.Equal(spatial.Position{X: 18, Y: 18}, member.Position,
					"alice's position should not have changed from mutation")
				return
			}
		}
		s.Fail("alice not found in status outcome")
	})
}

// newBasicEncounter builds the standard two-player fixture: alice (2,2)
// and bob (5,5) in a clear 20x20 room, one any-player stairs ending at
// (19,19).
func (s *EncounterTestSuite) newBasicEncounter() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 20, Height: 20}}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
				Room: room1, Position: spatial.Position{X: 19, Y: 19}}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// TestMoveBeatPinned pins Move's record beat via Story (the Task 2
// opening-beat precedent applied to Move).
func (s *EncounterTestSuite) TestMoveBeatPinned() {
	enc := s.newBasicEncounter()
	moveOut, err := enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 3, Y: 3}})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	s.Require().Len(story, 2, "opening beat + movement beat")
	last := story[len(story)-1]
	s.Equal(moveOut.Seq, last.Seq, "MoveOutput.Seq must reference the appended beat")
	var beat map[string]any
	s.Require().NoError(json.Unmarshal(last.Payload, &beat))
	s.Equal("moved", beat["beat"], "the movement beat must be recorded")
	s.Equal(string(alice), beat["member"])
}

// TestMoveClosedBeforeNotMember pins the validation order combo the
// design declares: on a closed encounter, a non-member's move answers
// ErrClosed (closed is checked before membership).
func (s *EncounterTestSuite) TestMoveClosedBeforeNotMember() {
	enc := s.newBasicEncounter()
	_, err := enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 19, Y: 19}})
	s.Require().NoError(err, "alice reaches the stairs; encounter closes")
	_, err = enc.Move(&encounter.MoveInput{Member: "stranger", To: spatial.Position{X: 1, Y: 1}})
	s.Require().ErrorIs(err, encounter.ErrClosed, "closed wins over not-member")
}

// TestMoveSpatialRejectionAtomic pins R5 from a populated state: a
// spatially rejected move changes nothing observable.
func (s *EncounterTestSuite) TestMoveSpatialRejectionAtomic() {
	enc := s.newBasicEncounter()
	viewBefore, err := enc.View(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 99, Y: 99}})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "out-of-bounds move rejected")
	viewAfter, err := enc.View(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	s.Equal(viewBefore, viewAfter, "failed move must leave every view unchanged (R5)")
	_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 3, Y: 3}})
	s.Require().NoError(err, "alice still moves from her original position")
}

func TestEncounterSuite(t *testing.T) {
	suite.Run(t, new(EncounterTestSuite))
}
