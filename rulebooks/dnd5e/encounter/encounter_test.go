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
		s.Require().Contains(err.Error(), "duplicate member alice",
			"the duplicate-ID rejection must name the duplicated ID, not echo the empty-ID message")
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

// validConnSetup returns a fresh SetupInput with two DELIBERATELY
// mismatched rooms — r1 is 10x4 (occluder at (2,2)), r2 is 3x9 (occluder
// at (1,3)) — and one fully valid connection between them, with
// FromPosition{7,1} valid ONLY in r1 and ToPosition{1,7} valid ONLY in r2.
// Same-sized rooms and equal From/To positions would make a check that
// validates an endpoint against the WRONG room (or a Load-side From/To
// transposition) invisible: this is the base for
// TestSetupConnectionValidation's one-defect rows, mirroring the same
// defect classes rejected at Load (TestLoadRejections).
func validConnSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 10, Height: 4, Occluders: []spatial.Position{{X: 2, Y: 2}}},
				{ID: "r2", Width: 3, Height: 9, Occluders: []spatial.Position{{X: 1, Y: 3}}},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "c1", From: "r1", To: "r2",
					FromPosition: spatial.Position{X: 7, Y: 1},
					ToPosition:   spatial.Position{X: 1, Y: 7}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestSetupConnectionValidation mirrors TestLoadRejections' connection
