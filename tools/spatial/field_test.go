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
const (
	axialSpan = 15
	axialHalf = 7
)

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

// blockedColumn blocks every cell on the axial column q except the one at
// gapR. Q changes by at most one per hex step, so a full column separates the
// grid into west and east and the gap is the only way across.
func blockedColumn(q, gapR float64) map[Position]bool {
	blocked := map[Position]bool{}
	for r := -axialHalf; r <= axialHalf; r++ {
		if float64(r) != gapR {
			blocked[Position{X: q, Y: float64(r)}] = true
		}
	}
	return blocked
}

func (s *FieldTestSuite) TestABlockedColumnForcesTheRouteThroughTheGap() {
	gap := Position{X: 3, Y: -axialHalf}
	blocked := blockedColumn(gap.X, gap.Y)
	from, goal := Position{X: 1, Y: 0}, Position{X: 5, Y: 0}

	out, err := Field(s.grid, FieldInput{
		Sources:  []Position{from},
		Passable: func(_, to Position) bool { return !blocked[to] },
	})
	s.Require().NoError(err)

	for cell := range blocked {
		_, reached := out.Dist[cell]
		s.False(reached, "a blocked cell is never entered: %v", cell)
	}
	s.Greater(out.Dist[goal], int(s.grid.Distance(from, goal)),
		"the detour is longer than the crow flies")

	path, ok := out.PathTo(goal)
	s.Require().True(ok)
	s.Contains(path, gap, "the only way east is the gap")
}

func (s *FieldTestSuite) TestLimitStopsTheFlood() {
	origin := Position{X: 0, Y: 0}
	out, err := Field(s.grid, FieldInput{
		Sources:  []Position{origin},
		Passable: func(_, _ Position) bool { return true },
		Limit:    2,
	})
	s.Require().NoError(err)

	for cell, d := range out.Dist {
		s.LessOrEqual(d, 2, "nothing past the limit is in the field: %v", cell)
	}
	s.Equal(6, countAt(out, 1), "the first hex ring")
	s.Equal(12, countAt(out, 2), "the second hex ring")
}

func (s *FieldTestSuite) TestPathToWalksPrevBackToASource() {
	from, goal := Position{X: 1, Y: 0}, Position{X: 5, Y: 0}
	out, err := Field(s.grid, FieldInput{
		Sources:  []Position{from},
		Passable: func(_, _ Position) bool { return true },
	})
	s.Require().NoError(err)

	path, ok := out.PathTo(goal)
	s.Require().True(ok)
	s.Equal(goal, path[len(path)-1], "path ends at the goal")
	s.NotContains(path, from, "path excludes the source, matching the route's contract")
	s.Len(path, out.Dist[goal], "one cell per unit of distance on open ground")
	for i := 1; i < len(path); i++ {
		s.True(s.grid.IsAdjacent(path[i-1], path[i]), "every step is a neighbour: %v -> %v", path[i-1], path[i])
	}

	_, ok = out.PathTo(Position{X: 100, Y: 100})
	s.False(ok, "an unreached cell has no path")
}

func (s *FieldTestSuite) TestTheFieldIsGridGeneric() {
	// The evidence that closes #614: the same function runs on a square grid.
	// Not the model — the game is hex by choice.
	sq := NewSquareGrid(SquareGridConfig{Width: 5, Height: 5})
	corner, far := Position{X: 0, Y: 0}, Position{X: 4, Y: 4}
	out, err := Field(sq, FieldInput{
		Sources:  []Position{corner},
		Passable: func(_, _ Position) bool { return true },
	})
	s.Require().NoError(err)
	s.Equal(int(sq.Distance(corner, far)), out.Dist[far])
}

func (s *FieldTestSuite) TestNoSourcesAndNoPredicateFailClosed() {
	_, err := Field(s.grid, FieldInput{Passable: func(_, _ Position) bool { return true }})
	s.ErrorIs(err, ErrNoSources)

	_, err = Field(s.grid, FieldInput{Sources: []Position{{}}})
	s.ErrorIs(err, ErrNoPassable)
}
