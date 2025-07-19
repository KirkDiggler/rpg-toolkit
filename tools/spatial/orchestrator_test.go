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
	eventBus := events.NewBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       "dungeon-orchestrator",
		Type:     "orchestrator",
		EventBus: eventBus,
		Layout:   spatial.LayoutTypeOrganic,
	})

	// Create some rooms
	grid1 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-1",
		Type:     "chamber",
		Grid:     grid1,
		EventBus: eventBus,
	})

	grid2 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 15, Height: 12})
	room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-2",
		Type:     "hallway",
		Grid:     grid2,
		EventBus: eventBus,
	})

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
	eventBus := events.NewBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       "test-orchestrator",
		Type:     "orchestrator",
		EventBus: eventBus,
		Layout:   spatial.LayoutTypeOrganic,
	})

	// Create rooms
	grid1 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-1",
		Type:     "chamber",
		Grid:     grid1,
		EventBus: eventBus,
	})

	grid2 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-2",
		Type:     "chamber",
		Grid:     grid2,
		EventBus: eventBus,
	})

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
	t.Skip("Skipping until spawn layer is implemented to handle entity placement via events (ADR-0015)")
	// Setup
	eventBus := events.NewBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       "movement-orchestrator",
		Type:     "orchestrator",
		EventBus: eventBus,
		Layout:   spatial.LayoutTypeOrganic,
	})

	// Create rooms
	grid1 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-a",
		Type:     "chamber",
		Grid:     grid1,
		EventBus: eventBus,
	})

	grid2 := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-b",
		Type:     "chamber",
		Grid:     grid2,
		EventBus: eventBus,
	})

	err := orchestrator.AddRoom(room1)
	require.NoError(t, err)
	err = orchestrator.AddRoom(room2)
	require.NoError(t, err)

	// Create connection
	door := spatial.CreateDoorConnection(
		"door-ab",
		"room-a", "room-b",
		1.0, // Standard movement cost
	)
	err = orchestrator.AddConnection(door)
	require.NoError(t, err)

	// Create and place an entity
	entity := NewMockEntity("hero", "character")
	err = room1.PlaceEntity(entity, spatial.Position{X: 5, Y: 5})
	require.NoError(t, err)

	// Verify entity room tracking
	currentRoom, exists := orchestrator.GetEntityRoom("hero")
	assert.True(t, exists)
	assert.Equal(t, "room-a", currentRoom)

	// Test movement capability
	canMove := orchestrator.CanMoveEntityBetweenRooms("hero", "room-a", "room-b", "door-ab")
	assert.True(t, canMove)

	// Test actual movement
	err = orchestrator.MoveEntityBetweenRooms("hero", "room-a", "room-b", "door-ab")
	require.NoError(t, err)

	// Verify entity moved
	currentRoom, exists = orchestrator.GetEntityRoom("hero")
	assert.True(t, exists)
	assert.Equal(t, "room-b", currentRoom)

	// Verify entity is no longer in source room (ADR-0015: Abstract Connections)
	entitiesInRoomA := room1.GetAllEntities()
	assert.NotContains(t, entitiesInRoomA, "hero")

	// Note: In abstract connection mode, entity placement in destination room
	// is handled by game layer via events. The orchestrator only tracks logical room assignment.

	// Test reverse movement (door is bidirectional)
	canMoveBack := orchestrator.CanMoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab")
	assert.True(t, canMoveBack)

	err = orchestrator.MoveEntityBetweenRooms("hero", "room-b", "room-a", "door-ab")
	require.NoError(t, err)

	// Verify entity moved back
	currentRoom, exists = orchestrator.GetEntityRoom("hero")
	assert.True(t, exists)
	assert.Equal(t, "room-a", currentRoom)
}

func TestPathfinding(t *testing.T) {
	// Setup
	eventBus := events.NewBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       "pathfinding-orchestrator",
		Type:     "orchestrator",
		EventBus: eventBus,
		Layout:   spatial.LayoutTypeOrganic,
	})

	// Create rooms: A -> B -> C
	roomA := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-a",
		Type:     "chamber",
		Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
		EventBus: eventBus,
	})
	roomB := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-b",
		Type:     "chamber",
		Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
		EventBus: eventBus,
	})
	roomC := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "room-c",
		Type:     "chamber",
		Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
		EventBus: eventBus,
	})

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
		ID:       "room-d",
		Type:     "chamber",
		Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
		EventBus: eventBus,
	})
	err = orchestrator.AddRoom(roomD)
	require.NoError(t, err)

	path, err = orchestrator.FindPath("room-a", "room-d", entity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no path found")
	assert.Nil(t, path)
}

