package spatial

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// FieldTestSuite exercises the distance field on the axial hex grid the
// encounter actually runs. Distances are asserted through the grid's own
// Distance, never as literals a square reader would recognise.
type FieldTestSuite struct {
	suite.Suite
	grid Grid
}

// axialSpan bounds the test grid symmetrically about the origin: with a span
// of 15, Q and R each range over -7..7 (AxialHexGrid is origin-centered).
const axialSpan = 15

func (s *FieldTestSuite) SetupTest() {
	s.grid = NewAxialHexGrid(AxialHexGridConfig{SpanWidth: axialSpan, SpanHeight: axialSpan})
}

func (s *FieldTestSuite) TestOpenHexDistancesMatchTheGrid() {
	origin := Position{X: 0, Y: 0}
	out, err := Field(s.grid, FieldInput{
		Sources:  []Position{origin},
		Passable: func(_, _ Position) bool { return true },
	})
	s.Require().NoError(err)
	s.Equal(0, out.Dist[origin])
	for cell, d := range out.Dist {
		s.Equal(int(s.grid.Distance(origin, cell)), d,
			"field distance is the grid's distance on open ground: %v", cell)
	}
	s.Equal(6, countAt(out, 1), "a hex has six neighbours")
}

func countAt(out FieldOutput, d int) int {
	n := 0
	for _, v := range out.Dist {
		if v == d {
			n++
		}
	}
	return n
}

func TestFieldTestSuite(t *testing.T) { suite.Run(t, new(FieldTestSuite)) }