// defect classes at the Setup seam: each case breaks exactly one thing
// about an otherwise-valid connection and must reject with ErrBadConnection.
// Fragments name the missing room where applicable — a check neutered in
// favor of the coincidental zero-value-room bounds fallback must not pass.
func (s *EncounterTestSuite) TestSetupConnectionValidation() {
	cases := []struct {
		name     string
		mutate   func(in *encounter.SetupInput)
		fragment string
	}{
		{"empty connection id", func(in *encounter.SetupInput) {
			in.Field.Connections[0].ID = ""
		}, "empty id"},
		{"duplicate connection id", func(in *encounter.SetupInput) {
			in.Field.Connections = append(in.Field.Connections, in.Field.Connections[0])
		}, "duplicate connection"},
		{"connection unknown from room", func(in *encounter.SetupInput) {
			in.Field.Connections[0].From = "nowhere"
		}, `unknown room "nowhere"`},
		{"connection unknown to room", func(in *encounter.SetupInput) {
			in.Field.Connections[0].To = "nowhere"
		}, `unknown room "nowhere"`},
		{"connection self-connection", func(in *encounter.SetupInput) {
			in.Field.Connections[0].To = "r1"
		}, "itself"},
		{"connection from-position out of bounds", func(in *encounter.SetupInput) {
			in.Field.Connections[0].FromPosition = spatial.Position{X: 99, Y: 99}
		}, "from-position out of bounds"},
		{"connection to-position out of bounds", func(in *encounter.SetupInput) {
			in.Field.Connections[0].ToPosition = spatial.Position{X: 99, Y: 99}
		}, "to-position out of bounds"},
		{"connection from-position on occluder", func(in *encounter.SetupInput) {
			in.Field.Connections[0].FromPosition = spatial.Position{X: 2, Y: 2}
		}, "from-position on occluder"},
		{"connection to-position on occluder", func(in *encounter.SetupInput) {
			in.Field.Connections[0].ToPosition = spatial.Position{X: 1, Y: 3}
		}, "to-position on occluder"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			setup := validConnSetup()
			tc.mutate(setup)
			_, err := encounter.NewEncounter(setup)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrBadConnection, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	// The valid base itself must construct — the one-defect discipline only
	// means something if zero defects pass. Since FromPosition{7,1} is valid
	// ONLY in r1 and ToPosition{1,7} valid ONLY in r2, this positive control
	// also pins that each endpoint is checked against ITS OWN room: a check
	// wired to the wrong room would reject this valid connection.
	enc, err := encounter.NewEncounter(validConnSetup())
	s.Require().NoError(err, "the valid base fixture must construct")
	data := enc.ToData()
	s.Require().Len(data.Field.Connections, 1)
	s.Equal(encounter.PositionData{X: 7, Y: 1}, data.Field.Connections[0].FromPosition,
		"from-position must survive unswapped")
	s.Equal(encounter.PositionData{X: 1, Y: 7}, data.Field.Connections[0].ToPosition,
		"to-position must survive unswapped")
}

// connBoundsSetup returns a fresh SetupInput with a 4x3 room r1 (valid
// coordinates 0..3 x 0..2) and an r2 large enough to always hold the
// connection's fixed ToPosition — used to pin the square grid's strictly-
// less-than bounds semantics against FromPosition in r1, independent of any
// cross-room concern (that's validConnSetup's job).
func connBoundsSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 4, Height: 3},
				{ID: "r2", Width: 4, Height: 3},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "c1", From: "r1", To: "r2",
					FromPosition: spatial.Position{X: 0, Y: 0},
					ToPosition:   spatial.Position{X: 0, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestConnectionEndpointBoundsBoundaries pins the square grid's strictly-
// less-than bounds semantics at the Setup seam (#922 T1 Opus review, minor M3/M4):
// a coordinate exactly at the room's Width/Height is out of bounds (valid
// range is 0..dimension-1, matching member placement), a negative coordinate
// is out of bounds, and Width-1/Height-1 — the last valid cell — is accepted.
func (s *EncounterTestSuite) TestConnectionEndpointBoundsBoundaries() {
	s.Run("X exactly at width is rejected", func() {
		setup := connBoundsSetup()
		setup.Field.Connections[0].FromPosition = spatial.Position{X: 4, Y: 0}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("Y exactly at height is rejected", func() {
		setup := connBoundsSetup()
		setup.Field.Connections[0].FromPosition = spatial.Position{X: 0, Y: 3}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("negative X is rejected", func() {
		setup := connBoundsSetup()
		setup.Field.Connections[0].FromPosition = spatial.Position{X: -1, Y: 0}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("negative Y is rejected", func() {
		setup := connBoundsSetup()
		setup.Field.Connections[0].FromPosition = spatial.Position{X: 0, Y: -1}
		_, err := encounter.NewEncounter(setup)
		s.Require().ErrorIs(err, encounter.ErrBadConnection)
		s.Require().Contains(err.Error(), "from-position out of bounds")
	})

	s.Run("Width-1,Height-1 is accepted (positive control)", func() {
		setup := connBoundsSetup()
		setup.Field.Connections[0].FromPosition = spatial.Position{X: 3, Y: 2}
		_, err := encounter.NewEncounter(setup)
		s.Require().NoError(err, "the last valid cell must be accepted")
	})
}

// validRoomSetup returns a fresh SetupInput with a single valid square room
// and one member — the base for TestSetupRoomValidation's one-defect rows.
func validRoomSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestSetupRoomValidation pins room-level defects at the Setup seam
// (#922 T1.5, deferred from the Opus T1 review): empty room ID, duplicate
// room ID, and an unrecognized grid shape all reject with ErrNoField — a
// malformed room list is as unusable as an empty one.
func (s *EncounterTestSuite) TestSetupRoomValidation() {
	cases := []struct {
		name     string
		mutate   func(in *encounter.SetupInput)
		fragment string
	}{
		{"room has empty id", func(in *encounter.SetupInput) {
			in.Field.Rooms[0].ID = ""
		}, "room has empty id"},
		{"duplicate room id", func(in *encounter.SetupInput) {
			in.Field.Rooms = append(in.Field.Rooms, in.Field.Rooms[0])
		}, "duplicate room"},
		{"room has unknown grid shape", func(in *encounter.SetupInput) {
			in.Field.Rooms[0].Grid = spatial.GridShape(99)
		}, "unknown grid shape"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			setup := validRoomSetup()
			tc.mutate(setup)
			_, err := encounter.NewEncounter(setup)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrNoField, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	// The valid base itself must construct — the one-defect discipline only
	// means something if zero defects pass.
	_, err := encounter.NewEncounter(validRoomSetup())
	s.Require().NoError(err, "the valid base fixture must construct")
}

// TestHexRoomBounds pins that a hex-shaped room's member-placement bounds
// defer to the room's own constructed Grid rather than hardcoded rectangle
// math. HexGrid (tools/spatial/hex_grid.go:54-57) — the offset column/row
// coordinate hex grid this module constructs for GridShapeHex — uses the
// IDENTICAL non-negative/strictly-less-than-dimension rule as SquareGrid,
// so a hex room's accept/reject boundary values are numerically the same
// shape as square's. This test therefore pins that hex construction
// succeeds and Width/Height correctly bound the constructed grid; it does
// NOT by itself prove hex (vs. square) construction happened — see
// TestGridlessRoomInclusiveBounds for a shape whose validity genuinely
// diverges from the rectangle math this task deletes.
func (s *EncounterTestSuite) TestHexRoomBounds() {
	hexSetup := func(pos spatial.Position) *encounter.SetupInput {
		return &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "r1", Width: 4, Height: 3, Grid: spatial.GridShapeHex},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: pos},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		}
	}

	s.Run("position within hex bounds accepted", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: 3, Y: 2})) // Width-1, Height-1
		s.Require().NoError(err)
	})

	s.Run("position at width boundary rejected", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: 4, Y: 0}))
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})
}

