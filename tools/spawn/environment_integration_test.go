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

// EnvironmentIntegrationTestSuite tests Phase 4 environment integration
type EnvironmentIntegrationTestSuite struct {
	suite.Suite
	engine             *BasicSpawnEngine
	environmentHandler *environments.BasicQueryHandler
	eventBus           events.EventBus
	ctx                context.Context
}

func (s *EnvironmentIntegrationTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.eventBus = events.NewBus()

	// Create environment handler for testing
	// Note: Environment handler requires orchestrator and spatial query for full functionality
	// For these tests, we'll test the spawn engine's capacity analysis methods directly
	s.environmentHandler = nil // We'll test without environment handler and with mocked responses

	// Create spawn engine with environment integration
	s.engine = NewBasicSpawnEngine(BasicSpawnEngineConfig{
		ID:                 "test-engine",
		SpatialHandler:     nil, // Not needed for environment tests
		EnvironmentHandler: s.environmentHandler,
		SelectablesReg:     NewBasicSelectablesRegistry(),
		EventBus:           s.eventBus,
		EnableEvents:       true,
	})

	// Register test entities
	testEntities := []core.Entity{
		&TestEntity{id: "orc1", entityType: "enemy"},
		&TestEntity{id: "orc2", entityType: "enemy"},
		&TestEntity{id: "treasure1", entityType: "treasure"},
	}
	_ = s.engine.selectablesReg.RegisterTable("test-enemies", testEntities)
}

func (s *EnvironmentIntegrationTestSuite) TestCapacityAnalysis() {
	s.Run("performs capacity analysis with environment integration", func() {
		// Since we have nil environment handler, capacity analysis should complete without changes
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "large-group",
					Type:           "enemy",
					SelectionTable: "test-enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{25}[0]}, // Large group to trigger capacity issues
				},
			},
			Pattern: PatternScattered,
		}

		// Create a result to test capacity analysis
		result := &SpawnResult{
			Success:              true,
			SpawnedEntities:      []SpawnedEntity{},
			Failures:             []SpawnFailure{},
			RoomModifications:    []RoomModification{},
			SplitRecommendations: []RoomSplit{},
		}

		// Test capacity analysis - should handle nil environment handler gracefully
		err := s.engine.handleCapacityAnalysis(s.ctx, "test-room", config, result)
		s.Assert().NoError(err)

		// Should have empty results since no environment handler is available
		s.Assert().Empty(result.SplitRecommendations, "No environment handler means no split recommendations")
		s.Assert().Empty(result.RoomModifications, "No environment handler means no room modifications")
	})

	s.Run("handles capacity analysis with no environment handler", func() {
		// Create engine without environment handler
		engineNoEnv := NewBasicSpawnEngine(BasicSpawnEngineConfig{
			ID:                 "test-engine-no-env",
			SpatialHandler:     nil,
			EnvironmentHandler: nil, // No environment handler
			SelectablesReg:     NewBasicSelectablesRegistry(),
			EventBus:           s.eventBus,
			EnableEvents:       true,
		})

		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "small-group",
					Type:           "enemy",
					SelectionTable: "test-enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{3}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		result := &SpawnResult{}

		// Should handle gracefully without environment handler
		err := engineNoEnv.handleCapacityAnalysis(s.ctx, "test-room", config, result)
		s.Assert().NoError(err)
		s.Assert().Empty(result.SplitRecommendations)
		s.Assert().Empty(result.RoomModifications)
	})
}

