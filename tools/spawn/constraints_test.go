package spawn

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ConstraintSolverTestSuite tests the Phase 3 constraint system
type ConstraintSolverTestSuite struct {
	suite.Suite
	solver *ConstraintSolver
	room   spatial.Room
}

func (s *ConstraintSolverTestSuite) SetupTest() {
	s.solver = NewConstraintSolver()
	// Use a real spatial BasicRoom for testing
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

func (s *ConstraintSolverTestSuite) TestMinDistanceConstraints() {
	s.Run("validates minimum distance between entity types", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}
		position := spatial.Position{X: 2.0, Y: 2.0}

		// Create existing entity too close
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "enemy1", entityType: "enemy"},
				Position: spatial.Position{X: 2.5, Y: 2.5}, // Distance ~0.7
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			MinDistance: map[string]float64{
				"player:enemy": 2.0, // Require 2 units minimum distance
			},
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, existingEntities)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "too close")
	})

	s.Run("allows sufficient distance", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}
		position := spatial.Position{X: 1.0, Y: 1.0}

		// Create existing entity far enough away
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "enemy1", entityType: "enemy"},
				Position: spatial.Position{X: 5.0, Y: 5.0}, // Distance ~5.6
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			MinDistance: map[string]float64{
				"player:enemy": 2.0,
			},
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, existingEntities)
		s.Assert().NoError(err)
	})
}

func (s *ConstraintSolverTestSuite) TestWallProximityConstraints() {
	s.Run("validates wall proximity constraints", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}

		// Position too close to wall
		position := spatial.Position{X: 0.5, Y: 2.0}

		constraints := SpatialConstraints{
			WallProximity: 1.0, // Require 1 unit from walls
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, []SpawnedEntity{})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "too close to wall")
	})

	s.Run("allows sufficient wall distance", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}

		// Position with good wall distance
		position := spatial.Position{X: 5.0, Y: 5.0}

		constraints := SpatialConstraints{
			WallProximity: 1.0,
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, []SpawnedEntity{})
		s.Assert().NoError(err)
	})
}

func (s *ConstraintSolverTestSuite) TestLineOfSightConstraints() {
	s.Run("validates required line of sight", func() {
		entity := &TestEntity{id: "guard1", entityType: "guard"}
		position := spatial.Position{X: 2.0, Y: 2.0}

		// Create treasure entity that guard must see
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "treasure1", entityType: "treasure"},
				Position: spatial.Position{X: 5.0, Y: 5.0}, // Within sight range
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			LineOfSight: LineOfSightRules{
				RequiredSight: []EntityPair{
					{From: "guard", To: "treasure"},
				},
			},
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, existingEntities)
		s.Assert().NoError(err)
	})

	s.Run("validates blocked line of sight", func() {
		entity := &TestEntity{id: "thief1", entityType: "thief"}
		position := spatial.Position{X: 2.0, Y: 2.0}

		// Create guard entity that thief must NOT see
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "guard1", entityType: "guard"},
				Position: spatial.Position{X: 3.0, Y: 3.0}, // Within sight range
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			LineOfSight: LineOfSightRules{
				BlockedSight: []EntityPair{
					{From: "thief", To: "guard"},
				},
			},
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, existingEntities)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "has line of sight")
	})
}

func (s *ConstraintSolverTestSuite) TestAreaOfEffectConstraints() {
	s.Run("validates area of effect constraints", func() {
		entity := &TestEntity{id: "spell1", entityType: "spell"}
		position := spatial.Position{X: 2.0, Y: 2.0}

		// Create existing entity within area of effect
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "player1", entityType: "player"},
				Position: spatial.Position{X: 2.5, Y: 2.5}, // Distance ~0.7
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			AreaOfEffect: map[string]float64{
				"spell": 2.0, // 2-unit radius area of effect
			},
		}

		err := s.solver.ValidatePosition(s.room, position, entity, constraints, existingEntities)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "within area of effect")
	})
}

func (s *ConstraintSolverTestSuite) TestFindValidPositions() {
	s.Run("finds positions that satisfy constraints", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}

		// Create constraint that blocks most positions
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "enemy1", entityType: "enemy"},
				Position: spatial.Position{X: 5.0, Y: 5.0},
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			MinDistance: map[string]float64{
				"player:enemy": 1.0, // Require 1 unit distance
			},
			WallProximity: 1.0, // Require 1 unit from walls
		}

		positions, err := s.solver.FindValidPositions(s.room, entity, constraints, existingEntities, 5)
		s.Assert().NoError(err)
		s.Assert().NotEmpty(positions)

		// Verify all returned positions satisfy constraints
		for _, pos := range positions {
			err := s.solver.ValidatePosition(s.room, pos, entity, constraints, existingEntities)
			s.Assert().NoError(err, "Position (%.2f, %.2f) should satisfy constraints", pos.X, pos.Y)
		}
	})

	s.Run("returns error when no valid positions exist", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}

		// Create constraints that are impossible to satisfy
		existingEntities := []SpawnedEntity{
			{
				Entity:   &TestEntity{id: "enemy1", entityType: "enemy"},
				Position: spatial.Position{X: 5.0, Y: 5.0},
				RoomID:   "room1",
			},
		}

		constraints := SpatialConstraints{
			MinDistance: map[string]float64{
				"player:enemy": 20.0, // Impossible distance in 10x10 room
			},
		}

		positions, err := s.solver.FindValidPositions(s.room, entity, constraints, existingEntities, 5)
		s.Assert().Error(err)
		s.Assert().Empty(positions)
		s.Assert().Contains(err.Error(), "no valid positions found")
	})
}

func (s *ConstraintSolverTestSuite) TestGridlessRoomHandling() {
	s.Run("handles gridless rooms appropriately", func() {
		entity := &TestEntity{id: "test1", entityType: "player"}

		// Gridless room should allow more flexible positioning
		constraints := SpatialConstraints{
			WallProximity: 0.5, // Small wall distance for gridless
		}

		positions, err := s.solver.FindValidPositions(s.room, entity, constraints, []SpawnedEntity{}, 3)
		s.Assert().NoError(err)
		s.Assert().NotEmpty(positions)

		// In gridless rooms, positions should be more varied (not locked to grid)
		// Check that we got multiple valid positions
		s.Assert().GreaterOrEqual(len(positions), 1)

		// Verify all positions respect wall proximity
		for _, pos := range positions {
			s.Assert().GreaterOrEqual(pos.X, 0.5, "Position should respect wall proximity")
			s.Assert().GreaterOrEqual(pos.Y, 0.5, "Position should respect wall proximity")
			s.Assert().LessOrEqual(pos.X, 9.5, "Position should respect wall proximity")
			s.Assert().LessOrEqual(pos.Y, 9.5, "Position should respect wall proximity")
		}
	})
}

func TestConstraintSolverTestSuite(t *testing.T) {
	suite.Run(t, new(ConstraintSolverTestSuite))
}
