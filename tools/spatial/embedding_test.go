package spatial

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type EmbeddingTestSuite struct {
	suite.Suite
}

func TestEmbeddingSuite(t *testing.T) {
	suite.Run(t, new(EmbeddingTestSuite))
}

// offAxisMultiple is how far deg sits from the nearest multiple of step,
// in degrees, never more than half a step.
func offAxisMultiple(deg, step float64) float64 {
	m := math.Mod(math.Mod(deg, step)+step, step)
	return math.Min(m, step-m)
}

func (s *EmbeddingTestSuite) TestPointyCentreAndCornersAtOrigin() {
	emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 5})

	c := emb.CellCentre(Position{X: 0, Y: 0})
	s.InDelta(0, c.X, 1e-9)
	s.InDelta(0, c.Y, 1e-9)

	corners := emb.CellCorners(Position{X: 0, Y: 0})
	s.Len(corners, 6)
	for _, k := range corners {
		s.InDelta(5/math.Sqrt(3), math.Hypot(k.X, k.Y), 1e-9, "circumradius for 5 across the flats")
	}
}

func (s *EmbeddingTestSuite) TestCellCentresAreOneCellWidthApartAcrossTheFlats() {
	emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 5})
	g := NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 9, SpanHeight: 9})

	origin := emb.CellCentre(Position{})
	for _, n := range g.GetNeighbors(Position{}) {
		p := emb.CellCentre(n)
		s.InDelta(5, math.Hypot(p.X-origin.X, p.Y-origin.Y), 1e-9,
			"a neighbour's centre is one width away: %v", n)
	}
}

func (s *EmbeddingTestSuite) TestFlatTopIsTheSameHexTurnedThirtyDegrees() {
	emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: HexOrientationFlatTop, CellWidth: 5})

	corners := emb.CellCorners(Position{X: 0, Y: 0})
	for _, k := range corners {
		s.InDelta(5/math.Sqrt(3), math.Hypot(k.X, k.Y), 1e-9, "same circumradius, turned")
	}
	// A flat-top hex has a corner due east; a pointy-top one has a flat there.
	s.InDelta(0, offAxisMultiple(math.Atan2(corners[0].Y, corners[0].X)*180/math.Pi, 60), 1e-9)
}

func (s *EmbeddingTestSuite) TestBearingBetweenNeighboursIsAMultipleOfSixty() {
	emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 5})
	g := NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 9, SpanHeight: 9})

	for _, n := range g.GetNeighbors(Position{}) {
		deg, ok := emb.Bearing(Position{}, n)
		s.True(ok)
		s.GreaterOrEqual(deg, 0.0)
		s.Less(deg, 360.0)
		s.InDelta(0, offAxisMultiple(deg, 60), 1e-9, "neighbours sit at k*60 degrees: %v is %v", n, deg)
	}

	_, ok := emb.Bearing(Position{}, Position{})
	s.False(ok, "no bearing to yourself")
}

func (s *EmbeddingTestSuite) TestANonPositiveCellWidthIsRefusedAndItsEmbeddingIsInert() {
	s.Require().Error(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 0}.Validate())
	s.Require().Error(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: -5}.Validate())
	s.Require().NoError(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 5}.Validate())

	emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 0})
	s.Equal(Point{}, emb.CellCentre(Position{X: 3, Y: -2}), "no frame, no point")
	for _, k := range emb.CellCorners(Position{X: 3, Y: -2}) {
		s.Equal(Point{}, k)
	}
	_, ok := emb.Bearing(Position{}, Position{X: 1, Y: 0})
	s.False(ok, "no frame, no bearing")
}
