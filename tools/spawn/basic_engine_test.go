package spawn

import (
	"context"
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
			Pattern: "formation", // Not supported in Phase 1
		}

		err := s.engine.ValidateSpawnConfig(config)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "only scattered pattern")
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

func (m *MockEventBus) Publish(ctx context.Context, event events.Event) error {
	return nil
}

func (m *MockEventBus) Subscribe(eventType string, handler events.Handler) string {
	return "mock-subscription"
}

func (m *MockEventBus) SubscribeFunc(eventType string, priority int, fn events.HandlerFunc) string {
	return "mock-subscription"
}

func (m *MockEventBus) Unsubscribe(subscriptionID string) error {
	return nil
}

func (m *MockEventBus) Clear(eventType string) {
}

func (m *MockEventBus) ClearAll() {
}