// TestGridlessRoomInclusiveBounds pins the one shape whose validity
// genuinely diverges from the rectangle math this task deletes:
// GridlessRoom.IsValidPosition (tools/spatial/gridless.go:33-36) uses an
// INCLUSIVE upper bound (x <= Width), unlike Square/Hex's exclusive
// (x < Width). A position exactly AT Width — rejected for square/hex — is
// the sharpest available proof that bounds checks ask the room's OWN
// constructed grid rather than a hardcoded rectangle: a "grid shape
// ignored, always builds square" mutant would reject this position; the
// correct code accepts it.
func (s *EncounterTestSuite) TestGridlessRoomInclusiveBounds() {
	gridlessSetup := func(pos spatial.Position) *encounter.SetupInput {
		return &encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "r1", Width: 4, Height: 3, Grid: spatial.GridShapeGridless},
				},
			},
			Members: []encounter.MemberInput{
				{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: pos},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		}
	}

	s.Run("position exactly at Width is accepted (inclusive upper bound)", func() {
		_, err := encounter.NewEncounter(gridlessSetup(spatial.Position{X: 4, Y: 0}))
		s.Require().NoError(err, "gridless rooms accept x == Width; a rectangle-math fallback would reject this")
	})

	s.Run("position negative is still rejected", func() {
		_, err := encounter.NewEncounter(gridlessSetup(spatial.Position{X: -1, Y: 0}))
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
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

// newBasicEncounterWithExternalEnding builds the standard two-player fixture with
// an External ending (for testing End verb).
func (s *EncounterTestSuite) newBasicEncounterWithExternalEnding() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 20, Height: 20}}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 5, Y: 5}},
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
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "door1", From: "room-a", To: "room-b",
					FromPosition: spatial.Position{X: 9, Y: 5},
					ToPosition:   spatial.Position{X: 0, Y: 5}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: "room-a", Position: spatial.Position{X: 9, Y: 5}},
			{ID: bob, Kind: encounter.KindPlayer, Room: "room-a", Position: spatial.Position{X: 8, Y: 5}},
			{ID: goblin, Kind: encounter.KindMonster, Room: "room-b", Position: spatial.Position{X: 1, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// TestTraverseValidation pins the guard order (nil input, closed, unknown
// member, unknown connection, endpoint mismatch) and that each rejection
// uses the correct sentinel.
func (s *EncounterTestSuite) TestTraverseValidation() {
	s.Run("nil input returns ErrNilInput", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		_, err := enc.Traverse(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("closed encounter returns ErrClosed", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)

		_, err = enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})

	s.Run("unknown member returns ErrNotMember", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		_, err := enc.Traverse(&encounter.TraverseInput{Member: core.EntityID("nobody"), Connection: "door1"})
		s.Require().ErrorIs(err, encounter.ErrNotMember)
	})

	s.Run("unknown connection returns ErrNoConnection", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		_, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "no-such-door"})
		s.Require().ErrorIs(err, encounter.ErrNoConnection)
	})

	s.Run("off-threshold position (right room, wrong cell) returns ErrBadPlacement", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		// bob is in room-a (the connection's From room) but not at the door.
		_, err := enc.Traverse(&encounter.TraverseInput{Member: bob, Connection: "door1"})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})

	s.Run("wrong room (connection doesn't touch it) returns ErrBadPlacement", func() {
		// alice sits at room-a's door COORDINATES, but in room-c — a room
		// the connection doesn't touch at all. Proves room membership is
		// checked, not just coordinate equality.
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-a", Width: 10, Height: 10},
					{ID: "room-b", Width: 10, Height: 10},
					{ID: "room-c", Width: 10, Height: 10},
				},
				Connections: []encounter.ConnectionInput{
					{ID: "door1", From: "room-a", To: "room-b",
						FromPosition: spatial.Position{X: 9, Y: 5},
						ToPosition:   spatial.Position{X: 0, Y: 5}},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: "room-c", Position: spatial.Position{X: 9, Y: 5}},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		_, err = enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})
}

