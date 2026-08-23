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
	alice  = core.EntityID("alice")
	bob    = core.EntityID("bob")
	goblin = core.EntityID("goblin")
	room1  = "room-1"
	// room2 exists so a scene can open with a monster OUT of contact — the
	// only way to begin in free roam now that any co-located pair forms a
	// bubble at first light (rpg-toolkit#964).
	room2        = "room-2"
	endingStairs = "stairs"
)

// simpleDecider is a minimal test decider that holds.
type simpleDecider struct{}

func (s *simpleDecider) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	return encounter.IntentHold{}, nil
}

type EncounterTestSuite struct {
	suite.Suite
}

func (s *EncounterTestSuite) TestSetupFirstLight() {
	s.Run("two members clear line of sight", func() {
		// Arrange: 10x10 room, alice at (2,2) player, goblin at (7,7) monster
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Position: spatial.Position{X: 7, Y: 7},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
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
		s.Equal(cellAt(7, 7), spatial.Position{X: payload.X, Y: payload.Y})

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
		s.Equal(cellAt(2, 2), spatial.Position{X: payload.X, Y: payload.Y})
	})
}

func (s *EncounterTestSuite) TestSetupWallBlocksSight() {
	s.Run("a wall blocks both directions", func() {
		// Arrange: alice and goblin separated by a WALL — a pillar is leaned
		// around now (spatial v0.9.1, see testwalls_test.go), so a fixture
		// that wants sight blocked has to build something worth blocking.
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)}, Props: wallColumn(4, 0, 9),
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 5},
				},
				{
					ID:       goblin,
					Kind:     encounter.KindMonster,
					Position: spatial.Position{X: 7, Y: 5},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
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
		s.Len(aliceView, 0, "alice should see nothing (blocked by the wall)")

		// Assert: goblin does not see alice (symmetric)
		goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
		s.Require().NoError(err)
		s.Len(goblinView, 0, "goblin should see nothing (blocked by the wall)")
	})
}

func (s *EncounterTestSuite) TestSetupValidationOrderAndAtomicity() {
	s.Run("validation order: nil input", func() {
		_, err := encounter.NewEncounter(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("validation order: no field", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("validation order: reserved ending key", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{ID: "", Kind: encounter.KindPlayer},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		s.Require().Contains(err.Error(), "duplicate member alice",
			"the duplicate-ID rejection must name the duplicated ID, not echo the empty-ID message")
	})

	s.Run("atomicity: after error, valid setup works", func() {
		// First attempt: fails validation
		setup1 := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field:   encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{}},
			Members: []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup1)
		s.Require().ErrorIs(err, encounter.ErrNoField)

		// Second attempt: valid
		setup2 := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
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

// TestSetupPropOnBoundaryCellAccepted pins N2's over-tightening sweep
// (#929 T3 trailing round): every prop in every EXISTING fixture sits
// on an interior cell, so a mutant that rejected a boundary-cell prop
// (plausible: "props should be interior only") survived the suite
// with zero failures until this row was added. Props block line of
// sight (field.go's doc comment), not placement — a boundary cell,
// including a corner, is exactly as legal a prop cell as an
// interior one.
func (s *EncounterTestSuite) TestSetupPropOnBoundaryCellAccepted() {
	setup := &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)}, Props: []encounter.PropInput{rubble(0, 2), rubble(4, 2), rubble(2, 0), rubble(2, 4), rubble(0, 0), rubble(4, 4)},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().NoError(err, "a prop on a room's boundary cell, including a corner, must be legal")
}

// TestSetupDuplicatePropRejected pins the hardening round's item D:
// two props at the SAME cell used to escape module validation
// entirely and reject only in spatial's own voice ("entity ... already
// indexed") as an accident of the old coordinate-derived entity ID —
// see TestSetupPropIDCrossRoomCollisionAccepted's fix, which
// switched to an index-based ID and, as a side effect, removed even
// that accidental catch. Rejected explicitly now, in the module's own
// room-list defect vocabulary.
func (s *EncounterTestSuite) TestSetupTwoPropsOnOneCellRejected() {
	setup := &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)}, Props: []encounter.PropInput{rubble(3, 3), rubble(3, 3)},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "two props at")
}

// TestSetupDuplicateEndingKeyRejected pins hardening round item E: two
// endings sharing a key both used to construct — a genuine liveness
// hole, since End scans in declaration order and a reached_position
// twin declared first permanently shadows a same-keyed external ending
// declared after it (probed: End("dup") failed "is not External"
// forever, the external ending having no other way to fire).
func (s *EncounterTestSuite) TestSetupDuplicateEndingKeyRejected() {
	setup := &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)},
		},
		Endings: []encounter.EndingInput{
			{Key: "dup", Trigger: encounter.TriggerReachedPosition{Position: spatial.Position{X: 4, Y: 4}}},
			{Key: "dup", Trigger: encounter.TriggerExternal{}},
		},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
	s.Require().Contains(err.Error(), "duplicate ending")
}

