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

func (m *MockEntity) GetID() string           { return m.id }
func (m *MockEntity) GetType() string         { return m.entityType }
func (m *MockEntity) GetSize() int            { return m.size }
func (m *MockEntity) BlocksMovement() bool    { return m.blocksMovement }
func (m *MockEntity) BlocksLineOfSight() bool { return m.blocksLOS }

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
	squareRoom     *spatial.BasicRoom
	hexRoom        *spatial.BasicRoom
	gridlessRoom   *spatial.BasicRoom
	eventBus       *events.Bus
	capturedEvents []events.Event
}

// SetupTest runs before EACH test function
func (s *RoomTestSuite) SetupTest() {
	s.eventBus = events.NewBus()
	s.capturedEvents = make([]events.Event, 0)

	// Capture spatial events
	s.eventBus.SubscribeFunc(spatial.EventEntityPlaced, 0, func(_ context.Context, event events.Event) error {
		s.capturedEvents = append(s.capturedEvents, event)
		return nil
	})
	s.eventBus.SubscribeFunc(spatial.EventEntityMoved, 0, func(_ context.Context, event events.Event) error {
		s.capturedEvents = append(s.capturedEvents, event)
		return nil
	})
	s.eventBus.SubscribeFunc(spatial.EventEntityRemoved, 0, func(_ context.Context, event events.Event) error {
		s.capturedEvents = append(s.capturedEvents, event)
		return nil
	})
	s.eventBus.SubscribeFunc(spatial.EventRoomCreated, 0, func(_ context.Context, event events.Event) error {
		s.capturedEvents = append(s.capturedEvents, event)
		return nil
	})

	// Create rooms with different grid types
	squareGrid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})

	hexGrid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:     10,
		Height:    10,
		PointyTop: true,
	})

	gridlessGrid := spatial.NewGridlessRoom(spatial.GridlessConfig{
		Width:  10,
		Height: 10,
	})

	s.squareRoom = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "square-room",
		Type:     "square",
		Grid:     squareGrid,
		EventBus: s.eventBus,
	})

	s.hexRoom = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "hex-room",
		Type:     "hex",
		Grid:     hexGrid,
		EventBus: s.eventBus,
	})

	s.gridlessRoom = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "gridless-room",
		Type:     "gridless",
		Grid:     gridlessGrid,
		EventBus: s.eventBus,
	})
}

// SetupSubTest runs before EACH s.Run()
func (s *RoomTestSuite) SetupSubTest() {
	s.capturedEvents = make([]events.Event, 0)
}

// TestRoomCreation tests basic room creation
func (s *RoomTestSuite) TestRoomCreation() {
	s.Run("square room", func() {
		s.Assert().Equal("square-room", s.squareRoom.GetID())
		s.Assert().Equal("square", s.squareRoom.GetType())
		s.Assert().Equal(spatial.GridShapeSquare, s.squareRoom.GetGrid().GetShape())
		s.Assert().Equal(s.eventBus, s.squareRoom.GetEventBus())
	})

	s.Run("hex room", func() {
		s.Assert().Equal("hex-room", s.hexRoom.GetID())
		s.Assert().Equal("hex", s.hexRoom.GetType())
		s.Assert().Equal(spatial.GridShapeHex, s.hexRoom.GetGrid().GetShape())
		s.Assert().Equal(s.eventBus, s.hexRoom.GetEventBus())
	})

	s.Run("gridless room", func() {
		s.Assert().Equal("gridless-room", s.gridlessRoom.GetID())
		s.Assert().Equal("gridless", s.gridlessRoom.GetType())
		s.Assert().Equal(spatial.GridShapeGridless, s.gridlessRoom.GetGrid().GetShape())
		s.Assert().Equal(s.eventBus, s.gridlessRoom.GetEventBus())
	})
}

