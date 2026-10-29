// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PartitionFloorSuite struct{ suite.Suite }

func TestPartitionFloorSuite(t *testing.T) { suite.Run(t, new(PartitionFloorSuite)) }

func (s *PartitionFloorSuite) TestClosedTopologyDoesNotMutateInitialDoorState() {
	field := encounter.FieldInput{
		Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)},
		Doors: []encounter.DoorInput{{ID: "door", Edges: []encounter.DoorEdge{{From: cellAt(0, 0), To: cellAt(1, 0)}}, State: encounter.DoorIsOpen()}},
	}
	open, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Equal([][]spatial.Position{{cellAt(0, 0)}, {cellAt(1, 0), cellAt(2, 0)}}, open.Components)
	s.Equal(encounter.DoorOpen, field.Doors[0].State.Kind())
	field.Doors[0].State = encounter.DoorIsClosed()
	closed, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Equal(open, closed)
	open.Components[0][0].X = 999
	s.Equal(cellAt(0, 0), field.Regions[0].Cells[0], "partition cannot alias its input")
}

func (s *PartitionFloorSuite) TestUsesCanonicalGeometryAndReturnsAuthoredCells() {
	for _, orientation := range []encounter.Orientation{encounter.HexesArePointyTop(), encounter.HexesAreFlatTop()} {
		field := encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsTransparent(), Orientation: orientation},
			Regions: []encounter.RegionInput{rectRegion("floor", 0, 2, 3, 1)},
			Walls:   []encounter.WallInput{wall(0, 2, 1, 2)},
		}
		out, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
		s.Require().NoError(err)
		s.Equal([][]spatial.Position{{{X: 0, Y: 2}}, {{X: 1, Y: 2}, {X: 2, Y: 2}}}, out.Components)
		field.Walls[0].BlocksLineOfSight = false
		transparent, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
		s.Require().NoError(err)
		s.Len(transparent.Components, 1, "movement blocking alone cannot hide floor")
	}
}

func (s *PartitionFloorSuite) TestAnInternalOpaquePillarIsNotAnotherRoom() {
	yes := true
	field := encounter.FieldInput{
		Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 3)},
		Props: []encounter.PropInput{{ID: "pillar", Ref: "dnd5e:props:pillar", At: cellAt(1, 1), BlocksMovement: &yes, BlocksLineOfSight: &yes}},
	}
	out, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Require().Len(out.Components, 1, "opaque footing bordering one room belongs to that room's fixed layout")
	s.ElementsMatch(field.Regions[0].Cells, out.Components[0])
}

func (s *PartitionFloorSuite) TestDoorAndSightOnlyCellsDoNotBecomePermanentFooting() {
	placement := coveredBox(1.5, spatial.Point{X: 5, Y: 0})
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)},
		Doors: []encounter.DoorInput{{ID: "door", Placement: &placement, State: encounter.DoorIsOpen()}},
	}
	out, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Empty(out.Footing, "an openable door cannot permanently remove ownership/standing")
	s.Require().NoError(encounter.ValidateStaticPlacements(field, []spatial.Position{cellAt(1, 0)}))
	field.Doors = nil
	field.Placed = []encounter.PlacedPropInput{placed("screen", placement, false, true)}
	out, err = encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Empty(out.Footing, "sight blocking alone must not remove standing")
	s.Require().NoError(encounter.ValidateStaticPlacements(field, []spatial.Position{cellAt(1, 0)}))
}

func (s *PartitionFloorSuite) TestMovablePropsDoNotDefineFixedRooms() {
	p := placed("movable", coveredBox(1.5, spatial.Point{X: 5, Y: 0}), true, true)
	p.Holdable = true
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)}, Placed: []encounter.PlacedPropInput{p}}
	out, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Require().Len(out.Components, 1)
	s.Len(out.Components[0], 3)
	s.Empty(out.Footing)
	s.True(field.Placed[0].Holdable)
	s.True(field.Placed[0].BlocksLineOfSight, "partition must not edit actual contributor state")
}

// TestPartitionFloor_TwoRegionsOneOpenSpace: two authored regions sharing an
// open edge, nothing built between them, are ONE partition — the floor is
// partitioned across region boundaries, never within them (rpg-project#725,
// R4). Each cell comes back in its input frame.
func (s *PartitionFloorSuite) TestPartitionFloor_TwoRegionsOneOpenSpace() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
		rectRegion("yard", 0, 0, 2, 1), rectRegion("gate", 2, 0, 2, 1),
	}}
	out, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Equal([][]spatial.Position{{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}}}, out.Components)

	// And a wall on the seam makes two.
	field.Walls = []encounter.WallInput{wall(1, 0, 2, 0)}
	out, err = encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Require().NoError(err)
	s.Equal([][]spatial.Position{{{X: 0, Y: 0}, {X: 1, Y: 0}}, {{X: 2, Y: 0}, {X: 3, Y: 0}}}, out.Components)
}

// TestPartitionFloor_RejectsInvalidInputs: the normal construction gates
// refuse a field or a door that is not one, and nothing is returned.
func (s *PartitionFloorSuite) TestPartitionFloor_RejectsInvalidInputs() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 2, 1)}}
	field.Doors = []encounter.DoorInput{{ID: "invalid", State: nil}}
	bad, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	s.Error(err)
	s.Empty(bad.Components)
	bad, err = encounter.PartitionFloor(encounter.PartitionFloorInput{Field: encounter.FieldInput{Canvas: pointyCanvas()}})
	s.ErrorIs(err, encounter.ErrNoField)
	s.Empty(bad.Components)
}