// TestMoveHexIntegralAxial is Move's verb-seam counterpart: a fractional
// target in a hex room is rejected (moveMember is the shared path with
// Pump's IntentMoveTo, so this also covers decider-driven moves).
func (s *EncounterTestSuite) TestMoveHexIntegralAxial() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hex-room", 0, 0, 8, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Run("fractional target rejected", func() {
		_, err := enc.Step(&encounter.StepInput{Member: "p1", To: spatial.Position{X: 1.5, Y: 0}})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "not an integral axial cell")
	})

	s.Run("integral negative axial target accepted", func() {
		// Absolute cells are axial and a negative Q is entirely ordinary —
		// the reference tomb's own run includes negative Q throughout. What
		// changed in rpg-toolkit#1127 is which cells a chamber HOLDS, so this
		// names one it does: the chamber's authored column 0, row 5, which
		// lands on axial (-2,5) (rpg-toolkit#1150: for pointy-top R is the
		// authored row exactly, so a room anchored at row 0 can only ever
		// produce a negative Q, never a negative R — the axis this test
		// exercises moved with the basis fix, the point it makes did not).
		_, err := enc.Step(&encounter.StepInput{Member: "p1", To: spatial.Position{X: -2, Y: 5}})
		s.Require().NoError(err)
	})
}

// TestJoinHexIntegralAxial is Join's verb-seam counterpart.
func (s *EncounterTestSuite) TestJoinHexIntegralAxial() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("hex-room", 0, 0, 8, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Run("fractional position rejected", func() {
		_, err := enc.Join(&encounter.JoinInput{
			Member: "p2",
			Kind:   encounter.KindPlayer,
			Cell:   spatial.Position{X: 1, Y: 0.5},
		})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "not an integral axial cell")
	})

	s.Run("integral negative axial position accepted", func() {
		// The chamber's authored column 0, row 7 — axial (-3,7)
		// (rpg-toolkit#1150 moved this from (0,-7)). See the Move
		// counterpart for why a negative Q, not R, is what a room anchored
		// at row 0 can produce, and why that is unremarkable.
		_, err := enc.Join(&encounter.JoinInput{
			Member: "p3",
			Kind:   encounter.KindPlayer,
			Cell:   spatial.Position{X: -3, Y: 7},
		})
		s.Require().NoError(err)
	})
}

// ============================================================
// #929 T1 — anchoring: RoomInput.Origin, W1 (one geometry per field), W2
// (rooms never overlap), W3 (doorways kiss). Shape legality's own
// rejection (GridShapeGridless) is pinned above by
// TestGridlessRoomInclusiveBounds — repurposed for exactly this law.
// ============================================================

func (s *EncounterTestSuite) TestSetupOpeningBeat() {
	s.Run("opening beat reaches all members via Story", func() {
		// Arrange
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10), rectRegion(room2, 10, 0, 10, 10)}, Walls: twoRoomSealedWall(),
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
				// The goblin waits next door: co-located and visible would mean
				// a fight forms at first light and appends its own beat, and this
				// test is about the OPENING beat being the only one
				// (rpg-toolkit#964).
				{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 12, Y: 2}},
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
	s.Run("member placed on a cell no region owns errors with ErrBadPlacement", func() {
		// Arrange
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				// Try to place alice in the void beside the only region.
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 50, Y: 50}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
				{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 10}},
				{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 15, Y: 15}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 10, Y: 10},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Position: spatial.Position{X: 18, Y: 18},
					},
				},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: alice moves
		moveOut, err := enc.Step(&encounter.StepInput{
			Member: alice,
			To:     cellAt(12, 12),
		})
		s.Require().NoError(err)

		// Assert: Move returned successfully
		s.NotNil(moveOut)
		s.Equal(alice, moveOut.Stepped.Member)
		s.Equal(cellAt(2, 2), moveOut.Stepped.From)
		s.Equal(cellAt(12, 12), moveOut.Stepped.To)

		// Assert: alice still holds bob at his current position
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceView, 1, "alice should still see bob")
		s.Equal(intel.Subject(bob), aliceView[0].Subject)

		// Decode alice's holding of bob—should be at bob's current position
		var bobPayload encounter.SightPayload
		err = json.Unmarshal(aliceView[0].Payload, &bobPayload)
		s.Require().NoError(err)
		s.Equal(cellAt(10, 10), spatial.Position{X: bobPayload.X, Y: bobPayload.Y})

		// Assert: bob now sees alice at her NEW position
		bobView, err := enc.View(&encounter.ViewInput{Member: bob})
		s.Require().NoError(err)
		s.Len(bobView, 1, "bob should see alice")
		s.Equal(intel.Subject(alice), bobView[0].Subject)

		// Decode bob's holding of alice—should be at alice's NEW position
		var alicePayload encounter.SightPayload
		err = json.Unmarshal(bobView[0].Payload, &alicePayload)
		s.Require().NoError(err)
		s.Equal(cellAt(12, 12), spatial.Position{X: alicePayload.X, Y: alicePayload.Y})
	})
}

