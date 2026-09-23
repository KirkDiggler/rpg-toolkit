package spatial

import (
	"errors"
	"math"
)

// PlacedCoverageInput declares the exact axial cells to inspect in an embedding.
type PlacedCoverageInput struct {
	Embedding HexEmbedding
	Placement FootprintPlacement
	Cells     []Position
}

// ErrBadCoverageCell reports a non-finite, fractional, or unrepresentable cell.
var ErrBadCoverageCell = errors.New("spatial: invalid coverage cell")

// PlacedCoverage returns area fractions for a freely placed footprint over Cells.
// Misses are absent. Inputs are not retained; errors return no partial map.
func PlacedCoverage(in PlacedCoverageInput) (CoverageOutput, error) {
	f, err := footprintPolygon(in.Placement)
	if err != nil {
		return CoverageOutput{}, err
	}
	if err := (HexEmbeddingConfig{CellWidth: in.Embedding.cellWidth}).Validate(); err != nil {
		return CoverageOutput{}, err
	}

	out := CoverageOutput{Cells: make(map[Position]float64)}
	for _, cell := range in.Cells {
		for _, v := range []float64{cell.X, cell.Y} {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
				return CoverageOutput{}, ErrBadCoverageCell
			}
		}
		hex := in.Embedding.CellCorners(cell)
		for _, p := range hex {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
				return CoverageOutput{}, ErrBadCoverageCell
			}
		}
		whole := polygonArea(hex[:])
		if whole <= 0 || math.IsNaN(whole) || math.IsInf(whole, 0) {
			return CoverageOutput{}, ErrBadCoverageCell
		}
		clipped := clipConvex(hex[:], f)
		area := polygonArea(clipped)
		fraction := area / whole
		if math.IsNaN(fraction) || math.IsInf(fraction, 0) {
			return CoverageOutput{}, ErrBadFootprintPlacement
		}
		if fraction > coverageEpsilon {
			out.Cells[cell] = math.Min(fraction, 1)
		}
	}

	return out, nil
}
