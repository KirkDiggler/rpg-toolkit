package environments

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BasicEnvironmentTestSuite tests basic environment functionality
type BasicEnvironmentTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	env      *BasicEnvironment
}

func (s *BasicEnvironmentTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	s.env = NewBasicEnvironment(BasicEnvironmentConfig{
		ID:    "test-env",
		Type:  "test",
		Theme: "dungeon",
	})
	s.env.ConnectToEventBus(s.eventBus)
}

func (s *BasicEnvironmentTestSuite) TestBasicProperties() {
	s.Run("has valid ID and type", func() {
		s.Assert().Equal("test-env", s.env.GetID())
		s.Assert().Equal("test", string(s.env.GetType()))
	})

	s.Run("has theme", func() {
		s.Assert().Equal("dungeon", s.env.GetTheme())
	})
}

func (s *BasicEnvironmentTestSuite) TestThemeChanges() {
	s.Run("has initial theme", func() {
		// Note: SetTheme requires orchestrator for room tracking
		// This test just verifies the initial theme is set correctly
		s.Assert().Equal("dungeon", s.env.GetTheme())
	})
}

func (s *BasicEnvironmentTestSuite) TestFindPathCube() {
	s.Run("returns error for nil input", func() {
		result, err := s.env.FindPathCube(nil)
		s.Assert().Error(err)
		s.Assert().Nil(result)
		s.Assert().Contains(err.Error(), "input cannot be nil")
	})

	s.Run("finds direct path with no obstacles", func() {
		input := &FindPathCubeInput{
			From:    spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
			To:      spatial.CubeCoordinate{X: 3, Y: 0, Z: -3},
			Blocked: make(map[spatial.CubeCoordinate]bool),
		}

		result, err := s.env.FindPathCube(input)
		s.Require().NoError(err)
		s.Assert().True(result.Found)
		s.Assert().Equal(3, result.TotalDistance)
		s.Assert().Len(result.Path, 3)
		s.Assert().Equal(input.To, result.Path[len(result.Path)-1])
	})

	s.Run("finds path around obstacles", func() {
		input := &FindPathCubeInput{
			From: spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
			To:   spatial.CubeCoordinate{X: 3, Y: 0, Z: -3},
			Blocked: map[spatial.CubeCoordinate]bool{
				{X: 1, Y: -1, Z: 0}: true,
				{X: 1, Y: 0, Z: -1}: true,
				{X: 2, Y: 0, Z: -2}: true,
			},
		}

		result, err := s.env.FindPathCube(input)
		s.Require().NoError(err)
		s.Assert().True(result.Found)
		s.Assert().Greater(result.TotalDistance, 3, "path should be longer due to obstacles")
		s.Assert().Equal(input.To, result.Path[len(result.Path)-1])

		// Verify path doesn't go through blocked hexes
		for _, pos := range result.Path {
			s.Assert().False(input.Blocked[pos], "path should not include blocked hex")
		}
	})

	s.Run("returns not found when surrounded", func() {
		input := &FindPathCubeInput{
			From: spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
			To:   spatial.CubeCoordinate{X: 5, Y: 0, Z: -5},
			Blocked: map[spatial.CubeCoordinate]bool{
				{X: 1, Y: -1, Z: 0}: true,
				{X: 1, Y: 0, Z: -1}: true,
				{X: 0, Y: 1, Z: -1}: true,
				{X: -1, Y: 1, Z: 0}: true,
				{X: -1, Y: 0, Z: 1}: true,
				{X: 0, Y: -1, Z: 1}: true,
			},
		}

		result, err := s.env.FindPathCube(input)
		s.Require().NoError(err)
		s.Assert().False(result.Found)
		s.Assert().Empty(result.Path)
		s.Assert().Equal(0, result.TotalDistance)
	})

	s.Run("returns found with empty path when already at goal", func() {
		pos := spatial.CubeCoordinate{X: 2, Y: -1, Z: -1}
		input := &FindPathCubeInput{
			From:    pos,
			To:      pos,
			Blocked: make(map[spatial.CubeCoordinate]bool),
		}

		result, err := s.env.FindPathCube(input)
		s.Require().NoError(err)
		s.Assert().True(result.Found)
		s.Assert().Empty(result.Path)
		s.Assert().Equal(0, result.TotalDistance)
	})

	s.Run("returns not found when goal is blocked", func() {
		goal := spatial.CubeCoordinate{X: 3, Y: 0, Z: -3}
		input := &FindPathCubeInput{
			From: spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
			To:   goal,
			Blocked: map[spatial.CubeCoordinate]bool{
				goal: true,
			},
		}

		result, err := s.env.FindPathCube(input)
		s.Require().NoError(err)
		s.Assert().False(result.Found)
		s.Assert().Empty(result.Path)
	})
}

func TestBasicEnvironmentSuite(t *testing.T) {
	suite.Run(t, new(BasicEnvironmentTestSuite))
}
