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
	"fmt"
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
func footprintCells(r RoomInput, o Orientation) map[[2]int]bool {
	out := make(map[[2]int]bool, r.Width*r.Height)
	for col := 0; col < r.Width; col++ {
		for row := 0; row < r.Height; row++ {
			c := HexCellAt(o, col+int(r.Origin.X), row+int(r.Origin.Y))
			out[[2]int{int(c.X), int(c.Y)}] = true
		}
	}
	return out
}

// spreadOfChambers is a range of shapes chosen to hit the parities that matter:
// odd and even widths, odd and even heights, odd and even anchors, and the
// degenerate single column and single row. Offset conversion staggers on
// parity, so a fixture that is even everywhere proves the arithmetic for half
// the inputs.
func spreadOfChambers() []RoomInput {
	var out []RoomInput
	for _, w := range []int{1, 2, 3, 6, 7} {
		for _, h := range []int{1, 2, 5, 8} {
			for _, anchor := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {3, 5}, {-4, -3}} {
				out = append(out, RoomInput{
					ID:   fmt.Sprintf("w%dh%d@%d,%d", w, h, anchor[0], anchor[1]),
					Grid: spatial.GridShapeHex, Width: w, Height: h,
					Origin: spatial.Position{X: float64(anchor[0]), Y: float64(anchor[1])},
				})
			}
		}
	}
	return out
}

// TestOffsetAndAxialAgreeWithSpatial pins the reading [HexCellAt] takes of
// spatial's own cube coordinate: Q is cube.X and R is cube.Y.
//
// That is not the only reading available — a cube triple has three components
// and axial keeps two — and picking the wrong pair produces cells that look
// plausible and are somewhere else. The check is that spatial's TWO directions
// agree: the cube it derives from an offset pair must be the cube
// [spatial.AxialHexGrid] derives from the axial pair this hands it (its
// axialToCube sets Z = -Q-R). So the two are the same coordinate, by
// construction rather than by coincidence.
func TestOffsetAndAxialAgreeWithSpatial(t *testing.T) {
	for _, o := range bothOrientations() {
		t.Run(string(o.Kind()), func(t *testing.T) {
			for col := -8; col <= 8; col++ {
				for row := -8; row <= 8; row++ {
					fromOffset := spatial.OffsetCoordinateToCubeWithOrientation(
						spatial.Position{X: float64(col), Y: float64(row)}, o.spatial())

					axial := HexCellAt(o, col, row)
					q, r := int(axial.X), int(axial.Y)
					fromAxial := spatial.CubeCoordinate{X: q, Y: r, Z: -q - r}

					require.Equal(t, fromOffset, fromAxial,
						"offset [%d,%d]: the cube spatial derives from the pair an author "+
							"wrote must be the cube it derives from the axial cell we hand it",
						col, row)

					// And back again, which is what [regionAt] runs to decide
					// ownership: a cell that does not round-trip is a cell the
					// mask would put in the wrong chamber.
					backCol, backRow := hexOffsetOf(o, axial)
					require.Equal(t, [2]int{col, row}, [2]int{backCol, backRow},
						"offset [%d,%d] must survive the round trip", col, row)
				}
			}
		})
	}
}

