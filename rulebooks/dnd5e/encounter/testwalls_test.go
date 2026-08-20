// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"reflect"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// A LONE PILLAR NO LONGER BLOCKS ANYTHING, and these helpers exist because of
// it.
//
// Sight used to be one rasterized line, so a single occluding cell sitting on
// it stopped the sightline dead. That was the defect behind rpg-toolkit#1022:
// measured against the rule 5e actually states — visible if a line from any
// corner of your space to any corner of theirs is unobstructed — the old rule
// blocked 4.5x too many pairs on hexes and hid about one in seven things a
// player should have seen. spatial v0.9.1 asks for a LANE instead: blocked
// only when the direct lane AND every lane from a neighbour closer to the
// target are obstructed. You lean around a pillar, exactly as you would at the
// table.
//
// So a fixture that wants sight BLOCKED needs a wall, not a pillar. That is
// not a workaround — the old fixtures were describing a rule that was wrong,
// and reading one of them now ("the pillar blocks sight") tells you something
// the engine no longer believes.
//
// A pillar still STOPS you, mind — a prop declares movement and sight
// separately (rpg-toolkit#1128), and leaning around one is a fact about the
// sightline, not about walking through it.
//
// The width is not decoration either. A wall must be wide enough that the
// neighbour lanes are obstructed too, which means covering the cells a viewer
// would lean toward, not just the one on the centre line.

// rubble is one cell of a fixture wall: solid to walk into and solid to look
// through, which is what every caller of wallRow and wallColumn has always
// meant by "wall".
//
// Both answers are stated because a prop has to state them (rpg-toolkit#1128).
// Before that, a fixture like this got the opposite of what it asked for on the
// movement axis — the module hardcoded every placed thing as walk-through — and
// no test noticed, because none of them tried to walk into one.
func rubble(x, y int) encounter.PropInput {
	solid := true
	return encounter.PropInput{
		Ref:               "test:props:rubble",
		At:                spatial.Position{X: float64(x), Y: float64(y)},
		BlocksMovement:    &solid,
		BlocksLineOfSight: &solid,
	}
}

// rubbleData is rubble as it persists — the PropData half, for fixtures that
// build a blob directly rather than saving one.
func rubbleData(x, y float64) encounter.PropData {
	solid := true
	return encounter.PropData{
		Ref:               "test:props:rubble",
		At:                encounter.PositionData{X: x, Y: y},
		BlocksMovement:    &solid,
		BlocksLineOfSight: &solid,
	}
}

// absoluteRubble is rubble as the map reports it: the same thing, at a
// dungeon-absolute cell, with the flags it was authored with.
func absoluteRubble(x, y float64) encounter.AtlasProp {
	return encounter.AtlasProp{
		Ref:               "test:props:rubble",
		At:                spatial.Position{X: x, Y: y},
		BlocksMovement:    true,
		BlocksLineOfSight: true,
	}
}

// wallRow returns a horizontal run of wall cells at the given row, inclusive of
// both ends.
func wallRow(y, fromX, toX int) []encounter.PropInput {
	out := make([]encounter.PropInput, 0, toX-fromX+1)
	for x := fromX; x <= toX; x++ {
		out = append(out, rubble(x, y))
	}
	return out
}

// wallColumn returns a vertical run of wall cells in the given column,
// inclusive of both ends.
func wallColumn(x, fromY, toY int) []encounter.PropInput {
	out := make([]encounter.PropInput, 0, toY-fromY+1)
	for y := fromY; y <= toY; y++ {
		out = append(out, rubble(x, y))
	}
	return out
}

// structFieldNames reports EVERY field name a struct declares, in declaration
// order — unexported ones included — so a test can assert on a type's SHAPE
// rather than on the values one instance happens to carry.
//
// Including the unexported ones is the point rather than an oversight. This
// exists to hold RecordInput's claim that prose is inexpressible, and a claim
// about what a type can EXPRESS is not weakened by a field being lowercase:
// an unexported string on an input that callers construct is a smell of its
// own, and a shape pin that looked away from it would be choosing not to see.
func structFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	return names
}