// TestEntityPlacement tests entity placement in different grid types
func (s *RoomTestSuite) TestEntityPlacement() {
	entity := NewMockEntity("test-entity", "character")

	// Test placement in all three room types
	rooms := []*spatial.BasicRoom{s.squareRoom, s.hexRoom, s.gridlessRoom}
	roomNames := []string{"square", "hex", "gridless"}

	for i, room := range rooms {
		s.Run(roomNames[i], func() {
			pos := spatial.Position{X: 5, Y: 5}

			err := room.PlaceEntity(entity, pos)
			s.Require().NoError(err)

			// Check entity is placed
			retrievedPos, exists := room.GetEntityPosition(entity.GetID())
			s.Assert().True(exists)
			s.Assert().Equal(pos, retrievedPos)

			// Check position is occupied
			s.Assert().True(room.IsPositionOccupied(pos))

			// Check entities at position
			entitiesAt := room.GetEntitiesAt(pos)
			s.Assert().Len(entitiesAt, 1)
			s.Assert().Equal(entity.GetID(), entitiesAt[0].GetID())

			// Check event was emitted
			s.Assert().True(len(s.capturedEvents) > 0)
			var placedEvent events.Event
			for _, event := range s.capturedEvents {
				if event.Type() == spatial.EventEntityPlaced {
					placedEvent = event
					break
				}
			}
			s.Require().NotNil(placedEvent)

			s.Assert().Equal(entity.GetID(), placedEvent.Source().GetID())
			position, exists := placedEvent.Context().Get("position")
			s.Assert().True(exists)
			s.Assert().Equal(pos, position)
			roomID, exists := placedEvent.Context().Get("room_id")
			s.Assert().True(exists)
			s.Assert().Equal(room.GetID(), roomID)
		})
	}
}

// TestEntityMovement tests entity movement in different grid types
func (s *RoomTestSuite) TestEntityMovement() {
	entity := NewMockEntity("test-entity", "character")

	rooms := []*spatial.BasicRoom{s.squareRoom, s.hexRoom, s.gridlessRoom}
	roomNames := []string{"square", "hex", "gridless"}

	for i, room := range rooms {
		s.Run(roomNames[i], func() {
			oldPos := spatial.Position{X: 2, Y: 2}
			newPos := spatial.Position{X: 7, Y: 7}

			// Place entity first
			err := room.PlaceEntity(entity, oldPos)
			s.Require().NoError(err)

			// Clear captured events
			s.capturedEvents = make([]events.Event, 0)

			// Move entity
			err = room.MoveEntity(entity.GetID(), newPos)
			s.Require().NoError(err)

			// Check entity is at new position
			retrievedPos, exists := room.GetEntityPosition(entity.GetID())
			s.Assert().True(exists)
			s.Assert().Equal(newPos, retrievedPos)

			// Check old position is not occupied
			s.Assert().False(room.IsPositionOccupied(oldPos))

			// Check new position is occupied
			s.Assert().True(room.IsPositionOccupied(newPos))

			// Check event was emitted
			s.Assert().True(len(s.capturedEvents) > 0)
			var movedEvent events.Event
			for _, event := range s.capturedEvents {
				if event.Type() == spatial.EventEntityMoved {
					movedEvent = event
					break
				}
			}
			s.Require().NotNil(movedEvent)

			s.Assert().Equal(entity.GetID(), movedEvent.Source().GetID())
			oldPosition, exists := movedEvent.Context().Get("old_position")
			s.Assert().True(exists)
			s.Assert().Equal(oldPos, oldPosition)
			newPosition, exists := movedEvent.Context().Get("new_position")
			s.Assert().True(exists)
			s.Assert().Equal(newPos, newPosition)
			roomID, exists := movedEvent.Context().Get("room_id")
			s.Assert().True(exists)
			s.Assert().Equal(room.GetID(), roomID)
		})
	}
}

