// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// orientation_test.go is A CHAMBER IS THE RECTANGLE SOMEBODY DREW
// (rpg-toolkit#1127's mask, and Kirk's ruling that "flat and pointy top are
// both valid and should be settable").
//
// A hex room's Width and Height have always been read as an AXIAL span — a
// rhombus, centred on the origin. Authors do not draw rhombi. Every authored
// dungeon in this project, the reference tomb included, is a run of rectangular
// chambers in OFFSET coordinates, and an offset rectangle shears when it is
// converted to axial: its cells are still a perfectly good, contiguous,
// non-overlapping set, but the smallest rhombus containing them is strictly
// bigger than they are.
//
// Measured against the constructor, on the reference tomb's own three
// chambers — the ones this file builds:
//
//   - NEITHER ORIENTATION CONSTRUCTED. `room "entrance" and room "hall"
//     overlap at absolute cell (1, -4)`, pointy and flat alike, because W2
//     compared origin-centred spans and those intersect even though the
//     chambers do not.
//   - With W2 disabled so the footprint itself could be seen: of the 224 cells
//     the author drew, 25 fell inside the rhombi under pointy-top and 24 under
//     flat-top. 88% of what RegionAt then called floor was somewhere nobody
//     drew, and 199 of the drawn cells were not floor at all.
//
// So "both settable" was not merely awkward, it was unreachable — and so was
// either. What fixes it is the room saying which rectangle it is rather than
// which rhombus contains it: the floor mask Kirk deferred on rpg-toolkit#1105
// until a caller forced one, and this is the caller.
//
// # The mask is a function, not a set
//
// Nothing here stores a cell list. A cell is floor iff converting it back to
// offset space lands inside [0,Width) x [0,Height) — one conversion and two
// comparisons, O(1), from four numbers RoomData already persists (width,
// height, origin, and the field's orientation). See
// [encounter.CanvasInput.Orientation].

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type OrientationSuite struct {
	suite.Suite
}

func TestOrientationSuite(t *testing.T) {
	suite.Run(t, new(OrientationSuite))
}

// The reference tomb's own shape: three chambers, 6/10/12 wide, all 8 tall,
// laid left to right in offset columns 0..27.
type authoredChamber struct {
	id      string
	col0, w int
}

var tombChambers = []authoredChamber{{"entrance", 0, 6}, {"hall", 6, 10}, {"tomb", 16, 12}}

const tombHeight = 8

// hexTomb builds the tomb as the author drew it: each chamber an offset
// rectangle, anchored so its columns land where the layout says.
//
// The Origin is in OFFSET columns now, which is the change: a room's anchor
// says where its rectangle starts, not where the centre of a rhombus sits.
func hexTomb(o encounter.Orientation) encounter.FieldInput {
	rooms := make([]encounter.RoomInput, 0, len(tombChambers))
	for _, c := range tombChambers {
		rooms = append(rooms, encounter.RoomInput{
			ID: c.id, Grid: spatial.GridShapeHex, Width: c.w, Height: tombHeight,
			Origin: spatial.Position{X: float64(c.col0), Y: 0},
		})
	}

	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: o},
		Rooms:  rooms,
	}
}

func (s *OrientationSuite) build(field encounter.FieldInput, at spatial.Position, room string) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: "delve", Kind: encounter.KindPlayer, Room: room, Position: at},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
}

