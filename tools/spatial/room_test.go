package spatial_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MockEntity implements the core.Entity interface for testing
type MockEntity struct {
	id             string
	entityType     string
	size           int
	blocksMovement bool
	blocksLOS      bool
}

// Ensure MockEntity implements core.Entity
var _ core.Entity = (*MockEntity)(nil)

func (m *MockEntity) GetID() string            { return m.id }
func (m *MockEntity) GetType() core.EntityType { return core.EntityType(m.entityType) }
func (m *MockEntity) GetSize() int             { return m.size }
func (m *MockEntity) BlocksMovement() bool     { return m.blocksMovement }
func (m *MockEntity) BlocksLineOfSight() bool  { return m.blocksLOS }

// NewMockEntity creates a new mock entity
func NewMockEntity(id, entityType string) *MockEntity {
	return &MockEntity{
		id:             id,
		entityType:     entityType,
		size:           1,
		blocksMovement: false,
		blocksLOS:      false,
	}
}

// WithBlocking sets blocking properties
func (m *MockEntity) WithBlocking(movement, los bool) *MockEntity {
	m.blocksMovement = movement
	m.blocksLOS = los
	return m
}

type RoomTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	room     *spatial.BasicRoom
}

// SetupTest runs before EACH test function
func (s *RoomTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()

	// Create test room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})

	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "square",
		Grid: grid,
	})
	s.room.ConnectToEventBus(s.eventBus)
}

func (s *RoomTestSuite) TestRoomBasics() {
	s.Run("room properties", func() {
		s.Assert().Equal("test-room", s.room.GetID())
		s.Assert().Equal(core.EntityType("square"), s.room.GetType())
		s.Assert().NotNil(s.room.GetGrid())
	})

	s.Run("room dimensions", func() {
		dimensions := s.room.GetGrid().GetDimensions()
		s.Assert().Equal(float64(10), dimensions.Width)
		s.Assert().Equal(float64(10), dimensions.Height)
	})
}

func (s *RoomTestSuite) TestEntityPlacement() {
	entity := NewMockEntity("test-entity", "character")

	s.Run("place entity successfully", func() {
		err := s.room.PlaceEntity(entity, spatial.Position{X: 5, Y: 5})
		s.Require().NoError(err)

		// Verify entity is in room
		pos, exists := s.room.GetEntityPosition(entity.GetID())
		s.Assert().True(exists)
		s.Assert().Equal(spatial.Position{X: 5, Y: 5}, pos)

		// Verify entity is in entity list
		entities := s.room.GetAllEntities()
		s.Assert().Contains(entities, entity.GetID())
	})

	s.Run("place entity out of bounds", func() {
		err := s.room.PlaceEntity(entity, spatial.Position{X: 20, Y: 20})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "not valid")
	})

	s.Run("place entity twice moves it", func() {
		// Create a fresh entity to avoid conflicts with previous tests
		moveEntity := NewMockEntity("move-duplicate", "character")

		// Place first time
		err := s.room.PlaceEntity(moveEntity, spatial.Position{X: 1, Y: 1})
		s.Require().NoError(err)

		// Verify initial position
		pos, exists := s.room.GetEntityPosition("move-duplicate")
		s.Assert().True(exists)
		s.Assert().Equal(spatial.Position{X: 1, Y: 1}, pos)

		// Place again - should move the entity, not error
		err = s.room.PlaceEntity(moveEntity, spatial.Position{X: 2, Y: 2})
		s.Assert().NoError(err)

		// Verify it moved
		pos, exists = s.room.GetEntityPosition("move-duplicate")
		s.Assert().True(exists)
		s.Assert().Equal(spatial.Position{X: 2, Y: 2}, pos)
	})
}

func (s *RoomTestSuite) TestEntityMovement() {
	entity := NewMockEntity("mover", "character")

	s.Run("move entity successfully", func() {
		// Place entity first
		err := s.room.PlaceEntity(entity, spatial.Position{X: 3, Y: 3})
		s.Require().NoError(err)

		// Move entity
		err = s.room.MoveEntity(entity.GetID(), spatial.Position{X: 7, Y: 7})
		s.Require().NoError(err)

		// Verify new position
		pos, exists := s.room.GetEntityPosition(entity.GetID())
		s.Assert().True(exists)
		s.Assert().Equal(spatial.Position{X: 7, Y: 7}, pos)
	})

	s.Run("move nonexistent entity", func() {
		err := s.room.MoveEntity("nonexistent", spatial.Position{X: 1, Y: 1})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "not found")
	})

	s.Run("move entity out of bounds", func() {
		// Create fresh entity to avoid conflicts
		moveEntity := NewMockEntity("move-test", "character")

		// Place entity first
		err := s.room.PlaceEntity(moveEntity, spatial.Position{X: 1, Y: 1})
		s.Require().NoError(err)

		// Try to move out of bounds
		err = s.room.MoveEntity(moveEntity.GetID(), spatial.Position{X: 20, Y: 20})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "not valid")
	})
}

