// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// lines_test.go is ACCEPTANCE A7, A8 and A12 (rpg-project#360, wall-geometry
// design §7): a room fenced by four lines, what each kind of line costs the
// cells it passes through, and corners that close.
//
// The scenes here are whole small dungeons rather than edits of the tomb,
// because what they are about is the SHAPE of a wall, which is easier to read
// written out than diffed in. Every expectation is computed from the design's
// own arithmetic or from the cell sets themselves — never echoed back out of
// the compiler.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type LinesSuite struct {
	suite.Suite
}

func TestLinesSuite(t *testing.T) {
	suite.Run(t, new(LinesSuite))
}

func (s *LinesSuite) load(raw string) dungeonspec.Compiled {
	compiled, err := dungeonspec.Load([]byte(raw))
	s.Require().NoError(err, "the scene was meant to compile")

	return compiled
}

func (s *LinesSuite) refuse(raw string) []dungeonspec.FieldError {
	spec, err := dungeonspec.Decode([]byte(raw))
	s.Require().NoError(err)

	return dungeonspec.Validate(spec)
}

// rows writes a rectangle of offset cells as the file's own nested rows.
func rows(indent string, colFrom, colTo, rowFrom, rowTo int, skip func(col, row int) bool) string {
	var b strings.Builder
	for row := rowFrom; row <= rowTo; row++ {
		var cells []string
		for col := colFrom; col <= colTo; col++ {
			if skip != nil && skip(col, row) {
				continue
			}
			cells = append(cells, fmt.Sprintf("[%d,%d]", col, row))
		}
		if len(cells) == 0 {
			continue
		}
		b.WriteString(indent + "- [" + strings.Join(cells, ",") + "]\n")
	}

	return b.String()
}

// theFencedRoom is A7's scene: a six-by-six room inside a hall, fenced by FOUR
// LINES — two midpoint lines along its rows and two quarter lines across them,
// each ending where the next begins.
//
// This is the shape the record's room tool drew as a list of crossings, and
// the shape the design calls a square room: its four inside corners are a
// quarter line meeting a midpoint line, which keeps exactly 3/4 of the corner
// cell (§4.3). The hall around it exists so there is something on the far side
// of the fence for the wall to stand between; without it every crossing out of
// the room would be into void and impassable already.
func theFencedRoom() string {
	inner := func(col, row int) bool { return col >= 0 && col <= 5 && row >= 0 && row <= 5 }

	return `
version: 2
key: fenced
orientation: pointy
void: opaque
regions:
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
` + rows("      ", -1, 6, -1, 6, inner) + `  - id: room
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
` + rows("      ", 0, 5, 0, 5, nil) + `start: [-1, -1]
walls:
  - start: { cell: [0,0], offset: [-0.25, -0.375] }
    end:   { cell: [6,0], offset: [-0.25, -0.375] }
    name: north
  - start: { cell: [6,0], offset: [-0.25, -0.375] }
    end:   { cell: [6,6], offset: [-0.25, -0.375] }
    name: east
  - start: { cell: [6,6], offset: [-0.25, -0.375] }
    end:   { cell: [0,6], offset: [-0.25, -0.375] }
    name: south
  - start: { cell: [0,6], offset: [-0.25, -0.375] }
    end:   { cell: [0,0], offset: [-0.25, -0.375] }
    name: west
`
}

