// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// occludingProps is what these scenes used to write as `Occluders`: cells that
// stop a sightline.
//
// The two answers are now independent and both are stated (rpg-toolkit#1128),
// so this says the thing the old field could only imply — these block SIGHT and
// not MOVEMENT. That was always the meaning of the word: a pillar you can see
// past is not an occluder, and these scenes are about who can see whom, never
// about who can walk where. A fixture that blocked movement too would be
// changing what the test is asking while appearing only to rename a field.
func occludingProps(at ...spatial.Position) []encounter.PropInput {
	blocks, passes := true, false
	out := make([]encounter.PropInput, 0, len(at))
	for _, cell := range at {
		out = append(out, encounter.PropInput{
			Ref:               "pillar",
			At:                cell,
			BlocksMovement:    &passes,
			BlocksLineOfSight: &blocks,
		})
	}

	return out
}

// encEveryoneSees is the sight capability these scenes run on: unbounded range,
// so the only thing that hides a member is something drawn on the map — the
// same position session itself takes in v1 (see sightRangeCells).
type encEveryoneSees struct{}

func (encEveryoneSees) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 1_000_000
	}

	return out, nil
}

// hexCell is the dungeon-absolute cell at an AUTHORED column and row of the hex
// fixtures — the frame a person counts in, not the one the map stores.
//
// The two stopped being the same thing when a chamber became the rectangle
// somebody drew (rpg-toolkit#1127). An authored rectangle SHEARS in axial
// space, so the corridor's top row is not (0,0) (1,0) (2,0) but
// (0,0) (1,-1) (2,-1) (3,-2) — walking it by incrementing X lands in the void
// after one step. These scenes say which column and row they mean and let the
// conversion happen once, here, so a fixture cannot drift from the map by
// having its arithmetic done in the wrong frame.
//
// Columns are canvas-wide: the corridor owns 0..5 and the vault 6..11, which is
// why crossing the gate is hexCell(5, 0) to hexCell(6, 0).
func hexCell(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// hexSeamWalls is the wall between two side-by-side hex chambers, with one row
// left open for the doorway.
//
// # Rooms used to imply this and no longer do
//
// When each chamber had its own grid, sight and movement could not cross
// between them except through a declared doorway — the separation was a
// property of the decomposition. On one canvas (rpg-toolkit#1127) two chambers
// sitting side by side are simply one open space, so a fixture that does not
// draw its walls has none, and a walker steps between chambers anywhere along
// the seam. The tomb compiler draws these for authored dungeons; a hand-built
// fixture has to say them itself.
//
// Returned in the WEST chamber's local frame, where column Width-1 is its last
// and column Width is the east chamber's first. Only real crossings are
// emitted: on hex, which pairs are adjacent staggers with the column's parity,
// so the candidates are filtered by actual cube distance rather than assumed.
func hexSeamWalls(westWidth, rows, openRow int) []encounter.WallInput {
	return hexSeamWallsFrom(westWidth, 0, rows, openRow)
}

// hexSeamWallsFrom is hexSeamWalls for a seam whose rows start at row0 rather
// than 0: the wall between authored column east-1 and column east, over rows
// [row0, row0+rows), leaving the straight crossing on openRow open (-1 for a
// solid wall).
func hexSeamWallsFrom(east, row0, rows, openRow int) []encounter.WallInput {
	westWidth := east
	out := make([]encounter.WallInput, 0, rows*2)
	for row := row0; row < row0+rows; row++ {
		for _, dr := range []int{-1, 0, 1} {
			to := row + dr
			if to < row0 || to >= row0+rows {
				continue
			}
			if dr == 0 && row == openRow {
				continue // the doorway itself
			}
			if hexSteps(hexCell(westWidth-1, row), hexCell(westWidth, to)) != 1 {
				continue // not a crossing on this grid
			}
			out = append(out, encounter.WallInput{Boundary: spatial.Boundary{
				From:              spatial.Position{X: float64(westWidth - 1), Y: float64(row)},
				To:                spatial.Position{X: float64(westWidth), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			}})
		}
	}

	return out
}

// hexSteps is cube distance between two axial cells — how many steps apart two
// hexes are, which is the only adjacency test that is correct in both layouts.
func hexSteps(a, b spatial.Position) int {
	aq, ar := int(a.X), int(a.Y)
	bq, br := int(b.X), int(b.Y)
	dq, dr, ds := aq-bq, ar-br, (-aq-ar)-(-bq-br)
	if dq < 0 {
		dq = -dq
	}
	if dr < 0 {
		dr = -dr
	}
	if ds < 0 {
		ds = -ds
	}

	return max(dq, max(dr, ds))
}