// TestEntityRemoval tests entity removal in different grid types
func (s *RoomTestSuite) TestEntityRemoval() {
	entity := NewMockEntity("test-entity", "character")

	rooms := []*spatial.BasicRoom{s.squareRoom, s.hexRoom, s.gridlessRoom}
	roomNames := []string{"square", "hex", "gridless"}

	for i, room := range rooms {
		s.Run(roomNames[i], func() {
			pos := spatial.Position{X: 5, Y: 5}

			// Place entity first
			err := room.PlaceEntity(entity, pos)
			s.Require().NoError(err)

			// Clear captured events
			s.capturedEvents = make([]events.Event, 0)

			// Remove entity
			err = room.RemoveEntity(entity.GetID())
			s.Require().NoError(err)

			// Check entity is removed
			_, exists := room.GetEntityPosition(entity.GetID())
			s.Assert().False(exists)

			// Check position is not occupied
			s.Assert().False(room.IsPositionOccupied(pos))

			// Check entities at position is empty
			entitiesAt := room.GetEntitiesAt(pos)
			s.Assert().Len(entitiesAt, 0)

			// Check event was emitted
			s.Assert().True(len(s.capturedEvents) > 0)
			var removedEvent events.Event
			for _, event := range s.capturedEvents {
				if event.Type() == spatial.EventEntityRemoved {
					removedEvent = event
					break
				}
			}
			s.Require().NotNil(removedEvent)

			s.Assert().Equal(entity.GetID(), removedEvent.Source().GetID())
			position, exists := removedEvent.Context().Get("position")
			s.Assert().True(exists)
			s.Assert().Equal(pos, position)
			roomID, exists := removedEvent.Context().Get("room_id")
			s.Assert().True(exists)
			s.Assert().Equal(room.GetID(), roomID)
		})
	}
}

// TestGetEntitiesInRange tests range queries with different grid distance calculations
func (s *RoomTestSuite) TestGetEntitiesInRange() {
	entity1 := NewMockEntity("entity1", "character")
	entity2 := NewMockEntity("entity2", "character")
	entity3 := NewMockEntity("entity3", "character")

	rooms := []*spatial.BasicRoom{s.squareRoom, s.hexRoom, s.gridlessRoom}
	roomNames := []string{"square", "hex", "gridless"}

	for i, room := range rooms {
		s.Run(roomNames[i], func() {
			center := spatial.Position{X: 5, Y: 5}

			// Place entities at different distances
			err := room.PlaceEntity(entity1, center)
			s.Require().NoError(err)

			err = room.PlaceEntity(entity2, spatial.Position{X: 6, Y: 6})
			s.Require().NoError(err)

			err = room.PlaceEntity(entity3, spatial.Position{X: 8, Y: 8})
			s.Require().NoError(err)

			// Test range queries
			entitiesInRange1 := room.GetEntitiesInRange(center, 1)
			s.Assert().True(len(entitiesInRange1) >= 1) // At least center entity

			entitiesInRange2 := room.GetEntitiesInRange(center, 2)
			s.Assert().True(len(entitiesInRange2) >= 2) // At least center + one more

			entitiesInRange5 := room.GetEntitiesInRange(center, 5)
			s.Assert().True(len(entitiesInRange5) >= 2) // Should include most entities

			// Verify distance calculations are grid-specific
			grid := room.GetGrid()
			distance1 := grid.Distance(center, spatial.Position{X: 6, Y: 6})
			distance2 := grid.Distance(center, spatial.Position{X: 8, Y: 8})

			// Different grid types should give different distances
			s.Assert().True(distance1 > 0)
			s.Assert().True(distance2 > distance1)
		})
	}
}

// TestBlockingEntities tests entities that block movement and line of sight
func (s *RoomTestSuite) TestBlockingEntities() {
	wall := NewMockEntity("wall", "wall").WithBlocking(true, true)
	character := NewMockEntity("character", "character")

	s.Run("movement blocking", func() {
		pos := spatial.Position{X: 5, Y: 5}

		// Place wall
		err := s.squareRoom.PlaceEntity(wall, pos)
		s.Require().NoError(err)

		// Try to place character at same position
		err = s.squareRoom.PlaceEntity(character, pos)
		s.Assert().Error(err) // Should be blocked

		// Check character cannot be placed there
		s.Assert().False(s.squareRoom.CanPlaceEntity(character, pos))
	})

	s.Run("line of sight blocking", func() {
		from := spatial.Position{X: 3, Y: 5}
		to := spatial.Position{X: 7, Y: 5}
		wallPos := spatial.Position{X: 5, Y: 5}

		// Place wall between from and to
		err := s.squareRoom.PlaceEntity(wall, wallPos)
		s.Require().NoError(err)

		// Check line of sight is blocked
		s.Assert().True(s.squareRoom.IsLineOfSightBlocked(from, to))

		// Remove wall
		err = s.squareRoom.RemoveEntity(wall.GetID())
		s.Require().NoError(err)

		// Check line of sight is clear
		s.Assert().False(s.squareRoom.IsLineOfSightBlocked(from, to))
	})
}