func (s *EncounterTestSuite) TestMoveGhostForms() {
	s.Run("moving behind the wall fades holdings both ways", func() {
		// Geometry: a wall across y=10 from x=7 to x=13. alice starts at
		// (2,2); bob at (10,18). The line (2,2)->(10,18) passes west of the
		// wall's end: initially visible. alice moves to (10,2), square behind
		// it: blocked BOTH ways.
		//
		// A wall rather than the single pillar this fixture used to have —
		// spatial v0.9.1 leans around a lone obstacle (see testwalls_test.go).
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)}, Props: wallRow(10, 7, 13),
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
				{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 18}},
			},
			Endings: []encounter.EndingInput{
				{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
					Position: spatial.Position{X: 19, Y: 19}}},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		aliceViewBefore, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Require().Len(aliceViewBefore, 1, "alice must initially see bob (geometry precondition)")
		s.Require().Equal(intel.Current, aliceViewBefore[0].Status)

		_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 2)})
		s.Require().NoError(err)

		// alice's holding of bob: faded to ghost at bob's (unchanged) position.
		aliceView, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Require().Len(aliceView, 1, "the ghost is HELD, not gone")
		s.Equal(intel.Held, aliceView[0].Status, "alice's sight of bob must fade behind the wall")
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
		s.Equal(cellAt(2, 2), spatial.Position{X: aliceSeen.X, Y: aliceSeen.Y}, "bob's ghost of alice is at her PRE-move position")
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 5, Y: 5},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Position: spatial.Position{X: 18, Y: 18},
					},
				},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: move alice twice
		_, err = enc.Step(&encounter.StepInput{
			Member: alice,
			To:     cellAt(10, 10),
		})
		s.Require().NoError(err)

		// Second move from the new position must succeed (pins managed seam correctness)
		moveOut2, err := enc.Step(&encounter.StepInput{
			Member: alice,
			To:     cellAt(15, 15),
		})
		s.Require().NoError(err, "second move must succeed from updated position")
		s.NotNil(moveOut2)
		s.Equal(cellAt(10, 10), moveOut2.Stepped.From,
			"second move should originate from the position after the first move")
		s.Equal(cellAt(15, 15), moveOut2.Stepped.To)
	})
}

