// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hex_axial_basis_test.go is this module's own copy of spatial's
// hex_axial_basis_test.go, aimed at [encounter.HexCellAt] rather than at
// spatial's exported conversion directly (rpg-toolkit#1150).
//
// # Why this module needs its own copy of that test
//
// spatial pinning its OWN basis does not pin this module's reading of it.
// rpg-toolkit#1150 happened exactly because [encounter.HexCellAt] used to
// rebuild the cube-to-axial conversion by hand instead of calling
// [spatial.CubeCoordinate.ToAxial] — a second definition of the same basis,
// silently disagreeing with the first. Every test inside this module that
// compared HexCellAt's output against ITSELF (round-trips, run-agrees-with-
// enumeration, region-ownership) stayed green throughout, because a value
// compared only against its own inverse cannot detect which basis produced
// it — see TestOffsetAndAxialAgreeWithSpatial's doc comment for the same
// argument made about spatial's internal consistency. The one thing that
// can tell the two bases apart is a DRAWING, and only an EXTERNAL reference
// — the standard pixel formula, not another conversion — counts as one.
//
// # What this measures
//
// An authored W x H room, projected cell-by-cell through HexCellAt, drawn
// with the standard formula for its orientation (pointy: x = sqrt(3)*(q +
// r/2), y = 1.5*r; flat: x = 1.5*q, y = sqrt(3)*(r + q/2)), must measure W
// columns wide and H rows tall. Under the OLD, buggy basis (axial R read as
// cube Y instead of cube Z) an authored rectangle draws as a diagonal band
// instead — narrow and tall regardless of W and H — which is exactly the
// failure mode #1150 and #1141 before it both produced and both survived
// every round-trip test in this module. See spatial's
// hex_axial_basis_test.go for the fuller argument; this file states the
// same contract one layer up, at the function content actually calls.

const hexBasisW, hexBasisH = 28, 8 // matches spatial's own fixture: the reference tomb's chain

func pointyHexPixel(p spatial.Position) (float64, float64) {
	return math.Sqrt(3) * (p.X + p.Y/2), 1.5 * p.Y
}

func flatHexPixel(p spatial.Position) (float64, float64) {
	return 1.5 * p.X, math.Sqrt(3) * (p.Y + p.X/2)
}

func authoredRoomAsAxial(o encounter.Orientation) []spatial.Position {
	out := make([]spatial.Position, 0, hexBasisW*hexBasisH)
	for row := 0; row < hexBasisH; row++ {
		for col := 0; col < hexBasisW; col++ {
			out = append(out, encounter.HexCellAt(o, col, row))
		}
	}
	return out
}

func hexBounds(cells []spatial.Position, pixel func(spatial.Position) (float64, float64)) (w, h float64) {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, c := range cells {
		x, y := pixel(c)
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	return maxX - minX, maxY - minY
}

// TestHexCellAtDrawsTheAuthoredRoom_PointyTop is the discriminator: a room
// this module compiled must measure as the rectangle its author drew, not as
// the rhombus an internally-consistent-but-wrong basis would also draw
// correctly by its own lights.
func TestHexCellAtDrawsTheAuthoredRoom_PointyTop(t *testing.T) {
	cells := authoredRoomAsAxial(encounter.HexesArePointyTop())
	w, h := hexBounds(cells, pointyHexPixel)

	// odd-r: columns are sqrt(3) apart, alternate rows stagger by half a
	// column, rows are 1.5 apart.
	require.InDelta(t, math.Sqrt(3)*(hexBasisW-1+0.5), w, 1e-9, "width must be the authored column count")
	require.InDelta(t, 1.5*(hexBasisH-1), h, 1e-9, "height must be the authored row count")
	require.Greater(t, w/h, 3.0, "a 28x8 room is wide, not a diagonal band")
}

// TestHexCellAtDrawsTheAuthoredRoom_FlatTop is the flat-top counterpart —
// Kirk's ruling that both orientations are valid means both need the same
// discriminator.
func TestHexCellAtDrawsTheAuthoredRoom_FlatTop(t *testing.T) {
	cells := authoredRoomAsAxial(encounter.HexesAreFlatTop())
	w, h := hexBounds(cells, flatHexPixel)

	// odd-q: columns are 1.5 apart, rows sqrt(3) apart, alternate columns
	// stagger by half a row.
	require.InDelta(t, 1.5*(hexBasisW-1), w, 1e-9, "width must be the authored column count")
	require.InDelta(t, math.Sqrt(3)*(hexBasisH-1+0.5), h, 1e-9, "height must be the authored row count")
	require.Greater(t, w/h, 2.5, "a 28x8 room is wide, not a diagonal band")
}