// TestMultipleEntitiesAtPosition tests multiple entities at the same position
func (s *RoomTestSuite) TestMultipleEntitiesAtPosition() {
	entity1 := NewMockEntity("entity1", "character")
	entity2 := NewMockEntity("entity2", "item")
	pos := spatial.Position{X: 5, Y: 5}

	// Place both entities at same position (neither blocks movement)
	err := s.squareRoom.PlaceEntity(entity1, pos)
	s.Require().NoError(err)

	err = s.squareRoom.PlaceEntity(entity2, pos)
	s.Require().NoError(err)

	// Check both entities are at position
	entitiesAt := s.squareRoom.GetEntitiesAt(pos)
	s.Assert().Len(entitiesAt, 2)

	// Check position is occupied
	s.Assert().True(s.squareRoom.IsPositionOccupied(pos))

	// Check entity count
	s.Assert().Equal(2, s.squareRoom.GetEntityCount())
}

// TestInvalidOperations tests error conditions
func (s *RoomTestSuite) TestInvalidOperations() {
	entity := NewMockEntity("test-entity", "character")

	s.Run("place nil entity", func() {
		err := s.squareRoom.PlaceEntity(nil, spatial.Position{X: 5, Y: 5})
		s.Assert().Error(err)
	})

	s.Run("place entity at invalid position", func() {
		err := s.squareRoom.PlaceEntity(entity, spatial.Position{X: -1, Y: 5})
		s.Assert().Error(err)
	})

	s.Run("move non-existent entity", func() {
		err := s.squareRoom.MoveEntity("non-existent", spatial.Position{X: 5, Y: 5})
		s.Assert().Error(err)
	})

	s.Run("remove non-existent entity", func() {
		err := s.squareRoom.RemoveEntity("non-existent")
		s.Assert().Error(err)
	})
}

// TestEventBusIntegration tests event bus functionality
func (s *RoomTestSuite) TestEventBusIntegration() {
	// Test setting a new event bus
	newBus := events.NewBus()
	s.squareRoom.SetEventBus(newBus)
	s.Assert().Equal(newBus, s.squareRoom.GetEventBus())

	// Test that events are emitted to the new bus
	capturedEvents := make([]events.Event, 0)
	newBus.SubscribeFunc(spatial.EventEntityPlaced, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	entity := NewMockEntity("test-entity", "character")
	err := s.squareRoom.PlaceEntity(entity, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)

	s.Assert().True(len(capturedEvents) > 0)
}

// TestConcurrentAccess tests thread safety
func (s *RoomTestSuite) TestConcurrentAccess() {
	entity1 := NewMockEntity("entity1", "character")
	entity2 := NewMockEntity("entity2", "character")

	// Place entities concurrently
	done := make(chan bool, 2)

	go func() {
		err := s.squareRoom.PlaceEntity(entity1, spatial.Position{X: 3, Y: 3})
		s.Assert().NoError(err)
		done <- true
	}()

	go func() {
		err := s.squareRoom.PlaceEntity(entity2, spatial.Position{X: 7, Y: 7})
		s.Assert().NoError(err)
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	// Check both entities are placed
	s.Assert().Equal(2, s.squareRoom.GetEntityCount())

	// Read operations should work concurrently
	go func() {
		_ = s.squareRoom.GetAllEntities()
		done <- true
	}()

	go func() {
		_ = s.squareRoom.GetOccupiedPositions()
		done <- true
	}()

	<-done
	<-done
}

// Run the test suite
func TestRoomSuite(t *testing.T) {
	suite.Run(t, new(RoomTestSuite))
}
