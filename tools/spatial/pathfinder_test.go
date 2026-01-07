package spatial

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type PathFinderTestSuite struct {
	suite.Suite
	pathFinder *SimplePathFinder
}

func TestPathFinderSuite(t *testing.T) {
	suite.Run(t, new(PathFinderTestSuite))
}

func (s *PathFinderTestSuite) SetupTest() {
	s.pathFinder = NewSimplePathFinder()
}

func (s *PathFinderTestSuite) TestDirectPath_NoObstacles() {
	start := CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := CubeCoordinate{X: 3, Y: 0, Z: -3}
	blocked := make(map[CubeCoordinate]bool)

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Require().Len(path, 3, "path should have 3 steps")
	s.Equal(goal, path[len(path)-1], "path should end at goal")

	// Verify each step is a valid neighbor of the previous
	current := start
	for _, next := range path {
		s.Equal(1, current.Distance(next), "each step should be 1 hex away")
		current = next
	}
}

func (s *PathFinderTestSuite) TestPathAroundLShapedWall() {
	// Monster at (0,0,0), target at (3,0,-3)
	// L-shaped wall blocking direct path:
	//   (1,-1,0), (1,0,-1), (2,0,-2)
	start := CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := CubeCoordinate{X: 3, Y: 0, Z: -3}
	blocked := map[CubeCoordinate]bool{
		{X: 1, Y: -1, Z: 0}: true,
		{X: 1, Y: 0, Z: -1}: true,
		{X: 2, Y: 0, Z: -2}: true,
	}

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Require().NotEmpty(path, "should find path around wall")
	s.Equal(goal, path[len(path)-1], "path should end at goal")

	// Verify path doesn't go through blocked hexes
	for _, pos := range path {
		s.Falsef(blocked[pos], "path should not include blocked hex %v", pos)
	}

	// Verify path is connected
	current := start
	for _, next := range path {
		s.Equal(1, current.Distance(next), "each step should be 1 hex away")
		current = next
	}
}

func (s *PathFinderTestSuite) TestNoPath_Surrounded() {
	start := CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := CubeCoordinate{X: 5, Y: 0, Z: -5}

	// Block all 6 neighbors of start
	blocked := map[CubeCoordinate]bool{
		{X: 1, Y: -1, Z: 0}: true,
		{X: 1, Y: 0, Z: -1}: true,
		{X: 0, Y: 1, Z: -1}: true,
		{X: -1, Y: 1, Z: 0}: true,
		{X: -1, Y: 0, Z: 1}: true,
		{X: 0, Y: -1, Z: 1}: true,
	}

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Empty(path, "should return empty path when completely surrounded")
}

func (s *PathFinderTestSuite) TestSamePosition() {
	pos := CubeCoordinate{X: 2, Y: -1, Z: -1}
	blocked := make(map[CubeCoordinate]bool)

	path := s.pathFinder.FindPath(pos, pos, blocked)

	s.Empty(path, "should return empty path when already at goal")
}

func (s *PathFinderTestSuite) TestNoPath_GoalBlocked() {
	start := CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := CubeCoordinate{X: 3, Y: 0, Z: -3}

	// Block the goal itself
	blocked := map[CubeCoordinate]bool{
		goal: true,
	}

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Empty(path, "should return empty path when goal is blocked")
}