func TestConnectionTypes(t *testing.T) {
	// Test different connection factory functions

	// Test door
	door := spatial.CreateDoorConnection("door-1", "room-a", "room-b", 1.0)
	assert.Equal(t, spatial.ConnectionTypeDoor, door.GetConnectionType())
	assert.True(t, door.IsReversible())
	assert.True(t, door.IsPassable(NewMockEntity("test", "character")))

	// Test stairs
	stairs := spatial.CreateStairsConnection("stairs-1", "floor-1", "floor-2", 2.0, true)
	assert.Equal(t, spatial.ConnectionTypeStairs, stairs.GetConnectionType())
	assert.True(t, stairs.IsReversible())
	assert.Contains(t, stairs.GetRequirements(), "can_climb")
	assert.Equal(t, 2.0, stairs.GetTraversalCost(NewMockEntity("test", "character")))

	// Test secret passage
	secret := spatial.CreateSecretPassageConnection(
		"secret-1", "room-a", "room-b", 1.0, []string{"found_secret", "has_key"})
	assert.Equal(t, spatial.ConnectionTypePassage, secret.GetConnectionType())
	assert.Contains(t, secret.GetRequirements(), "found_secret")
	assert.Contains(t, secret.GetRequirements(), "has_key")

	// Test portal
	portal := spatial.CreatePortalConnection("portal-1", "room-a", "room-b", 0.5, false)
	assert.Equal(t, spatial.ConnectionTypePortal, portal.GetConnectionType())
	assert.False(t, portal.IsReversible())
	assert.Equal(t, 0.5, portal.GetTraversalCost(NewMockEntity("test", "character")))

	// Test bridge
	bridge := spatial.CreateBridgeConnection("bridge-1", "side-a", "side-b", 1.0)
	assert.Equal(t, spatial.ConnectionTypeBridge, bridge.GetConnectionType())
	assert.True(t, bridge.IsReversible())

	// Test tunnel
	tunnel := spatial.CreateTunnelConnection("tunnel-1", "cave-a", "cave-b", 1.5)
	assert.Equal(t, spatial.ConnectionTypeTunnel, tunnel.GetConnectionType())
	assert.Equal(t, 1.5, tunnel.GetTraversalCost(NewMockEntity("test", "character")))
}

func TestOrchestratorEvents(t *testing.T) {
	// Setup event capture
	eventBus := events.NewBus()
	var capturedEvents []events.Event

	eventBus.SubscribeFunc(spatial.EventRoomAdded, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	eventBus.SubscribeFunc(spatial.EventConnectionAdded, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	eventBus.SubscribeFunc(spatial.EventLayoutChanged, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	// Create orchestrator
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       "event-orchestrator",
		Type:     "orchestrator",
		EventBus: eventBus,
		Layout:   spatial.LayoutTypeOrganic,
	})

	// Test operations that should generate events
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "test-room",
		Type:     "chamber",
		Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10}),
		EventBus: eventBus,
	})

	err := orchestrator.AddRoom(room)
	require.NoError(t, err)

	// Change layout
	err = orchestrator.SetLayout(spatial.LayoutTypeTower)
	require.NoError(t, err)

	// Verify events were captured
	assert.True(t, len(capturedEvents) >= 2, "Should have captured room added and layout changed events")

	// Check event types
	eventTypes := make(map[string]bool)
	for _, event := range capturedEvents {
		eventTypes[event.Type()] = true
	}

	assert.True(t, eventTypes[spatial.EventRoomAdded], "Should have room added event")
	assert.True(t, eventTypes[spatial.EventLayoutChanged], "Should have layout changed event")
}

func TestLayoutTypes(t *testing.T) {
	eventBus := events.NewBus()
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       "layout-orchestrator",
		Type:     "orchestrator",
		EventBus: eventBus,
		Layout:   spatial.LayoutTypeOrganic,
	})

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
