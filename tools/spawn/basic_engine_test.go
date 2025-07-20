package spawn

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicSpawnEngineTestSuite tests the Phase 1 implementation
type BasicSpawnEngineTestSuite struct {
	suite.Suite
	engine       *BasicSpawnEngine
	registry     *BasicSelectablesRegistry
	mockEventBus *MockEventBus
	testEntities []core.Entity
}

func (s *BasicSpawnEngineTestSuite) SetupTest() {
	s.mockEventBus = NewMockEventBus(s.T())
	s.registry = NewBasicSelectablesRegistry()

	s.engine = NewBasicSpawnEngine(BasicSpawnEngineConfig{
		ID:             "test-engine",
		SpatialHandler: nil, // Phase 1: not implemented
		SelectablesReg: s.registry,
		EventBus:       s.mockEventBus,
		EnableEvents:   true,
		MaxAttempts:    10,
	})

	s.testEntities = []core.Entity{
		&TestEntity{id: "entity1", entityType: "enemy"},
		&TestEntity{id: "entity2", entityType: "enemy"},
		&TestEntity{id: "entity3", entityType: "treasure"},
	}
}

func (s *BasicSpawnEngineTestSuite) TestConfigValidation() {
	s.Run("validates valid config", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "test-group",
					Type:           "enemy",
					SelectionTable: "enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{2}[0]},
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

	s.Run("rejects unsupported pattern", func() {
		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "test-group",
					Type:           "enemy",
					SelectionTable: "enemies",
					Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
				},
			},
			Pattern: "invalid_pattern", // Truly unsupported pattern
		}

		err := s.engine.ValidateSpawnConfig(config)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "unsupported pattern")
	})
}

func (s *BasicSpawnEngineTestSuite) TestEntitySelection() {
	s.Run("selects entities from registry", func() {
		// Register test entities
		err := s.registry.RegisterTable("enemies", s.testEntities[:2])
		s.Require().NoError(err)

		entities, err := s.engine.selectEntitiesForGroup(EntityGroup{
			ID:             "test-group",
			Type:           "enemy",
			SelectionTable: "enemies",
			Quantity:       QuantitySpec{Fixed: &[]int{2}[0]},
		})

		s.Assert().NoError(err)
		s.Assert().Len(entities, 2)
	})

	s.Run("handles missing table", func() {
		entities, err := s.engine.selectEntitiesForGroup(EntityGroup{
			ID:             "test-group",
			Type:           "enemy",
			SelectionTable: "nonexistent",
			Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
		})

		s.Assert().Error(err)
		s.Assert().Nil(entities)
		s.Assert().Contains(err.Error(), "not found")
	})
}

func (s *BasicSpawnEngineTestSuite) TestConfigValidationComprehensive() {
	s.Run("validates complex configurations", func() {
		// Register test entities
		err := s.registry.RegisterTable("mixed-entities", s.testEntities)
		s.Require().NoError(err)

		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "group1",
					Type:           "enemy",
					SelectionTable: "mixed-entities",
					Quantity:       QuantitySpec{Fixed: &[]int{2}[0]},
				},
				{
					ID:             "group2",
					Type:           "treasure",
					SelectionTable: "mixed-entities",
					Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		err = s.engine.ValidateSpawnConfig(config)
		s.Assert().NoError(err, "Complex config should validate successfully")
	})
}

func (s *BasicSpawnEngineTestSuite) TestPatternValidation() {
	s.Run("validates all supported patterns", func() {
		// Register test entities
		err := s.registry.RegisterTable("test-entities", s.testEntities)
		s.Require().NoError(err)

		patterns := []SpawnPattern{
			PatternScattered,
			PatternFormation,
			PatternTeamBased,
			PatternPlayerChoice,
			PatternClustered,
		}

		for _, pattern := range patterns {
			s.Run(fmt.Sprintf("pattern_%s_validates", pattern), func() {
				config := SpawnConfig{
					EntityGroups: []EntityGroup{
						{
							ID:             "test-group",
							Type:           "mixed",
							SelectionTable: "test-entities",
							Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
						},
					},
					Pattern: pattern,
				}

				// Add required config for specific patterns
				if pattern == PatternTeamBased {
					config.TeamConfiguration = &TeamConfig{
						Teams: []Team{
							{ID: "team1", EntityTypes: []string{"enemy"}},
						},
					}
				}
				if pattern == PatternPlayerChoice {
					config.PlayerSpawnZones = []SpawnZone{
						{
							ID:          "zone1",
							EntityTypes: []string{"player"},
							MaxEntities: 5,
						},
					}
				}

				err := s.engine.ValidateSpawnConfig(config)
				s.Assert().NoError(err, "Pattern %s should validate", pattern)
			})
		}
	})
}

func (s *BasicSpawnEngineTestSuite) TestInterfaceCompliance() {
	s.Run("implements all SpawnEngine methods", func() {
		// This test verifies interface compliance by calling all methods
		// Even if they don't fully work due to missing spatial integration

		config := SpawnConfig{
			EntityGroups: []EntityGroup{
				{
					ID:             "test-group",
					Type:           "test",
					SelectionTable: "nonexistent",
					Quantity:       QuantitySpec{Fixed: &[]int{1}[0]},
				},
			},
			Pattern: PatternScattered,
		}

		// All these methods should exist and be callable
		info := s.engine.AnalyzeRoomStructure("test-room")
		s.Assert().False(info.IsSplit)
		s.Assert().Equal("test-room", info.PrimaryRoomID)

		err := s.engine.ValidateSpawnConfig(config)
		s.Assert().NoError(err) // Validation only checks config structure, not table existence

		// PopulateSpace should also be callable (even if it fails)
		_, err = s.engine.PopulateSpace(context.Background(), "test-room", config)
		s.Assert().Error(err) // Expected to fail due to spatial integration

		_, err = s.engine.PopulateRoom(context.Background(), "test-room", config)
		s.Assert().Error(err) // Expected to fail due to spatial integration

		_, err = s.engine.PopulateSplitRooms(context.Background(), []string{"room1"}, config)
		s.Assert().Error(err) // Expected to fail due to spatial integration
	})
}

func (s *BasicSpawnEngineTestSuite) TestRoomStructureAnalysis() {
	s.Run("analyzes room structure correctly", func() {
		// Test single room
		info := s.engine.AnalyzeRoomStructure("single-room")
		s.Assert().False(info.IsSplit)
		s.Assert().Equal([]string{"single-room"}, info.ConnectedRooms)
		s.Assert().Equal("single-room", info.PrimaryRoomID)
	})
}

func TestBasicSpawnEngineTestSuite(t *testing.T) {
	suite.Run(t, new(BasicSpawnEngineTestSuite))
}

// TestEntity implements core.Entity for testing
type TestEntity struct {
	id         string
	entityType string
}

func (e *TestEntity) GetID() string   { return e.id }
func (e *TestEntity) GetType() string { return e.entityType }

// MockEventBus for testing
type MockEventBus struct {
	t *testing.T
}

func NewMockEventBus(t *testing.T) *MockEventBus {
	return &MockEventBus{t: t}
}

func (m *MockEventBus) Publish(_ context.Context, _ events.Event) error {
	return nil
}

func (m *MockEventBus) Subscribe(_ string, _ events.Handler) string {
	return "mock-subscription"
}

func (m *MockEventBus) SubscribeFunc(_ string, _ int, _ events.HandlerFunc) string {
	return "mock-subscription"
}

func (m *MockEventBus) Unsubscribe(_ string) error {
	return nil
}

func (m *MockEventBus) Clear(_ string) {
}

func (m *MockEventBus) ClearAll() {
}