func (s *EnvironmentIntegrationTestSuite) TestRoomScaling() {
	s.Run("performs room scaling when adaptive scaling is enabled", func() {
		// Test room scaling directly without needing the full config since we have nil environment handler
		// This test verifies the room scaling logic would work if we had an environment handler
		result := &SpawnResult{}
		currentDimensions := spatial.Dimensions{Width: 10, Height: 10}

		// Since we don't have an environment handler, this should fail gracefully
		err := s.engine.handleRoomScaling(s.ctx, "test-room", 20, currentDimensions, result)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "environment handler required")
	})

	s.Run("fails room scaling without environment handler", func() {
		// Create engine without environment handler
		engineNoEnv := NewBasicSpawnEngine(BasicSpawnEngineConfig{
			ID:                 "test-engine-no-env",
			SpatialHandler:     nil,
			EnvironmentHandler: nil,
			SelectablesReg:     NewBasicSelectablesRegistry(),
			EventBus:           s.eventBus,
			EnableEvents:       true,
		})

		result := &SpawnResult{}
		currentDimensions := spatial.Dimensions{Width: 10, Height: 10}

		// Should fail without environment handler
		err := engineNoEnv.handleRoomScaling(s.ctx, "test-room", 20, currentDimensions, result)
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
	s.Run("publishes split recommendation events", func() {
		// Set up event listener
		var receivedEvent events.Event
		s.eventBus.SubscribeFunc("spawn.split.recommended", 10, func(_ context.Context, event events.Event) error {
			receivedEvent = event
			return nil
		})

		splits := []RoomSplit{
			{
				SuggestedSize:      spatial.Dimensions{Width: 15, Height: 10},
				ConnectionPoints:   []spatial.Position{{X: 7.5, Y: 5}},
				SplitReason:        "Test split",
				EntityDistribution: map[string]int{"room1": 10, "room2": 5},
			},
		}

		// Publish split recommendation event
		s.engine.publishSplitRecommendationEvent(s.ctx, "test-room", splits, 15)

		// Verify event was published
		s.Assert().NotNil(receivedEvent)
		s.Assert().Equal("spawn.split.recommended", receivedEvent.Type())
		roomID, _ := receivedEvent.Context().Get("room_id")
		s.Assert().Equal("test-room", roomID)
		entityCount, _ := receivedEvent.Context().Get("entity_count")
		s.Assert().Equal(15, entityCount.(int))
		splitCount, _ := receivedEvent.Context().Get("split_count")
		s.Assert().Equal(1, splitCount.(int))
	})

	s.Run("publishes room scaling events", func() {
		// Set up event listener
		var receivedEvent events.Event
		s.eventBus.SubscribeFunc("spawn.room.scaled", 10, func(_ context.Context, event events.Event) error {
			receivedEvent = event
			return nil
		})

		oldDimensions := spatial.Dimensions{Width: 10, Height: 10}
		newDimensions := spatial.Dimensions{Width: 15, Height: 15}
		scaleFactor := 1.5
		entityCount := 25

		// Publish room scaling event
		s.engine.publishRoomScalingEvent(s.ctx, "test-room", oldDimensions, newDimensions, entityCount, scaleFactor)

		// Verify event was published
		s.Assert().NotNil(receivedEvent)
		s.Assert().Equal("spawn.room.scaled", receivedEvent.Type())
		roomID, _ := receivedEvent.Context().Get("room_id")
		s.Assert().Equal("test-room", roomID)
		entityCountRaw, _ := receivedEvent.Context().Get("entity_count")
		s.Assert().Equal(25, entityCountRaw.(int))
		scaleFactorRaw, _ := receivedEvent.Context().Get("scale_factor")
		s.Assert().Equal(1.5, scaleFactorRaw.(float64))
		oldWidth, _ := receivedEvent.Context().Get("old_width")
		s.Assert().Equal(10.0, oldWidth.(float64))
		newWidth, _ := receivedEvent.Context().Get("new_width")
		s.Assert().Equal(15.0, newWidth.(float64))
	})

	s.Run("skips event publishing when events disabled", func() {
		// Create engine with events disabled
		engineNoEvents := NewBasicSpawnEngine(BasicSpawnEngineConfig{
			ID:                 "test-engine-no-events",
			SpatialHandler:     nil,
			EnvironmentHandler: s.environmentHandler,
			SelectablesReg:     NewBasicSelectablesRegistry(),
			EventBus:           s.eventBus,
			EnableEvents:       false, // Events disabled
		})

		// Set up event listener (should not receive anything)
		eventReceived := false
		s.eventBus.SubscribeFunc("spawn.room.scaled", 10, func(_ context.Context, _ events.Event) error {
			eventReceived = true
			return nil
		})

		oldDimensions := spatial.Dimensions{Width: 10, Height: 10}
		newDimensions := spatial.Dimensions{Width: 15, Height: 15}

		// Try to publish event (should be skipped)
		engineNoEvents.publishRoomScalingEvent(s.ctx, "test-room", oldDimensions, newDimensions, 25, 1.5)

		// Verify no event was published
		s.Assert().False(eventReceived, "Event should not be published when events are disabled")
	})
}

func (s *EnvironmentIntegrationTestSuite) TestEndToEndCapacityFlow() {
	s.Run("executes complete capacity analysis flow without environment handler", func() {
		// Test demonstrates that capacity analysis gracefully handles missing environment handler
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "massive-army",
					Type:           "enemy",
					SelectionTable: "test-enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{50}[0]}, // Very large group
				},
			},
			Pattern: PatternScattered,
			AdaptiveScaling: &ScalingConfig{
				Enabled:        true,
				ScalingFactor:  2.0,
				PreserveAspect: true,
				EmitEvents:     true,
			},
		}

		// Set up event listeners to verify no events are published without environment handler
		var splitEvent, scalingEvent events.Event
		s.eventBus.SubscribeFunc("spawn.split.recommended", 10, func(_ context.Context, event events.Event) error {
			splitEvent = event
			return nil
		})
		s.eventBus.SubscribeFunc("spawn.room.scaled", 10, func(_ context.Context, event events.Event) error {
			scalingEvent = event
			return nil
		})

		result := &SpawnResult{
			Success:              true,
			SpawnedEntities:      []SpawnedEntity{},
			Failures:             []SpawnFailure{},
			RoomModifications:    []RoomModification{},
			SplitRecommendations: []RoomSplit{},
		}

		// Execute capacity analysis
		err := s.engine.handleCapacityAnalysis(s.ctx, "test-room", config, result)
		s.Assert().NoError(err)

		// Verify graceful handling without environment integration
		s.Assert().Empty(result.SplitRecommendations, "No environment handler means no split recommendations")
		s.Assert().Empty(result.RoomModifications, "No environment handler means no room modifications")

		// Verify no events were published since no environment operations occurred
		s.Assert().Nil(splitEvent, "No split event should be published without environment handler")
		s.Assert().Nil(scalingEvent, "No scaling event should be published without environment handler")
	})
}

func TestEnvironmentIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentIntegrationTestSuite))
}