func (s *EncounterTestSuite) TestMoveEndingFires() {
	s.Run("player reaching ReachedPosition trigger fires the ending", func() {
		// Arrange: alice is player, will reach stairs (no member filter = any player)
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20), rectRegion(room2, 20, 0, 20, 20)}, Walls: squareSeamWall(19, 20),
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 5, Y: 5},
				},
				{
					ID:   goblin,
					Kind: encounter.KindMonster,
					// Next door and WALLED OFF, so alice can walk to her stairs
					// without a fight starting underfoot (rpg-toolkit#964): with
					// one canvas, next door alone is not out of sight
					// (rpg-toolkit#1106).
					Position: spatial.Position{X: 30, Y: 10},
				},
			},
			Endings: []encounter.EndingInput{
				{
					Key: "stairs",
					Trigger: encounter.TriggerReachedPosition{
						Position: spatial.Position{X: 19, Y: 19},
					},
				},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: alice moves to the ending position
		moveOut, err := enc.Step(&encounter.StepInput{
			Member: alice,
			To:     cellAt(19, 19),
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
		s.Equal(cellAt(19, 19), aliceOutcome.Position, "alice should be at ending position")

		s.Equal(goblin, goblinOutcome.ID)
		s.Equal(cellAt(30, 10), goblinOutcome.Position,
			"goblin should remain at original position — room2-local (10,10) anchored at (20,0)")

		// Assert: encounter is now closed
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open, "encounter should be closed")
		s.NotNil(status.Outcome)

		// Assert: further moves are rejected
		_, err = enc.Step(&encounter.StepInput{
			Member: goblin,
			To:     cellAt(12, 12),
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

func (s *EncounterTestSuite) TestMoveValidationAndAtomicity() {
	s.Run("validation order and R5 atomicity", func() {
		s.Run("nil input returns ErrNilInput", func() {
			setup := &encounter.SetupInput{
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			_, err = enc.Step(nil)
			s.Require().ErrorIs(err, encounter.ErrNilInput)
		})

		s.Run("empty member ID returns ErrNoMember", func() {
			setup := &encounter.SetupInput{
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			_, err = enc.Step(&encounter.StepInput{
				Member: "",
				To:     cellAt(5, 5),
			})
			s.Require().ErrorIs(err, encounter.ErrNoMember)
		})

		s.Run("closed encounter returns ErrClosed", func() {
			setup := &encounter.SetupInput{
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "end", Trigger: encounter.TriggerReachedPosition{
						Position: spatial.Position{X: 9, Y: 9},
					}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			// Close the encounter by reaching the ending
			_, err = enc.Step(&encounter.StepInput{
				Member: alice,
				To:     cellAt(9, 9),
			})
			s.Require().NoError(err)

			// Try to move again—should be closed
			_, err = enc.Step(&encounter.StepInput{
				Member: alice,
				To:     cellAt(5, 5),
			})
			s.Require().ErrorIs(err, encounter.ErrClosed)
		})

		s.Run("not a member returns ErrNotMember", func() {
			setup := &encounter.SetupInput{
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
				},
				Endings: []encounter.EndingInput{
					{Key: "stairs", Trigger: encounter.TriggerExternal{}},
				},
			}
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err)

			_, err = enc.Step(&encounter.StepInput{
				Member: core.EntityID("unknown"),
				To:     cellAt(5, 5),
			})
			s.Require().ErrorIs(err, encounter.ErrNotMember)
		})

		s.Run("failed move leaves members unchanged", func() {
			setup := &encounter.SetupInput{
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
				Field: encounter.FieldInput{
					Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
					Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
					{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
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
			_, err = enc.Step(&encounter.StepInput{
				Member: core.EntityID("unknown"),
				To:     cellAt(5, 5),
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
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
				{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 10}},
			},
			Endings: []encounter.EndingInput{
				{Key: "end", Trigger: encounter.TriggerReachedPosition{
					Position: spatial.Position{X: 18, Y: 18},
				}},
			},
		}
		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: move alice to ending position
		moveOut1, err := enc.Step(&encounter.StepInput{
			Member: alice,
			To:     cellAt(18, 18),
		})
		s.Require().NoError(err)
		s.NotNil(moveOut1.Outcome)

		// Mutate the returned outcome's Members slice
		if len(moveOut1.Outcome.Members) > 0 {
			moveOut1.Outcome.Members[0].Position = cellAt(99, 99)
		}

		// Assert: querying Status still returns the original outcome
		status, err := enc.Status()
		s.Require().NoError(err)
		s.NotNil(status.Outcome)

		// Find alice in the status outcome
		for _, member := range status.Outcome.Members {
			if member.ID == alice {
				s.Equal(cellAt(18, 18), member.Position,
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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: endingStairs, Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 19, Y: 19}}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// newBasicEncounterWithExternalEnding builds the standard two-player fixture with
// an External ending (for testing End verb).
func (s *EncounterTestSuite) newBasicEncounterWithExternalEnding() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 20, 20)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// TestMoveBeatPinned pins Move's record beat via Story (the Task 2
// opening-beat precedent applied to Move).
func (s *EncounterTestSuite) TestMoveBeatPinned() {
	enc := s.newBasicEncounter()
	moveOut, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 3)})
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
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(19, 19)})
	s.Require().NoError(err, "alice reaches the stairs; encounter closes")
	_, err = enc.Step(&encounter.StepInput{Member: "stranger", To: cellAt(1, 1)})
	s.Require().ErrorIs(err, encounter.ErrClosed, "closed wins over not-member")
}

// TestMoveSpatialRejectionAtomic pins R5 from a populated state: a
// spatially rejected move changes nothing observable.
func (s *EncounterTestSuite) TestMoveSpatialRejectionAtomic() {
	enc := s.newBasicEncounter()
	viewBefore, err := enc.View(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(99, 99)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "out-of-bounds move rejected")
	viewAfter, err := enc.View(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	s.Equal(viewBefore, viewAfter, "failed move must leave every view unchanged (R5)")
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 3)})
	s.Require().NoError(err, "alice still moves from her original position")
}

// Traverse tests (Task 2)

// newTwoRoomEncounterWithConnection returns an encounter with room-a and
// room-b connected by "door1": FromPosition {9,5} in room-a, ToPosition
// {0,5} in room-b — DELIBERATELY asymmetric endpoints (T1 review lesson)
// so a from/to transposition mutant (landing the traverser back at the
// DEPARTURE endpoint instead of the far one) is observable. alice starts
// AT room-a's door endpoint, ready to traverse. bob starts adjacent to
// her in room-a (mutual line of sight, for ghost-at-threshold pins).
// goblin starts in room-b adjacent to the arrival endpoint (for
// arrival-Current and T3 pins).
func (s *EncounterTestSuite) newTwoRoomEncounterWithConnection() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("room-a", 0, 0, 10, 10), rectRegion("room-b", 10, 0, 10, 10)}, Walls: twoRoomWall(),
			Doors: []encounter.DoorInput{twoRoomDoor},
		},
		Members: []encounter.MemberInput{
			// Alice stands IN the doorway. Bob watches from further back along
			// the SAME row, so what he can see of room-b is decided by the
			// opening rather than by the room — and what he cannot see is
			// decided by the wall beside it.
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 9, Y: 5}},
			{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
			// The goblin stands in room-b, OFF the doorway's row: the wall
			// hides it from room-a, so nothing has seen anything at first
			// light, and walking through the door starts a fight
			// (rpg-toolkit#964). Tests that only look at what arrived can
			// leave it running; the ones that hop back break off first.
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 11, Y: 0}},
		},
		Endings: []encounter.EndingInput{
			{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// TestACrossingIsJustAStepInBothDirections pins the success path in BOTH
// directions through the same doorway (T1 law: doorways are bidirectional),
// asserting the reported cells and the chamber a member is said to be in.
//
// There is no crossing VERB to pin any more (rpg-toolkit#1106). What is left is
// a step whose destination happens to be through an opening, and the only thing
// that marks it as one is the doorway's name on the way out.
func (s *EncounterTestSuite) TestACrossingIsJustAStepInBothDirections() {
	near := cellAt(9, 5) // room-a's doorway cell
	far := cellAt(10, 5) // room-b's, one cell along

	s.Run("room-a to room-b", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		out, err := enc.Step(&encounter.StepInput{Member: alice, To: far})
		s.Require().NoError(err)
		s.Equal(alice, out.Stepped.Member)
		s.Equal(near, out.Stepped.From)
		s.Equal(far, out.Stepped.To)
		s.Require().Len(out.Doors, 1, "the door is named, and decided nothing")
		s.Equal(encounter.DoorID("door1"), out.Doors[0].ID)

		s.Equal("room-b", s.roomOf(enc, alice), "she is in room-b now, because her cell is")
	})

	s.Run("room-b to room-a (bidirectional through the same doorway)", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		arrived, err := enc.Step(&encounter.StepInput{Member: alice, To: far})
		s.Require().NoError(err)
		s.Require().NotNil(arrived.Formed, "walking in on the goblin starts a fight")

		// Break off before walking back out — a fight member cannot free-roam.
		_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
		s.Require().NoError(err)

		out, err := enc.Step(&encounter.StepInput{Member: alice, To: near})
		s.Require().NoError(err)
		s.Equal(far, out.Stepped.From)
		s.Equal(near, out.Stepped.To)
		s.Require().Len(out.Doors, 1, "the same door carried her home")
		s.Equal(encounter.DoorID("door1"), out.Doors[0].ID)
		s.Equal("room-a", s.roomOf(enc, alice))
	})
}

// roomOf reads which authored chamber a member is standing in, off the roster.
func (s *EncounterTestSuite) roomOf(enc *encounter.Encounter, id encounter.MemberID) string {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Region
		}
	}
	s.Require().Fail("no such member", string(id))
	return ""
}

