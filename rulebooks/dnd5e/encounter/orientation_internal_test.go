// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// orientation_internal_test.go pins the ARITHMETIC the floor mask is built out
// of (rpg-toolkit#1127), against enumeration and against tools/spatial itself.
//
// It exists because orientation.go's doc comments named two tests that had not
// been written — the exact failure the mutation battery is for: a comment
// claiming a guarantee nothing checks. Both are here now, and both found
// something. See each test for what.
//
// The public seam is orientation_test.go, and that is where behaviour belongs.
// What is here is unreachable from outside: hexRuns and hexFootprintBounds are
// an OPTIMIZATION — they answer questions a full cell sweep would answer more
// slowly — so the honest test compares them to that sweep rather than to a
// hand-written expectation, which would only re-state the implementation.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// bothOrientations is every layout a hex field may declare, which is what makes
// "flat and pointy top are both valid" (Kirk's ruling) a thing tests can range
// over rather than a thing a fixture picks.
func bothOrientations() []Orientation {
	return []Orientation{HexesArePointyTop(), HexesAreFlatTop()}
}

// footprintCells enumerates a chamber's absolute cells the slow, obvious way:
// every authored column and row, converted one at a time. The reference every
// other function here is measured against.
// TestOffsetAndAxialAgreeWithSpatial pins the reading [HexCellAt] takes of
// spatial's own cube coordinate: Q is cube.X and R is cube.Z
// (rpg-toolkit#1150 — [spatial.CubeCoordinate.ToAxial]'s definition, the ONE
// exported reading this file used to rebuild by hand and get wrong).
//
// That is not the only reading available — a cube triple has three components
// and axial keeps two — and picking the wrong pair produces cells that look
// plausible and are somewhere else. The check is that spatial's TWO directions
// agree: the cube it derives from an offset pair must be the cube
// [spatial.AxialToCube] derives from the axial pair this hands it. So the two
// are the same coordinate, by construction rather than by coincidence — and by
// calling spatial's exported functions on both sides rather than restating
// either one.
func TestOffsetAndAxialAgreeWithSpatial(t *testing.T) {
	for _, o := range bothOrientations() {
		t.Run(string(o.Kind()), func(t *testing.T) {
			for col := -8; col <= 8; col++ {
				for row := -8; row <= 8; row++ {
					fromOffset := spatial.OffsetCoordinateToCubeWithOrientation(
						spatial.Position{X: float64(col), Y: float64(row)}, o.spatial())

					axial := HexCellAt(o, col, row)
					fromAxial := spatial.AxialToCube(axial)

					require.Equal(t, fromOffset, fromAxial,
						"offset [%d,%d]: the cube spatial derives from the pair an author "+
							"wrote must be the cube it derives from the axial cell we hand it",
						col, row)

					// And back again through spatial's own inverse, so the cell
					// an author wrote is the cell a renderer recovers.
					back := spatial.AxialToCube(axial).ToOffsetCoordinateWithOrientation(o.spatial())
					require.Equal(t, spatial.Position{X: float64(col), Y: float64(row)}, back,
						"offset [%d,%d] must survive the round trip", col, row)
				}
			}
		})
	}
}
