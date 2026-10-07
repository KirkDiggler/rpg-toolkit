// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PartitionRegionSuite struct{ suite.Suite }

func TestPartitionRegionSuite(t *testing.T) { suite.Run(t, new(PartitionRegionSuite)) }

func (s *PartitionRegionSuite) TestClosedTopologyDoesNotMutateInitialDoorState() {
	field := encounter.FieldInput{
		Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)},
		Doors: []encounter.DoorInput{{ID: "door", Edges: []encounter.DoorEdge{{From: cellAt(0, 0), To: cellAt(1, 0)}}, State: encounter.DoorIsOpen()}},
	}
	open, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
	s.Require().NoError(err)
	s.Equal([][]spatial.Position{{cellAt(0, 0)}, {cellAt(1, 0), cellAt(2, 0)}}, open.Components)
	s.Equal(encounter.DoorOpen, field.Doors[0].State.Kind())
	field.Doors[0].State = encounter.DoorIsClosed()
	closed, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
	s.Require().NoError(err)
	s.Equal(open, closed)
	open.Components[0][0].X = 999
	s.Equal(cellAt(0, 0), field.Regions[0].Cells[0], "partition cannot alias its input")
}

func (s *PartitionRegionSuite) TestUsesCanonicalGeometryAndReturnsAuthoredCells() {
	for _, orientation := range []encounter.Orientation{encounter.HexesArePointyTop(), encounter.HexesAreFlatTop()} {
		field := encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsTransparent(), Orientation: orientation},
			Regions: []encounter.RegionInput{rectRegion("floor", 0, 2, 3, 1)},
			Walls:   []encounter.WallInput{wall(0, 2, 1, 2)},
		}
		out, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
		s.Require().NoError(err)
		s.Equal([][]spatial.Position{{{X: 0, Y: 2}}, {{X: 1, Y: 2}, {X: 2, Y: 2}}}, out.Components)
		field.Walls[0].BlocksLineOfSight = false
		transparent, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
		s.Require().NoError(err)
		s.Len(transparent.Components, 1, "movement blocking alone cannot hide floor")
	}
}

func (s *PartitionRegionSuite) TestAnInternalOpaquePillarIsNotAnotherRoom() {
	yes := true
	field := encounter.FieldInput{
		Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 3)},
		Props: []encounter.PropInput{{ID: "pillar", Ref: "dnd5e:props:pillar", At: cellAt(1, 1), BlocksMovement: &yes, BlocksLineOfSight: &yes}},
	}
	out, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
	s.Require().NoError(err)
	s.Require().Len(out.Components, 1, "opaque footing bordering one room belongs to that room's fixed layout")
	s.ElementsMatch(field.Regions[0].Cells, out.Components[0])
}

func (s *PartitionRegionSuite) TestDoorAndSightOnlyCellsDoNotBecomePermanentFooting() {
	placement := coveredBox(1.5, spatial.Point{X: 5, Y: 0})
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)},
		Doors: []encounter.DoorInput{{ID: "door", Placement: &placement, State: encounter.DoorIsOpen()}},
	}
	out, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
	s.Require().NoError(err)
	s.Empty(out.Footing, "an openable door cannot permanently remove ownership/standing")
	s.Require().NoError(encounter.ValidateStaticPlacements(field, []spatial.Position{cellAt(1, 0)}))
	field.Doors = nil
	field.Placed = []encounter.PlacedPropInput{placed("screen", placement, false, true)}
	out, err = encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
	s.Require().NoError(err)
	s.Empty(out.Footing, "sight blocking alone must not remove standing")
	s.Require().NoError(encounter.ValidateStaticPlacements(field, []spatial.Position{cellAt(1, 0)}))
}

func (s *PartitionRegionSuite) TestMovablePropsDoNotDefineFixedRooms() {
	p := placed("movable", coveredBox(1.5, spatial.Point{X: 5, Y: 0}), true, true)
	p.Holdable = true
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)}, Placed: []encounter.PlacedPropInput{p}}
	out, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "floor"})
	s.Require().NoError(err)
	s.Require().Len(out.Components, 1)
	s.Len(out.Components[0], 3)
	s.Empty(out.Footing)
	s.True(field.Placed[0].Holdable)
	s.True(field.Placed[0].BlocksLineOfSight, "partition must not edit actual contributor state")
}

func (s *PartitionRegionSuite) TestNamesItsUniverseAndRejectsInvalidInputs() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
		rectRegion("selected", 0, 0, 2, 1), rectRegion("other", 2, 0, 2, 1),
	}}
	out, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "selected"})
	s.Require().NoError(err)
	s.Equal([][]spatial.Position{{cellAt(0, 0), cellAt(1, 0)}}, out.Components)
	bad, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "missing"})
	s.ErrorIs(err, encounter.ErrNoRegion)
	s.Empty(bad.Components)
	field.Doors = []encounter.DoorInput{{ID: "invalid", State: nil}}
	bad, err = encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: "selected"})
	s.Error(err)
	s.Empty(bad.Components)
}