// TestA7_FourLinesFenceARoom — four authored walls emit four segments, and the
// crossings they block are exactly the room's own boundary.
//
// THE EXPECTED SET IS COMPUTED FROM THE TWO CELL LISTS, not read back out of
// the compiler: a crossing belongs on the fence if and only if one of its
// cells is in the room and the other is in the hall. A wall derivation that
// blocked one crossing too few (a hole in the fence) or one too many (a step
// inside the room refused) disagrees with that immediately.
func (s *LinesSuite) TestA7_FourLinesFenceARoom() {
	c := s.load(theFencedRoom())

	s.Require().Len(c.Field.Segments, 4, "four walls, four lines for a client to draw")
	for i, want := range []string{"north", "east", "south", "west"} {
		s.Equal(want, c.Field.Segments[i].Name, "in authored order")
	}

	inRoom := map[spatial.Position]bool{}
	for _, r := range c.Field.Regions {
		if r.ID != "room" {
			continue
		}
		for _, cell := range r.Cells {
			inRoom[encounter.HexCellAt(encounter.HexesArePointyTop(), int(cell.X), int(cell.Y))] = true
		}
	}
	s.Require().Len(inRoom, 36, "six by six")

	floor := map[spatial.Position]bool{}
	for _, r := range c.Field.Regions {
		for _, cell := range r.Cells {
			floor[encounter.HexCellAt(encounter.HexesArePointyTop(), int(cell.X), int(cell.Y))] = true
		}
	}

	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})
	want := map[[2]spatial.Position]bool{}
	for cell := range floor {
		for _, n := range grid.GetNeighbors(cell) {
			if !floor[n] || inRoom[cell] == inRoom[n] {
				continue
			}
			pair := [2]spatial.Position{cell, n}
			if n.X < cell.X || (n.X == cell.X && n.Y < cell.Y) {
				pair = [2]spatial.Position{n, cell}
			}
			want[pair] = true
		}
	}

	got := map[[2]spatial.Position]bool{}
	for _, w := range c.Field.Walls {
		got[[2]spatial.Position{
			encounter.HexCellAt(encounter.HexesArePointyTop(), int(w.From.X), int(w.From.Y)),
			encounter.HexCellAt(encounter.HexesArePointyTop(), int(w.To.X), int(w.To.Y)),
		}] = true
	}
	s.Len(want, 46, "a six-by-six room has 46 boundary crossings on a hex grid")
	for crossing := range want {
		s.Contains(got, crossing, "the fence blocks every one of them: %v", crossing)
	}

	// WHAT ELSE IT BLOCKS, and why that is right. Two crossings outside the
	// room are blocked as well, and both are a diagonal step between two hall
	// cells that passes exactly through a CORNER of the fence. A wall's corner
	// is a point on the wall, so a step through it meets the wall (design C7
	// reads both segments closed) — you do not slip past a corner diagonally.
	// The other two corners have no such step through them; which corners do
	// is decided by the row stagger, not by anything a rule could even out.
	for crossing := range got {
		if want[crossing] {
			continue
		}
		s.False(inRoom[crossing[0]], "%v is a step outside the room", crossing)
		s.False(inRoom[crossing[1]], "and so is its other end")
	}
	s.Len(got, 48, "the 46 boundary crossings, plus a corner nobody squeezes past, twice")

	// AND NOTHING IS SEALED. A quarter line takes 5/24, a midpoint line 1/24,
	// and the corners where they meet keep exactly 3/4 — every one of which
	// clears MinStandable, so a room you can fence is a room you can walk.
	s.Empty(c.Field.Sealed, "four thin lines seal nothing, corners included")
}

