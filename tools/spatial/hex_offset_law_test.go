// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spatial_test

// hex_offset_law_test.go pins WHICH OFFSET SCHEME each hex orientation uses,
// which is the one thing about offset coordinates that cannot be checked by
// round-tripping.
//
// Every existing test converts offset -> cube -> offset through the same pair of
// functions, so a scheme mismatched to its orientation name round-trips
// perfectly, produces self-consistent distances, neighbours and lines, and is
// invisible to all of them. It becomes visible exactly once: when somebody draws
// it (rpg-toolkit#1140, found by rendering a dungeon in a browser).
//
// The convention these assert is the standard one:
//
//   - "odd-q" / "even-q" offset  <->  FLAT-top hexes. Columns are shifted, so
//     the offset applies per COLUMN.
//   - "odd-r" / "even-r" offset  <->  POINTY-top hexes. Rows are shifted, so the
//     offset applies per ROW.
//
// The discriminator below is a neighbour that exists under one scheme and not
// the other, so it cannot be satisfied by a rotation or a relabelling.

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cubeDistance is the hex distance between two cube coordinates.
func cubeDistance(a, b spatial.CubeCoordinate) int {
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}
	dx, dy, dz := abs(a.X-b.X), abs(a.Y-b.Y), abs(a.Z-b.Z)
	if dy > dx {
		dx = dy
	}
	if dz > dx {
		dx = dz
	}
	return dx
}

// stepsFromOrigin is how many hex steps the offset cell [col,row] is from the
// offset origin, under the given orientation.
//
// It answers a DISTANCE, and the assertions below all turn on whether that
// distance is 1 -- "is this cell a neighbour of the origin" -- so the value one
// step out and the value two steps out are both meaningful and both appear.
func stepsFromOrigin(t *testing.T, o spatial.HexOrientation, col, row int) int {
	t.Helper()
	origin := spatial.OffsetCoordinateToCubeWithOrientation(spatial.Position{}, o)
	other := spatial.OffsetCoordinateToCubeWithOrientation(
		spatial.Position{X: float64(col), Y: float64(row)}, o)

	return cubeDistance(origin, other)
}

// TestPointyTopUsesRowOffsets pins pointy-top to an odd-r scheme.
//
// Under odd-r the odd ROWS are shifted, which makes [-1,1] a neighbour of the
// origin. Under odd-q -- the COLUMN-shifted scheme, which belongs to flat-top --
// that same pair is two steps apart. So this single assertion says which scheme
// is in use and cannot be satisfied by the other.
func TestPointyTopUsesRowOffsets(t *testing.T) {
	if got := stepsFromOrigin(t, spatial.HexOrientationPointyTop, -1, 1); got != 1 {
		t.Fatalf("pointy-top should use odd-r (row-shifted) offsets, so [-1,1] should "+
			"neighbour the origin; got distance %d. A distance of 2 means pointy-top is "+
			"running the odd-q scheme, which belongs to FLAT-top.", got)
	}
}

// TestFlatTopUsesColumnOffsets is the mirror: under odd-q the odd COLUMNS are
// shifted, which makes [1,-1] a neighbour of the origin, and under odd-r it is
// two steps away.
func TestFlatTopUsesColumnOffsets(t *testing.T) {
	if got := stepsFromOrigin(t, spatial.HexOrientationFlatTop, 1, -1); got != 1 {
		t.Fatalf("flat-top should use odd-q (column-shifted) offsets, so [1,-1] should "+
			"neighbour the origin; got distance %d. A distance of 2 means flat-top is "+
			"running the odd-r scheme, which belongs to POINTY-top.", got)
	}
}

// TestTheTwoOrientationsAreNotTheSameScheme guards the fix from being applied to
// both branches at once, which would restore the symmetry while leaving them
// swapped -- the bug wearing different labels.
func TestTheTwoOrientationsAreNotTheSameScheme(t *testing.T) {
	pointy := stepsFromOrigin(t, spatial.HexOrientationPointyTop, -1, 1)
	flat := stepsFromOrigin(t, spatial.HexOrientationFlatTop, -1, 1)
	if pointy == flat {
		t.Fatalf("the two orientations answered identically (%d) for [-1,1]; they use "+
			"different offset schemes and must disagree somewhere", pointy)
	}
}
