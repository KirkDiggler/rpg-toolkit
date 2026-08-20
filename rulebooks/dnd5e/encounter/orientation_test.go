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
// Measured against the constructor before this file existed, on the reference
// tomb's own three chambers:
//
//   - pointy-top constructed, and RegionAt then reported 380 cells as floor
//     against the 224 the author drew — 156 places a member may stand, and be
//     reported standing, that nobody put there.
//   - flat-top DID NOT CONSTRUCT AT ALL: `room "entrance" and room "hall"
//     overlap at absolute cell (3, -9)`, because W2 compared bounding boxes and
//     the sheared rhombi intersect even though the chambers do not.
//
// So "both settable" was not merely awkward, it was unreachable. What fixes it
// is the room saying which rectangle it is rather than which rhombus contains
// it — the floor mask Kirk deferred on rpg-toolkit#1105 until a caller forced
// one, and this is the caller.
//
// # The mask is a function, not a set
//
// Nothing here stores a cell list. A cell is floor iff converting it back to
// offset space lands inside [0,Width) x [0,Height) — one conversion and two
// comparisons, O(1), from four numbers RoomData already persists (width,
// height, origin, and the field's orientation). See [OrientationInput].

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
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
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

			// And NOTHING else is. Swept over the whole canvas, which is where
			// the 156 phantom cells used to be.
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
				"no cell is floor but undrawn — this counted 380 against the rhombus reading")
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
		Initiative: orderAsGiven{},
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
			Initiative: orderAsGiven{},
		})
		s.Require().ErrorIs(lerr, encounter.ErrNoField)
		s.Contains(lerr.Error(), "orientation")
	})

	s.Run("and one this build has never heard of is too", func() {
		d := enc.ToData()
		d.Field.Canvas.Orientation = "sideways"
		_, lerr := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: d, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
			Initiative: orderAsGiven{},
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