// TestA12_CornersClose — two walls carrying the same position at an end meet
// there EXACTLY (design F5, A12), because a position is half an axial step and
// halves are exact in binary.
//
// There is no corner concept in the compiler and there does not need to be:
// the designer writes a join by writing the same point twice, and the two
// segments arrive at the far end sharing an endpoint bit for bit. A client
// draws them as one unbroken outline without knowing anything about corners.
func (s *LinesSuite) TestA12_CornersClose() {
	c := s.load(theFencedRoom())
	segs := c.Field.Segments
	s.Require().Len(segs, 4)

	for i, seg := range segs {
		next := segs[(i+1)%len(segs)]
		s.Equal(seg.To, next.From,
			"%s ends exactly where %s begins", seg.Name, next.Name)
	}
	s.Equal(segs[0].From, segs[3].To, "and the outline closes on itself")

	// A THIN CORNER KEEPS ITS CELL. The room's four corner cells are cut by
	// two walls each and are standable; the design's number for them is 3/4.
	for _, corner := range [][2]int{{0, 0}, {5, 0}, {0, 5}, {5, 5}} {
		s.NotContains(c.Field.Sealed, spatial.Position{X: float64(corner[0]), Y: float64(corner[1])},
			"the corner cell [%d,%d] keeps 3/4 and stays standable", corner[0], corner[1])
	}

	// A THICK CORNER TURNS AT A CENTRE, which is the other join the design
	// allows (F15): the cell it turns in is sealed, and that is its whole
	// price. Two lines at 60° to each other through one cell's middle.
	thick := s.load(theTurningWall(
		"  - start: { cell: [1,2], offset: [-0.5, 0] }\n"+
			"    end:   { cell: [3,2], offset: [0, 0] }\n"+
			"    name: in\n",
		"  - start: { cell: [3,2], offset: [0, 0] }\n"+
			"    end:   { cell: [4,4], offset: [0, 0] }\n"+
			"    name: out\n"))
	s.Require().Len(thick.Field.Segments, 2)
	s.Equal(thick.Field.Segments[0].To, thick.Field.Segments[1].From, "the thick corner closes too")
	s.Contains(thick.Field.Sealed, spatial.Position{X: 3, Y: 2},
		"and the cell it turns in is halved, which is what a centre end costs")
}

// theTurningWall is a plain hall for a wall to turn in: eight columns and five
// rows with nothing in it but the two walls the caller writes.
func theTurningWall(walls ...string) string {
	return `
version: 2
key: turning
orientation: pointy
void: opaque
regions:
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
` + rows("      ", 0, 7, 0, 4, nil) + `start: [0, 0]
walls:
` + strings.Join(walls, "")
}

