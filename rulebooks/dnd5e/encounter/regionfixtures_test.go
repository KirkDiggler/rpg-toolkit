// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// regionfixtures_test.go is how this package's scenes paint a floor since
// regions replaced rooms (rpg-project#256). A scene that used to say "a 5x5
// room anchored at [6,0]" now says rectRegion("hall", 6, 0, 5, 5): the same
// cells, listed rather than described, with the archetype and lighting every
// region must carry supplied once here so no scene is secretly about them.

// testArchetype is the presentation ref every fixture region carries. Never
// read by the composition, which is the law the fixtures rely on.
const testArchetype = "crypt"

// fullLight is the lighting every fixture region carries unless a scene is
// about light, in which case it says so itself.
func fullLight() *encounter.Lighting { return &encounter.Lighting{Intensity: 1} }

// rectRegion paints a w x h rectangle of authored offset cells whose top-left
// corner is [col,row], in row-major order.
func rectRegion(id string, col, row, w, h int) encounter.RegionInput {
	return encounter.RegionInput{
		ID:        id,
		Name:      id,
		Cells:     rectCells(col, row, w, h),
		Archetype: testArchetype,
		Lighting:  fullLight(),
	}
}

// rectCells is the authored offset cells of a w x h rectangle at [col,row].
func rectCells(col, row, w, h int) []spatial.Position {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return cells
}

// pointyCanvas is the declaration every fixture field makes unless a scene is
// about the void or the layout: opaque void, pointy-top hexes.
func pointyCanvas() encounter.CanvasInput {
	return encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}
}

// cellAt is the absolute axial cell an authored [col,row] pair names under
// the fixture orientation — what a verb reports back for a seat authored
// there.
func cellAt(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// wall is one authored wall edge between two adjacent cells, blocking both
// movement and sight.
func wall(fromCol, fromRow, toCol, toRow int) spatial.Boundary {
	return spatial.Boundary{
		From:              spatial.Position{X: float64(fromCol), Y: float64(fromRow)},
		To:                spatial.Position{X: float64(toCol), Y: float64(toRow)},
		BlocksMovement:    true,
		BlocksLineOfSight: true,
	}
}

// seamWallRows is seamWall over rows [row0, row0+rows), leaving the straight
// crossing open at each of the (absolute) gap rows named. Only candidate pairs
// that are actually adjacent under pointy-top are emitted: a hex cell has six
// neighbours, not eight, and compileField refuses an edge between two cells
// that do not touch (ErrEdgeNotAdjacent).
func seamWallRows(west, row0, rows int, gaps ...int) []spatial.Boundary {
	open := make(map[int]bool, len(gaps))
	for _, g := range gaps {
		open[g] = true
	}
	var out []spatial.Boundary
	for row := row0; row < row0+rows; row++ {
		near := cellAt(west, row)
		for _, dr := range []int{-1, 0, 1} {
			to := row + dr
			if to < row0 || to >= row0+rows || (dr == 0 && open[row]) {
				continue
			}
			if adjacencyDistance(near, cellAt(west+1, to)) != 1 {
				continue
			}
			out = append(out, wall(west, row, west+1, to))
		}
	}
	return out
}

var fixtureGrid = spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})

func adjacencyDistance(a, b spatial.Position) float64 { return fixtureGrid.Distance(a, b) }

// openAir is the declaration a scene makes when it is NOT about the void:
// transparent, pointy-top. An authored rectangle shears when it becomes
// axial, so a sightline hugging its edge passes through cells no region owns
// — under an opaque void that blocks, which is the honest answer for a tomb
// cut from rock and the wrong subject for a scene about clocks or standing.
func openAir() encounter.CanvasInput {
	return encounter.CanvasInput{Void: encounter.VoidIsTransparent(), Orientation: encounter.HexesArePointyTop()}
}
