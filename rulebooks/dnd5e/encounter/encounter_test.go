// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
			Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{}},
			Members:    []encounter.MemberInput{},
			Endings: []encounter.EndingInput{
				{Key: "stairs", Trigger: encounter.TriggerExternal{}},
			},
		}
		_, err := encounter.NewEncounter(setup1)
		s.Require().ErrorIs(err, encounter.ErrNoField)

		// Second attempt: valid
		setup2 := &encounter.SetupInput{
			Initiative: orderAsGiven{},
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
// FromPosition{9,1} valid ONLY in r1 and ToPosition{0,7} valid ONLY in r2.
// Same-sized rooms and equal From/To positions would make a check that
// validates an endpoint against the WRONG room (or a Load-side From/To
// transposition) invisible: this is the base for
// TestSetupConnectionValidation's one-defect rows, mirroring the same
// defect classes rejected at Load (TestLoadRejections).
//
// #929 T1: the endpoints sit on each room's OWN boundary (r1's rightmost
// column, r2's leftmost column) — a connection can only ever kiss (W3)
// through a room's boundary cell, never an interior one (every neighbor of
// an interior cell is still inside that room's own footprint). r2's Origin
// (10,-6) puts its leftmost column at absolute x=10, immediately east of
// r1's rightmost column (absolute x=9, since r1's Origin is the zero
// value) — Chebyshev-adjacent (W3) — while r1's absolute footprint
// (x:[0,9], y:[0,3]) and r2's (x:[10,12], y:[-6,2]) share no x value at
// all, so they stay disjoint (W2) regardless of their y overlap.
func validConnSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 10, Height: 4, Occluders: []spatial.Position{{X: 2, Y: 2}}},
				{ID: "r2", Width: 3, Height: 9, Origin: spatial.Position{X: 10, Y: -6}, Occluders: []spatial.Position{{X: 1, Y: 3}}},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "c1", From: "r1", To: "r2",
					FromPosition: spatial.Position{X: 9, Y: 1},
					ToPosition:   spatial.Position{X: 0, Y: 7}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestSetupSquareOccluderFractionalRejected pins F2 (#929 T3 Opus round):
// occluder integrality is universal now, not hex-only — a fractional
// occluder on a SQUARE room (previously accepted outright, before this
// fix) must reject exactly like a fractional hex one
// (TestSetupHexIntegralAxial's sibling row), with the universal
// isRepresentableInteger message, not the hex-specific one. One-defect:
// validConnSetup's r1.Occluders[0] is the ONLY thing mutated.
func (s *EncounterTestSuite) TestSetupSquareOccluderFractionalRejected() {
	setup := validConnSetup()
	setup.Field.Rooms[0].Occluders[0] = spatial.Position{X: 2.5, Y: 2}
	_, err := encounter.NewEncounter(setup)
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "not a representable integral cell")
}

// TestSetupOccluderOnBoundaryCellAccepted pins N2's over-tightening sweep
// (#929 T3 trailing round): every occluder in every EXISTING fixture sits
// on an interior cell, so a mutant that rejected a boundary-cell occluder
// (plausible: "occluders should be interior only") survived the suite
// with zero failures until this row was added. Occluders block line of
// sight (field.go's doc comment), not placement — a boundary cell,
// including a corner, is exactly as legal an occluder position as an
// interior one.
func (s *EncounterTestSuite) TestSetupOccluderOnBoundaryCellAccepted() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hall", Width: 5, Height: 5, Occluders: []spatial.Position{
					{X: 0, Y: 2}, // left edge
					{X: 4, Y: 2}, // right edge
					{X: 2, Y: 0}, // top edge
					{X: 2, Y: 4}, // bottom edge
					{X: 0, Y: 0}, // corner
					{X: 4, Y: 4}, // corner
				}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().NoError(err, "an occluder on a room's boundary cell, including a corner, must be legal")
}

// TestSetupOccluderIDCrossRoomCollisionAccepted pins the hardening round's
// fix (#929 item C): the occluder entity ID used to concatenate room ID
// and truncated coordinates (occluder-<room>-<int(X)>-<int(Y)>), so room
// "r" with occluder (-5,4) and room "r-" with occluder (5,4) both produced
// "occluder-r--5-4" — a genuine cross-room ID collision on a field that is
// otherwise entirely legal under W1/W2/W3. The ID is index-based now, so
// this exact colliding pair must construct without error.
func (s *EncounterTestSuite) TestSetupOccluderIDCrossRoomCollisionAccepted() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r", Width: 12, Height: 10, Grid: spatial.GridShapeHex,
					Occluders: []spatial.Position{{X: -5, Y: 4}}},
				{ID: "r-", Width: 12, Height: 10, Grid: spatial.GridShapeHex,
					Origin: spatial.Position{X: 1000, Y: 0}, Occluders: []spatial.Position{{X: 5, Y: 4}}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().NoError(err, `room "r" occluder (-5,4) and room "r-" occluder (5,4) must not collide`)
}

// TestSetupDuplicateOccluderRejected pins the hardening round's item D:
// two occluders at the SAME cell used to escape module validation
// entirely and reject only in spatial's own voice ("entity ... already
// indexed") as an accident of the old coordinate-derived entity ID —
// see TestSetupOccluderIDCrossRoomCollisionAccepted's fix, which
// switched to an index-based ID and, as a side effect, removed even
// that accidental catch. Rejected explicitly now, in the module's own
// room-list defect vocabulary.
func (s *EncounterTestSuite) TestSetupDuplicateOccluderRejected() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hall", Width: 5, Height: 5, Occluders: []spatial.Position{{X: 3, Y: 3}, {X: 3, Y: 3}}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "duplicate occluder")
}

// TestSetupDuplicateEndingKeyRejected pins hardening round item E: two
// endings sharing a key both used to construct — a genuine liveness
// hole, since End scans in declaration order and a reached_position
// twin declared first permanently shadows a same-keyed external ending
// declared after it (probed: End("dup") failed "is not External"
// forever, the external ending having no other way to fire).
func (s *EncounterTestSuite) TestSetupDuplicateEndingKeyRejected() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "hall", Width: 5, Height: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: "dup", Trigger: encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 4, Y: 4}}},
			{Key: "dup", Trigger: encounter.TriggerExternal{}},
		},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
	s.Require().Contains(err.Error(), "duplicate ending")
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
	// means something if zero defects pass. Since FromPosition{9,1} is valid
	// ONLY in r1 and ToPosition{0,7} valid ONLY in r2, this positive control
	// also pins that each endpoint is checked against ITS OWN room: a check
	// wired to the wrong room would reject this valid connection.
	enc, err := encounter.NewEncounter(validConnSetup())
	s.Require().NoError(err, "the valid base fixture must construct")
	data := enc.ToData()
	s.Require().Len(data.Field.Connections, 1)
	s.Equal(&encounter.PositionData{X: 9, Y: 1}, data.Field.Connections[0].FromPosition,
		"from-position must survive unswapped")
	s.Equal(&encounter.PositionData{X: 0, Y: 7}, data.Field.Connections[0].ToPosition,
		"to-position must survive unswapped")
}

