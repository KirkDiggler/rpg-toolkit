package spatial

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SightLanesSuite struct{ suite.Suite }

func TestSightLanesSuite(t *testing.T) {
	suite.Run(t, new(SightLanesSuite))
}

type footprintSight struct {
	emb       HexEmbedding
	placement FootprintPlacement
}

func (f footprintSight) Along(in SightLaneInput) (SightLaneOutput, error) {
	out, err := TraceFootprint(FootprintTraceInput{
		Placement: f.placement,
		From:      f.emb.CellCentre(in.From),
		To:        f.emb.CellCentre(in.To),
	})
	return SightLaneOutput{SoftBlocked: out.Interior}, err
}

func (f footprintSight) At(in SightCellInput) (SightCellOutput, error) {
	point := f.emb.CellCentre(in.At)
	out, err := TraceFootprint(FootprintTraceInput{
		Placement: f.placement,
		From:      point,
		To:        point,
	})
	return SightCellOutput{Blocked: out.Contact}, err
}

type canonicalRaySight struct {
	grid Grid
}

func (obstruction canonicalRaySight) Along(in SightLaneInput) (SightLaneOutput, error) {
	expected := CanonicalBoundaryRay(obstruction.grid, in.From, in.To)
	matchesRequest := len(in.Ray) > 2 && in.Ray[0] == in.From && slices.Equal(in.Ray, expected)
	return SightLaneOutput{HardBlocked: matchesRequest}, nil
}

func (canonicalRaySight) At(SightCellInput) (SightCellOutput, error) {
	return SightCellOutput{}, nil
}

func (s *SightLanesSuite) TestSuppliesCanonicalRayInRequestedDirection() {
	grid := NewSquareGrid(SquareGridConfig{Width: 10, Height: 10})
	from := Position{X: 1, Y: 2}
	to := Position{X: 7, Y: 5}
	obstructions := canonicalRaySight{grid: grid}

	for _, endpoints := range []struct {
		name     string
		from, to Position
	}{
		{name: "forward", from: from, to: to},
		{name: "reverse", from: to, to: from},
	} {
		s.Run(endpoints.name, func() {
			out, err := SightLanes(SightLanesInput{
				Grid:         grid,
				From:         endpoints.from,
				To:           endpoints.to,
				Obstructions: obstructions,
			})
			s.Require().NoError(err)
			s.True(out.Blocked, "adapter only blocks when Ray is canonical and begins at From")
		})
	}
}

func (s *SightLanesSuite) TestThinFootprintNeedsNoRoomOccupants() {
	grid := NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 9, SpanHeight: 9})
	obstruction := footprintSight{
		emb: NewHexEmbedding(HexEmbeddingConfig{CellWidth: 5}),
		placement: FootprintPlacement{
			Footprint: Footprint{Box: &Box{W: 30, D: 0.2}},
		},
	}
	in := SightLanesInput{
		Grid:         grid,
		From:         Position{X: -2},
		To:           Position{X: 2},
		Obstructions: obstruction,
	}

	out, err := SightLanes(in)
	s.Require().NoError(err)
	s.True(out.Blocked)

	in.From, in.To = in.To, in.From
	reverse, err := SightLanes(in)
	s.Require().NoError(err)
	s.Equal(out, reverse)

	obstruction.placement.Origin.X = 30
	in.Obstructions = obstruction
	out, err = SightLanes(in)
	s.Require().NoError(err)
	s.False(out.Blocked, "moving the rectangle away clears sight")
}

type directObstruction struct {
	hard bool
	err  error
}

func (d directObstruction) Along(in SightLaneInput) (SightLaneOutput, error) {
	if d.err != nil {
		return SightLaneOutput{}, d.err
	}
	direct := (in.From == (Position{}) && in.To == (Position{X: 3})) ||
		(in.To == (Position{}) && in.From == (Position{X: 3}))
	return SightLaneOutput{
		HardBlocked: direct && d.hard,
		SoftBlocked: direct && !d.hard,
	}, nil
}

func (directObstruction) At(SightCellInput) (SightCellOutput, error) {
	return SightCellOutput{}, nil
}

func (s *SightLanesSuite) TestHardDirectLaneCannotBeLeanedAround() {
	in := SightLanesInput{
		Grid:         NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 11, SpanHeight: 11}),
		To:           Position{X: 3},
		Obstructions: directObstruction{hard: true},
	}

	out, err := SightLanes(in)
	s.Require().NoError(err)
	s.True(out.Blocked)

	in.Obstructions = directObstruction{}
	out, err = SightLanes(in)
	s.Require().NoError(err)
	s.False(out.Blocked, "a clear progress-making alternate bypasses soft obstruction")
}

func (s *SightLanesSuite) TestRefusalsAndCallbackError() {
	_, err := SightLanes(SightLanesInput{})
	s.ErrorIs(err, ErrNoSightGrid)

	in := SightLanesInput{
		Grid: NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 9, SpanHeight: 9}),
	}
	_, err = SightLanes(in)
	s.ErrorIs(err, ErrNoSightObstructions)

	failure := errors.New("obstruction unavailable")
	in.Obstructions = directObstruction{err: failure}
	out, err := SightLanes(in)
	s.ErrorIs(err, failure)
	s.Equal(SightLanesOutput{}, out)

	in.From.X = math.NaN()
	_, err = SightLanes(in)
	s.ErrorIs(err, ErrBadSightPosition)
}

type originErrorObstruction struct{ failure error }

func (originErrorObstruction) Along(SightLaneInput) (SightLaneOutput, error) {
	return SightLaneOutput{SoftBlocked: true}, nil
}

func (o originErrorObstruction) At(SightCellInput) (SightCellOutput, error) {
	return SightCellOutput{}, o.failure
}

func (s *SightLanesSuite) TestAlternateOriginFailureIsNotClearSight() {
	failure := errors.New("origin unavailable")
	out, err := SightLanes(SightLanesInput{
		Grid:         NewAxialHexGrid(AxialHexGridConfig{SpanWidth: 9, SpanHeight: 9}),
		To:           Position{X: 2},
		Obstructions: originErrorObstruction{failure: failure},
	})
	s.ErrorIs(err, failure)
	s.Equal(SightLanesOutput{}, out)
}
