// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// wall_footprint_test.go is fog-of-war R9 (rpg-project#600): a wall is known
// where sight reached a hex its footprint overlaps. The structural wall record
// carries those hexes, in order along the wall, so a client grades each
// stretch by them; presenting the wall teaches none of them, and sight learns
// a footing hex through the wall's face by the cover rule.

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type WallFootprintSuite struct {
	suite.Suite
}

func TestWallFootprintSuite(t *testing.T) {
	suite.Run(t, new(WallFootprintSuite))
}

// hexSide is the edge length of the fixture hexes: CellWidth 5, pointy top,
// so the flat-to-flat width is 5 and each edge is 5/sqrt(3).
var hexSide = 5 / math.Sqrt(3)

// wallAlong is a structural wall on the line from→to, with a presence
// placement that is that line at the given thickness. blocks makes the
// presence stop movement and sight, which is what the cover rule reads.
func wallAlong(id string, from, to spatial.Point, thickness float64, blocks bool) (encounter.PlacedPropInput, encounter.StructuralWallInput) {
	dx, dy := to.X-from.X, to.Y-from.Y
	facing := math.Atan2(dy, dx) * 180 / math.Pi
	mid := spatial.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	presence := placed(id, thinWall(math.Hypot(dx, dy), thickness, facing, mid), blocks, blocks)

	return presence, encounter.StructuralWallInput{
		ID: encounter.PropID(id), Ref: "dnd5e:env:test:wall",
		From: from, To: to, Height: 3, Thickness: thickness,
	}
}