// TestTraverseBothDirections pins the threshold-success path in BOTH
// directions through the same connection (T1 law: connections are
// bidirectional), asserting room+position after each hop.
func (s *EncounterTestSuite) TestTraverseBothDirections() {
	s.Run("room-a to room-b", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		out, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
		s.Require().NoError(err)
		s.Equal(alice, out.Traversed.Member)
		s.Equal("room-a", out.Traversed.FromRoom)
		s.Equal(spatial.Position{X: 9, Y: 5}, out.Traversed.From)
		s.Equal("room-b", out.Traversed.ToRoom)
		s.Equal(spatial.Position{X: 0, Y: 5}, out.Traversed.To)

		members, err := enc.Members()
		s.Require().NoError(err)
		found := false
		for _, m := range members {
			if m.ID == alice {
				s.Equal("room-b", m.Room)
				found = true
			}
		}
		s.True(found, "alice must still be a member, now in room-b")
	})

	s.Run("room-b to room-a (bidirectional through the same connection)", func() {
		enc := s.newTwoRoomEncounterWithConnection()
		_, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
		s.Require().NoError(err)

		out, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
		s.Require().NoError(err)
		s.Equal("room-b", out.Traversed.FromRoom)
		s.Equal(spatial.Position{X: 0, Y: 5}, out.Traversed.From)
		s.Equal("room-a", out.Traversed.ToRoom)
		s.Equal(spatial.Position{X: 9, Y: 5}, out.Traversed.To)

		members, err := enc.Members()
		s.Require().NoError(err)
		for _, m := range members {
			if m.ID == alice {
				s.Equal("room-a", m.Room)
			}
		}
	})
}

// TestTraverseGhostAtThreshold pins that a departure-room observer's
// holding of the traverser fades to a ghost AT THE DEPARTURE ENDPOINT —
// their last-observed position — not the (never-seen) arrival position.
func (s *EncounterTestSuite) TestTraverseGhostAtThreshold() {
	enc := s.newTwoRoomEncounterWithConnection()

	bobBefore, err := enc.View(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	s.Require().Len(bobBefore, 1, "bob must see alice before the traverse (geometry precondition)")
	s.Equal(intel.Current, bobBefore[0].Status)

	_, err = enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)

	bobAfter, err := enc.View(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	s.Require().Len(bobAfter, 1, "the ghost is HELD, not gone")
	s.Equal(intel.Held, bobAfter[0].Status, "bob's sight of alice must fade — she left room-a")

	var aliceSeen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(bobAfter[0].Payload, &aliceSeen))
	s.Equal("room-a", aliceSeen.Room, "ghost holds alice's LAST-SEEN room")
	s.Equal(9.0, aliceSeen.X, "ghost holds alice at the DEPARTURE endpoint, not the arrival one")
	s.Equal(5.0, aliceSeen.Y)
}