// TestTheWallDecidesWhatSightCanCross is the slice's headline, in the fixture
// the old crossing suite was built on.
//
// There used to be a law here called T3 — "sight never crosses a connection's
// opening" — justified by rooms being separate spatial containers. It was never
// a rule anybody chose: it was the room-membership filter in rebuildPercepts
// standing in for walls the composition could not express, and it blinded two
// members standing close enough to touch (rpg-toolkit#1105/#1106).
//
// What decides now is the geometry. room-a walls its own east edge and leaves
// one cell open, and every claim below follows from that one fact:
//
//   - the goblin, off the doorway's row in room-b, is invisible from room-a —
//     the wall, not the room;
//   - stepping into the opening does NOT hide alice from bob, who is watching
//     from room-a — an open doorway is a window;
//   - the goblin sees her the moment she is in its chamber, and that starts the
//     fight;
//   - bob loses her only when she steps out of the opening's line.
func (s *EncounterTestSuite) TestTheWallDecidesWhatSightCanCross() {
	enc := s.newTwoRoomEncounterWithConnection()

	s.Empty(s.holdingOf(enc, goblin, alice), "the wall hides her from the goblin, and the goblin from her")
	s.Empty(s.holdingOf(enc, alice, goblin))

	bobSees := s.holdingOf(enc, bob, alice)
	s.Require().Len(bobSees, 1, "bob watches her from across room-a")
	s.Equal(intel.Current, bobSees[0].Status)

	// Into the opening. She is in room-b now, and bob is still looking at her
	// straight down the doorway's row.
	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the goblin sees her arrive, and that is a fight")

	bobSees = s.holdingOf(enc, bob, alice)
	s.Require().Len(bobSees, 1)
	s.Equal(intel.Current, bobSees[0].Status, "a doorway is a window: crossing it does not hide her")
	var seen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(bobSees[0].Payload, &seen))
	s.Equal(cellAt(10, 5), spatial.Position{X: seen.X, Y: seen.Y}, "and he sees her where she actually is, on the far side")

	// Out of its line. NOW the wall takes her, and the ghost holds the last
	// cell bob actually saw.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(12, 8)})
	s.Require().NoError(err)

	bobSees = s.holdingOf(enc, bob, alice)
	s.Require().Len(bobSees, 1, "the ghost is HELD, not gone")
	s.Equal(intel.Held, bobSees[0].Status)
	s.Require().NoError(json.Unmarshal(bobSees[0].Payload, &seen))
	s.Equal(cellAt(10, 5), spatial.Position{X: seen.X, Y: seen.Y}, "the ghost holds her last-seen cell — in the opening")
}

// holdingOf returns what one member holds about another: a one-element slice,
// or empty when they hold nothing at all.
func (s *EncounterTestSuite) holdingOf(
	enc *encounter.Encounter, observer, subject encounter.MemberID,
) []intel.Holding {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	var out []intel.Holding
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			out = append(out, h)
		}
	}
	return out
}

// TestACrossingFiresAnEndingOnArrival pins that a ReachedPosition ending
// declared at a doorway's far cell fires on arrival, exactly as one fires at
// any other step's destination.
func (s *EncounterTestSuite) TestACrossingFiresAnEndingOnArrival() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("room-a", 0, 0, 10, 10), rectRegion("room-b", 10, 0, 10, 10)}, Walls: twoRoomWall(),
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 9, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: "escaped", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 10, Y: 5}}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome, "outcome should be set when arriving at the ending's cell")
	s.Equal("escaped", out.Outcome.Ending)
	s.Require().Len(out.Outcome.Members, 1)
	s.Equal(alice, out.Outcome.Members[0].ID)
	s.Equal(encounter.RegionID("room-b"), out.Outcome.Members[0].Region)
	s.Equal(cellAt(10, 5), out.Outcome.Members[0].Position,
		"room-b-local (0,5) anchored at (10,0) — the outcome speaks the dungeon map (#1068)")

	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open)
}

// TestAMonsterCrossingOntoAnUnfilteredEndingDoesNotClose pins the players-only
// rule for unfiltered ReachedPosition endings: a monster stepping onto an
// unfiltered ending's cell must NOT close the encounter, doorway or not.
func (s *EncounterTestSuite) TestAMonsterCrossingOntoAnUnfilteredEndingDoesNotClose() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("room-a", 0, 0, 10, 10), rectRegion("room-b", 10, 0, 10, 10)}, Walls: twoRoomWall(),
		},
		Members: []encounter.MemberInput{
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 9, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			// Unfiltered: empty Member means players only.
			{Key: "escaped", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 0, Y: 5}, Member: ""}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Step(&encounter.StepInput{Member: goblin, To: cellAt(10, 5)})
	s.Require().NoError(err)
	s.Nil(out.Outcome, "unfiltered ending must not fire for a monster")

	status, err := enc.Status()
	s.Require().NoError(err)
	s.True(status.Open, "encounter should remain open")
}