func (s *WallFootprintSuite) play(field encounter.FieldInput, sight encounter.Sight, members ...encounter.MemberInput) *encounter.Encounter {
	s.T().Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sheets:        zeroSheets{},
			Sight:         sight,
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

func (s *WallFootprintSuite) wallIn(atlas encounter.Atlas, id encounter.PropID) encounter.AtlasStructuralWall {
	s.T().Helper()
	for _, w := range atlas.StructuralWalls {
		if w.ID == id {
			return w
		}
	}
	s.FailNow("wall not in atlas", "%s", id)

	return encounter.AtlasStructuralWall{}
}

// centreLine is a vertical wall through the centre of offset column 2 on the
// odd rows, from row 4's centre up to row 0's. On pointy hexes the odd rows sit
// half a width right of the even ones, so the line runs THROUGH the odd-row
// hexes' centres (footing) and ALONG the shared edge of two hexes on each even
// row (straddled) — the cell block's west wall in rpg-project#600.
func centreLine() (spatial.Point, spatial.Point) {
	x := centreOf(cellAt(2, 1)).X
	return spatial.Point{X: x, Y: centreOf(cellAt(2, 4)).Y}, spatial.Point{X: x, Y: centreOf(cellAt(2, 0)).Y}
}

// TestStructuralWallCarriesItsFootprintCells: the record lists every floor hex
// the wall's footprint overlaps by positive area, in order from From to To.
func (s *WallFootprintSuite) TestStructuralWallCarriesItsFootprintCells() {
	s.Run("an edge wall lists the hex on either side", func() {
		a, b := centreOf(cellAt(2, 1)), centreOf(cellAt(3, 1))
		x, y := (a.X+b.X)/2, a.Y
		// Most of the shared vertical edge, so the box touches no third hex
		// at the edge's end vertices.
		half := 0.4 * hexSide
		presence, wall := wallAlong("edge", spatial.Point{X: x, Y: y + half}, spatial.Point{X: x, Y: y - half}, 0.3, false)
		field := placedField(presence)
		field.StructuralWalls = []encounter.StructuralWallInput{wall}

		atlas, err := s.play(field, everyoneSeesTheWholeMap{},
			encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: cellAt(0, 0)}).Atlas()
		s.Require().NoError(err)
		s.Equal([]spatial.Position{cellAt(2, 1), cellAt(3, 1)}, s.wallIn(atlas, "edge").FootprintCells,
			"both sides of the edge and nothing else, level along the line so in map order")
	})

	s.Run("a centre-line wall lists its footing hexes and the straddled neighbours, in order along the line", func() {
		from, to := centreLine()
		presence, wall := wallAlong("west", from, to, 0.3, false)
		field := placedField(presence)
		field.StructuralWalls = []encounter.StructuralWallInput{wall}

		atlas, err := s.play(field, everyoneSeesTheWholeMap{},
			encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: cellAt(0, 0)}).Atlas()
		s.Require().NoError(err)

		// Rows 4 → 0, because the wall runs from row 4 to row 0: on each
		// even row the two hexes the line straddles, on each odd row the
		// one it stands on. Within a row the map's own order.
		want := []spatial.Position{
			cellAt(2, 4), cellAt(3, 4),
			cellAt(2, 3),
			cellAt(2, 2), cellAt(3, 2),
			cellAt(2, 1),
			cellAt(2, 0), cellAt(3, 0),
		}
		for _, row := range [][]spatial.Position{want[0:2], want[3:5], want[6:8]} {
			s.Require().Less(math.Abs(centreOf(row[0]).X+centreOf(row[1]).X-2*from.X), 1e-9,
				"fixture: the even-row pair straddles the line")
		}
		got := s.wallIn(atlas, "west").FootprintCells
		s.Equal(want, got)

		// The order is the line's, not the map's: map order would put the
		// rows the other way round.
		mapOrder := append([]spatial.Position(nil), got...)
		sort.Slice(mapOrder, func(i, j int) bool {
			if mapOrder[i].X != mapOrder[j].X {
				return mapOrder[i].X < mapOrder[j].X
			}
			return mapOrder[i].Y < mapOrder[j].Y
		})
		s.NotEqual(mapOrder, got, "fixture: line order differs from map order")
	})

	s.Run("every member's atlas carries the same cells, known or not", func() {
		from, to := centreLine()
		presence, wall := wallAlong("west", from, to, 0.3, true)
		field := placedField(presence)
		field.StructuralWalls = []encounter.StructuralWallInput{wall}
		enc := s.play(field, &sightList{reach: map[encounter.MemberID]int{"walker": 1}},
			encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: cellAt(1, 0)})

		full, err := enc.Atlas()
		s.Require().NoError(err)
		scoped, err := enc.AtlasFor("walker")
		s.Require().NoError(err)
		s.Equal(s.wallIn(full, "west").FootprintCells, s.wallIn(scoped, "west").FootprintCells,
			"the record is the wall's geometry; what the walker knows is the atlas's cells")
		s.NotContains(scoped.Cells, cellAt(2, 3), "fixture: some footprint cell is unknown to the walker")
	})
}

// TestSightLearnsAWallsFootingByItsFace is the cover rule pinned for walls: a
// member who reaches the floor beside a span learns the footing hex behind
// that face, and no other footing hex — presenting the wall taught none.
func (s *WallFootprintSuite) TestSightLearnsAWallsFootingByItsFace() {
	from, to := centreLine()
	presence, wall := wallAlong("west", from, to, 0.3, true)
	field := placedField(presence)
	field.StructuralWalls = []encounter.StructuralWallInput{wall}

	// One hex of light at the top of the west side: the walker reaches the
	// floor beside the wall's row-1 span and nothing beside its row-3 span.
	enc := s.play(field, &sightList{reach: map[encounter.MemberID]int{"walker": 1}},
		encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: cellAt(1, 0)})
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)

	s.Equal(encounter.PropID("west"), s.wallIn(atlas, "west").ID, "the wall is presented")
	s.Contains(atlas.Cells, cellAt(1, 1), "fixture: the floor beside the row-1 face is reached")
	s.Contains(atlas.Cells, cellAt(2, 1), "the footing hex behind the face the walker reached is learned")
	s.NotContains(atlas.Cells, cellAt(2, 3), "a footing hex whose face nobody reached is not on the map")
	for _, c := range []spatial.Position{cellAt(3, 0), cellAt(3, 2), cellAt(3, 4)} {
		s.NotContains(atlas.Cells, c, "nor any hex on the far side of the wall: %v", c)
	}
}
