package spawn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicSpawnEngineTestSuite tests spawn engine functionality
type BasicSpawnEngineTestSuite struct {
	suite.Suite
	engine     *BasicSpawnEngine
	registry   *BasicSelectablesRegistry
	eventBus   events.EventBus
	testEntity *MockEntity
}

func (s *BasicSpawnEngineTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	s.registry = NewBasicSelectablesRegistry()
	s.testEntity = &MockEntity{id: "test-entity", entityType: "test"}

	// Create engine with basic config
	s.engine = NewBasicSpawnEngine(BasicSpawnEngineConfig{
		ID:             "test-engine",
		SelectablesReg: s.registry,
		EnableEvents:   true,
		MaxAttempts:    10,
	})

	// Connect to event bus
	s.engine.ConnectToEventBus(s.eventBus)

	// Register test entity
	err := s.registry.RegisterTable("test-table", []core.Entity{s.testEntity})
	s.Require().NoError(err)
}

func (s *BasicSpawnEngineTestSuite) TestSpawnConfigValidation() {
	s.Run("validates basic config", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "group1",
					Type:           "test",
					SelectionTable: "test-table",
					Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		err := s.engine.ValidateSpawnConfig(config)
		s.Assert().NoError(err)
	})

	s.Run("rejects empty entity groups", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{},
			Pattern:      PatternScattered,
		}

		err := s.engine.ValidateSpawnConfig(config)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "no entity groups")
	})

	s.Run("validates all supported patterns", func() {
		patterns := []SpawnPattern{
			PatternScattered,
			PatternFormation,
			PatternClustered,
			PatternTeamBased,
			PatternPlayerChoice,
		}

		for _, pattern := range patterns {
			config := SpawnConfig{
				EntityGroups: []EntityGroup{
					{
						ID:             "group1",
						Type:           "test",
						SelectionTable: "test-table",
						Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
					},
				},
				Pattern: pattern,
			}

			err := s.engine.ValidateSpawnConfig(config)
			s.Assert().NoError(err, "Pattern %s should validate", pattern)
		}
	})
}

func (s *BasicSpawnEngineTestSuite) TestEntitySelection() {
	s.Run("selects entities from registry", func() {
		group := EntityGroup{
			ID:             "test-group",
			Type:           "test",
			SelectionTable: "test-table",
			Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
		}

		entities, err := s.engine.selectEntitiesForGroup(group)
		s.Assert().NoError(err)
		s.Assert().Len(entities, 1)
		s.Assert().Equal("test-entity", entities[0].GetID())
	})

	s.Run("handles missing table", func() {
		group := EntityGroup{
			ID:             "test-group",
			Type:           "test",
			SelectionTable: "nonexistent",
			Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
		}

		entities, err := s.engine.selectEntitiesForGroup(group)
		s.Assert().Error(err)
		s.Assert().Nil(entities)
	})
}

func (s *BasicSpawnEngineTestSuite) TestRoomStructureAnalysis() {
	s.Run("analyzes single room", func() {
		info := s.engine.AnalyzeRoomStructure("test-room")
		s.Assert().False(info.IsSplit)
		s.Assert().Equal([]string{"test-room"}, info.ConnectedRooms)
		s.Assert().Equal("test-room", info.PrimaryRoomID)
	})
}

func (s *BasicSpawnEngineTestSuite) TestPopulateSpaceMethods() {
	s.Run("handles interface compliance", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "group1",
					Type:           "test",
					SelectionTable: "test-table",
					Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		// Methods should be callable even if they fail due to missing spatial integration
		_, err := s.engine.PopulateSpace(context.Background(), "test-room", config)
		s.Assert().Error(err) // Expected to fail without spatial handler

		_, err = s.engine.PopulateRoom(context.Background(), "test-room", config)
		s.Assert().Error(err) // Expected to fail without spatial handler

		_, err = s.engine.PopulateSplitRooms(context.Background(), []string{"room1", "room2"}, config)
		s.Assert().Error(err) // Expected to fail without spatial handler
	})
}

func TestBasicSpawnEngineTestSuite(t *testing.T) {
	suite.Run(t, new(BasicSpawnEngineTestSuite))
}

// MockEntity implements core.Entity for testing
type MockEntity struct {
	id         string
	entityType string
}

func (e *MockEntity) GetID() string {
	return e.id
}

func (e *MockEntity) GetType() core.EntityType {
	return core.EntityType(e.entityType)
}