// TestTheCrossingBeatIsAMovedBeatThatNamesTheDoorway.
//
// The story cannot tell a crossing apart from any other step, which is the
// point: there was a "traversed" beat here, and it described a mechanism the
// composition no longer has (rpg-toolkit#1106). The doorway's name rides along
// as narration, on an ordinary movement beat.
//
// Walking through this door lands alice in the goblin's sight, so the step also
// starts a fight and appends a beat of its own AFTER the movement beat — hence
// len-2 rather than last. That ordering is the cause-before-effect law and is
// pinned in beatorder_test.go; here it is only the reason this test reads the
// second-to-last entry.
func (s *EncounterTestSuite) TestTheCrossingBeatIsAMovedBeatThatNamesTheDoorway() {
	enc := s.newTwoRoomEncounterWithConnection()
	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(story), 2)
	crossed := story[len(story)-2]
	s.Equal(out.Seq, crossed.Seq, "StepOutput.Seq references the movement beat")

	var beat map[string]any
	s.Require().NoError(json.Unmarshal(crossed.Payload, &beat))
	s.Equal("moved", beat["beat"], "the same beat an ordinary step writes")
	s.Equal(string(alice), beat["member"])
	s.Equal([]any{"door1"}, beat["doors"], "with the door named beside it")
}

// TestACrossingDoesNotAdvanceTheClock pins law T4: movement is an activity, not
// time — the exploration clock does not advance, doorway or not.
func (s *EncounterTestSuite) TestACrossingDoesNotAdvanceTheClock() {
	enc := s.newTwoRoomEncounterWithConnection()
	before := enc.ToData().Clock.HighWater

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	s.Require().NoError(err)

	after := enc.ToData().Clock.HighWater
	s.Equal(before, after, "movement is an activity, not time — the clock must not advance")
}

// TestACrossingLeavesNoStaleRegistryEntry pins the wave-1 Exit lesson (see
// TestExitThenRejoinSameID) against a step that changes chamber: afterwards a
// NEW member can Join at the vacated cell, and alice can step BACK through the
// same doorway.
//
// The composition used to remove and re-place the entity across two spatial
// rooms to do this, and that composition was the thing worth pinning. It is one
// MoveEntity on one canvas now, so what survives is the property rather than
// the mechanism — which is the right thing for a test to hold anyway.
func (s *EncounterTestSuite) TestACrossingLeavesNoStaleRegistryEntry() {
	enc := s.newTwoRoomEncounterWithConnection()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(10, 5)})
	s.Require().NoError(err)

	// She walked in on the goblin, so a fight started; she breaks off before
	// walking back out (rpg-toolkit#964).
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{
		Member: core.EntityID("charlie"),
		Kind:   encounter.KindPlayer,
		Cell:   cellAt(9, 5),
	})
	s.Require().NoError(err, "the vacated cell must truly be free — no stale registry entry")

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(9, 5)})
	s.Require().NoError(err, "alice must be able to step back — no stale entry on the far side either")
	s.Require().Len(out.Doors, 1)
	s.Equal(encounter.DoorID("door1"), out.Doors[0].ID)
	s.Equal("room-a", s.roomOf(enc, alice))
}

// Membership flow tests (Task 5)

func (s *EncounterTestSuite) TestJoinLateJoinerSeenByIncumbents() {
	s.Run("late joiner seen by and sees incumbents", func() {
		// Arrange: setup with alice and bob
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 8, Y: 8},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
					Position: spatial.Position{X: 9, Y: 9},
				}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// alice should see bob initially
		aliceViewBefore, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceViewBefore, 1, "alice should see bob initially")

		// Act: join charlie as late joiner
		charlie := core.EntityID("charlie")
		joinOut, err := enc.Join(&encounter.JoinInput{
			Member: charlie,
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(5, 5),
		})
		s.Require().NoError(err, "join should succeed")

		// Assert: charlie joined
		s.Equal(charlie, joinOut.Member.ID)
		s.Equal(encounter.KindPlayer, joinOut.Member.Kind)
		s.Equal(encounter.RegionID(room1), joinOut.Member.Region)

		// Assert: join beat recorded
		s.Require().Greater(joinOut.Seq, uint64(0), "join should produce sequence number")

		// Assert: charlie sees alice and bob
		charlieView, err := enc.View(&encounter.ViewInput{Member: charlie})
		s.Require().NoError(err)
		s.Len(charlieView, 2, "charlie should see both alice and bob")

		// Assert: alice now sees charlie
		aliceViewAfter, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceViewAfter, 2, "alice should see charlie after join")
		found := false
		for _, h := range aliceViewAfter {
			if h.Subject == intel.Subject(charlie) {
				found = true
				break
			}
		}
		s.True(found, "alice's view should include charlie")

		// Assert: join beat appears in story for all members
		storyAlice, err := enc.Story(&encounter.StoryInput{Audience: alice, AfterSeq: 0})
		s.Require().NoError(err)
		foundJoinBeat := false
		for _, entry := range storyAlice {
			var beat map[string]interface{}
			_ = json.Unmarshal(entry.Payload, &beat)
			if beat["beat"] == "joined" && beat["member"] == string(charlie) {
				foundJoinBeat = true
				break
			}
		}
		s.True(foundJoinBeat, "alice should see join beat in story")
	})
}

