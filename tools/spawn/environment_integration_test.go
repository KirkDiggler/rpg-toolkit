package spawn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EnvironmentIntegrationTestSuite tests spawn environment integration
type EnvironmentIntegrationTestSuite struct {
	suite.Suite
	engine   *BasicSpawnEngine
	eventBus events.EventBus
	ctx      context.Context
}

func (s *EnvironmentIntegrationTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.eventBus = events.NewEventBus()

	// Create spawn engine with nil environment handler for testing
	s.engine = NewBasicSpawnEngine(BasicSpawnEngineConfig{
		ID:                 "test-engine",
		SpatialHandler:     nil,
		EnvironmentHandler: nil,
		SelectablesReg:     NewBasicSelectablesRegistry(),
		EnableEvents:       true,
	})

	// Connect to event bus
	s.engine.ConnectToEventBus(s.eventBus)

	// Register test entities
	testEntities := []core.Entity{
		&MockEntity{id: "orc1", entityType: "enemy"},
		&MockEntity{id: "orc2", entityType: "enemy"},
		&MockEntity{id: "treasure1", entityType: "treasure"},
	}
	_ = s.engine.selectablesReg.RegisterTable("test-enemies", testEntities)
}

func (s *EnvironmentIntegrationTestSuite) TestCapacityAnalysis() {
	s.Run("handles capacity analysis without environment handler", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "large-group",
					Type:           "enemy",
					SelectionTable: "test-enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{25}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		result := &SpawnResult{
			Success:              true,
			SpawnedEntities:      []SpawnedEntity{},
			Failures:             []SpawnFailure{},
			RoomModifications:    []RoomModification{},
			SplitRecommendations: []RoomSplit{},
		}

		// Should handle nil environment handler gracefully
		err := s.engine.handleCapacityAnalysis(s.ctx, "test-room", config, result)
		s.Assert().NoError(err)
		s.Assert().Empty(result.SplitRecommendations)
		s.Assert().Empty(result.RoomModifications)
	})
}

func (s *EnvironmentIntegrationTestSuite) TestRoomScaling() {
	s.Run("fails room scaling without environment handler", func() {
		result := &SpawnResult{}
		currentDimensions := spatial.Dimensions{Width: 10, Height: 10}

		// Should fail without environment handler
		err := s.engine.handleRoomScaling(s.ctx, "test-room", 20, currentDimensions, result)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "environment handler required")
	})
}

func (s *EnvironmentIntegrationTestSuite) TestSplitRecommendations() {
	s.Run("converts environment split options correctly", func() {
		// Create mock environment split options
		envSplits := []environments.RoomSplit{
			{
				SuggestedSize: spatial.Dimensions{Width: 15, Height: 10},
				ConnectionPoints: []spatial.Position{
					{X: 7.5, Y: 5},
				},
				SplitReason: "Entity density too high",
				RecommendedEntityDistribution: map[string]int{
					"room1": 10,
					"room2": 15,
				},
			},
			{
				SuggestedSize: spatial.Dimensions{Width: 10, Height: 15},
				ConnectionPoints: []spatial.Position{
					{X: 5, Y: 7.5},
				},
				SplitReason: "Tactical positioning",
				RecommendedEntityDistribution: map[string]int{
					"room1": 12,
					"room2": 13,
				},
			},
		}

		// Test conversion
		convertedSplits := s.engine.convertEnvironmentSplitOptions(envSplits)

		s.Assert().Len(convertedSplits, 2)

		// Verify first split
		split1 := convertedSplits[0]
		s.Assert().Equal(spatial.Dimensions{Width: 15, Height: 10}, split1.SuggestedSize)
		s.Assert().Len(split1.ConnectionPoints, 1)
		s.Assert().Equal(spatial.Position{X: 7.5, Y: 5}, split1.ConnectionPoints[0])
		s.Assert().Equal("Entity density too high", split1.SplitReason)
		s.Assert().Equal(map[string]int{"room1": 10, "room2": 15}, split1.EntityDistribution)

		// Verify second split
		split2 := convertedSplits[1]
		s.Assert().Equal(spatial.Dimensions{Width: 10, Height: 15}, split2.SuggestedSize)
		s.Assert().Equal("Tactical positioning", split2.SplitReason)
	})
}

func (s *EnvironmentIntegrationTestSuite) TestEntityCountCalculation() {
	s.Run("calculates total entity count correctly", func() {
		groups := []EntityGroup{
			{
				ID:             "group1",
				Type:           "enemy",
				SelectionTable: "test-enemies",
				Quantity:       QuantitySpec{Fixed: &[]int{5}[0]},
			},
			{
				ID:             "group2",
				Type:           "treasure",
				SelectionTable: "test-enemies",
				Quantity:       QuantitySpec{Fixed: &[]int{3}[0]},
			},
			{
				ID:             "group3",
				Type:           "ally",
				SelectionTable: "test-enemies",
				Quantity:       QuantitySpec{}, // No fixed quantity - should default to 1
			},
		}

		total := s.engine.calculateTotalEntityCount(groups)
		s.Assert().Equal(9, total) // 5 + 3 + 1 = 9
	})
}

func (s *EnvironmentIntegrationTestSuite) TestEventPublishing() {
	s.Run("handles event publishing methods", func() {
		// Test that event publishing methods exist and can be called
		splits := []RoomSplit{
			{
				SuggestedSize:      spatial.Dimensions{Width: 15, Height: 10},
				ConnectionPoints:   []spatial.Position{{X: 7.5, Y: 5}},
				SplitReason:        "Test split",
				EntityDistribution: map[string]int{"room1": 10, "room2": 5},
			},
		}

		// These methods should not panic
		s.engine.publishSplitRecommendationEvent(s.ctx, "test-room", splits, 15)

		oldDimensions := spatial.Dimensions{Width: 10, Height: 10}
		newDimensions := spatial.Dimensions{Width: 15, Height: 15}
		s.engine.publishRoomScalingEvent(s.ctx, "test-room", oldDimensions, newDimensions, 25, 1.5)

		// Test passes if no panic occurs
		s.Assert().True(true)
	})
}

func (s *EnvironmentIntegrationTestSuite) TestCapacityFlow() {
	s.Run("executes capacity analysis flow", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "massive-army",
					Type:           "enemy",
					SelectionTable: "test-enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{50}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		result := &SpawnResult{
			Success:              true,
			SpawnedEntities:      []SpawnedEntity{},
			Failures:             []SpawnFailure{},
			RoomModifications:    []RoomModification{},
			SplitRecommendations: []RoomSplit{},
		}

		// Should handle gracefully without environment handler
		err := s.engine.handleCapacityAnalysis(s.ctx, "test-room", config, result)
		s.Assert().NoError(err)
		s.Assert().Empty(result.SplitRecommendations)
		s.Assert().Empty(result.RoomModifications)
	})
}

func TestEnvironmentIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentIntegrationTestSuite))
}