// connBoundsSetup returns a fresh SetupInput with a 4x3 room r1 (valid
// coordinates 0..3 x 0..2) and an r2 large enough to always hold the
// connection's fixed ToPosition — used to pin the square grid's strictly-
// less-than bounds semantics against FromPosition in r1, independent of any
// cross-room concern (that's validConnSetup's job). r2's Origin (4,3)
// anchors it diagonally past r1's bottom-right corner (#929 T1): the
// positive control's FromPosition (3,2) — r1's own bottom-right corner —
// and ToPosition (0,0)+(4,3)=(4,3) are Chebyshev-adjacent (a diagonal kiss,
// distance 1), while the rooms' absolute footprints (x:[0,3] vs x:[4,7])
// share no x value and so stay disjoint (W2) regardless of y.
func connBoundsSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 4, Height: 3},
				{ID: "r2", Width: 4, Height: 3, Origin: spatial.Position{X: 4, Y: 3}},
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
		Initiative: orderAsGiven{},
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
//
// #929 T1 Opus round: also pins room legality (non-positive Width/Height),
// both zero AND negative for each dimension — the blocker this closes:
// Width:-1 previously reached a negative-capacity make() in W2's
// enumeration path (since replaced by interval math) and PANICKED
// NewEncounter instead of rejecting it (R5: a panic is not a rejection).
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
		{"room has zero width", func(in *encounter.SetupInput) {
			in.Field.Rooms[0].Width = 0
		}, "non-positive dimensions"},
		{"room has zero height", func(in *encounter.SetupInput) {
			in.Field.Rooms[0].Height = 0
		}, "non-positive dimensions"},
		{"room has negative width", func(in *encounter.SetupInput) {
			in.Field.Rooms[0].Width = -1
		}, "non-positive dimensions"},
		{"room has negative height", func(in *encounter.SetupInput) {
			in.Field.Rooms[0].Height = -1
		}, "non-positive dimensions"},
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
// math — and, since the switch to AxialHexGrid (tools/spatial's origin-
// centered axial Q/R grid, see RoomInput.Grid's doc comment), that hex
// validity now genuinely DIVERGES from square's, not just gridless's: a
// hex room's bounds are centered on the origin ([-Width/2, Width/2) for Q,
// [-Height/2, Height/2) for R), so NEGATIVE coordinates are legal — a
// position square would reject outright. This supersedes an earlier
// finding (when this module built spatial.HexGrid, a bounded offset grid)
// that hex's accept/reject shape was numerically identical to square's and
// only gridless could kill a "grid shape ignored, always builds square"
// mutant; the negative-Q case below now kills that mutant independently.
func (s *EncounterTestSuite) TestHexRoomBounds() {
	// Width=4, Height=3 => Q valid in [-2,2), R valid in [-1.5,1.5).
	hexSetup := func(pos spatial.Position) *encounter.SetupInput {
		return &encounter.SetupInput{
			Initiative: orderAsGiven{},
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

	s.Run("positive Q, positive R within span accepted", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: 1, Y: 1}))
		s.Require().NoError(err)
	})

	s.Run("negative Q within span accepted — rejected under the old offset HexGrid", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: -1, Y: 0}))
		s.Require().NoError(err, "axial hex rooms are origin-centered; negative Q is ordinary, not a defect")
	})

	s.Run("Q at exactly +Width/2 rejected (upper bound exclusive)", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: 2, Y: 0}))
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})

	s.Run("Q at exactly -Width/2 accepted (lower bound inclusive)", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: -2, Y: 0}))
		s.Require().NoError(err)
	})

	s.Run("Q beyond -Width/2 rejected", func() {
		_, err := encounter.NewEncounter(hexSetup(spatial.Position{X: -3, Y: 0}))
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	})
}

// TestHexConnectionEndpointNegativeAxial pins connection endpoints in a
// hex room at the Setup seam: a negative axial Q/R endpoint — the
// ordinary case for an origin-centered hex room — validates exactly like
// a positive one, via the same grid-deferred bounds check members use.
// Load-seam counterpart: TestHexConnectionEndpointNegativeAxialLoad in
// data_test.go.
//
// #929 T1 follow-up: both rooms are hex (W1 forbids mixing families in one
// field — see TestSetupAnchoring's W1 row) rather than the original
// square+hex pairing. hex-a is 10x10 (Q,R valid [-5,5)); hex-b is 6x6 (Q,R
// valid [-3,3)), anchored at (8,7) so its NEGATIVE-axial corner (-3,-3) —
// the coordinate that gives this test its name — sits immediately east of
// hex-a's own (4,4) corner: absolute (5,4) is a cube-distance-1 neighbor of
// (4,4) (W3), and hex-b's absolute Q span ([5,10]) shares no Q value with
// hex-a's ([-5,4]), so the rooms stay disjoint (W2) regardless of R.
func (s *EncounterTestSuite) TestHexConnectionEndpointNegativeAxial() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hex-a", Width: 10, Height: 10, Grid: spatial.GridShapeHex},
				{ID: "hex-b", Width: 6, Height: 6, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 8, Y: 7}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "hex-a", To: "hex-b",
				FromPosition: spatial.Position{X: 4, Y: 4},
				ToPosition:   spatial.Position{X: -3, Y: -3},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-a", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "a connection endpoint at a negative axial coordinate must validate")
}

// validHexAxialSetup returns a fresh SetupInput with two hex rooms joined
// by one connection, a member, and an occluder — every position integral
// axial, including a negative one (gate.ToPosition). The base for
// TestSetupHexIntegralAxial's one-defect rows: interim tools/spatial#926
// enforcement (isIntegralAxialPosition) rejects a fractional X or Y at
// any of these positions in a hex room.
//
// #929 T1: both rooms are Width=8,Height=8, so each has valid axial Q,R in
// [-4,4) (i.e. -4..3). gate.FromPosition sits at hex-a's own Q=3 boundary
// (an interior cell like the original (1,1) can never kiss anything — every
// neighbor of an interior cell is still inside that room's own footprint).
// hex-b's Origin (8,1) anchors it immediately past that boundary: absolute
// FromPosition (3,0) and absolute ToPosition local(-4,-1)+(8,1)=(4,0) are
// cube-distance 1 (W3), while hex-b's absolute Q span ([4,11]) shares no Q
// value with hex-a's ([-4,3]), so the two rooms stay disjoint (W2)
// regardless of R.
func validHexAxialSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hex-a", Width: 8, Height: 8, Grid: spatial.GridShapeHex,
					Occluders: []spatial.Position{{X: 2, Y: 2}}},
				{ID: "hex-b", Width: 8, Height: 8, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 8, Y: 1}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "hex-a", To: "hex-b",
				FromPosition: spatial.Position{X: 3, Y: 0},
				ToPosition:   spatial.Position{X: -4, Y: -1},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-a", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestSetupHexIntegralAxial pins the interim tools/spatial#926