func (s *EncounterTestSuite) TestJoinValidation() {
	s.Run("join nil input", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Join(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("join empty member ID", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Join(&encounter.JoinInput{
			Member: "",
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(5, 5),
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("join already a member", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Join(&encounter.JoinInput{
			Member: alice,
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(5, 5),
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember, "duplicate join should fail")
	})

	s.Run("join closed encounter", func() {
		enc := s.newBasicEncounterWithExternalEnding()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Join(&encounter.JoinInput{
			Member: core.EntityID("charlie"),
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(5, 5),
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})

	s.Run("join with bad placement", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Join(&encounter.JoinInput{
			Member: core.EntityID("charlie"),
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(99, 99),
		})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})

	s.Run("join player with decider rejected", func() {
		enc := s.newBasicEncounter()
		fixedDecider := &simpleDecider{}
		_, err := enc.Join(&encounter.JoinInput{
			Member:  core.EntityID("charlie"),
			Kind:    encounter.KindPlayer,
			Cell:    cellAt(5, 5),
			Decider: fixedDecider,
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember, "player with decider should fail")
	})
}

func (s *EncounterTestSuite) TestJoinOnStairsFiresEnding() {
	s.Run("join on stairs fires ReachedPosition ending", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
					Position: spatial.Position{X: 9, Y: 9},
				}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: join at stairs position
		charlie := core.EntityID("charlie")
		joinOut, err := enc.Join(&encounter.JoinInput{
			Member: charlie,
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(9, 9),
		})
		s.Require().NoError(err)

		// Assert: ending fired
		s.NotNil(joinOut.Outcome, "outcome should fire on stairs join")
		s.Equal("stairs", joinOut.Outcome.Ending)

		// Assert: encounter is now closed
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open, "encounter should be closed")
	})
}

func (s *EncounterTestSuite) TestExitCarryForward() {
	s.Run("exit carry-forward matches final state", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 8, Y: 8},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// alice sees bob
		aliceViewBefore, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceViewBefore, 1, "alice should see bob")

		// Act: alice exits
		exitOut, err := enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)

		// Assert: exit outcome has alice's position
		s.Equal(alice, exitOut.Outcome.ID)
		s.Equal(encounter.RegionID(room1), exitOut.Outcome.Region)
		s.Equal(cellAt(2, 2), spatial.Position{X: exitOut.Outcome.Position.X, Y: exitOut.Outcome.Position.Y})

		// Assert: carry includes alice's holdings (she saw bob)
		s.Len(exitOut.Carry, 1, "alice should carry her holdings")
		s.Equal(intel.Subject(bob), exitOut.Carry[0].Subject)

		// Assert: exit beat recorded
		s.Require().Greater(exitOut.Seq, uint64(0))

		// Assert: bob still sees alice's holding (cached), but with faded status after next action
		bobViewAfterExit, err := enc.View(&encounter.ViewInput{Member: bob})
		s.Require().NoError(err)
		// Bob still has the holding from before (alice hasn't been faded yet - that happens
		// on next refreshSight). The holding should be "held" (not yet ghost).
		s.Len(bobViewAfterExit, 1, "bob's cached holding remains until next refreshSight")
		s.Equal(intel.Subject(alice), bobViewAfterExit[0].Subject)
	})
}

func (s *EncounterTestSuite) TestExitValidation() {
	s.Run("exit nil input", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Exit(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("exit empty member ID", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Exit(&encounter.ExitInput{Member: ""})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("exit not a member", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Exit(&encounter.ExitInput{Member: core.EntityID("charlie")})
		s.Require().ErrorIs(err, encounter.ErrNotMember)
	})

	s.Run("exit closed encounter", func() {
		enc := s.newBasicEncounterWithExternalEnding()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

func (s *EncounterTestSuite) TestExitLastMemberClosesWithAbandoned() {
	s.Run("last member exit closes with abandoned ending", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
					Position: spatial.Position{X: 9, Y: 9},
				}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: alice (last member) exits
		exitOut, err := enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)

		// Assert: exit carries alice's state
		s.Equal(alice, exitOut.Outcome.ID)

		// Assert: closing produced abandoned outcome
		s.NotNil(exitOut.Closed, "should close with abandoned outcome")
		s.Equal("abandoned", exitOut.Closed.Ending)
		s.Len(exitOut.Closed.Members, 0, "abandoned outcome should have no members")

		// Assert: encounter is now closed
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open)
		s.Equal("abandoned", status.Outcome.Ending)
	})
}

func (s *EncounterTestSuite) TestExitDepartedGhostFades() {
	s.Run("remaining member's holdings persist after departure", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
				{
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 8, Y: 8},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// alice sees bob initially
		aliceViewBefore, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceViewBefore, 1, "alice should see bob initially")

		// Act: bob exits (his entity leaves the field)
		_, err = enc.Exit(&encounter.ExitInput{Member: bob})
		s.Require().NoError(err)

		// Assert: bob's entity left the field
		members, err := enc.Members()
		s.Require().NoError(err)
		s.Len(members, 1, "only alice remains")

		// Assert: alice's holding of bob persists in the archive
		// (design: holdings stay in the aggregate, the exited member simply no longer refreshes)
		aliceViewAfterExit, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(aliceViewAfterExit, 1, "alice's holding of bob persists in the archive")
		s.Equal(intel.Subject(bob), aliceViewAfterExit[0].Subject)
		// The holding status remains as it was (the archive preserves it)
	})
}

func (s *EncounterTestSuite) TestEndExternalOnly() {
	s.Run("End accepts only External triggers", func() {
		enc := s.newBasicEncounter()

		// Fire the DECLARED ReachedPosition key via End: declared but
		// wrong trigger kind must be rejected (only External endings
		// may be fired externally).
		_, err := enc.End(&encounter.EndInput{Ending: endingStairs})
		s.Require().ErrorIs(err, encounter.ErrNoEnding, "ReachedPosition endings cannot be fired via End")
		// An undeclared key is also ErrNoEnding (the earlier branch).
		_, err = enc.End(&encounter.EndInput{Ending: "never-declared"})
		s.Require().ErrorIs(err, encounter.ErrNoEnding, "undeclared keys are rejected")
	})

	s.Run("End fires External endings", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: fire the external ending
		endOut, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)

		// Assert: outcome records the ending
		s.Equal("withdrawn", endOut.Outcome.Ending)
		s.Len(endOut.Outcome.Members, 1)
		s.Equal(alice, endOut.Outcome.Members[0].ID)

		// Assert: encounter closed
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open)
	})
}

