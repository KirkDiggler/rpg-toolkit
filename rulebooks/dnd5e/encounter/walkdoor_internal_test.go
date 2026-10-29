// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// walkdoor_internal_test.go pins the two answers of rpg-project#715 that no
// walk can show from outside: the door question refuses a step that a second
// door would still stop, and a threat zone — an estimate of somebody ELSE'S
// reach — still treats a shut door as shut.
//
// THE HALL is the placed-props fixture's 5x5 room. The leaf stands on the
// line x = 12.5 and spans the hall end to end, so it lies across the crossing
// between the even rows' (2,0) and (3,0), whose centres are x = 10 and x = 15.

type WalkDoorInternalSuite struct{ suite.Suite }

func TestWalkDoorInternalSuite(t *testing.T) { suite.Run(t, new(WalkDoorInternalSuite)) }

const (
	hallLeaf  DoorID = "leaf"
	hallStone DoorID = "stone"
)

// theLeafPlacement is the door across the hall.
func theLeafPlacement() *spatial.FootprintPlacement {
	return &spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 40, D: 1}},
		Origin:    spatial.Point{X: 12.5, Y: 8.66},
	}
}

// theStonePlacement is a second, small door standing on (3,0)'s centre: its
// west edge lies on the centre, so it covers the cell (contact, boundary
// included) without lying across the crossing into it (interior). Only the
// standing question sees it.
func theStonePlacement() *spatial.FootprintPlacement {
	return &spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 2, D: 2}},
		Origin:    spatial.Point{X: 16, Y: 0},
	}
}

// hall builds the room with the given doors, a guard at (0,4) and Alice, a
// thirty-foot spear carrier, at (0,0).
func (s *WalkDoorInternalSuite) hall(doors ...DoorInput) *Encounter {
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("hall", 0, 0, 5, 5)},
			Doors:   doors,
		},
		Members: []MemberInput{
			{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 0, Y: 4}},
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  UnobservedEquipment{},
			Conditions: UnobservedEquipment{},
			Sheets: sheetFacts{
				"guard": {SpeedFeet: 30},
				"alice": {SpeedFeet: 30, Actions: []ActionView{{RangeFeet: 5}}},
			},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	s.Require().NoError(err)

	return enc
}

func (s *WalkDoorInternalSuite) query(enc *Encounter) *cellQuery {
	q, err := enc.cellQuery()
	s.Require().NoError(err)

	return q
}

// TestASecondDoorOnTheCellMeansTheStepOpensNothing: the leaf lies across the
// step from (2,0) to (3,0), and the stone stands on (3,0). Opening the leaf
// would leave the step refused by the stone, so the step opens nothing. With
// the stone open, the same step opens the leaf.
func (s *WalkDoorInternalSuite) TestASecondDoorOnTheCellMeansTheStepOpensNothing() {
	leaf := DoorInput{ID: hallLeaf, Placement: theLeafPlacement(), State: DoorIsClosed()}
	stone := DoorInput{ID: hallStone, Placement: theStonePlacement(), State: DoorIsClosed()}
	enc := s.hall(leaf, stone)
	from, to := enc.field.cellAt(spatial.Position{X: 2, Y: 0}), enc.field.cellAt(spatial.Position{X: 3, Y: 0})
	s.Require().NotNil(enc.field.doorStandingOn(to), "precondition: the stone stands on the far cell")
	_, across, err := enc.field.crossingBlocksExcept(from, to, hallLeaf)
	s.Require().NoError(err)
	s.Require().False(across, "precondition: and lies across nothing the leaf does not")

	door, ok, err := s.query(enc).doorToOpen("guard", from, to)
	s.Require().NoError(err)
	s.False(ok, "two doors in the way: opening one would be the creature doing something for nothing")
	s.Nil(door)

	stone.State = DoorIsOpen()
	enc = s.hall(leaf, stone)
	door, ok, err = s.query(enc).doorToOpen("guard", from, to)
	s.Require().NoError(err)
	s.Require().True(ok, "control: with the stone open, the step opens the leaf")
	s.Equal(hallLeaf, door.id)
}

// TestAThreatZoneStillStopsAtAShutDoor: the zone is where Alice could reach
// on her next turn, as the guard pictures it. With the leaf shut it does not
// reach (4,0) on the far side; with the leaf open it does. A creature's own
// walk opens doors; an estimate of somebody else's does not.
func (s *WalkDoorInternalSuite) TestAThreatZoneStillStopsAtAShutDoor() {
	zoneReaches := func(state DoorState) bool {
		enc := s.hall(DoorInput{ID: hallLeaf, Placement: theLeafPlacement(), State: state})
		at := enc.field.cellAt(spatial.Position{X: 0, Y: 0})
		zone, err := s.query(enc).threatZone("guard", "alice", at)
		s.Require().NoError(err)
		far := enc.field.cellAt(spatial.Position{X: 4, Y: 0})

		return slices.Contains(zone.Walk, far) || slices.Contains(zone.Dash, far)
	}

	s.True(zoneReaches(DoorIsOpen()), "control: through an open door she reaches the far side")
	s.False(zoneReaches(DoorIsClosed()), "a shut door still bounds where she can reach")
}
