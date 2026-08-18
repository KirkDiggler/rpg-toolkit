// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// atlasmap_test.go pins that the map this seam hands out is ONE MAP
// (rpg-toolkit#1042). The composition underneath keeps rooms; by the time a
// client sees a map, the decomposition has done its job and is gone.

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type AtlasMapSuite struct {
	suite.Suite

	mgr *session.Manager
}

func TestAtlasMapSuite(t *testing.T) {
	suite.Run(t, new(AtlasMapSuite))
}

// backwardsWorld is two 4x4 square rooms joined by a doorway, anchored so that
// ROOM ORDER AND COORDINATE ORDER DISAGREE: the alphabetically-first room
// ("alpha") sits to the RIGHT, at x 4..7, and "beta" occupies x 0..3.
//
// That disagreement is the entire reason this fixture exists rather than
// reusing hexWorld. There, the first room by ID is also the leftmost, so a map
// concatenated room by room comes out in coordinate order BY ACCIDENT — and an
// order pin written against it passes with the sorting deleted, which is
// exactly what the first version of this file did.
func backwardsWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: encOrderAsGiven{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "alpha", Width: 4, Height: 4, Origin: spatial.Position{X: 4, Y: 0}},
				{ID: "beta", Width: 4, Height: 4, Origin: spatial.Position{X: 0, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "alpha", To: "beta",
				FromPosition: spatial.Position{X: 0, Y: 0},
				ToPosition:   spatial.Position{X: 3, Y: 0},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "beta", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("building backwards world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *AtlasMapSuite) SetupTest() {
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	// Both rooms are anchored away from where their local coordinates would
	// put them, which is what makes the projection observable at all: in a
	// field where a room sits at (0,0), local and absolute are the same number
	// and a map that never projected would look identical to one that did.
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: backwardsWorld(s.T()),
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *AtlasMapSuite) atlas() *session.Atlas {
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess"})
	s.Require().NoError(err)
	return atlas
}

// TestNothingOnTheMapNamesARoom is the slice's whole claim, checked
// structurally rather than by eye.
//
// A field-name assertion rather than a "no room ids in the values" scan: the
// concern is not that today's fixture happens to leave rooms out, it is that
// the TYPE cannot carry one. Adding a room id back — for a renderer that wants
// to chunk by chamber, which is exactly how this would return — fails here
// instead of passing review.
func (s *AtlasMapSuite) TestNothingOnTheMapNamesARoom() {
	s.Equal(
		[]string{"Grid", "Cells", "Occluders", "Boundaries", "Doorways"},
		fieldsOf(session.Atlas{}),
		"the map is a grid, its cells, what blocks sight, its walls, and its doorways",
	)
	s.Equal(
		[]string{"Connection", "From", "To"},
		fieldsOf(session.AtlasDoorway{}),
		"a doorway is an identity and two cells",
	)
	s.Equal(
		[]string{"From", "To", "BlocksMovement", "BlocksLineOfSight"},
		fieldsOf(session.AtlasBoundary{}),
		"a wall is two cells and what it stops",
	)
}

// TestTheMapIsEveryCellOnce: the two rooms' footprints are folded together,
// nothing lost and nothing counted twice.
//
// The count is exact and derived from the fixture's own geometry: two 4x4
// rooms, disjoint by W2, so 32 distinct cells. A projection that dropped the
// second room, or emitted one room twice, moves this number.
func (s *AtlasMapSuite) TestTheMapIsEveryCellOnce() {
	atlas := s.atlas()

	s.Len(atlas.Cells, 32, "two 4x4 rooms, folded into one map")

	seen := map[spatial.Position]bool{}
	for _, cell := range atlas.Cells {
		s.False(seen[cell], "cell %v appears twice on a map whose rooms cannot overlap", cell)
		seen[cell] = true
	}

	// Cells from BOTH anchors are present, and alpha's only exist at these
	// coordinates if the projection happened.
	s.True(seen[spatial.Position{X: 0, Y: 0}], "beta's corner, anchored at the origin")
	s.True(seen[spatial.Position{X: 7, Y: 3}], "alpha's far corner, anchored at (4,0)")
}

// TestTheMapIsInCoordinateOrder guards the subtler half of flattening.
//
// Concatenating room by room would leave the grouping perfectly visible in the
// iteration order — a client could recover the decomposition by looking for the
// seams, and would eventually depend on doing so. Sorting by coordinate means
// the order says nothing about which chamber a cell came from.
func (s *AtlasMapSuite) TestTheMapIsInCoordinateOrder() {
	atlas := s.atlas()

	for i := 1; i < len(atlas.Cells); i++ {
		previous, cell := atlas.Cells[i-1], atlas.Cells[i]
		ordered := previous.X < cell.X || (previous.X == cell.X && previous.Y < cell.Y)
		s.True(ordered, "cell %d %v comes after %v", i, cell, previous)
	}
}

// TestADoorwayIsTwoAdjacentCells is why a crossing can stop being its own verb:
// on one map the two sides of a doorway are one step apart, so walking through
// it is walking.
func (s *AtlasMapSuite) TestADoorwayIsTwoAdjacentCells() {
	atlas := s.atlas()
	s.Require().Len(atlas.Doorways, 1)

	gate := atlas.Doorways[0]
	s.Equal("gate", gate.Connection)
	s.Equal(spatial.Position{X: 4, Y: 0}, gate.From, "alpha-local (0,0), anchored at (4,0)")
	s.Equal(spatial.Position{X: 3, Y: 0}, gate.To, "beta-local (3,0), anchored at the origin")
	s.Equal(float64(1), gate.From.X-gate.To.X, "one step apart on the map")
	s.Equal(gate.From.Y, gate.To.Y)
}

// fieldsOf reports a struct's exported field names in declaration order.
func fieldsOf(v any) []string {
	t := reflect.TypeOf(v)
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			names = append(names, t.Field(i).Name)
		}
	}
	return names
}
