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

// backwardsWorld is two 4x4 regions joined by a door, painted so that REGION
// ORDER AND COORDINATE ORDER DISAGREE: the alphabetically-first region
// ("alpha") sits to the RIGHT, at columns 4..7, and "beta" occupies 0..3.
//
// That disagreement is the entire reason this fixture exists rather than
// reusing hexWorld. There, the first region by ID is also the leftmost, so a
// map concatenated region by region comes out in coordinate order BY ACCIDENT
// — and an order pin written against it passes with the sorting deleted,
// which is exactly what the first version of this file did.
func backwardsWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("alpha", 4, 0, 4, 4),
				rectRegion("beta", 0, 0, 4, 4),
			},
			Doors: []encounter.DoorInput{{
				ID:    "gate",
				Edges: []encounter.DoorEdge{{From: hexCell(4, 0), To: hexCell(3, 0)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
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
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	// Alpha is painted away from the origin, so its cells only exist at
	// coordinates a projection that dropped or duplicated a region could
	// not produce by accident.
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
		[]string{"Grid", "Layout", "Cells", "Props", "Boundaries", "Doorways", "Regions"},
		fieldsOf(session.Atlas{}),
		"the map is a grid, which way its hexes point, its cells, the things standing on it, its walls, its doorways, and its regions",
	)
	s.Equal(
		[]string{"ID", "Name", "Cells", "Archetype", "Lighting"},
		fieldsOf(session.AtlasRegion{}),
		"a region is a named set of cells and the facts true of that area — never a frame of its own",
	)
	s.Equal(
		[]string{"Door", "From", "To"},
		fieldsOf(session.AtlasDoorway{}),
		"a doorway is a door's identity and two cells",
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

	s.Len(atlas.Cells, 32, "two 4x4 regions, folded into one map")

	seen := map[spatial.Position]bool{}
	for _, cell := range atlas.Cells {
		s.False(seen[cell], "cell %v appears twice on a map whose regions cannot overlap", cell)
		seen[cell] = true
	}

	// Cells from BOTH regions are present.
	s.True(seen[hexCell(0, 0)], "beta's corner, at the origin")
	s.True(seen[hexCell(7, 3)], "alpha's far corner, painted at [4,0]")
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
	s.Equal("gate", gate.Door)
	s.Equal(hexCell(3, 0), gate.From, "beta's east edge, first in coordinate order")
	s.Equal(hexCell(4, 0), gate.To, "alpha's west edge")
	s.Equal(1, hexSteps(gate.From, gate.To), "one step apart on the map")
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