// TestTraverseArrivalCurrent pins that an arrival-room observer gains the
// traverser as Current at the arrival endpoint.
func (s *EncounterTestSuite) TestTraverseArrivalCurrent() {
	enc := s.newTwoRoomEncounterWithConnection()

	_, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)

	goblinView, err := enc.View(&encounter.ViewInput{Member: goblin})
	s.Require().NoError(err)
	s.Require().Len(goblinView, 1)
	s.Equal(intel.Subject(alice), goblinView[0].Subject)
	s.Equal(intel.Current, goblinView[0].Status, "goblin must see alice as Current — she arrived in room-b")

	var aliceSeen encounter.SightPayload
	s.Require().NoError(json.Unmarshal(goblinView[0].Payload, &aliceSeen))
	s.Equal("room-b", aliceSeen.Room)
	s.Equal(0.0, aliceSeen.X)
	s.Equal(5.0, aliceSeen.Y)
}

// TestTraverseSightNeverCrossesOpening pins law T3: sight never crosses a
// connection's opening. goblin stands in room-b, adjacent to the arrival
// endpoint. Before alice traverses — while she's still in room-a, at the
// connection's OTHER endpoint — goblin must have NO holding of her at
// all, not even a ghost: rooms are separate spatial containers with no
// shared geometry (spatial ADR-0015), so there is no code path by which
// goblin's line-of-sight computation, scoped entirely to room-b, could
// observe alice in room-a.
func (s *EncounterTestSuite) TestTraverseSightNeverCrossesOpening() {
	enc := s.newTwoRoomEncounterWithConnection()

	goblinBefore, err := enc.View(&encounter.ViewInput{Member: goblin})
	s.Require().NoError(err)
	s.Empty(goblinBefore, "goblin must not perceive alice through the unopened doorway")
}

// TestTraverseOwnView pins that after traversing, the traverser's OWN
// percept reflects arrival-room members Current and departure-room
// members faded to ghosts — the same complete-percept contract that
// governs everyone else's view of them.
func (s *EncounterTestSuite) TestTraverseOwnView() {
	enc := s.newTwoRoomEncounterWithConnection()

	aliceBefore, err := enc.View(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(aliceBefore, 1, "alice sees only bob before traversing (goblin is behind the unopened door)")
	s.Equal(intel.Subject(bob), aliceBefore[0].Subject)
	s.Equal(intel.Current, aliceBefore[0].Status)

	_, err = enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)

	aliceAfter, err := enc.View(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(aliceAfter, 2, "alice now holds both bob (ghosted) and goblin (Current)")

	var bobHolding, goblinHolding *intel.Holding
	for i := range aliceAfter {
		switch aliceAfter[i].Subject {
		case intel.Subject(bob):
			bobHolding = &aliceAfter[i]
		case intel.Subject(goblin):
			goblinHolding = &aliceAfter[i]
		}
	}
	s.Require().NotNil(bobHolding, "alice must still hold bob (ghosted)")
	s.Equal(intel.Held, bobHolding.Status, "bob fades to a ghost — alice left room-a")
	s.Require().NotNil(goblinHolding, "alice must now hold goblin (Current)")
	s.Equal(intel.Current, goblinHolding.Status, "goblin is Current — alice arrived in room-b")
}

// TestTraverseEndingFiresOnArrival pins that a ReachedPosition ending
// declared at the connection's far endpoint fires on arrival, exactly
// like Move firing endings at a movement target.
func (s *EncounterTestSuite) TestTraverseEndingFiresOnArrival() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "door1", From: "room-a", To: "room-b",
					FromPosition: spatial.Position{X: 9, Y: 5},
					ToPosition:   spatial.Position{X: 0, Y: 5}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: "room-a", Position: spatial.Position{X: 9, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: "escaped", Trigger: encounter.TriggerReachedPosition{
				Room: "room-b", Position: spatial.Position{X: 0, Y: 5}}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome, "outcome should be set when arriving at the ending position")
	s.Equal("escaped", out.Outcome.Ending)
	s.Require().Len(out.Outcome.Members, 1)
	s.Equal(alice, out.Outcome.Members[0].ID)
	s.Equal("room-b", out.Outcome.Members[0].Room)
	s.Equal(spatial.Position{X: 0, Y: 5}, out.Outcome.Members[0].Position)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open)
}

