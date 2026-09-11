package spatial

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CoverageTestSuite struct {
	suite.Suite

	emb  HexEmbedding
	grid *AxialHexGrid
}

func TestCoverageSuite(t *testing.T) {
	suite.Run(t, new(CoverageTestSuite))
}

func (s *CoverageTestSuite) SetupTest() {
	s.emb = NewHexEmbedding(HexEmbeddingConfig{Orientation: HexOrientationPointyTop, CellWidth: 5})
	s.grid = NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 15, SpanHeight: 15})
}

// stepAlong is the cell n steps from origin in the direction of toward.
func stepAlong(origin, toward Position, n float64) Position {
	d := toward.Subtract(origin)
	return origin.Add(d.Scale(n))
}

// keys is the covered cells in a stable order, so two sets compare.
func keys(cells map[Position]float64) []Position {
	out := make([]Position, 0, len(cells))
	for c := range cells {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		return out[i].Y < out[j].Y
	})

	return out
}

// sumFractions is the footprint's area in cells.
func sumFractions(cells map[Position]float64) float64 {
	var total float64
	for _, f := range cells {
		total += f
	}

	return total
}

func (s *CoverageTestSuite) TestABoxOnTheEdgeAlongAnAxisCoversThreeDeep() {
	origin := Position{}
	toward := s.grid.GetNeighbors(origin)[0]
	deg, ok := s.emb.Bearing(origin, toward)
	s.Require().True(ok)

	out, err := Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: 15}},
		At:        origin,
		Facing:    deg,
		Anchor:    AnchorAtEdge,
	})
	s.Require().NoError(err)

	_, casterCovered := out.Cells[origin]
	s.False(casterCovered, "the caster's own cell is never under an edge-anchored box")
	s.InDelta(1.0, out.Cells[toward], 1e-9, "the cell straight ahead is fully under the box")

	third := stepAlong(origin, toward, 3)
	fourth := stepAlong(origin, toward, 4)
	s.GreaterOrEqual(out.Cells[third], 0.5, "a 15-foot box reaches three cells deep")
	s.Less(out.Cells[fourth], 0.5, "and no further")

	for cell, f := range out.Cells {
		s.True(f > 0 && f <= 1, "fraction in (0,1]: %v=%v", cell, f)
	}
}

func (s *CoverageTestSuite) TestABoxOnTheEdgeIsSymmetricAcrossItsOwnBearing() {
	origin := Position{}
	toward := s.grid.GetNeighbors(origin)[0]
	deg, _ := s.emb.Bearing(origin, toward)

	out, err := Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: 15}},
		At:        origin, Facing: deg, Anchor: AnchorAtEdge,
	})
	s.Require().NoError(err)

	// Along an axis the box is mirror-symmetric about its own centre line, so
	// every covered cell has a mirror covered by the same fraction.
	centre := s.emb.CellCentre(origin)
	u := Point{X: math.Cos(deg * math.Pi / 180), Y: math.Sin(deg * math.Pi / 180)}
	for cell, f := range out.Cells {
		p := s.emb.CellCentre(cell)
		across := (p.X-centre.X)*(-u.Y) + (p.Y-centre.Y)*u.X
		if math.Abs(across) < 1e-9 {
			continue
		}
		var mirrored float64
		for other, g := range out.Cells {
			q := s.emb.CellCentre(other)
			along := (q.X-centre.X)*u.X + (q.Y-centre.Y)*u.Y
			sameAlong := math.Abs(along-((p.X-centre.X)*u.X+(p.Y-centre.Y)*u.Y)) < 1e-9
			otherAcross := (q.X-centre.X)*(-u.Y) + (q.Y-centre.Y)*u.X
			if sameAlong && math.Abs(otherAcross+across) < 1e-9 {
				mirrored = g
			}
		}
		s.InDelta(f, mirrored, 1e-9, "cell %v at %v across has no equal mirror", cell, across)
	}
}

func (s *CoverageTestSuite) TestABoxAtAnOffAxisBearingIsAsymmetricButHonest() {
	origin := Position{}
	toward := s.grid.GetNeighbors(origin)[0]
	deg, _ := s.emb.Bearing(origin, toward)

	on, err := Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: 15}},
		At:        origin, Facing: deg, Anchor: AnchorAtEdge,
	})
	s.Require().NoError(err)
	off, err := Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: 15}},
		At:        origin, Facing: deg + 15, Anchor: AnchorAtEdge,
	})
	s.Require().NoError(err)

	s.NotEqual(keys(on.Cells), keys(off.Cells), "a rotated box covers a different set; nothing snaps to an axis")
	s.InDelta(sumFractions(on.Cells), sumFractions(off.Cells), 0.5,
		"total covered area is the box's area either way, plus or minus half a cell of edge effects")

	// The turned box is lopsided about the bearing it was drawn on: that is
	// the hexes disagreeing with the rectangle, not the rasteriser snapping.
	var left, right float64
	centre := s.emb.CellCentre(origin)
	u := Point{X: math.Cos(deg * math.Pi / 180), Y: math.Sin(deg * math.Pi / 180)}
	for cell, f := range off.Cells {
		p := s.emb.CellCentre(cell)
		if (p.X-centre.X)*(-u.Y)+(p.Y-centre.Y)*u.X > 0 {
			left += f
		} else {
			right += f
		}
	}
	s.Greater(math.Abs(left-right), 1e-9, "off the axis the two sides do not match")
}

func (s *CoverageTestSuite) TestACentredBoxCoversTheCellItSitsOn() {
	origin := Position{}
	toward := s.grid.GetNeighbors(origin)[0]
	deg, _ := s.emb.Bearing(origin, toward)

	out, err := Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: 15}},
		At:        origin, Facing: deg, Anchor: AnchorAtCentre,
	})
	s.Require().NoError(err)
	s.InDelta(1.0, out.Cells[origin], 1e-9, "a centred box sits on its own anchor")
}

func (s *CoverageTestSuite) TestRefusals() {
	_, err := Coverage(s.emb, s.grid, CoverageInput{At: Position{}})
	s.ErrorIs(err, ErrNoFootprint)

	_, err = Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 0, D: 15}}, At: Position{},
	})
	s.ErrorIs(err, ErrBadFootprint)

	_, err = Coverage(s.emb, s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: -1}}, At: Position{},
	})
	s.ErrorIs(err, ErrBadFootprint)

	_, err = Coverage(NewHexEmbedding(HexEmbeddingConfig{CellWidth: 0}), s.grid, CoverageInput{
		Footprint: Footprint{Box: &Box{W: 15, D: 15}}, At: Position{},
	})
	s.ErrorIs(err, ErrBadCellWidth, "an embedding with no frame rasterises nothing")
}