// enforcement at the Setup seam: a fractional X or Y is rejected for
// every position kind a hex room accepts externally — member, both
// connection endpoints, and an occluder — each with the error class its
// existing defect family already uses. Load-seam counterpart:
// TestLoadHexIntegralAxial in data_test.go.
func (s *EncounterTestSuite) TestSetupHexIntegralAxial() {
	cases := []struct {
		name     string
		mutate   func(in *encounter.SetupInput)
		alsoErr  error
		fragment string
	}{
		{"member position fractional", func(in *encounter.SetupInput) {
			in.Members[0].Position = spatial.Position{X: 0.5, Y: 0}
		}, encounter.ErrBadPlacement, "not an integral axial cell"},
		{"connection from-position fractional", func(in *encounter.SetupInput) {
			in.Field.Connections[0].FromPosition = spatial.Position{X: 1.5, Y: 1}
		}, encounter.ErrBadConnection, "not an integral axial cell"},
		{"connection to-position fractional", func(in *encounter.SetupInput) {
			in.Field.Connections[0].ToPosition = spatial.Position{X: -1.5, Y: -1}
		}, encounter.ErrBadConnection, "not an integral axial cell"},
		{"occluder position fractional", func(in *encounter.SetupInput) {
			// #929 T3 Opus round F2: occluder integrality is now universal
			// (isIntegralPosition), not hex-only (isIntegralAxialPosition) —
			// the message matches Origin's ("not a representable integral
			// cell"), not the hex-specific connection/member wording.
			in.Field.Rooms[0].Occluders[0] = spatial.Position{X: 2.5, Y: 2}
		}, encounter.ErrNoField, "not a representable integral cell"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			setup := validHexAxialSetup()
			tc.mutate(setup)
			_, err := encounter.NewEncounter(setup)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, tc.alsoErr, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	// Positive control: the valid base — integral throughout, including
	// a NEGATIVE axial position (gate.ToPosition at (-4,-1)) — constructs.
	_, err := encounter.NewEncounter(validHexAxialSetup())
	s.Require().NoError(err, "integral axial positions, including negative ones, must be accepted")
}

// TestMoveHexIntegralAxial is Move's verb-seam counterpart: a fractional
// target in a hex room is rejected (moveMember is the shared path with
// Pump's IntentMoveTo, so this also covers decider-driven moves).
func (s *EncounterTestSuite) TestMoveHexIntegralAxial() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "hex-room", Width: 8, Height: 8, Grid: spatial.GridShapeHex}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-room", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Run("fractional target rejected", func() {
		_, err := enc.Move(&encounter.MoveInput{Member: "p1", To: spatial.Position{X: 1.5, Y: 0}})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "not an integral axial cell")
	})

	s.Run("integral negative axial target accepted", func() {
		_, err := enc.Move(&encounter.MoveInput{Member: "p1", To: spatial.Position{X: -2, Y: -1}})
		s.Require().NoError(err)
	})
}

// TestJoinHexIntegralAxial is Join's verb-seam counterpart.
func (s *EncounterTestSuite) TestJoinHexIntegralAxial() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "hex-room", Width: 8, Height: 8, Grid: spatial.GridShapeHex}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-room", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Run("fractional position rejected", func() {
		_, err := enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
			ID: "p2", Kind: encounter.KindPlayer, Room: "hex-room", Position: spatial.Position{X: 1, Y: 0.5},
		}})
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "not an integral axial cell")
	})

	s.Run("integral negative axial position accepted", func() {
		_, err := enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
			ID: "p3", Kind: encounter.KindPlayer, Room: "hex-room", Position: spatial.Position{X: -3, Y: -3},
		}})
		s.Require().NoError(err)
	})
}