func (s *RoomTestSuite) TestEntityRemoval() {
	entity := NewMockEntity("removable", "item")

	s.Run("remove entity successfully", func() {
		// Place entity first
		err := s.room.PlaceEntity(entity, spatial.Position{X: 4, Y: 4})
		s.Require().NoError(err)

		// Remove entity
		err = s.room.RemoveEntity(entity.GetID())
		s.Require().NoError(err)

		// Verify entity is gone
		_, exists := s.room.GetEntityPosition(entity.GetID())
		s.Assert().False(exists)

		entities := s.room.GetAllEntities()
		s.Assert().NotContains(entities, entity.GetID())
	})

	s.Run("remove nonexistent entity", func() {
		err := s.room.RemoveEntity("nonexistent")
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "not found")
	})
}

func (s *RoomTestSuite) TestPositionQueries() {
	s.Run("valid position", func() {
		isValid := s.room.GetGrid().IsValidPosition(spatial.Position{X: 5, Y: 5})
		s.Assert().True(isValid)
	})

	s.Run("invalid position", func() {
		isValid := s.room.GetGrid().IsValidPosition(spatial.Position{X: 15, Y: 15})
		s.Assert().False(isValid)
	})

	s.Run("get entities at position", func() {
		entity := NewMockEntity("positioned", "character")
		err := s.room.PlaceEntity(entity, spatial.Position{X: 6, Y: 6})
		s.Require().NoError(err)

		entitiesAtPos := s.room.GetEntitiesAt(spatial.Position{X: 6, Y: 6})
		s.Assert().Len(entitiesAtPos, 1)
		s.Assert().Equal("positioned", entitiesAtPos[0].GetID())

		// Test empty position
		entitiesAtEmpty := s.room.GetEntitiesAt(spatial.Position{X: 8, Y: 8})
		s.Assert().Len(entitiesAtEmpty, 0)
	})
}

func (s *RoomTestSuite) TestBlockingEntities() {
	blockingEntity := NewMockEntity("blocker", "wall").WithBlocking(true, true)
	nonBlockingEntity := NewMockEntity("passable", "character")

	s.Run("blocking entity placement", func() {
		err := s.room.PlaceEntity(blockingEntity, spatial.Position{X: 5, Y: 5})
		s.Require().NoError(err)

		// Should not be able to place another entity in same position
		err = s.room.PlaceEntity(nonBlockingEntity, spatial.Position{X: 5, Y: 5})
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "cannot be placed")
	})
}

func (s *RoomTestSuite) TestEventGeneration() {
	var capturedEvents []interface{}

	// Subscribe to spatial events using typed topics
	_, _ = spatial.EntityPlacedTopic.On(s.eventBus).Subscribe(
		context.Background(),
		func(_ context.Context, event spatial.EntityPlacedEvent) error {
			capturedEvents = append(capturedEvents, event)
			return nil
		})

	_, _ = spatial.EntityMovedTopic.On(s.eventBus).Subscribe(
		context.Background(),
		func(_ context.Context, event spatial.EntityMovedEvent) error {
			capturedEvents = append(capturedEvents, event)
			return nil
		})

	_, _ = spatial.EntityRemovedTopic.On(s.eventBus).Subscribe(
		context.Background(),
		func(_ context.Context, event spatial.EntityRemovedEvent) error {
			capturedEvents = append(capturedEvents, event)
			return nil
		})

	s.Run("events are generated", func() {
		entity := NewMockEntity("eventer", "character")

		// Place entity - should generate event
		err := s.room.PlaceEntity(entity, spatial.Position{X: 2, Y: 2})
		s.Require().NoError(err)

		// Move entity - should generate event
		err = s.room.MoveEntity(entity.GetID(), spatial.Position{X: 3, Y: 3})
		s.Require().NoError(err)

		// Remove entity - should generate event
		err = s.room.RemoveEntity(entity.GetID())
		s.Require().NoError(err)

		// Verify events were captured
		s.Assert().True(len(capturedEvents) >= 3, "Should have captured placement, movement, and removal events")
	})
}

func (s *RoomTestSuite) TestMultipleEntities() {
	s.Run("place multiple entities", func() {
		entities := []*MockEntity{
			NewMockEntity("entity1", "character"),
			NewMockEntity("entity2", "monster"),
			NewMockEntity("entity3", "item"),
		}

		positions := []spatial.Position{
			{X: 1, Y: 1},
			{X: 2, Y: 2},
			{X: 3, Y: 3},
		}

		// Place all entities
		for i, entity := range entities {
			err := s.room.PlaceEntity(entity, positions[i])
			s.Require().NoError(err)
		}

		// Verify all are in room
		allEntities := s.room.GetAllEntities()
		s.Assert().Len(allEntities, 3)

		for _, entity := range entities {
			s.Assert().Contains(allEntities, entity.GetID())
		}
	})
}

