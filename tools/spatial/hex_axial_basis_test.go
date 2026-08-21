// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spatial_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The axial basis, pinned against an EXTERNAL reference (rpg-toolkit#1150).
//
// Any two cube axes make a valid axial pair, and every property that stays
// inside the lattice — distance, neighbours, sight, round-trips — is identical
// under all of them. So no test of those properties can tell which pair this
// package hands out. The one thing that can is a DRAWING: "pointy-top" means
// the r axis runs straight across the screen as a row, and the standard
// formula (x = √3·(q + r/2), y = 1.5·r) assumes r is cube z. Hand it cube y
// and call it r and an authored rectangle comes out as a diagonal band.
//
// spatial's offset conversion already puts the row in z, and its own hexPixel
// reads z; the axial read was the odd one out. These tests state the contract
// the wire relies on: an authored W×H rectangle, converted to the axial pair
// this package emits and drawn with the standard formula for its orientation,
// is W wide and H tall.

const basisW, basisH = 28, 8 // the reference tomb's chain: three chambers in a row

func pointyPixel(p spatial.Position) (float64, float64) {
	return math.Sqrt(3) * (p.X + p.Y/2), 1.5 * p.Y
}

func flatPixel(p spatial.Position) (float64, float64) {
	return 1.5 * p.X, math.Sqrt(3) * (p.Y + p.X/2)
}

func authoredRectAsAxial(o spatial.HexOrientation) []spatial.Position {
	out := make([]spatial.Position, 0, basisW*basisH)
	for row := 0; row < basisH; row++ {
		for col := 0; col < basisW; col++ {
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(col), Y: float64(row)}, o)
			out = append(out, cube.ToAxial())
		}
	}
	return out
}

func bounds(cells []spatial.Position, pixel func(spatial.Position) (float64, float64)) (w, h float64) {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, c := range cells {
		x, y := pixel(c)
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	return maxX - minX, maxY - minY
}

func TestAxialBasisDrawsTheAuthoredRectangle_PointyTop(t *testing.T) {
	cells := authoredRectAsAxial(spatial.HexOrientationPointyTop)
	w, h := bounds(cells, pointyPixel)

	// odd-r: columns are √3 apart, alternate rows stagger by half a column,
	// rows are 1.5 apart.
	require.InDelta(t, math.Sqrt(3)*(basisW-1+0.5), w, 1e-9, "width must be the authored column count")
	require.InDelta(t, 1.5*(basisH-1), h, 1e-9, "height must be the authored row count")
	require.Greater(t, w/h, 3.0, "a 28x8 chain is wide, not a diagonal band")
}

func TestAxialBasisDrawsTheAuthoredRectangle_FlatTop(t *testing.T) {
	cells := authoredRectAsAxial(spatial.HexOrientationFlatTop)
	w, h := bounds(cells, flatPixel)

	// odd-q: columns are 1.5 apart, rows √3 apart, alternate columns stagger
	// by half a row.
	require.InDelta(t, 1.5*(basisW-1), w, 1e-9, "width must be the authored column count")
	require.InDelta(t, math.Sqrt(3)*(basisH-1+0.5), h, 1e-9, "height must be the authored row count")
	require.Greater(t, w/h, 2.5, "a 28x8 chain is wide, not a diagonal band")
}

// TestAxialBasisIsCubeXZ names the basis outright, so a reader does not have
// to derive it from a drawing: axial (q, r) is cube (x, z), the convention
// every renderer and formula assumes.
func TestAxialBasisIsCubeXZ(t *testing.T) {
	for x := -5; x <= 5; x++ {
		for z := -5; z <= 5; z++ {
			cube := spatial.CubeCoordinate{X: x, Y: -x - z, Z: z}
			axial := cube.ToAxial()
			require.Equal(t, spatial.Position{X: float64(x), Y: float64(z)}, axial,
				"cube %v must read as axial (x, z)", cube)
			require.Equal(t, cube, spatial.AxialToCube(axial),
				"and the read must invert exactly")
		}
	}
}

// TestAxialHexGridSpeaksTheSameBasis pins that the grid's own neighbour
// arithmetic agrees with the exported conversion — one basis, not two.
func TestAxialHexGridSpeaksTheSameBasis(t *testing.T) {
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 100, SpanHeight: 100})
	origin := spatial.Position{X: 0, Y: 0}
	got := grid.GetNeighbors(origin)

	want := make([]spatial.Position, 0, 6)
	for _, d := range []spatial.CubeCoordinate{
		{X: 1, Y: -1, Z: 0}, {X: 1, Y: 0, Z: -1}, {X: 0, Y: 1, Z: -1},
		{X: -1, Y: 1, Z: 0}, {X: -1, Y: 0, Z: 1}, {X: 0, Y: -1, Z: 1},
	} {
		want = append(want, d.ToAxial())
	}
	require.ElementsMatch(t, want, got)
}