// TestTheFloorIsExactlyWhatWasDrawn is the whole point, asked of RegionAt
// itself rather than of any arithmetic of this test's own.
func (s *OrientationSuite) TestTheFloorIsExactlyWhatWasDrawn() {
	for _, o := range []struct {
		name string
		o    encounter.Orientation
	}{
		{"pointy-top", encounter.HexesArePointyTop()},
		{"flat-top", encounter.HexesAreFlatTop()},
	} {
		s.Run(o.name, func() {
			enc, err := s.build(hexTomb(o.o), spatial.Position{X: 0, Y: 0}, "entrance")
			s.Require().NoError(err, "the authored chambers must construct — flat-top could not, before the mask")

			// Every cell the author drew is floor, and belongs to its own
			// chamber.
			drawn := 0
			for _, c := range tombChambers {
				for col := c.col0; col < c.col0+c.w; col++ {
					for row := 0; row < tombHeight; row++ {
						cell := encounter.HexCellAt(o.o, col, row)
						region, floor := enc.RegionAt(cell)
						s.Require().True(floor, "%s [%d,%d] -> %v must be floor", c.id, col, row, cell)
						s.Require().Equal(c.id, region, "and must belong to %s", c.id)
						drawn++
					}
				}
			}
			s.Require().Equal(224, drawn, "the tomb is 6+10+12 wide and 8 tall")

			// And NOTHING else is. Swept over the whole canvas, which is
			// where the rhombus reading's phantom floor was.
			canvas, cerr := enc.Canvas()
			s.Require().NoError(cerr)
			dims := canvas.GetGrid().GetDimensions()

			floorCells := 0
			for q := -int(dims.Width); q <= int(dims.Width); q++ {
				for r := -int(dims.Height); r <= int(dims.Height); r++ {
					if _, ok := enc.RegionAt(spatial.Position{X: float64(q), Y: float64(r)}); ok {
						floorCells++
					}
				}
			}
			s.Equal(224, floorCells,
				"no cell is floor but undrawn — under the rhombus reading 88%% of the "+
					"reported floor was somewhere nobody drew")
		})
	}
}

// TestTheChambersStillKiss — W3's promise, on the authored shape. A seam is
// only a seam if the cells either side of it are adjacent, and the whole
// dungeon depends on it: a doorway is an ordinary step between two adjacent
// absolute cells.
func (s *OrientationSuite) TestTheChambersStillKiss() {
	for _, o := range []struct {
		name string
		o    encounter.Orientation
	}{
		{"pointy-top", encounter.HexesArePointyTop()},
		{"flat-top", encounter.HexesAreFlatTop()},
	} {
		s.Run(o.name, func() {
			enc, err := s.build(hexTomb(o.o), spatial.Position{X: 0, Y: 0}, "entrance")
			s.Require().NoError(err)

			canvas, cerr := enc.Canvas()
			s.Require().NoError(cerr)
			grid := canvas.GetGrid()

			for _, seam := range []int{5, 15} {
				adjacent := 0
				for row := 0; row < tombHeight; row++ {
					near := encounter.HexCellAt(o.o, seam, row)
					far := encounter.HexCellAt(o.o, seam+1, row)
					if grid.Distance(near, far) == 1 {
						adjacent++
					}
				}
				s.Equal(tombHeight, adjacent,
					"every row of the seam at %d|%d must kiss, or a doorway is not a step", seam, seam+1)
			}
		})
	}
}

// TestAFieldSaysWhichWayItsHexesPoint — required exactly where it means
// something, and refused where it does not.
//
// A square grid has no orientation, so declaring one is an author believing
// something that cannot be true about their own dungeon. Refusing it is the
// same call [Void] makes about a word this build does not know: the honest
// answer to a statement with no meaning is not to ignore it.
func (s *OrientationSuite) TestAFieldSaysWhichWayItsHexesPoint() {
	s.Run("a hex field must say", func() {
		field := hexTomb(encounter.HexesArePointyTop())
		field.Canvas.Orientation = nil

		_, err := s.build(field, spatial.Position{X: 0, Y: 0}, "entrance")
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(strings.ToLower(err.Error()), "orientation")
	})

	s.Run("a square field must not", func() {
		field := encounter.FieldInput{
			Canvas: encounter.CanvasInput{
				Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop(),
			},
			Rooms: []encounter.RoomInput{{ID: "hall", Width: 6, Height: 6}},
		}

		_, err := s.build(field, spatial.Position{X: 1, Y: 1}, "hall")
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(strings.ToLower(err.Error()), "orientation")
	})

	s.Run("and a square field is untouched by any of this", func() {
		field := encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "hall", Width: 6, Height: 6}},
		}

		enc, err := s.build(field, spatial.Position{X: 1, Y: 1}, "hall")
		s.Require().NoError(err)

		region, floor := enc.RegionAt(spatial.Position{X: 5, Y: 5})
		s.True(floor)
		s.Equal("hall", region)
		_, floor = enc.RegionAt(spatial.Position{X: 6, Y: 5})
		s.False(floor, "a square room is its rectangle, exactly as it always was")
	})
}