// TestHexRoomCubeCoordinates tests cube coordinate functionality for hex grids
func (s *RoomTestSuite) TestHexRoomCubeCoordinates() {
	// Create a hex room for cube coordinate tests
	hexGrid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:       10,
		Height:      10,
		Orientation: spatial.HexOrientationPointyTop,
	})

	hexRoom := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "hex-room",
		Type: "hex",
		Grid: hexGrid,
	})
	hexRoom.ConnectToEventBus(s.eventBus)

	s.Run("GetEntityCubePosition returns cube coordinates for hex grid", func() {
		entity := NewMockEntity("hero", "character")
		pos := spatial.Position{X: 3, Y: 2}

		err := hexRoom.PlaceEntity(entity, pos)
		s.Require().NoError(err)

		// Get cube position
		cubePos := hexRoom.GetEntityCubePosition("hero")
		s.Require().NotNil(cubePos, "Cube position should not be nil for hex grid")

		// Verify the cube coordinate constraint: x + y + z = 0
		s.Assert().Equal(0, cubePos.X+cubePos.Y+cubePos.Z, "Cube coordinates must satisfy x + y + z = 0")

		// Verify the conversion is consistent with the grid's OffsetToCube
		expectedCube := hexGrid.OffsetToCube(pos)
		s.Assert().Equal(expectedCube.X, cubePos.X)
		s.Assert().Equal(expectedCube.Y, cubePos.Y)
		s.Assert().Equal(expectedCube.Z, cubePos.Z)
	})

	s.Run("GetEntityCubePosition returns nil for non-existent entity", func() {
		cubePos := hexRoom.GetEntityCubePosition("non-existent")
		s.Assert().Nil(cubePos, "Cube position should be nil for non-existent entity")
	})

	s.Run("GetEntityCubePosition returns nil for square grid", func() {
		// Use the default square room from SetupTest
		entity := NewMockEntity("square-hero", "character")
		err := s.room.PlaceEntity(entity, spatial.Position{X: 5, Y: 5})
		s.Require().NoError(err)

		cubePos := s.room.GetEntityCubePosition("square-hero")
		s.Assert().Nil(cubePos, "Cube position should be nil for square grid")
	})

	s.Run("entity placed event includes cube position for hex grid", func() {
		var receivedEvent spatial.EntityPlacedEvent
		eventReceived := false

		_, err := spatial.EntityPlacedTopic.On(s.eventBus).Subscribe(
			context.Background(),
			func(_ context.Context, event spatial.EntityPlacedEvent) error {
				if event.EntityID == "cube-test-entity" {
					receivedEvent = event
					eventReceived = true
				}
				return nil
			})
		s.Require().NoError(err)

		entity := NewMockEntity("cube-test-entity", "character")
		err = hexRoom.PlaceEntity(entity, spatial.Position{X: 4, Y: 3})
		s.Require().NoError(err)

		s.Require().True(eventReceived, "Should have received entity placed event")
		s.Assert().NotNil(receivedEvent.CubePosition, "Event should include cube position for hex grid")
		s.Assert().Equal("hex", receivedEvent.GridType)

		// Verify cube coordinate constraint
		s.Assert().Equal(0, receivedEvent.CubePosition.X+receivedEvent.CubePosition.Y+receivedEvent.CubePosition.Z)
	})

	s.Run("entity moved event includes cube positions for hex grid", func() {
		var receivedEvent spatial.EntityMovedEvent
		eventReceived := false

		_, err := spatial.EntityMovedTopic.On(s.eventBus).Subscribe(
			context.Background(),
			func(_ context.Context, event spatial.EntityMovedEvent) error {
				if event.EntityID == "move-test-entity" {
					receivedEvent = event
					eventReceived = true
				}
				return nil
			})
		s.Require().NoError(err)

		// Place entity first
		entity := NewMockEntity("move-test-entity", "character")
		err = hexRoom.PlaceEntity(entity, spatial.Position{X: 2, Y: 2})
		s.Require().NoError(err)

		// Move entity
		err = hexRoom.MoveEntity("move-test-entity", spatial.Position{X: 3, Y: 3})
		s.Require().NoError(err)

		s.Require().True(eventReceived, "Should have received entity moved event")
		s.Assert().NotNil(receivedEvent.FromCubePosition, "Event should include from cube position")
		s.Assert().NotNil(receivedEvent.ToCubePosition, "Event should include to cube position")

		// Verify both cube coordinates satisfy the constraint x + y + z = 0
		fromSum := receivedEvent.FromCubePosition.X + receivedEvent.FromCubePosition.Y +
			receivedEvent.FromCubePosition.Z
		s.Assert().Equal(0, fromSum)
		toSum := receivedEvent.ToCubePosition.X + receivedEvent.ToCubePosition.Y +
			receivedEvent.ToCubePosition.Z
		s.Assert().Equal(0, toSum)
	})
}

// Run the test suite
func TestRoomSuite(t *testing.T) {
	suite.Run(t, new(RoomTestSuite))
}