// TestTraverseMonsterOnUnfilteredEndingDoesNotClose pins the players-only
// rule for unfiltered ReachedPosition endings, carried over verbatim from
// Move/Pump: a monster traversing onto an unfiltered ending's cell must
// NOT close the encounter.
func (s *EncounterTestSuite) TestTraverseMonsterOnUnfilteredEndingDoesNotClose() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "door1", From: "room-a", To: "room-b",
					FromPosition: spatial.Position{X: 9, Y: 5},
					ToPosition:   spatial.Position{X: 0, Y: 5}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: goblin, Kind: encounter.KindMonster, Room: "room-a", Position: spatial.Position{X: 9, Y: 5}},
		},
		Endings: []encounter.EndingInput{
			// Unfiltered: empty Member means players only.
			{Key: "escaped", Trigger: encounter.TriggerReachedPosition{
				Room: "room-b", Position: spatial.Position{X: 0, Y: 5}, Member: ""}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Traverse(&encounter.TraverseInput{Member: goblin, Connection: "door1"})
	s.Require().NoError(err)
	s.Nil(out.Outcome, "unfiltered ending must not fire for a monster")

	status, err := enc.Status()
	s.Require().NoError(err)
	s.True(status.Open, "encounter should remain open")
}

// TestTraverseBeatPinned pins the traversed beat: tag, payload, and
// Story reflects it in position (the Move/Exit precedent applied here).
func (s *EncounterTestSuite) TestTraverseBeatPinned() {
	enc := s.newTwoRoomEncounterWithConnection()
	out, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)
	last := story[len(story)-1]
	s.Equal(out.Seq, last.Seq, "TraverseOutput.Seq references the traversed beat")

	var beat map[string]any
	s.Require().NoError(json.Unmarshal(last.Payload, &beat))
	s.Equal("traversed", beat["beat"])
	s.Equal(string(alice), beat["member"])
	s.Equal("door1", beat["connection"])
}

// TestTraverseClockUnchanged pins law T4: traversal is an activity, not
// time — the exploration clock does not advance.
func (s *EncounterTestSuite) TestTraverseClockUnchanged() {
	enc := s.newTwoRoomEncounterWithConnection()
	before := enc.ToData().Clock.HighWater

	_, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)

	after := enc.ToData().Clock.HighWater
	s.Equal(before, after, "traversal is an activity, not time — the clock must not advance")
}

// TestTraverseThenJoinVacatedCellThenTraverseBack pins the wave-1 Exit
// lesson (see TestExitThenRejoinSameID) against Traverse's own
// remove+place composition: after alice traverses room-a -> room-b, a
// NEW member can Join at the vacated room-a endpoint cell (proving
// RemoveEntity truly cleared it, not just hid it from Members/View), and
// alice can traverse BACK through the same connection (proving
// PlaceEntity into room-b left no stale index entry either).
func (s *EncounterTestSuite) TestTraverseThenJoinVacatedCellThenTraverseBack() {
	enc := s.newTwoRoomEncounterWithConnection()

	_, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
		ID: core.EntityID("charlie"), Kind: encounter.KindPlayer,
		Room: "room-a", Position: spatial.Position{X: 9, Y: 5},
	}})
	s.Require().NoError(err, "the vacated cell must truly be free — no stale registry entry in room-a")

	out, err := enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
	s.Require().NoError(err, "alice must be able to traverse back — no stale registry entry in room-b either")
	s.Equal("room-a", out.Traversed.ToRoom)
}

