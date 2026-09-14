package spatial

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PlacedCoverageSuite struct{ suite.Suite }

func TestPlacedCoverageSuite(t *testing.T) {
	suite.Run(t, new(PlacedCoverageSuite))
}

func (s *PlacedCoverageSuite) TestFractionalPlacementAndArea() {
	for _, orientation := range []HexOrientation{HexOrientationPointyTop, HexOrientationFlatTop} {
		emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: orientation, CellWidth: 5})
		g := NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 15, SpanHeight: 15})
		in := PlacedCoverageInput{
			Embedding: emb,
			Placement: FootprintPlacement{
				Footprint: Footprint{Box: &Box{W: 2, D: 4}},
				Origin:    Point{X: 3.25, Y: -1.75}, Facing: 37,
				LocalOffset: Point{X: 1, Y: 0.5},
			},
			Cells: g.GetPositionsInRange(Position{}, 7),
		}
		first, err := PlacedCoverage(in)
		s.Require().NoError(err)
		s.InDelta(8, sumFractions(first.Cells)*25*math.Sqrt(3)/2, 1e-8)
		in.Placement.Origin.X += 0.35
		second, err := PlacedCoverage(in)
		s.Require().NoError(err)
		s.NotEqual(first.Cells, second.Cells, "fractional movement must not snap")
		s.InDelta(8, sumFractions(second.Cells)*25*math.Sqrt(3)/2, 1e-8)
	}
}

func (s *PlacedCoverageSuite) TestUniverseAndInvalidEmptyInput() {
	emb := NewHexEmbedding(HexEmbeddingConfig{CellWidth: 5})
	in := PlacedCoverageInput{
		Embedding: emb,
		Placement: FootprintPlacement{Footprint: Footprint{Box: &Box{W: 1, D: 1}}},
	}
	out, err := PlacedCoverage(in)
	s.Require().NoError(err)
	s.NotNil(out.Cells)
	s.Empty(out.Cells)
	in.Cells = []Position{{}, {}}
	out, err = PlacedCoverage(in)
	s.Require().NoError(err)
	s.Len(out.Cells, 1)
	s.InDelta(1, out.Cells[Position{}]*25*math.Sqrt(3)/2, 1e-9)
	in.Cells = []Position{{X: 0.5}}
	out, err = PlacedCoverage(in)
	s.ErrorIs(err, ErrBadCoverageCell)
	s.Nil(out.Cells)
	in.Cells = nil
	in.Placement.Facing = math.NaN()
	out, err = PlacedCoverage(in)
	s.ErrorIs(err, ErrBadFootprintPlacement)
	s.Nil(out.Cells)
}

func (s *PlacedCoverageSuite) TestSubsetAndNoPartialResult() {
	emb := NewHexEmbedding(HexEmbeddingConfig{CellWidth: 5})
	cells := []Position{{}, {X: 1}, {X: math.NaN()}}
	in := PlacedCoverageInput{
		Embedding: emb,
		Placement: FootprintPlacement{Footprint: Footprint{Box: &Box{W: 12, D: 12}}},
		Cells:     cells[:2],
	}
	out, err := PlacedCoverage(in)
	s.Require().NoError(err)
	s.Len(out.Cells, 2)
	s.Equal(Position{X: 1}, cells[1])
	in.Cells = cells
	out, err = PlacedCoverage(in)
	s.ErrorIs(err, ErrBadCoverageCell)
	s.Nil(out.Cells)
	in.Cells = []Position{{X: math.MaxFloat64}}
	out, err = PlacedCoverage(in)
	s.ErrorIs(err, ErrBadCoverageCell)
	s.Nil(out.Cells)
}