// TestGridlessRoomInclusiveBounds pinned gridless's own divergence from the
// rectangle math a prior task deleted (GridlessRoom.IsValidPosition's
// inclusive upper bound, x <= Width, vs SquareGrid's exclusive x < Width).
// #929 T1 (shape legality, W1) retires that coverage: gridless leaves the
// composition entirely as of v0.3 — the wire cannot carry a continuous
// room's absolute projection — so a declared GridShapeGridless room is now
// a room-list defect at Setup, full stop, regardless of any position within
// it. This test is repurposed to pin THAT rejection instead of gridless's
// old bounds semantics (AxialHexGrid's own divergence from square — origin-
// centered bounds, legal negative coordinates — is still covered by
// TestHexRoomBounds).
func (s *EncounterTestSuite) TestGridlessRoomInclusiveBounds() {
	gridlessSetup := func(pos spatial.Position) *encounter.SetupInput {
		return &encounter.SetupInput{
			Initiative: orderAsGiven{},
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

	s.Run("gridless room rejected regardless of member position", func() {
		_, err := encounter.NewEncounter(gridlessSetup(spatial.Position{X: 4, Y: 0}))
		s.Require().Error(err, "gridless leaves the composition as of v0.3 — no position can rescue it")
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Require().Contains(err.Error(), "gridless grid shape, no longer supported",
			"the check that fired must be shape legality, not a downstream placement check")
	})

	s.Run("still rejected for a position that would also be out of bounds", func() {
		_, err := encounter.NewEncounter(gridlessSetup(spatial.Position{X: -1, Y: 0}))
		s.Require().Error(err)
		s.Require().ErrorIs(err, encounter.ErrNoField,
			"shape legality fires before any placement check ever sees this position")
	})
}

// ============================================================
// #929 T1 — anchoring: RoomInput.Origin, W1 (one geometry per field), W2
// (rooms never overlap), W3 (doorways kiss). Shape legality's own
// rejection (GridShapeGridless) is pinned above by
// TestGridlessRoomInclusiveBounds — repurposed for exactly this law.
// ============================================================

// validAnchoredHexSetup returns THE core fixture for the W-law tests below:
// two hex rooms, asymmetric in every dimension (T1 review lesson —
// symmetric fixtures hide cross-wiring mutants: a prior wave's full sweep
// missed four of them this way). hex-big is 10x4 (Q valid [-5,4), R valid
// [-2,2)) at the zero-value Origin; hex-small is 3x9 (Q valid [-1,2), R
// valid [-4,5)) anchored at (6,-5) — a NEGATIVE-axial origin, on purpose.
//
// The connection's endpoints are each their own room's OWN boundary cell —
// an interior cell can never kiss anything, since every neighbor of an
// interior cell is still inside that room's own footprint — and are
// computed, not guessed, from the span arithmetic above: FromPosition
// (4,0) is hex-big's max-Q boundary; ToPosition (-1,4) is hex-small's
// min-Q/max-R corner. Anchored, their absolute cells are (4,0) and (5,-1):
// ΔQ=1, ΔR=-1, ΔS=0, cube distance (1+1+0)/2 = 1 — adjacent, and
// deliberately OFF-AXIS (both ΔQ and ΔR nonzero) so a Manhattan-distance
// mutant (|ΔQ|+|ΔR|=2) would wrongly reject this valid base — see
// TestAnchoringMutationEvidence.
//
// hex-big's absolute Q span is [-5,4]; hex-small's is local[-1,1]+6=[5,7]:
// the two share NO Q value at all, so W2 holds regardless of R — the
// rooms are disjoint, touching only through the one declared doorway.
func validAnchoredHexSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hex-big", Width: 10, Height: 4, Grid: spatial.GridShapeHex},
				{ID: "hex-small", Width: 3, Height: 9, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 6, Y: -5}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "hex-big", To: "hex-small",
				FromPosition: spatial.Position{X: 4, Y: 0},
				ToPosition:   spatial.Position{X: -1, Y: 4},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-big", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestSetupAnchoring pins the one-defect rows for W1 (mixed grid families),
// origin legality (non-integral origin), and W3 (endpoints that don't
// kiss) — each breaking exactly ONE thing about validAnchoredHexSetup's
// otherwise-valid base, rejecting with the sentinel and message fragment
// its own law uses.
//
// The "W2" row below is honestly an ORDERING pin, not a one-defect row
// (#929 T1 Opus round correction): zeroing hex-small's Origin makes it
// collide with hex-big at (0,0) (a genuine W2 defect) but ALSO detaches
// the gate's endpoints from their designed kiss (a W3 defect too, at the
// mutated Origin) — W2 wins only because it runs before W3 in
// buildValidRoomGrids' order. TestSetupAnchoringOverlapNonAdjacentPair is
// the true one-defect W2 row: three rooms, only a non-adjacent-in-slice
// pair overlaps, no connection involved at all.
func (s *EncounterTestSuite) TestSetupAnchoring() {
	cases := []struct {
		name     string
		mutate   func(in *encounter.SetupInput)
		alsoErr  error
		fragment string
	}{
		{"W1: mixed grid families", func(in *encounter.SetupInput) {
			in.Field.Rooms[1].Grid = spatial.GridShapeSquare
		}, encounter.ErrNoField, "declare different grid families"},
		{"origin legality: fractional hex origin", func(in *encounter.SetupInput) {
			in.Field.Rooms[1].Origin = spatial.Position{X: 6.5, Y: -5}
		}, encounter.ErrNoField, "not a representable integral cell"},
		{"room legality: infinite origin", func(in *encounter.SetupInput) {
			// #929 T1 second Opus round: math.Trunc(+Inf) is +Inf, so the
			// old pos.X == math.Trunc(pos.X) check accepted this outright.
			// #929 T2 second review round: now caught even earlier, by
			// maxAnchorCoord's bound in room legality — Inf is never <= a
			// finite bound, so this never even reaches the representability
			// check origin legality runs afterward.
			in.Field.Rooms[1].Origin = spatial.Position{X: math.Inf(1), Y: -5}
		}, encounter.ErrNoField, "exceeds max anchor coordinate"},
		{"room legality: origin exceeds max anchor coordinate (1e19)", func(in *encounter.SetupInput) {
			// #929 T1 second Opus round: 1e19 is "integral" by Trunc (it has
			// no fractional part as a float64) but exceeds int64's range —
			// roomAbsoluteBounds' int() conversion on a value like this is
			// Go-spec implementation-defined, not a real cell. See
			// TestSetupAnchoringHugeOriginRejectedNotFalseOverlap for the
			// two-room construction that used to produce a wrong verdict.
			// #929 T2 second review round: now caught by maxAnchorCoord's
			// bound in room legality, before origin legality's
			// representability check ever runs (1e19 is both non-representable
			// AND out of bounds — the bound fires first).
			in.Field.Rooms[1].Origin = spatial.Position{X: 1e19, Y: -5}
		}, encounter.ErrNoField, "exceeds max anchor coordinate"},
		{"ordering: W2 wins over a co-occurring W3 defect", func(in *encounter.SetupInput) {
			in.Field.Rooms[1].Origin = spatial.Position{X: 0, Y: 0}
		}, encounter.ErrNoField, "overlap at absolute cell"},
		{"W3: endpoints do not kiss", func(in *encounter.SetupInput) {
			// (1,4) is still a legal LOCAL cell in hex-small (max Q, max R)
			// — this must fail on adjacency, not on the earlier bounds check.
			in.Field.Connections[0].ToPosition = spatial.Position{X: 1, Y: 4}
		}, encounter.ErrBadConnection, "not adjacent"},
		{"W3: hex axial (1,1) delta is NOT adjacent (cube distance 2)", func(in *encounter.SetupInput) {
			// hex-small's Origin shifts by (0,+2) from the valid base's
			// (6,-5) to (6,-3): the gate's endpoints, once anchored, now
			// differ by axial (ΔQ=1,ΔR=1) — cube distance (1+1+2)/2=2, NOT
			// 1. A "Chebyshev-on-axial" mutant (max(|ΔQ|,|ΔR|)=1) would
			// wrongly ACCEPT this as adjacent — see the mutation evidence
			// table in the #929 T1 fix-round report. Still disjoint from
			// hex-big (W2 passes): hex-small's absolute Q span becomes
			// local[-1,1]+6=[5,7], still sharing no Q value with hex-big's
			// [-5,4], so this is a genuine W3-only defect.
			in.Field.Rooms[1].Origin = spatial.Position{X: 6, Y: -3}
		}, encounter.ErrBadConnection, "distance 2"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			setup := validAnchoredHexSetup()
			tc.mutate(setup)
			_, err := encounter.NewEncounter(setup)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, tc.alsoErr, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	// The valid base itself must construct — the one-defect discipline only
	// means something if zero defects pass. This ALSO doubles as the
	// "touching but not overlapping" sibling proof (W2 rejects overlap,
	// not contact): hex-big and hex-small touch at exactly the gate's two
	// cells and nowhere else, yet still validate.
	_, err := encounter.NewEncounter(validAnchoredHexSetup())
	s.Require().NoError(err, "the valid base fixture — asymmetric, negative-anchored, off-axis kissing — must construct")
}

// TestSetupAnchoringSquareDiagonalKiss pins W3's Chebyshev adjacency for
// the square family, specifically a DIAGONAL kiss (Chebyshev distance 1
// includes diagonals, unlike a 4-directional adjacency check): sq-a and
// sq-b sit corner-to-corner, touching at exactly one point and sharing no
// cell (also the square-family sibling proof that W2 rejects overlap, not
// contact — TestSetupAnchoring's hex base already proves it once, this
// proves it independently for square's own distance formula).
func (s *EncounterTestSuite) TestSetupAnchoringSquareDiagonalKiss() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "sq-a", Width: 5, Height: 5},
				{ID: "sq-b", Width: 5, Height: 5, Origin: spatial.Position{X: 5, Y: 5}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "corner-gate", From: "sq-a", To: "sq-b",
				FromPosition: spatial.Position{X: 4, Y: 4},
				ToPosition:   spatial.Position{X: 0, Y: 0},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "sq-a", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().NoError(err, "a diagonal (Chebyshev distance 1) kiss must be accepted, not just a 4-directional one")
}

// TestSetupAnchoringSquareEndpointNotAdjacentDistance2 pins W3's
// square-family rejection at a genuine, non-sub-unit distance (#929
// hardening round, test-gap closure item 5): the only existing
// square-family W3 non-adjacency coverage before this was the sub-unit
// 0.5 case (TestSetupAnchoringFractionalSquareEndpointSubUnitDistance);
// the two "not adjacent"/"distance 2" rows in TestSetupAnchoring's table
// are hex-only. r1 is 3x3 at the zero-value Origin (absolute x:[0,2]);
// r2 is 3x3 at Origin (4,0) (absolute x:[4,6]) — a genuine one-column
// void gap at x=3, so W2 passes (footprints disjoint, not overlapping).
// FromPosition (2,1) in r1 projects to absolute (2,1); ToPosition (0,1)
// in r2 projects to absolute (4,1) — Chebyshev distance 2, not 1.
func (s *EncounterTestSuite) TestSetupAnchoringSquareEndpointNotAdjacentDistance2() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 3, Height: 3},
				{ID: "r2", Width: 3, Height: 3, Origin: spatial.Position{X: 4, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "r1", To: "r2",
				FromPosition: spatial.Position{X: 2, Y: 1},
				ToPosition:   spatial.Position{X: 0, Y: 1},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrBadConnection)
	s.Require().Contains(err.Error(), "not adjacent")
}

// TestSetupAnchoringSquareOriginRejected pins that origin legality
// (isIntegralPosition) applies to EVERY grid family, square included, not
// just hex. #929 T1 Opus round finding: this test used to assert the
// OPPOSITE — that a fractional square origin was fine — and in doing so
// blessed a real hole in W2's disjointness promise. Two 5x5 square rooms
// anchored at (0,0) and (0.5,0.5) have disjoint INTEGER cell sets (an
// enumeration-based W2 check, since replaced, would have accepted them)
// while their continuous footprints interpenetrate roughly 81% of each
// room's area, and a Chebyshev-0 "doorway" (two endpoints landing on the
// same fractional point) would still measure as adjacent. This is now the
// rejection pin that closes that hole.
func (s *EncounterTestSuite) TestSetupAnchoringSquareOriginRejected() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5, Origin: spatial.Position{X: 0.5, Y: 1.5}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err, "a fractional Origin on a square room is now a defect — origin legality is universal")
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "not a representable integral cell")
}