// TestA8_Calibration is what each kind of line COSTS, at MinStandable as it
// stands (design §4.3's table, and the reason 0.7 is the number).
//
// Read against geometry_internal_test.go, which pins the four fractions
// themselves — 1/24, 5/24, 3/4, 7/12 — from the design's arithmetic. This is
// the same four facts one layer up, where an author meets them: which cells
// come back sealed.
func (s *LinesSuite) TestA8_Calibration() {
	s.Run("a midpoint line along the rows seals nothing", func() {
		// Shaves 1/24 off the row above and the row below. 23/24 is standable
		// by a wide margin, so a thin wall never seals a cell on its own.
		c := s.load(theTurningWall(
			"  - start: { cell: [0,2], offset: [-0.25, -0.375] }\n" +
				"    end:   { cell: [7,2], offset: [0.25, -0.375] }\n"))
		s.Empty(c.Field.Sealed)
		s.NotEmpty(c.Field.Segments[0].Footprint, "it does stand on cells — it just leaves them")
	})

	s.Run("a quarter line across the rows seals nothing", func() {
		// Shaves 5/24, alternating sides row to row. 19/24 still clears 0.7.
		c := s.load(theTurningWall(
			"  - start: { cell: [4,0], offset: [-0.25, -0.375] }\n" +
				"    end:   { cell: [3,5], offset: [0.25, 0.375] }\n"))
		s.Empty(c.Field.Sealed)
		s.Len(c.Field.Segments[0].Footprint, 5,
			"the three even rows of one column and the two odd rows of the next")
	})

	s.Run("a flat-side line seals exactly the cells it centres", func() {
		// The thick line across the rows: it runs ALONG the sides of the even
		// rows, leaving them whole, and through the MIDDLES of the odd ones,
		// halving them. Half is below 0.7, so those and only those are sealed.
		c := s.load(theTurningWall(
			"  - start: { cell: [4,0], offset: [-0.5, 0] }\n" +
				"    end:   { cell: [3,5], offset: [0, 0] }\n"))
		s.Equal([]spatial.Position{{X: 3, Y: 1}, {X: 3, Y: 3}}, c.Field.Sealed,
			"the odd rows the line runs through the middle of, and no other cell")

		// AND IT STANDS ON THE ONES IT LEAVES WHOLE. A thick line runs ALONG
		// the sides of the even rows, touching them without taking anything,
		// and those cells are its footing all the same — a wall a client
		// draws there needs floor under it on both sides (design C18).
		s.ElementsMatch([]spatial.Position{
			{X: 3, Y: 0}, {X: 4, Y: 0},
			{X: 3, Y: 1},
			{X: 3, Y: 2}, {X: 4, Y: 2},
			{X: 3, Y: 3},
			{X: 3, Y: 4}, {X: 4, Y: 4},
		}, c.Field.Segments[0].Footprint,
			"the two odd rows it halves, and the even rows either side whose "+
				"shared edge it runs along")
	})

	s.Run("a hexagonal corner is sealed", func() {
		// Two quarter lines meeting at 60° take 5/24 each with nothing but
		// the corner point in common, so the corner cell keeps 7/12 — below
		// 0.7, and the design's answer is that it goes scenery and the
		// designer shows it rather than that the wall is refused.
		c := s.load(theTurningWall(
			"  - start: { cell: [3,0], offset: [0.25, 0.375] }\n"+
				"    end:   { cell: [3,2], offset: [0.25, 0.375] }\n    name: first\n",
			"  - start: { cell: [3,2], offset: [0.25, 0.375] }\n"+
				"    end:   { cell: [4,3], offset: [0.25, 0.375] }\n    name: second\n"))
		s.Require().Len(c.Field.Segments, 2)
		s.Equal(c.Field.Segments[0].To, c.Field.Segments[1].From, "they do meet at the corner")
		s.Contains(c.Field.Sealed, spatial.Position{X: 3, Y: 2},
			"the corner cell keeps 7/12, which is not enough to stand in")
	})

	s.Run("a door past the end of its wall has no wall through it", func() {
		// F10 IS ABOUT THE SEGMENT, NOT THE LINE. This door stands exactly on
		// the line the wall lies along — same bearing, same offset from the
		// centres — and a hex beyond where the wall stops. There is no stone
		// at that point, so there is no door there either.
		errs := s.refuse(theTurningWall(
			"  - start: { cell: [4,0], offset: [-0.5, 0] }\n"+
				"    end:   { cell: [4,2], offset: [-0.5, 0] }\n    name: the short wall\n") + `doors:
  - id: past-the-end
    at: { cell: [4,4], offset: [-0.5, 0] }
    closed: true
`)
		s.Require().Len(errs, 1)
		s.Equal("doors[0].at", errs[0].Path)
		s.Contains(errs[0].Message, "no wall passes through this point")
	})

	s.Run("a monster on a sealed cell is refused, naming the wall", func() {
		// C12: the refusal names the WALL, because the wall is what an author
		// moves to fix it. A prop on the same cell is fine — a cut cell is
		// exactly where rubble belongs.
		scene := theTurningWall(
			"  - start: { cell: [4,0], offset: [-0.5, 0] }\n" +
				"    end:   { cell: [3,5], offset: [0, 0] }\n    name: the long divider\n")
		errs := s.refuse(scene + `place:
  - { ref: "dnd5e:monsters:skeleton", at: [3,1] }
`)
		s.Require().Len(errs, 1)
		s.Equal("place[0].at", errs[0].Path)
		s.Contains(errs[0].Message, "the long divider")
		s.Contains(errs[0].Message, "leaves no room to stand")

		s.Empty(s.refuse(scene+`place:
  - { ref: "dnd5e:props:rubble", at: [3,1], blocks_movement: false, blocks_los: false }
`), "a prop on a cut cell is what the cut is for")
	})

	s.Run("the party may not start on a sealed cell", func() {
		scene := strings.Replace(theTurningWall(
			"  - start: { cell: [1,0], offset: [-0.5, 0] }\n"+
				"    end:   { cell: [0,5], offset: [0, 0] }\n    name: the west divider\n"),
			"start: [0, 0]", "start: [0, 1]", 1)
		errs := s.refuse(scene)
		s.Require().Len(errs, 1)
		s.Equal("start", errs[0].Path)
		s.Contains(errs[0].Message, "the west divider")
		s.Contains(errs[0].Message, "leaves no room to stand")
	})
}
