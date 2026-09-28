package spatial

import (
	"errors"
	"math"
)

func (s *SightLanesSuite) TestDoesNotLeanThroughOffCenterFootprint() {
	for _, orientation := range []HexOrientation{HexOrientationPointyTop, HexOrientationFlatTop} {
		emb := NewHexEmbedding(HexEmbeddingConfig{Orientation: orientation, CellWidth: 5})
		step := emb.CellCentre(Position{X: 1})
		for _, wallAt := range []float64{0.3, 3.7} {
			obstruction := footprintSight{
				emb: emb,
				placement: FootprintPlacement{
					Footprint: Footprint{Box: &Box{W: 30, D: 0.2}},
					Origin:    Point{X: wallAt * step.X, Y: wallAt * step.Y},
					Facing:    math.Atan2(step.Y, step.X) * 180 / math.Pi,
				},
			}
			for _, ends := range [][2]Position{{{}, {X: 4}}, {{X: 4}, {}}} {
				out, err := SightLanes(SightLanesInput{
					Grid: NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 11, SpanHeight: 11}),
					From: ends[0], To: ends[1], Obstructions: obstruction,
				})
				s.Require().NoError(err)
				s.True(out.Blocked, "must not jump through rectangle: orientation=%v wall=%v from=%v to=%v",
					orientation, wallAt, ends[0], ends[1])
			}
		}
	}
}

func (s *SightLanesSuite) TestDoesNotEscapeAnOpaqueFootprint() {
	obstruction := footprintSight{
		emb: NewHexEmbedding(HexEmbeddingConfig{CellWidth: 5}),
		placement: FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 2, D: 2}},
		},
	}
	for _, ends := range [][2]Position{{{}, {X: 4}}, {{X: 4}, {}}} {
		out, err := SightLanes(SightLanesInput{
			Grid: NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 11, SpanHeight: 11}),
			From: ends[0], To: ends[1], Obstructions: obstruction,
		})
		s.Require().NoError(err)
		s.True(out.Blocked, "an opaque endpoint cannot borrow an outside viewpoint through its own footprint")
	}
}

func (s *SightLanesSuite) TestCanLeanAroundFootprintWithClearConnection() {
	emb := NewHexEmbedding(HexEmbeddingConfig{CellWidth: 5})
	from, to := Position{}, Position{X: 3, Y: 1}
	target := emb.CellCentre(to)
	obstruction := footprintSight{
		emb: emb,
		placement: FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 1, D: 1}},
			Origin:    Point{X: target.X / 2, Y: target.Y / 2},
		},
	}
	direct, err := obstruction.Along(SightLaneInput{From: from, To: to})
	s.Require().NoError(err)
	s.Require().True(direct.SoftBlocked, "control: the prop obstructs the direct ray")
	for _, ends := range [][2]Position{{from, to}, {to, from}} {
		out, err := SightLanes(SightLanesInput{
			Grid: NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 11, SpanHeight: 11}),
			From: ends[0], To: ends[1], Obstructions: obstruction,
		})
		s.Require().NoError(err)
		s.False(out.Blocked, "a connected viewpoint can still see around a small prop")
	}
}

type connectionErrorSight struct {
	directObstruction
	failure error
}

func (o connectionErrorSight) Along(in SightLaneInput) (SightLaneOutput, error) {
	if in.From == (Position{}) && in.To == (Position{X: 1}) {
		return SightLaneOutput{}, o.failure
	}
	return o.directObstruction.Along(in)
}

func (s *SightLanesSuite) TestConnectionFailureIsNotClearSight() {
	failure := errors.New("connection geometry unavailable")
	out, err := SightLanes(SightLanesInput{
		Grid:         NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 11, SpanHeight: 11}),
		To:           Position{X: 3},
		Obstructions: connectionErrorSight{failure: failure},
	})
	s.ErrorIs(err, failure)
	s.Equal(SightLanesOutput{}, out)
}