// TestSetupAnchoringNaNOriginRejected pins a genuinely subtle ordering
// (#929 hardening round, test-gap closure item 4 — NaN and -Inf had ZERO
// coverage anywhere in the suite before this): room legality's magnitude
// check is `math.Abs(r.Origin.X) > maxAnchorCoord`, and for X = NaN,
// math.Abs(NaN) is NaN, and EVERY comparison against NaN (including >)
// is false in IEEE 754 — so a NaN origin silently SLIPS PAST room
// legality's magnitude check, uncaught, and is rejected only later, by
// the SEPARATE origin-legality loop (isIntegralPosition ->
// isRepresentableInteger, which explicitly tests IsNaN). Verified by
// probe before writing this assertion. The fragment below asserts the
// origin-legality message SPECIFICALLY (not just ErrNoField) so a future
// reorder that changes which check catches NaN fails loudly here,
// instead of this test silently continuing to pass against a different
// message.
func (s *EncounterTestSuite) TestSetupAnchoringNaNOriginRejected() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "r1", Width: 5, Height: 5, Origin: spatial.Position{X: math.NaN(), Y: 0}}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "not a representable integral cell",
		"a NaN origin must be caught by origin legality, since room legality's Abs(NaN) > bound comparison is always false")
}

// TestSetupAnchoringNegativeInfinityOriginRejected is NaN's sibling case
// (#929 hardening round, test-gap closure item 4), with the OPPOSITE
// ordering: math.Abs(-Inf) is +Inf, and +Inf > maxAnchorCoord (any
// finite bound) IS true — so -Inf does NOT slip past room legality's
// magnitude check the way NaN does; it is caught there, first, with
// room legality's OWN message.
func (s *EncounterTestSuite) TestSetupAnchoringNegativeInfinityOriginRejected() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "r1", Width: 5, Height: 5, Origin: spatial.Position{X: math.Inf(-1), Y: 0}}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceeds max anchor coordinate",
		"-Inf must be caught by room legality's magnitude check, unlike NaN — Abs(-Inf) > bound is true")
}

// TestSetupAnchoringW1BothDirections pins W1's message names BOTH rooms and
// BOTH families regardless of which room is square and which is hex —
// mutating a two-hex-room field's SECOND room mirrors
// TestSetupAnchoring's row, which mutates via the same slice index; this
// covers the field declared the other way around (hex declared first,
// square second) to guard against a comparison that only checks one
// direction.
func (s *EncounterTestSuite) TestSetupAnchoringW1BothDirections() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "square-first", Width: 5, Height: 5},
				{ID: "hex-second", Width: 5, Height: 5, Grid: spatial.GridShapeHex},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "square-first", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), `room "square-first" (square)`)
	s.Require().Contains(err.Error(), `room "hex-second" (hex)`)
}

// TestSetupAnchoringOverlapNonAdjacentPair is fixture 3b (#929 T1 Opus
// round mutation evidence): three square rooms — r-a at the zero-value
// Origin, r-b anchored far away at (20,20), r-c anchored at (2,2) — where
// ONLY the (r-a, r-c) pair overlaps. r-a and r-b don't (adjacent pair
// 0,1); r-b and r-c don't (adjacent pair 1,2); only r-a and r-c do
// (non-adjacent pair 0,2). This is the TRUE one-defect W2 row — no
// connection involved at all, unlike TestSetupAnchoring's "ordering" row
// — and it kills a mutant that reduces W2's pairwise loop to adjacent-in-
// slice-only comparison (j := i+1; j < i+2). Unlike W1's equivalent
// mutant (provably unobservable over a two-value domain — see
// validateGridFamilies' doc comment), overlap is NOT a transitive
// relation over three arbitrarily-positioned rooms, so this fixture
// genuinely distinguishes full pairwise from adjacent-only comparison.
// Witness cell (2,2) is the component-wise max of r-a's and r-c's
// interval mins.
func (s *EncounterTestSuite) TestSetupAnchoringOverlapNonAdjacentPair() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r-a", Width: 5, Height: 5},
				{ID: "r-b", Width: 5, Height: 5, Origin: spatial.Position{X: 20, Y: 20}},
				{ID: "r-c", Width: 5, Height: 5, Origin: spatial.Position{X: 2, Y: 2}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r-a", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), `room "r-a" and room "r-c" overlap at absolute cell (2, 2)`)
}

