// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// FloorTopologySuite holds [encounter.FloorTopology] to the geometry play
// reads (rpg-project#971): open seams between regions, each door's crossings
// and each placed prop's cells, in the input's authored frame.
type FloorTopologySuite struct{ suite.Suite }

func TestFloorTopologySuite(t *testing.T) { suite.Run(t, new(FloorTopologySuite)) }

// TestTwoRegionsOnOpenFloorMeetOnce: two regions with nothing between them
// meet at their one seam; a wall on the seam leaves no open crossing.
func (s *FloorTopologySuite) TestTwoRegionsOnOpenFloorMeetOnce() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
		rectRegion("yard", 0, 0, 2, 1), rectRegion("gate", 2, 0, 2, 1),
	}}
	out, err := encounter.FloorTopology(encounter.FloorTopologyInput{Field: field})
	s.Require().NoError(err)
	s.Equal([]encounter.FloorCrossing{{From: spatial.Position{X: 1, Y: 0}, To: spatial.Position{X: 2, Y: 0}}}, out.Open)

	field.Walls = []encounter.WallInput{wall(1, 0, 2, 0)}
	out, err = encounter.FloorTopology(encounter.FloorTopologyInput{Field: field})
	s.Require().NoError(err)
	s.Empty(out.Open, "a wall on the seam closes it")
}

// TestADoorIsNotOpenFloor: a door on the seam is the door's crossing, never an
// open one, whatever state it starts in.
func (s *FloorTopologySuite) TestADoorIsNotOpenFloor() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
		rectRegion("yard", 0, 0, 2, 1), rectRegion("gate", 2, 0, 2, 1),
	}, Doors: []encounter.DoorInput{{ID: "door", Edges: []encounter.DoorEdge{{From: cellAt(1, 0), To: cellAt(2, 0)}}, State: encounter.DoorIsOpen()}}}
	out, err := encounter.FloorTopology(encounter.FloorTopologyInput{Field: field})
	s.Require().NoError(err)
	s.Empty(out.Open)
	s.Equal([]encounter.DoorCrossings{{Door: "door", Crossings: []encounter.FloorCrossing{
		{From: spatial.Position{X: 1, Y: 0}, To: spatial.Position{X: 2, Y: 0}},
	}}}, out.Doors)
}

// TestAPlacedPropStandsWhereItCovers: a placed prop's cells come back in the
// authored frame; a cell prop is not listed.
func (s *FloorTopologySuite) TestAPlacedPropStandsWhereItCovers() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 3, 1)},
		Placed: []encounter.PlacedPropInput{placed("crate", coveredBox(1.5, spatial.Point{X: 5, Y: 0}), false, false)},
	}
	out, err := encounter.FloorTopology(encounter.FloorTopologyInput{Field: field})
	s.Require().NoError(err)
	s.Require().Len(out.Placed, 1)
	s.Equal(encounter.PropID("crate"), out.Placed[0].Prop)
	s.Equal([]spatial.Position{{X: 1, Y: 0}}, out.Placed[0].Cells)
}

// TestRefusesWhatConstructionRefuses: an invalid door is refused and nothing
// is returned.
func (s *FloorTopologySuite) TestRefusesWhatConstructionRefuses() {
	field := encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 2, 1)},
		Doors: []encounter.DoorInput{{ID: "invalid"}}}
	out, err := encounter.FloorTopology(encounter.FloorTopologyInput{Field: field})
	s.Error(err)
	s.Equal(encounter.FloorTopologyOutput{}, out)
}