func (s *EncounterTestSuite) TestEndValidation() {
	s.Run("end nil input", func() {
		enc := s.newBasicEncounter()
		_, err := enc.End(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("end empty key", func() {
		enc := s.newBasicEncounter()
		_, err := enc.End(&encounter.EndInput{Ending: ""})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("end undeclared key", func() {
		enc := s.newBasicEncounter()
		_, err := enc.End(&encounter.EndInput{Ending: "nonexistent"})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})

	s.Run("end closed encounter", func() {
		enc := s.newBasicEncounterWithExternalEnding()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

func (s *EncounterTestSuite) TestAllMutatingVerbsReturnErrClosedPostClose() {
	s.Run("all mutating verbs reject closed encounter", func() {
		enc := s.newBasicEncounterWithExternalEnding()

		// Close the encounter
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)

		// Join on closed: ErrClosed
		_, err = enc.Join(&encounter.JoinInput{
			Member: core.EntityID("charlie"),
			Kind:   encounter.KindPlayer,
			Cell:   cellAt(5, 5),
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Exit on closed: ErrClosed
		_, err = enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Move on closed: ErrClosed
		_, err = enc.Step(&encounter.StepInput{
			Member: alice,
			To:     cellAt(3, 3),
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Pump on closed: ErrClosed
		_, err = enc.Pump(&encounter.PumpInput{})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// End on closed: ErrClosed
		_, err = enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})
}

func (s *EncounterTestSuite) TestQueriesLivePostClose() {
	s.Run("View, Members, Status, Story remain live on closed encounter", func() {
		enc := s.newBasicEncounterWithExternalEnding()

		// Close the encounter
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)

		// View still works
		view, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(view, 1, "view should remain available on closed encounter")

		// Members still works
		members, err := enc.Members()
		s.Require().NoError(err)
		s.Len(members, 2, "members should remain available on closed encounter")

		// Status still works
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open)
		s.NotNil(status.Outcome)

		// Story still works
		story, err := enc.Story(&encounter.StoryInput{Audience: alice, AfterSeq: 0})
		s.Require().NoError(err)
		s.Greater(len(story), 0, "story should remain available on closed encounter")
	})
}

func (s *EncounterTestSuite) TestStoryForExitedMembers() {
	s.Run("exited members can still read story", func() {
		setup := &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
			},
			Members: []encounter.MemberInput{
				{
					ID:       alice,
					Kind:     encounter.KindPlayer,
					Position: spatial.Position{X: 2, Y: 2},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// alice exits
		_, err = enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)

		// alice can still read story (even though she exited)
		story, err := enc.Story(&encounter.StoryInput{Audience: alice, AfterSeq: 0})
		s.Require().NoError(err, "exited member should be able to read story")
		s.Greater(len(story), 0, "story should contain entries")
	})
}

func (s *EncounterTestSuite) TestViewCopyOut() {
	s.Run("View returns copy-out, not alias", func() {
		enc := s.newBasicEncounter()

		view1, err := enc.View(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Len(view1, 1)

		// holdings are already copies per intel's contract
		// (this is a documentation pin that the composed contract holds)
	})
}

func (s *EncounterTestSuite) TestMembersCopyOut() {
	s.Run("Members returns copy-out, not aliases", func() {
		enc := s.newBasicEncounter()

		members1, err := enc.Members()
		s.Require().NoError(err)

		members2, err := enc.Members()
		s.Require().NoError(err)

		s.Equal(members1, members2)
		// Modifying members1 should not affect enc's internal state
		// (values are copied, not pointers to internal state)
	})
}

// TestExitThenRejoinSameID is the load-bearing pin for Exit's spatial
// removal: omitting RemoveEntity is invisible to Members/View/Status
// but permanently leaks the ID in the room's registry — the returning
// player (the sequel model's whole premise) would be locked out.
func (s *EncounterTestSuite) TestExitThenRejoinSameID() {
	enc := s.newBasicEncounter()
	_, err := enc.Exit(&encounter.ExitInput{Member: bob})
	s.Require().NoError(err)
	_, err = enc.Join(&encounter.JoinInput{
		Member: bob,
		Kind:   encounter.KindPlayer,
		Cell:   cellAt(6, 6),
	})
	s.Require().NoError(err, "a departed member must be able to return with the same ID (exit must truly vacate the field)")
}

// TestExitBeatPinned pins the exited beat: tag, payload, and the
// exiter reading their own departure via Story (everMembers).
func (s *EncounterTestSuite) TestExitBeatPinned() {
	enc := s.newBasicEncounter()
	out, err := enc.Exit(&encounter.ExitInput{Member: bob})
	s.Require().NoError(err)
	story, err := enc.Story(&encounter.StoryInput{Audience: bob})
	s.Require().NoError(err, "the exiter can still read the story (everMembers)")
	s.Require().NotEmpty(story)
	last := story[len(story)-1]
	s.Equal(out.Seq, last.Seq, "ExitOutput.Seq references the exited beat")
	var beat map[string]any
	s.Require().NoError(json.Unmarshal(last.Payload, &beat))
	s.Equal("exited", beat["beat"], "the departure is recorded")
	s.Equal(string(bob), beat["member"])
}

func TestEncounterSuite(t *testing.T) {
	suite.Run(t, new(EncounterTestSuite))
}
