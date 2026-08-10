package spatial_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestBasicRoomOrchestrator(t *testing.T) {
	// Setup
	eventBus := events.NewEventBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "dungeon-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})
	orchestrator.ConnectToEventBus(eventBus)

	// Create some rooms
	grid1 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-1",
		Type: "chamber",
		Grid: grid1,
	})
	room1.ConnectToEventBus(eventBus)

	grid2 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 15, Height: 12})
	room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-2",
		Type: "hallway",
		Grid: grid2,
	})
	room2.ConnectToEventBus(eventBus)

	// Test adding rooms
	err := orchestrator.AddRoom(room1)
	require.NoError(t, err)

	err = orchestrator.AddRoom(room2)
	require.NoError(t, err)

	// Verify rooms were added
	allRooms := orchestrator.GetAllRooms()
	assert.Len(t, allRooms, 2)
	assert.Contains(t, allRooms, "room-1")
	assert.Contains(t, allRooms, "room-2")

	// Test getting specific room
	retrievedRoom, exists := orchestrator.GetRoom("room-1")
	assert.True(t, exists)
	assert.Equal(t, "room-1", retrievedRoom.GetID())

	// Test duplicate room addition
	err = orchestrator.AddRoom(room1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestBasicConnectionSystem(t *testing.T) {
	// Setup
	eventBus := events.NewEventBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "test-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})
	orchestrator.ConnectToEventBus(eventBus)

	// Create rooms
	grid1 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-1",
		Type: "chamber",
		Grid: grid1,
	})
	room1.ConnectToEventBus(eventBus)

	grid2 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-2",
		Type: "chamber",
		Grid: grid2,
	})
	room2.ConnectToEventBus(eventBus)

	err := orchestrator.AddRoom(room1)
	require.NoError(t, err)
	err = orchestrator.AddRoom(room2)
	require.NoError(t, err)

	// Create a door connection
	door := spatial.CreateDoorConnection(
		"door-1",
		"room-1", "room-2",
		1.0, // Standard movement cost
	)

	// Test adding connection
	err = orchestrator.AddConnection(door)
	require.NoError(t, err)

	// Verify connection was added
	allConnections := orchestrator.GetAllConnections()
	assert.Len(t, allConnections, 1)
	assert.Contains(t, allConnections, "door-1")

	// Test getting specific connection
	retrievedConn, exists := orchestrator.GetConnection("door-1")
	assert.True(t, exists)
	assert.Equal(t, "door-1", retrievedConn.GetID())
	assert.Equal(t, spatial.ConnectionTypeDoor, retrievedConn.GetConnectionType())

	// Test getting room connections
	room1Connections := orchestrator.GetRoomConnections("room-1")
	assert.Len(t, room1Connections, 1)
	assert.Equal(t, "door-1", room1Connections[0].GetID())

	room2Connections := orchestrator.GetRoomConnections("room-2")
	assert.Len(t, room2Connections, 1) // Should include reversible connection

	// Test connection to non-existent room
	badConnection := spatial.CreateDoorConnection(
		"bad-door",
		"room-1", "non-existent-room",
		1.0, // Standard movement cost
	)
	err = orchestrator.AddConnection(badConnection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestEntityMovementBetweenRooms(t *testing.T) {
	eventBus := events.NewEventBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID: "movement-orchestrator", Type: "orchestrator", Layout: spatial.LayoutTypeOrganic,
	})
	orchestrator.ConnectToEventBus(eventBus)
	room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "room-a", Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	room1.ConnectToEventBus(eventBus)
	room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "room-b", Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	room2.ConnectToEventBus(eventBus)
	require.NoError(t, orchestrator.AddRoom(room1))
	require.NoError(t, orchestrator.AddRoom(room2))
	require.NoError(t, orchestrator.AddConnection(spatial.CreateDoorConnection(
		"door-ab", "room-a", "room-b", 1,
	)))

	entity := NewMockEntity("hero", "character")
	_, err := orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-a", Entity: entity, Position: spatial.Position{X: 5, Y: 5},
	})
	require.NoError(t, err)
	currentRoom, exists := orchestrator.GetEntityRoom("hero")
	assert.True(t, exists)
	assert.Equal(t, "room-a", currentRoom)
	assert.True(t, orchestrator.CanMoveEntityBetweenRooms("hero", "room-a", "room-b", "door-ab"))

	// The legacy wrapper keeps its signature but now reports the physical
	// truth: departure leaves the entity unplaced until managed placement.
	require.NoError(t, orchestrator.MoveEntityBetweenRooms("hero", "room-a", "room-b", "door-ab"))
	_, exists = orchestrator.GetEntityRoom("hero")
	assert.False(t, exists)
	assert.Empty(t, room1.GetAllEntities())
	assert.Empty(t, room2.GetAllEntities())
	assert.False(t, orchestrator.CanMoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab"))

	_, err = orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID: "room-b", Entity: entity, Position: spatial.Position{X: 4, Y: 4},
	})
	require.NoError(t, err)
	assert.True(t, orchestrator.CanMoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab"))
	require.NoError(t, orchestrator.MoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab"))
	_, exists = orchestrator.GetEntityRoom("hero")
	assert.False(t, exists)
}

func TestPathfinding(t *testing.T) {
	// Setup
	eventBus := events.NewEventBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "pathfinding-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})
	orchestrator.ConnectToEventBus(eventBus)

	// Create rooms: A -> B -> C
	roomA := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-a",
		Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	roomA.ConnectToEventBus(eventBus)
	roomB := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-b",
		Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	roomB.ConnectToEventBus(eventBus)
	roomC := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-c",
		Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	roomC.ConnectToEventBus(eventBus)

	err := orchestrator.AddRoom(roomA)
	require.NoError(t, err)
	err = orchestrator.AddRoom(roomB)
	require.NoError(t, err)
	err = orchestrator.AddRoom(roomC)
	require.NoError(t, err)

	// Create connections: A <-> B <-> C
	doorAB := spatial.CreateDoorConnection(
		"door-ab", "room-a", "room-b",
		1.0, // Standard movement cost
	)
	doorBC := spatial.CreateDoorConnection(
		"door-bc", "room-b", "room-c",
		1.0, // Standard movement cost
	)

	err = orchestrator.AddConnection(doorAB)
	require.NoError(t, err)
	err = orchestrator.AddConnection(doorBC)
	require.NoError(t, err)

	// Create entity
	entity := NewMockEntity("explorer", "character")

	// Test direct path A -> B
	path, err := orchestrator.FindPath("room-a", "room-b", entity)
	require.NoError(t, err)
	assert.Equal(t, []string{"room-a", "room-b"}, path)

	// Test longer path A -> C
	path, err = orchestrator.FindPath("room-a", "room-c", entity)
	require.NoError(t, err)
	assert.Equal(t, []string{"room-a", "room-b", "room-c"}, path)

	// Test same room
	path, err = orchestrator.FindPath("room-a", "room-a", entity)
	require.NoError(t, err)
	assert.Equal(t, []string{"room-a"}, path)

	// Test no path (isolated room)
	roomD := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "room-d",
		Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	roomD.ConnectToEventBus(eventBus)
	err = orchestrator.AddRoom(roomD)
	require.NoError(t, err)

	path, err = orchestrator.FindPath("room-a", "room-d", entity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no path found")
	assert.Nil(t, path)
}

func TestOrchestratorEvents(t *testing.T) {
	// Setup event capture
	eventBus := events.NewEventBus()
	var capturedEvents []interface{}

	_, _ = spatial.RoomAddedTopic.On(eventBus).Subscribe(
		context.Background(),
		func(_ context.Context, event spatial.RoomAddedEvent) error {
			capturedEvents = append(capturedEvents, event)
			return nil
		})

	_, _ = spatial.ConnectionAddedTopic.On(eventBus).Subscribe(
		context.Background(),
		func(_ context.Context, event spatial.ConnectionAddedEvent) error {
			capturedEvents = append(capturedEvents, event)
			return nil
		})

	_, _ = spatial.LayoutChangedTopic.On(eventBus).Subscribe(
		context.Background(),
		func(_ context.Context, event spatial.LayoutChangedEvent) error {
			capturedEvents = append(capturedEvents, event)
			return nil
		})

	// Create orchestrator
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "event-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})
	orchestrator.ConnectToEventBus(eventBus)

	// Test operations that should generate events
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "chamber",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
	})
	room.ConnectToEventBus(eventBus)

	err := orchestrator.AddRoom(room)
	require.NoError(t, err)

	// Change layout
	err = orchestrator.SetLayout(spatial.LayoutTypeTower)
	require.NoError(t, err)

	// Verify events were captured
	assert.True(t, len(capturedEvents) >= 2, "Should have captured room added and layout changed events")

	// Check that we have the right types of events
	hasRoomAdded := false
	hasLayoutChanged := false
	for _, event := range capturedEvents {
		switch event.(type) {
		case spatial.RoomAddedEvent:
			hasRoomAdded = true
		case spatial.LayoutChangedEvent:
			hasLayoutChanged = true
		}
	}

	assert.True(t, hasRoomAdded, "Should have room added event")
	assert.True(t, hasLayoutChanged, "Should have layout changed event")
}

func TestLayoutTypes(t *testing.T) {
	eventBus := events.NewEventBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "layout-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})
	orchestrator.ConnectToEventBus(eventBus)

	// Test initial layout
	assert.Equal(t, spatial.LayoutTypeOrganic, orchestrator.GetLayout())

	// Test changing layouts
	layouts := []spatial.LayoutType{
		spatial.LayoutTypeTower,
		spatial.LayoutTypeBranching,
		spatial.LayoutTypeGrid,
		spatial.LayoutTypeOrganic,
	}

	for _, layout := range layouts {
		err := orchestrator.SetLayout(layout)
		require.NoError(t, err)
		assert.Equal(t, layout, orchestrator.GetLayout())
	}
}

// Note: MockEntity is defined in room_test.go