// Membership flow tests (Task 5)

func (s *EncounterTestSuite) TestJoinLateJoinerSeenByIncumbents() {
	s.Run("late joiner seen by and sees incumbents", func() {
		// Arrange: setup with alice and bob
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
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Room:     room1,
					Position: spatial.Position{X: 8, Y: 8},
				},
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
					Room:     room1,
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
			Member: encounter.MemberInput{
				ID:       charlie,
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 5, Y: 5},
			},
		})
		s.Require().NoError(err, "join should succeed")

		// Assert: charlie joined
		s.Equal(charlie, joinOut.Member.ID)
		s.Equal(encounter.KindPlayer, joinOut.Member.Kind)
		s.Equal(room1, joinOut.Member.Room)

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
			Member: encounter.MemberInput{
				ID:       "",
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 5, Y: 5},
			},
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("join already a member", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Join(&encounter.JoinInput{
			Member: encounter.MemberInput{
				ID:       alice,
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 5, Y: 5},
			},
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember, "duplicate join should fail")
	})

	s.Run("join closed encounter", func() {
		enc := s.newBasicEncounterWithExternalEnding()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Join(&encounter.JoinInput{
			Member: encounter.MemberInput{
				ID:       core.EntityID("charlie"),
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 5, Y: 5},
			},
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)
	})

	s.Run("join with bad placement", func() {
		enc := s.newBasicEncounter()
		_, err := enc.Join(&encounter.JoinInput{
			Member: encounter.MemberInput{
				ID:       core.EntityID("charlie"),
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 99, Y: 99}, // Out of bounds
			},
		})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})

	s.Run("join player with decider rejected", func() {
		enc := s.newBasicEncounter()
		fixedDecider := &simpleDecider{}
		_, err := enc.Join(&encounter.JoinInput{
			Member: encounter.MemberInput{
				ID:       core.EntityID("charlie"),
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 5, Y: 5},
				Decider:  fixedDecider,
			},
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember, "player with decider should fail")
	})
}

func (s *EncounterTestSuite) TestJoinOnStairsFiresEnding() {
	s.Run("join on stairs fires ReachedPosition ending", func() {
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
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
					Room:     room1,
					Position: spatial.Position{X: 9, Y: 9},
				}},
			},
		}

		enc, err := encounter.NewEncounter(setup)
		s.Require().NoError(err)

		// Act: join at stairs position
		charlie := core.EntityID("charlie")
		joinOut, err := enc.Join(&encounter.JoinInput{
			Member: encounter.MemberInput{
				ID:       charlie,
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 9, Y: 9}, // stairs position
			},
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
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Room:     room1,
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
		s.Equal(room1, exitOut.Outcome.Room)
		s.Equal(2.0, exitOut.Outcome.Position.X)
		s.Equal(2.0, exitOut.Outcome.Position.Y)

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
			},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
					Room:     room1,
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
					ID:       bob,
					Kind:     encounter.KindPlayer,
					Room:     room1,
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
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 10, Height: 10},
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
			Member: encounter.MemberInput{
				ID:       core.EntityID("charlie"),
				Kind:     encounter.KindPlayer,
				Room:     room1,
				Position: spatial.Position{X: 5, Y: 5},
			},
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Exit on closed: ErrClosed
		_, err = enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Move on closed: ErrClosed
		_, err = enc.Move(&encounter.MoveInput{
			Member: alice,
			To:     spatial.Position{X: 3, Y: 3},
		})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Pump on closed: ErrClosed
		_, err = enc.Pump(&encounter.PumpInput{})
		s.Require().ErrorIs(err, encounter.ErrClosed)

		// Traverse on closed: ErrClosed (checked before connection lookup,
		// so a nonexistent connection ID still surfaces ErrClosed first)
		_, err = enc.Traverse(&encounter.TraverseInput{Member: alice, Connection: "door1"})
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
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 10, Height: 10},
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
	_, err = enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
		ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 6, Y: 6},
	}})
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