// TestTheOrientationSurvivesASave — it is construction truth, and a reload that
// forgot it would read every stored cell in the wrong frame.
func (s *OrientationSuite) TestTheOrientationSurvivesASave() {
	enc, err := s.build(hexTomb(encounter.HexesAreFlatTop()), spatial.Position{X: 0, Y: 0}, "entrance")
	s.Require().NoError(err)

	data := enc.ToData()
	s.Equal("flat", data.Field.Canvas.Orientation)

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().NoError(err)

	region, floor := back.RegionAt(encounter.HexCellAt(encounter.HexesAreFlatTop(), 20, 6))
	s.True(floor)
	s.Equal("tomb", region)

	s.Run("and a hex blob without one is refused by name", func() {
		d := enc.ToData()
		d.Field.Canvas.Orientation = ""
		_, lerr := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: d, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		})
		s.Require().ErrorIs(lerr, encounter.ErrNoField)
		s.Contains(lerr.Error(), "orientation")
	})

	s.Run("and one this build has never heard of is too", func() {
		d := enc.ToData()
		d.Field.Canvas.Orientation = "sideways"
		_, lerr := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: d, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		})
		s.Require().ErrorIs(lerr, encounter.ErrNoField)
		s.Contains(lerr.Error(), "sideways")
	})
}

// TestTheMapReportsTheOrientation — a host drawing the floor needs it, because
// the region report says "a Width x Height rectangle anchored here" and that
// sentence is ambiguous without knowing which way the hexes point.
func (s *OrientationSuite) TestTheMapReportsTheOrientation() {
	enc, err := s.build(hexTomb(encounter.HexesArePointyTop()), spatial.Position{X: 0, Y: 0}, "entrance")
	s.Require().NoError(err)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().NotNil(atlas.Orientation)
	s.Equal(encounter.OrientationPointyTop, atlas.Orientation.Kind())
}

// TestChambersThatDoOverlapAreStillRefused — the mask must not buy flat-top's
// construction by making W2 toothless.
func (s *OrientationSuite) TestChambersThatDoOverlapAreStillRefused() {
	for _, o := range []struct {
		name string
		o    encounter.Orientation
	}{
		{"pointy-top", encounter.HexesArePointyTop()},
		{"flat-top", encounter.HexesAreFlatTop()},
	} {
		s.Run(o.name, func() {
			field := hexTomb(o.o)
			// Slide the hall one column west, straight into the entrance.
			field.Rooms[1].Origin = spatial.Position{X: 5, Y: 0}

			_, err := s.build(field, spatial.Position{X: 0, Y: 0}, "entrance")
			s.Require().ErrorIs(err, encounter.ErrNoField)
			s.Contains(err.Error(), "overlap")
		})
	}
}