// TestRunsAgreeWithEnumeration pins [hexRuns] and [hexFootprintBounds] against
// a full cell-by-cell sweep of the same chamber.
//
// Both exist so W2 and the canvas span can be computed WITHOUT enumerating —
// maxFieldCells allows four million cells, and building two footprints to
// intersect them is a real cost on a legal field — so the only honest
// expectation to compare against is the enumeration they replace.
//
// THIS FOUND A REAL DEFECT and is the reason it is written as two claims rather
// than one. The runs alone are insensitive to the sign of the key that groups
// them: flip it on both sides and every lookup still matches, so W2 cannot tell.
// The BOUNDS decode that key back into an authored row, and there the sign is
// load-bearing. A mutant flipping it survived the entire public suite.
func TestRunsAgreeWithEnumeration(t *testing.T) {
	for _, o := range bothOrientations() {
		for _, r := range spreadOfChambers() {
			t.Run(string(o.Kind())+"/"+r.ID, func(t *testing.T) {
				cells := footprintCells(r, o)

				// Claim 1: the runs cover exactly the footprint. Expanded back
				// into cells, they must BE the enumeration — not a superset
				// (W2 would refuse legal fields) and not a subset (it would
				// accept overlapping ones).
				expanded := map[[2]int]bool{}
				for key, run := range hexRuns(r, o) {
					require.LessOrEqual(t, run[0], run[1], "a run's interval must be ordered")
					for v := run[0]; v <= run[1]; v++ {
						// Mirrors hexRuns' contract, which swapped orientations
						// when rpg-toolkit#1141 corrected the offset schemes.
						if o.Kind() == OrientationFlatTop {
							expanded[[2]int{key, v}] = true // key is Q, run is R
						} else {
							expanded[[2]int{v, -v - key}] = true // key is the row, run is Q
						}
					}
				}
				require.Equal(t, cells, expanded,
					"the runs must expand to exactly the cells the author drew")

				// Claim 2: the bounding box is the enumeration's own.
				qMin, qMax, rMin, rMax := hexFootprintBounds(r, o)
				eqMin, eqMax, erMin, erMax := boundsOf(cells)
				require.Equal(t,
					[4]int{eqMin, eqMax, erMin, erMax}, [4]int{qMin, qMax, rMin, rMax},
					"the footprint's box must be the box its cells actually occupy")
			})
		}
	}
}

// boundsOf is the bounding box of an enumerated cell set — the slow answer
// [hexFootprintBounds] is measured against.
func boundsOf(cells map[[2]int]bool) (qMin, qMax, rMin, rMax int) {
	first := true
	for c := range cells {
		if first {
			qMin, qMax, rMin, rMax = c[0], c[0], c[1], c[1]
			first = false
			continue
		}
		qMin, qMax = min(qMin, c[0]), max(qMax, c[0])
		rMin, rMax = min(rMin, c[1]), max(rMax, c[1])
	}
	return
}

// TestOverlapAgreesWithEnumeration pins [hexFootprintsOverlap] — W2's verdict
// on hex — against intersecting the two enumerated cell sets.
//
// The pairs are built to include the case the reference tomb IS: two chambers
// whose bounding boxes intersect and which share no cell. That case is the
// whole reason this function exists (the box test refused the tomb outright),
// and it is also the one an implementation is most likely to get wrong in the
// safe-looking direction — reporting an overlap that is not there refuses a
// legal dungeon, and no fixture notices until somebody authors one.
func TestOverlapAgreesWithEnumeration(t *testing.T) {
	var chambers []RoomInput
	for _, w := range []int{1, 3, 6} {
		for _, h := range []int{1, 4, 8} {
			for _, anchor := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {3, 0}, {6, 0}, {0, 4}, {2, 3}, {-2, 1}} {
				chambers = append(chambers, RoomInput{
					ID:   fmt.Sprintf("w%dh%d@%d,%d", w, h, anchor[0], anchor[1]),
					Grid: spatial.GridShapeHex, Width: w, Height: h,
					Origin: spatial.Position{X: float64(anchor[0]), Y: float64(anchor[1])},
				})
			}
		}
	}

	for _, o := range bothOrientations() {
		t.Run(string(o.Kind()), func(t *testing.T) {
			boxesMetButDisjoint := 0
			for i := range chambers {
				for j := i + 1; j < len(chambers); j++ {
					a, b := chambers[i], chambers[j]
					cellsA, cellsB := footprintCells(a, o), footprintCells(b, o)
					shared := false
					for c := range cellsA {
						if cellsB[c] {
							shared = true
							break
						}
					}
					require.Equal(t, shared, hexFootprintsOverlap(a, b, o),
						"%s vs %s: the runs must agree with the cells", a.ID, b.ID)

					aqMin, aqMax, arMin, arMax := hexFootprintBounds(a, o)
					bqMin, bqMax, brMin, brMax := hexFootprintBounds(b, o)
					if !shared && aqMin <= bqMax && bqMin <= aqMax && arMin <= brMax && brMin <= arMax {
						boxesMetButDisjoint++
					}
				}
			}
			// The discriminating case has to be PRESENT, or this test passes
			// by never asking the question that matters — the box test would
			// satisfy it on its own.
			require.Positive(t, boxesMetButDisjoint,
				"the spread must contain pairs whose boxes meet and whose cells do not, "+
					"which is the case the box test gets wrong")
		})
	}
}
