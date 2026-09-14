package spatial

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PlacedFootprintSuite struct{ suite.Suite }

func TestPlacedFootprintSuite(t *testing.T) {
	suite.Run(t, new(PlacedFootprintSuite))
}

func (s *PlacedFootprintSuite) TestOffsetRotatesBeforeTranslation() {
	frame, err := footprintBox(FootprintPlacement{
		Footprint: Footprint{Box: &Box{W: 2, D: 4}},
		Origin:    Point{X: 3.25, Y: -1.75}, Facing: 90,
		LocalOffset: Point{X: 1, Y: 0.5},
	})
	s.Require().NoError(err)
	want := [4]Point{{X: 3.75, Y: -2.75}, {X: 3.75, Y: 1.25},
		{X: 1.75, Y: 1.25}, {X: 1.75, Y: -2.75}}
	for i, p := range want {
		s.InDelta(p.X, frame.corners[i].X, 1e-12)
		s.InDelta(p.Y, frame.corners[i].Y, 1e-12)
	}
}

func (s *PlacedFootprintSuite) TestInvalidPlacements() {
	cases := []struct {
		name string
		in   FootprintPlacement
		want error
	}{
		{"no shape", FootprintPlacement{}, ErrNoFootprint},
		{"zero width", FootprintPlacement{Footprint: Footprint{Box: &Box{W: 0, D: 2}}}, ErrBadFootprint},
		{"negative depth", FootprintPlacement{Footprint: Footprint{Box: &Box{W: 1, D: -2}}}, ErrBadFootprint},
		{"NaN width", FootprintPlacement{Footprint: Footprint{Box: &Box{W: math.NaN(), D: 2}}}, ErrBadFootprint},
		{"infinite depth", FootprintPlacement{Footprint: Footprint{Box: &Box{W: 1, D: math.Inf(1)}}}, ErrBadFootprint},
		{"NaN origin", FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 1, D: 2}}, Origin: Point{X: math.NaN()},
		}, ErrBadFootprintPlacement},
		{"infinite angle", FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 1, D: 2}}, Facing: math.Inf(-1),
		}, ErrBadFootprintPlacement},
		{"infinite offset", FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 1, D: 2}}, LocalOffset: Point{Y: math.Inf(1)},
		}, ErrBadFootprintPlacement},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := footprintBox(tc.in)
			s.ErrorIs(err, tc.want)
		})
	}
}

func (s *PlacedFootprintSuite) TestEquivalentAnglesAndCopiedDimensions() {
	box := &Box{W: 2, D: 4}
	in := FootprintPlacement{Footprint: Footprint{Box: box}, Facing: -270}
	before, err := footprintBox(in)
	s.Require().NoError(err)
	in.Facing = 450
	after, err := footprintBox(in)
	s.Require().NoError(err)
	for i := range before.corners {
		s.InDelta(before.corners[i].X, after.corners[i].X, 1e-12)
		s.InDelta(before.corners[i].Y, after.corners[i].Y, 1e-12)
	}
	box.W = 100
	s.Equal(1.0, before.halfWidth)
}