// TestTheMapReportsDoorwaysInTheAuthoredFrame — [Atlas.Doorways] carries the
// two absolute cells a connection joins, and a HOST STEERS BY THEM: the pursuit
// decider in this package's own fixtures walks through a door by moving to
// AtlasDoorway.ToCell.
//
// Found by mutation, and it was a live defect rather than a gap: Atlas built
// those two cells by adding the room's anchor in axial, which is the arithmetic
// rpg-toolkit#1127 replaced everywhere else. Nothing noticed, because every
// fixture that read a doorway off the Atlas was square. So this asserts the
// pair against the projection an author's own columns and rows name, and then
// asserts the property that makes a doorway a doorway at all — the two cells
// are ADJACENT, which the wrong arithmetic breaks and a coordinate comparison
// alone would not reveal.
func (s *OrientationSuite) TestTheMapReportsDoorwaysInTheAuthoredFrame() {
	for _, o := range []struct {
		name string
		o    encounter.Orientation
	}{
		{"pointy-top", encounter.HexesArePointyTop()},
		{"flat-top", encounter.HexesAreFlatTop()},
	} {
		s.Run(o.name, func() {
			field := hexTomb(o.o)
			// The seams the chambers already share: entrance|hall at column
			// 5|6, hall|tomb at 15|16, both on row 3.
			field.Connections = []encounter.ConnectionInput{
				{ID: "arch", From: "entrance", To: "hall",
					FromPosition: spatial.Position{X: 5, Y: 3}, ToPosition: spatial.Position{X: 0, Y: 3}},
				{ID: "stair", From: "hall", To: "tomb",
					FromPosition: spatial.Position{X: 9, Y: 3}, ToPosition: spatial.Position{X: 0, Y: 3}},
			}

			enc, err := s.build(field, spatial.Position{X: 0, Y: 0}, "entrance")
			s.Require().NoError(err)

			atlas, err := enc.Atlas()
			s.Require().NoError(err)
			s.Require().Len(atlas.Doorways, 2)

			canvas, err := enc.Canvas()
			s.Require().NoError(err)
			grid := canvas.GetGrid()

			// Sorted by connection ID (C8), so "arch" then "stair".
			s.Equal(encounter.HexCellAt(o.o, 5, 3), atlas.Doorways[0].FromCell)
			s.Equal(encounter.HexCellAt(o.o, 6, 3), atlas.Doorways[0].ToCell)
			s.Equal(encounter.HexCellAt(o.o, 15, 3), atlas.Doorways[1].FromCell)
			s.Equal(encounter.HexCellAt(o.o, 16, 3), atlas.Doorways[1].ToCell)

			for _, d := range atlas.Doorways {
				s.Equal(1.0, grid.Distance(d.FromCell, d.ToCell),
					"doorway %q joins two cells that must be one step apart", d.Connection)

				fromRegion, fromFloor := enc.RegionAt(d.FromCell)
				toRegion, toFloor := enc.RegionAt(d.ToCell)
				s.True(fromFloor, "doorway %q's near cell must be floor", d.Connection)
				s.True(toFloor, "doorway %q's far cell must be floor", d.Connection)
				s.Equal(string(d.From), string(fromRegion))
				s.Equal(string(d.To), string(toRegion))
			}
		})
	}
}

// TestAChambersEdgeIsAnEdge — the mask's upper bound, asked at the Setup seam
// rather than of RegionAt.
//
// Column Width is the first column the chamber does not have, and a member
// authored there is authored outside their own room. It reads like a
// restatement of TestTheFloorIsExactlyWhatWasDrawn and is not: that test asks
// [Encounter.RegionAt], which runs footprintHolds, while this asks
// NewEncounter, which runs localIsInRoom — a SECOND bounds rule, in the
// authored frame, and mutation found it unpinned at exactly this edge.
func (s *OrientationSuite) TestAChambersEdgeIsAnEdge() {
	o := encounter.HexesArePointyTop()

	s.Run("the last column is inside", func() {
		_, err := s.build(hexTomb(o), spatial.Position{X: 5, Y: 7}, "entrance")
		s.Require().NoError(err, "entrance is 6 wide and 8 tall, so [5,7] is its far corner")
	})

	s.Run("one past it is not", func() {
		_, err := s.build(hexTomb(o), spatial.Position{X: 6, Y: 0}, "entrance")
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Contains(err.Error(), "out of bounds")
	})

	s.Run("nor is one past the last row", func() {
		_, err := s.build(hexTomb(o), spatial.Position{X: 0, Y: 8}, "entrance")
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Contains(err.Error(), "out of bounds")
	})
}

// TestASquareBlobMustNotDeclareAnOrientation — the Load seam's half of the
// refusal TestAFieldSaysWhichWayItsHexesPoint pins at Setup.
//
// #929 T2's standing arrangement is that Load routes through the SAME
// validators Setup uses, and that arrangement is a claim about the code which
// only a test at BOTH seams can hold to. Mutation found this one unpinned:
// deleting the square refusal from the Load path broke nothing.
func (s *OrientationSuite) TestASquareBlobMustNotDeclareAnOrientation() {
	field := encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
		Rooms:  []encounter.RoomInput{{ID: "hall", Width: 6, Height: 6}},
	}
	enc, err := s.build(field, spatial.Position{X: 1, Y: 1}, "hall")
	s.Require().NoError(err)

	data := enc.ToData()
	s.Empty(data.Field.Canvas.Orientation, "a square field writes none")

	data.Field.Canvas.Orientation = "pointy"
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(strings.ToLower(err.Error()), "orientation")
}