// A WALL BUILT OUT OF EDGES NEEDS EVERY EDGE, and that is what these two
// helpers exist to get right.
//
// A room walls its own boundary by naming the cell beyond it: {From: (x,y),
// To: (x+1,y)} is a crossing between two chambers, which is a sentence no room
// could say until the field became one canvas (rpg-toolkit#1106). But a grid's
// adjacency is not only orthogonal — a square cell has eight neighbours, a hex
// six — and spatial registers a boundary between ANY two adjacent cells, so a
// wall made of same-row crossings alone has a hole at every corner: a sightline
// or a step that crosses the seam DIAGONALLY passes straight through it.
//
// Measured, on a 20x10 canvas walled at 9|10 with same-row edges only: a viewer
// at (2,2) sees a member at (17,7), because the canonical ray crosses the seam
// on a diagonal step and meets no registered edge. Adding the diagonal
// crossings blocks it. That is not a defect in spatial — a diagonal pair IS
// adjacent, and an edge between them is a real thing to register — it is what
// "a wall" has to mean when it is built out of edges rather than out of cells.

// squareSeamWall returns every crossing from column atX to the column beyond
// it, over rows [0,height), except through the named gap rows.
//
// A gap row is one cell wide in both senses: only the straight crossing
// (atX,g)-(atX+1,g) is left open, so a doorway is a doorway rather than a
// diagonal shortcut around its own frame.
func squareSeamWall(atX, height int, gapRows ...int) []spatial.Boundary {
	gap := make(map[int]bool, len(gapRows))
	for _, g := range gapRows {
		gap[g] = true
	}

	out := make([]spatial.Boundary, 0, height*3)
	for y := 0; y < height; y++ {
		for _, dy := range []int{-1, 0, 1} {
			to := y + dy
			if to < 0 || to >= height {
				continue
			}
			if dy == 0 && gap[y] {
				continue // the doorway itself
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(atX), Y: float64(y)},
				To:                spatial.Position{X: float64(atX + 1), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}
	return out
}

// hexSeamWall is squareSeamWall's axial-hex sibling. A hex cell's neighbours
// across the +Q edge are (q+1,r) and (q+1,r-1) — two crossings per cell, not
// three — and rows are given as an inclusive [rMin,rMax] range because an axial
// room's span is origin-centred and its R values are routinely negative.
func hexSeamWall(atQ, rMin, rMax int, gapRows ...int) []spatial.Boundary {
	gap := make(map[int]bool, len(gapRows))
	for _, g := range gapRows {
		gap[g] = true
	}

	out := make([]spatial.Boundary, 0, (rMax-rMin+1)*2)
	for r := rMin; r <= rMax; r++ {
		for _, dr := range []int{0, -1} {
			if dr == 0 && gap[r] {
				continue // the doorway itself
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(atQ), Y: float64(r)},
				To:                spatial.Position{X: float64(atQ + 1), Y: float64(r + dr)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}
	return out
}

// hexOffsetSeamWall is hexSeamWall re-derived for the frame a hex chamber is
// authored in since rpg-toolkit#1127: OFFSET columns and rows, counted from the
// chamber's own corner.
//
// It computes which crossings exist rather than knowing them, and that is the
// point. In axial space a cell's +Q neighbours are (q+1,r) and (q+1,r-1),
// always — a fact hexSeamWall could hardcode. In offset space the answer
// STAGGERS with the column's parity, so a wall built from a hardcoded pair has
// a hole in every other row. Asking spatial which offset pairs are actually
// adjacent is both shorter and correct for either orientation.
//
// A gap row leaves only the straight crossing open, exactly as squareSeamWall's
// does, so a doorway is a doorway rather than a diagonal shortcut around its
// own frame.
func hexOffsetSeamWall(o encounter.Orientation, atCol, rowMin, rowMax int, gapRows ...int) []spatial.Boundary {
	gap := make(map[int]bool, len(gapRows))
	for _, g := range gapRows {
		gap[g] = true
	}

	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})

	out := make([]spatial.Boundary, 0, (rowMax-rowMin+1)*2)
	for row := rowMin; row <= rowMax; row++ {
		near := encounter.HexCellAt(o, atCol, row)
		for _, dr := range []int{-1, 0, 1} {
			to := row + dr
			if to < rowMin || to > rowMax {
				continue
			}
			if dr == 0 && gap[row] {
				continue // the doorway itself
			}
			if grid.Distance(near, encounter.HexCellAt(o, atCol+1, to)) != 1 {
				continue // not a crossing at all on this grid
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(atCol), Y: float64(row)},
				To:                spatial.Position{X: float64(atCol + 1), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}

	return out
}