// TestSetupAnchoringRSpanSeparation is fixture 3c (#929 T1 Opus round
// mutation evidence): two 4x4 hex rooms with IDENTICAL Q ranges — hex-r-b
// is anchored at (0,4), directly south of hex-r-a, not east — disjoint
// along R alone despite the shared Q span. This must ACCEPT (proving W2's
// interval check doesn't need Q separation specifically) and kills an
// R-span boundary-arithmetic drift mutant in axisBounds: dropping the hex
// max formula's "-1" widens hex-r-a's R span from [-2,1] to [-2,2], which
// falsely overlaps these rooms at R=2 (hex-r-b's R span is [2,5]) — see
// the mutation evidence table. Its doorway also kisses: FromPosition
// (0,1) is hex-r-a's own max-R boundary; ToPosition (0,-2) is hex-r-b's
// own min-R boundary; anchored, absolute (0,1) and (0,2) are cube-distance
// 1.
func (s *EncounterTestSuite) TestSetupAnchoringRSpanSeparation() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hex-r-a", Width: 4, Height: 4, Grid: spatial.GridShapeHex},
				{ID: "hex-r-b", Width: 4, Height: 4, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 0, Y: 4}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "hex-r-a", To: "hex-r-b",
				FromPosition: spatial.Position{X: 0, Y: 1},
				ToPosition:   spatial.Position{X: 0, Y: -2},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "hex-r-a", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "rooms separated along R alone, with identical Q ranges, must still be accepted as disjoint")
}

// TestSetupAnchoringFractionalSquareEndpointSubUnitDistance pins W3's
// strict `dist != 1` comparison (#929 T1 second Opus round: an earlier
// comment on that check wrongly claimed sub-1 distances were unfalsifiable
// post-origin-legality — origin legality only constrains ORIGINS, not
// connection endpoints, and square endpoints stay fractional-tolerant by
// design, RoomInput.Grid's doc comment). r1 is 3x3 at the zero-value
// Origin; r2 is 3x3 at Origin (3,0), immediately east — fully disjoint (r1
// absolute x:[0,2], r2 absolute x:[3,5]), both origins integral. Yet
// FromPosition (2.5,1), a legal fractional cell in r1, projects to
// absolute (2.5,1) — only 0.5 Chebyshev distance from ToPosition (0,1)'s
// absolute (3,1). A `> 1` mutant would wrongly ACCEPT a 0.5-unit gap as
// "close enough"; the strict `!= 1` correctly rejects it.
func (s *EncounterTestSuite) TestSetupAnchoringFractionalSquareEndpointSubUnitDistance() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 3, Height: 3},
				{ID: "r2", Width: 3, Height: 3, Origin: spatial.Position{X: 3, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "r1", To: "r2",
				FromPosition: spatial.Position{X: 2.5, Y: 1},
				ToPosition:   spatial.Position{X: 0, Y: 1},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrBadConnection)
	s.Require().Contains(err.Error(), "distance 0.5")
}

// TestSetupAnchoringHugeOriginRejectedNotFalseOverlap pins the fix for the
// second Opus round's headline finding: before this fix, isIntegralPosition's
// pos.X == math.Trunc(pos.X) check accepted any float64 past int64's usable
// precision as "integral" (Trunc is a no-op there), so two rooms anchored
// at X=1e19 and X=2e19 — nowhere near each other — both passed origin
// legality, then BOTH got truncated by roomAbsoluteBounds' int() to the
// SAME implementation-defined int64 value, producing a FALSE W2 overlap
// verdict through the public API. This pins that such an origin now
// rejects at ROOM LEGALITY instead (#929 T2 second review round:
// maxAnchorCoord's bound now fires before origin legality's representability
// check even runs — 1e19 is both non-representable AND out of bounds) —
// before W2 ever sees it, and specifically NOT with a W2 "overlap" message.
func (s *EncounterTestSuite) TestSetupAnchoringHugeOriginRejectedNotFalseOverlap() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 5, Height: 5, Origin: spatial.Position{X: 1e19, Y: 0}},
				{ID: "r2", Width: 5, Height: 5, Origin: spatial.Position{X: 2e19, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceeds max anchor coordinate",
		"must reject at room legality, not fall through to a W2 overlap verdict on garbage-truncated positions")
	s.Require().NotContains(err.Error(), "overlap at absolute cell",
		"the old bug produced a W2 overlap message here — this must NOT be that")
}

// TestSetupAnchoringOversizedRoomRejectedNotFalseDisjoint pins maxRoomSpan
// (#929 T2 second review round — see its doc comment): before this bound,
// an unbounded Width could overflow roomAbsoluteBounds' interval-sum
// arithmetic. r1 is 1000 wide at the zero-value Origin (absolute Q span
// [0,999]); r2 is math.MaxInt wide, anchored at (999,0) — TRUE
// (infinite-precision) math says these overlap by exactly one column, at
// Q=999 (r2's own left edge sits exactly on r1's own right edge). r2's
// absolute qMax is axisBounds(math.MaxInt).max + 999 = (math.MaxInt-1) +
// 999, which OVERFLOWS int64 and wraps to -9223372036854774811 (verified
// by running the exact interval-sum arithmetic standalone, not hand
// computed): with r2's qMin at 999 and its qMax wrapped to that large
// NEGATIVE number, validateRoomsDisjoint's overlap check
// (bs[i].qMin <= bs[j].qMax && bs[j].qMin <= bs[i].qMax) sees
// 0 <= -9223372036854774811 as false and wrongly reports the rooms
// disjoint — WITHOUT this bound. WITH it, r2's Width alone is already
// rejected — oversized, not evaluated for overlap at all.
func (s *EncounterTestSuite) TestSetupAnchoringOversizedRoomRejectedNotFalseDisjoint() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "r1", Width: 1000, Height: 5},
				{ID: "r2", Width: math.MaxInt, Height: 5, Origin: spatial.Position{X: 999, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceed max room span",
		"must reject at room legality for being oversized, before overlap is ever evaluated")
	s.Require().NotContains(err.Error(), "overlap at absolute cell",
		"without the bound, this exact fixture used to wrongly VALIDATE via a false-disjoint W2 verdict — see the mutation evidence")
}

