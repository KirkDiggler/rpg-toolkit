package spatial

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FootprintTraceSuite struct{ suite.Suite }

func TestFootprintTraceSuite(t *testing.T) {
	suite.Run(t, new(FootprintTraceSuite))
}

func (s *FootprintTraceSuite) TestContactIntervals() {
	cases := []struct {
		name              string
		from, to          Point
		contact, interior bool
		enter, leave      float64
	}{
		{"cross", Point{X: -4}, Point{X: 4}, true, true, .25, .75},
		{"miss", Point{X: -4, Y: 2}, Point{X: 4, Y: 2}, false, false, 0, 0},
		{"edge overlap", Point{X: -4, Y: 1}, Point{X: 4, Y: 1}, true, false, .25, .75},
		{"edge tangent", Point{X: -4, Y: 2}, Point{Y: 1}, true, false, 1, 1},
		{"corner", Point{X: -4, Y: -1}, Point{Y: 3}, true, false, .5, .5},
		{"start inside", Point{}, Point{X: 4}, true, true, 0, .5},
		{"contained", Point{X: -1}, Point{X: 1}, true, true, 0, 1},
		{"stationary inside", Point{}, Point{}, true, false, 0, 0},
		{"stationary edge", Point{X: 2}, Point{X: 2}, true, false, 0, 0},
		{"stationary outside", Point{X: 3}, Point{X: 3}, false, false, 0, 0},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			in := FootprintTraceInput{
				Placement: FootprintPlacement{Footprint: Footprint{Box: &Box{W: 2, D: 4}}},
				From:      tc.from, To: tc.to,
			}
			out, err := TraceFootprint(in)
			s.Require().NoError(err)
			s.Equal(tc.contact, out.Contact)
			s.Equal(tc.interior, out.Interior)
			s.InDelta(tc.enter, out.Enter, 1e-12)
			s.InDelta(tc.leave, out.Leave, 1e-12)
			in.From, in.To = in.To, in.From
			reverse, err := TraceFootprint(in)
			s.Require().NoError(err)
			s.Equal(out.Contact, reverse.Contact)
			s.Equal(out.Interior, reverse.Interior)
			if out.Contact && tc.from != tc.to {
				s.InDelta(1-out.Leave, reverse.Enter, 1e-12)
				s.InDelta(1-out.Enter, reverse.Leave, 1e-12)
			}
		})
	}
}

func (s *FootprintTraceSuite) TestRotatedOffsetUsesSameFrame() {
	out, err := TraceFootprint(FootprintTraceInput{
		Placement: FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 2, D: 4}},
			Origin:    Point{X: 3.25, Y: -1.75}, Facing: 90,
			LocalOffset: Point{X: 1, Y: .5},
		},
		From: Point{X: 2.75, Y: -4.75}, To: Point{X: 2.75, Y: 3.25},
	})
	s.Require().NoError(err)
	s.True(out.Interior)
	s.InDelta(.25, out.Enter, 1e-12)
	s.InDelta(.75, out.Leave, 1e-12)
}

func (s *FootprintTraceSuite) TestThinBoxAndInvalidEndpoints() {
	placement := FootprintPlacement{Footprint: Footprint{Box: &Box{W: 4, D: .2}}}
	emb := NewHexEmbedding(HexEmbeddingConfig{CellWidth: 5})
	g := NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 9, SpanHeight: 9})
	covered, err := PlacedCoverage(PlacedCoverageInput{
		Embedding: emb, Placement: placement, Cells: g.GetPositionsInRange(Position{}, 4),
	})
	s.Require().NoError(err)
	s.NotEmpty(covered.Cells)
	for _, fraction := range covered.Cells {
		s.Less(fraction, .5)
	}
	in := FootprintTraceInput{Placement: placement, From: Point{X: -2}, To: Point{X: 2}}
	trace, err := TraceFootprint(in)
	s.Require().NoError(err)
	s.True(trace.Interior)
	s.InDelta(.475, trace.Enter, 1e-12)
	s.InDelta(.525, trace.Leave, 1e-12)
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		in.To.Y = bad
		_, err = TraceFootprint(in)
		s.ErrorIs(err, ErrBadFootprintTrace)
	}
	in.From = Point{X: -math.MaxFloat64}
	in.To = Point{X: math.MaxFloat64}
	_, err = TraceFootprint(in)
	s.ErrorIs(err, ErrBadFootprintTrace, "overflowed intermediate arithmetic must be refused")
}
