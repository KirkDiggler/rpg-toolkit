package spawn

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ConstraintSolverTestSuite tests spawn constraint system
type ConstraintSolverTestSuite struct {
	suite.Suite
	solver     *ConstraintSolver
	room       spatial.Room
	mockEntity *MockEntity
}

func (s *ConstraintSolverTestSuite) SetupTest() {
	s.solver = NewConstraintSolver()
	s.mockEntity = &MockEntity{id: "test-entity", entityType: "test"}

	// Create a simple gridless room for testing
	gridConfig := spatial.GridlessConfig{
		Width:  10,
		Height: 10,
	}
	grid := spatial.NewGridlessRoom(gridConfig)

	roomConfig := spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "test",
		Grid: grid,
	}
	s.room = spatial.NewBasicRoom(roomConfig)
}

func (s *ConstraintSolverTestSuite) TestBasicConstraintValidation() {
	s.Run("validates minimum distance constraints", func() {
		position := spatial.Position{X: 2.0, Y: 2.0}

		// Create existing entity too close
		existingEntities := []SpawnedEntity{
			{
				Entity:   &MockEntity{id: "enemy1", entityType: "enemy"},
				Position: spatial.Position{X: 2.5, Y: 2.5},
				RoomID:   "test-room",
			},
		}

		constraints := SpatialConstraints{
			MinDistance: map[string]float64{
				"test:enemy": 2.0,
			},
		}

		err := s.solver.ValidatePosition(s.room, position, s.mockEntity, constraints, existingEntities)
		s.Assert().Error(err)
	})

	s.Run("allows sufficient distance", func() {
		position := spatial.Position{X: 1.0, Y: 1.0}

		existingEntities := []SpawnedEntity{
			{
				Entity:   &MockEntity{id: "enemy1", entityType: "enemy"},
				Position: spatial.Position{X: 8.0, Y: 8.0},
				RoomID:   "test-room",
			},
		}

		constraints := SpatialConstraints{
			MinDistance: map[string]float64{
				"test:enemy": 2.0,
			},
		}

		err := s.solver.ValidatePosition(s.room, position, s.mockEntity, constraints, existingEntities)
		s.Assert().NoError(err)
	})
}

func (s *ConstraintSolverTestSuite) TestWallProximityConstraints() {
	s.Run("validates wall proximity", func() {
		// Position too close to wall (left edge)
		position := spatial.Position{X: 0.2, Y: 5.0}

		constraints := SpatialConstraints{
			WallProximity: 1.0,
		}

		err := s.solver.ValidatePosition(s.room, position, s.mockEntity, constraints, []SpawnedEntity{})
		s.Assert().Error(err)
	})

	s.Run("allows sufficient wall distance", func() {
		// Position with good wall distance
		position := spatial.Position{X: 5.0, Y: 5.0}

		constraints := SpatialConstraints{
			WallProximity: 1.0,
		}

		err := s.solver.ValidatePosition(s.room, position, s.mockEntity, constraints, []SpawnedEntity{})
		s.Assert().NoError(err)
	})
}

func (s *ConstraintSolverTestSuite) TestFindValidPositions() {
	s.Run("finds valid positions", func() {
		constraints := SpatialConstraints{
			WallProximity: 1.0,
		}

		positions, err := s.solver.FindValidPositions(s.room, s.mockEntity, constraints, []SpawnedEntity{}, 3)
		s.Assert().NoError(err)
		s.Assert().NotEmpty(positions)
		s.Assert().LessOrEqual(len(positions), 3)

		// Verify all returned positions satisfy constraints
		for _, pos := range positions {
			err := s.solver.ValidatePosition(s.room, pos, s.mockEntity, constraints, []SpawnedEntity{})
			s.Assert().NoError(err, "Position should satisfy constraints")
		}
	})

	s.Run("handles impossible constraints", func() {
		// Impossible constraints
		constraints := SpatialConstraints{
			WallProximity: 15.0, // Impossible in 10x10 room
		}

		positions, err := s.solver.FindValidPositions(s.room, s.mockEntity, constraints, []SpawnedEntity{}, 3)
		s.Assert().Error(err)
		s.Assert().Empty(positions)
	})
}

func TestConstraintSolverTestSuite(t *testing.T) {
	suite.Run(t, new(ConstraintSolverTestSuite))
}