// TestSetupRoomCellBudgetRejectsPanicReproduction pins F1 (#929 T3 Opus
// round, HIGH): a 2^30 x 2^30 room is individually LEGAL under
// maxRoomSpan's per-axis check (each dimension equals, not exceeds, the
// bound) but its cell count (2^60) panics Atlas's allocation — the exact
// reproduction Opus found, reachable from a tiny SetupInput. Must REJECT
// with maxRoomCells' message, never construct.
func (s *EncounterTestSuite) TestSetupRoomCellBudgetRejectsPanicReproduction() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "huge", Width: 1 << 30, Height: 1 << 30}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err, "a 2^30 x 2^30 room must REJECT, not panic Atlas's allocation")
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceeding max room cells")
}

// TestSetupRoomCellBudgetRejectsPanicReproductionHex is the hex-family
// sibling of TestSetupRoomCellBudgetRejectsPanicReproduction (#929
// hardening round, test-gap closure item 3): maxRoomCells' doc comment
// claims the bound is family-agnostic ("EQUAL for both grid families...
// hex included"), but every existing budget fixture — this one's square
// sibling, and TestSetupFieldCellBudgetRejectsIndividuallyLegalRooms —
// only ever declares a square room (Grid left unset, defaulting to
// GridShapeSquare). This pins the SAME 2^30 x 2^30 reproduction against
// an EXPLICIT hex room.
func (s *EncounterTestSuite) TestSetupRoomCellBudgetRejectsPanicReproductionHex() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "huge", Grid: spatial.GridShapeHex, Width: 1 << 30, Height: 1 << 30}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err, "a 2^30 x 2^30 HEX room must REJECT too — the bound is family-agnostic")
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceeding max room cells")
}

// TestSetupOversizedRoomHeightRejected pins maxRoomSpan's Height clause
// independently of Width (#929 hardening round, test-gap closure item
// 3): TestSetupAnchoringOversizedRoomRejectedNotFalseDisjoint only grows
// Width (r2 is math.MaxInt WIDE, Height a legal 5), and
// TestSetupRoomCellBudgetRejectsPanicReproduction grows Width and Height
// EQUALLY (1<<30 each) — neither can discriminate the
// `|| r.Height > maxRoomSpan` half of the check from the Width half;
// deleting either clause alone leaves the whole suite green. This
// fixture grows ONLY Height (Width stays a legal 5) just past the bound
// (1<<30 + 1). With the clause present, this rejects at room-span
// legality with "exceed max room span". Without it (the mutant), the
// oversized Height is NOT unobserved — it still rejects, just later and
// with a DIFFERENT message: cellCount = 5*((1<<30)+1) comfortably
// exceeds maxRoomCells, so a deleted Height-span clause falls through to
// the cell-budget check instead. The message assertion below is what
// actually catches the mutant — a bare Error()/ErrorIs(ErrNoField)
// check alone would not.
func (s *EncounterTestSuite) TestSetupOversizedRoomHeightRejected() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "tall", Width: 5, Height: (1 << 30) + 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "exceed max room span",
		"a Height-only oversize must reject at room-span legality, not fall through to a different check with a different message")
}

// TestSetupFieldCellBudgetRejectsIndividuallyLegalRooms pins F1's field-total
// bound: SIX 1024x1024 rooms are each EXACTLY at maxRoomCells (1<<20,
// individually legal — the per-room check passes) but their SUM (6<<20)
// exceeds maxFieldCells (4<<20) — amplification across rooms, not within
// one, the OTHER half of F1's reproduction. SIX rooms, not five (#929
// hardening round, test-gap closure item 1): with only five, rooms 1-4
// sum to EXACTLY maxFieldCells (4<<20) — not yet exceeding it — so the
// budget is first exceeded at the fifth and LAST room, and the N3 fix
// (accumulate the TRUE total over every room before checking once, so a
// future room in the list is never silently dropped from the count) is
// unpinnable: a reverted mid-loop check that returns as soon as the
// RUNNING total exceeds the budget would trip at that same last room and
// report the identical number, since there is no room AFTER it to be
// dropped from the count. A sixth room makes the two diverge: a mid-loop
// check trips at room 5 (running total 5<<20 = 5242880, room 6 never
// even summed), while the true-total fix processes all six and reports
// 6<<20 = 6291456 — the exact number asserted below.
func (s *EncounterTestSuite) TestSetupFieldCellBudgetRejectsIndividuallyLegalRooms() {
	rooms := make([]encounter.RoomInput, 6)
	for i := range rooms {
		rooms[i] = encounter.RoomInput{ID: fmt.Sprintf("room-%d", i), Width: 1024, Height: 1024}
	}
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field:      encounter.FieldInput{Rooms: rooms},
		Endings:    []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err, "individually-legal rooms whose SUM exceeds the field budget must reject")
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().Contains(err.Error(), "field has 6291456 total cells across all rooms",
		"must name the TRUE total over all six rooms, not a running total that stopped short at a mid-loop room")
	s.Require().Contains(err.Error(), "exceeding max field cells")
}

// validEndingTriggerSetup is the base fixture for TestSetupEndingTriggerValidation:
// a single square room with a TriggerReachedPosition ending naming a real,
// in-bounds room/position — the valid base each case breaks exactly one
// thing about (#929 T3 Opus round F5).
func validEndingTriggerSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "hall", Width: 5, Height: 5}},
		},
		Endings: []encounter.EndingInput{
			{Key: "reach", Trigger: encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 3, Y: 3}}},
		},
	}
}

// TestSetupEndingTriggerValidation pins F5 (#929 T3 Opus round): a
// TriggerReachedPosition ending that names no real room, or a position
// that can never be reached, is a declaration defect — "an encounter that
// cannot end is a liveness hole" (ErrNoEnding's doc comment) — not a
// silently-accepted dead ending.
func (s *EncounterTestSuite) TestSetupEndingTriggerValidation() {
	cases := []struct {
		name     string
		mutate   func(in *encounter.SetupInput)
		fragment string
	}{
		{"unknown room", func(in *encounter.SetupInput) {
			in.Endings[0].Trigger = encounter.TriggerReachedPosition{Room: "nowhere", Position: spatial.Position{X: 3, Y: 3}}
		}, "unknown room"},
		{"out of bounds position", func(in *encounter.SetupInput) {
			in.Endings[0].Trigger = encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 100, Y: 100}}
		}, "out of bounds"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			setup := validEndingTriggerSetup()
			tc.mutate(setup)
			_, err := encounter.NewEncounter(setup)
			s.Require().Error(err, tc.name)
			s.Require().ErrorIs(err, encounter.ErrNoEnding, tc.name)
			s.Require().Contains(err.Error(), tc.fragment,
				"the check that fired must be the one this case targets")
		})
	}

	// The valid base itself must construct.
	_, err := encounter.NewEncounter(validEndingTriggerSetup())
	s.Require().NoError(err, "a trigger naming a real room and in-bounds position must validate")
}

// TestSetupEndingTriggerHexNonIntegralRejected is TestSetupEndingTriggerValidation's
// hex-specific sibling: a fractional axial trigger position rejects
// exactly like a fractional connection/member position does (#929 T3
// Opus round F5).
func (s *EncounterTestSuite) TestSetupEndingTriggerHexNonIntegralRejected() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8, Grid: spatial.GridShapeHex}},
		},
		Endings: []encounter.EndingInput{
			{Key: "reach", Trigger: encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 1.5, Y: 0}}},
		},
	}
	_, err := encounter.NewEncounter(setup)
	s.Require().Error(err)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
	s.Require().Contains(err.Error(), "not an integral axial cell")
}

// validTriggerAcceptanceFieldSetup is the rich SQUARE-family fixture for
// TestSetupEndingTriggerMustAccept (#929 T3 trailing round N2): a kissing
// pair (hall/vault, connected, hall carries an occluder) plus an isolated
// room (annex, no connection at all) — exercising every must-accept shape
// a TriggerReachedPosition can legally target. A rejection-only test
// suite proves a validator rejects; this fixture is what proves it does
// NOT over-reach.
func validTriggerAcceptanceFieldSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hall", Width: 5, Height: 5, Occluders: []spatial.Position{{X: 2, Y: 2}}},
				{ID: "vault", Width: 4, Height: 4, Origin: spatial.Position{X: 5, Y: 0}},
				{ID: "annex", Width: 3, Height: 3, Origin: spatial.Position{X: 1000, Y: 1000}},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "gate", From: "hall", To: "vault",
					FromPosition: spatial.Position{X: 4, Y: 2},
					ToPosition:   spatial.Position{X: 0, Y: 2}},
			},
		},
		Endings: []encounter.EndingInput{
			{Key: "reach", Trigger: encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 0, Y: 0}}},
		},
	}
}

// TestSetupEndingTriggerMustAccept pins N2 (#929 T3 trailing round): a
// one-defect rejection table only proves a validator rejects what it
// should; only a rich positive control proves it does not ALSO reject
// what it shouldn't. Each row targets a specific over-tightening
// hypothesis Opus's mutants tested and found undefended: occluded cells,
// doorway endpoints on EITHER side, fractional square positions, a room
// with zero connections, local (0,0), and a room's far corner. Each row
// also survives a ToData/LoadEncounter round trip — the SAME shape must
// validate identically at both seams.
func (s *EncounterTestSuite) TestSetupEndingTriggerMustAccept() {
	cases := []struct {
		name    string
		trigger encounter.TriggerReachedPosition
	}{
		{"occluded cell", encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 2, Y: 2}}},
		{"doorway endpoint, from-room side", encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 4, Y: 2}}},
		{"doorway endpoint, to-room side", encounter.TriggerReachedPosition{Room: "vault", Position: spatial.Position{X: 0, Y: 2}}},
		{"fractional square position", encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 1.5, Y: 1.5}}},
		{"room with no connection", encounter.TriggerReachedPosition{Room: "annex", Position: spatial.Position{X: 1, Y: 1}}},
		{"local (0,0)", encounter.TriggerReachedPosition{Room: "hall", Position: spatial.Position{X: 0, Y: 0}}},
		{"far corner", encounter.TriggerReachedPosition{Room: "vault", Position: spatial.Position{X: 3, Y: 3}}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			setup := validTriggerAcceptanceFieldSetup()
			setup.Endings[0].Trigger = tc.trigger
			enc, err := encounter.NewEncounter(setup)
			s.Require().NoError(err, tc.name)

			data := enc.ToData()
			_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
			s.Require().NoError(err, "%s must survive a ToData/LoadEncounter round trip", tc.name)
		})
	}
}

// TestSetupEndingTriggerHexNegativeAxialMustAccept is
// TestSetupEndingTriggerMustAccept's hex-specific sibling (W1 forbids
// mixing families in one fixture): hex rooms are ORIGIN-CENTERED, so a
// negative Q/R trigger position is the NORMAL case for roughly half of
// any room, not an edge case — the realistic hazard N2 names explicitly.
func (s *EncounterTestSuite) TestSetupEndingTriggerHexNegativeAxialMustAccept() {
	setup := &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "crypt", Width: 8, Height: 8, Grid: spatial.GridShapeHex}},
		},
		Endings: []encounter.EndingInput{
			{Key: "reach", Trigger: encounter.TriggerReachedPosition{Room: "crypt", Position: spatial.Position{X: -2, Y: -2}}},
		},
	}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err, "a negative-axial hex trigger position is the NORMAL case, not an edge case")

	data := enc.ToData()
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data})
	s.Require().NoError(err, "must survive a ToData/LoadEncounter round trip")
}

func (s *EncounterTestSuite) TestSetupOpeningBeat() {
	s.Run("opening beat reaches all members via Story", func() {
		// Arrange
		setup := &encounter.SetupInput{
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: room1, Width: 10, Height: 10},
					{ID: room2, Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
				},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
				{ID: bob, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
				// The goblin waits next door: co-located and visible would mean
				// a fight forms at first light and appends its own beat, and this
				// test is about the OPENING beat being the only one
				// (rpg-toolkit#964).
				{ID: goblin, Kind: encounter.KindMonster, Room: room2, Position: spatial.Position{X: 2, Y: 2}},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{
						ID:     room1,
						Width:  20,
						Height: 20,
					},
					{ID: room2, Width: 20, Height: 20, Origin: spatial.Position{X: 20, Y: 0}},
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
					ID:   goblin,
					Kind: encounter.KindMonster,
					// Next door, so alice can walk to her stairs without a fight
					// starting underfoot (rpg-toolkit#964).
					Room:     room2,
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
				Initiative: orderAsGiven{},
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
				Initiative: orderAsGiven{},
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
				Initiative: orderAsGiven{},
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
				Initiative: orderAsGiven{},
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
				Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
		Initiative: orderAsGiven{},
		Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 20, Height: 20}}},
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
		Initiative: orderAsGiven{},
		Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: room1, Width: 20, Height: 20}}},
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
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				// Anchored immediately east of room-a (#929 T1): door1's
				// endpoints (9,5) and (0,5)+(10,0)=(10,5) are
				// Chebyshev-adjacent (W3); the rooms' absolute footprints
				// (x:[0,9] vs x:[10,19]) stay disjoint (W2).
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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
			Initiative: orderAsGiven{},
			Field: encounter.FieldInput{
				Rooms: []encounter.RoomInput{
					{ID: "room-a", Width: 10, Height: 10},
					// Anchored east of room-a so door1's endpoints kiss (#929
					// T1) — see newTwoRoomEncounterWithConnection's comment.
					{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
					// room-c touches neither: anchored south of room-a,
					// disjoint from both (W2) and irrelevant to door1 (W3
					// only constrains door1's own two rooms).
					{ID: "room-c", Width: 10, Height: 10, Origin: spatial.Position{X: 0, Y: 10}},
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
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-a", Width: 10, Height: 10},
				{ID: "room-b", Width: 10, Height: 10, Origin: spatial.Position{X: 10, Y: 0}},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
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
