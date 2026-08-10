package spatial

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicRoomOrchestrator implements the RoomOrchestrator interface
type BasicRoomOrchestrator struct {
	mu         sync.RWMutex
	id         OrchestratorID
	entityType string
	eventBus   events.EventBus // Store the event bus for EventBusIntegration interface

	// Type-safe event publishers (replaces eventBus events.EventBus)
	roomAdded            events.TypedTopic[RoomAddedEvent]
	roomRemoved          events.TypedTopic[RoomRemovedEvent]
	connectionAdded      events.TypedTopic[ConnectionAddedEvent]
	connectionRemoved    events.TypedTopic[ConnectionRemovedEvent]
	entityRoomTransition events.TypedTopic[EntityRoomTransitionEvent]
	layoutChanged        events.TypedTopic[LayoutChangedEvent]

	rooms       map[RoomID]Room
	connections map[ConnectionID]Connection
	entityRooms map[core.EntityID]RoomID // entityID -> roomID mapping
	layout      LayoutType
}

// BasicRoomOrchestratorConfig holds configuration for creating a basic room orchestrator
type BasicRoomOrchestratorConfig struct {
	ID     OrchestratorID // Optional: if empty, will auto-generate
	Type   string
	Layout LayoutType
}

// NewBasicRoomOrchestrator creates a new basic room orchestrator
func NewBasicRoomOrchestrator(config BasicRoomOrchestratorConfig) *BasicRoomOrchestrator {
	layout := config.Layout
	if layout == "" {
		layout = LayoutTypeOrganic // Default to organic layout
	}

	id := config.ID
	if id == "" {
		id = NewOrchestratorID()
	}

	return &BasicRoomOrchestrator{
		id:          id,
		entityType:  config.Type,
		rooms:       make(map[RoomID]Room),
		connections: make(map[ConnectionID]Connection),
		entityRooms: make(map[core.EntityID]RoomID),
		layout:      layout,
	}
}

// ConnectToEventBus connects all typed topics to the event bus
func (bro *BasicRoomOrchestrator) ConnectToEventBus(bus events.EventBus) {
	bro.SetEventBus(bus)
}

// SetEventBus sets the event bus for the orchestrator (implements EventBusIntegration)
func (bro *BasicRoomOrchestrator) SetEventBus(bus events.EventBus) {
	bro.mu.Lock()
	defer bro.mu.Unlock()

	bro.eventBus = bus

	// Connect orchestrator event publishers
	bro.roomAdded = RoomAddedTopic.On(bus)
	bro.roomRemoved = RoomRemovedTopic.On(bus)
	bro.connectionAdded = ConnectionAddedTopic.On(bus)
	bro.connectionRemoved = ConnectionRemovedTopic.On(bus)
	bro.entityRoomTransition = EntityRoomTransitionTopic.On(bus)
	bro.layoutChanged = LayoutChangedTopic.On(bus)
}

// GetEventBus returns the current event bus (implements EventBusIntegration)
func (bro *BasicRoomOrchestrator) GetEventBus() events.EventBus {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return bro.eventBus
}

// GetID returns the orchestrator ID
func (bro *BasicRoomOrchestrator) GetID() string {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return bro.id.String()
}

// GetType returns the entity type
func (bro *BasicRoomOrchestrator) GetType() core.EntityType {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return core.EntityType(bro.entityType)
}

// AddRoom adds a room to the orchestrator. Entities already in the room are
// indexed synchronously. Adding a room that duplicates an entity already
// indexed in another managed room fails without changing the orchestrator.
func (bro *BasicRoomOrchestrator) AddRoom(room Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	roomID := RoomID(room.GetID())
	roomType := string(room.GetType())
	entities := room.GetAllEntities()

	bro.mu.Lock()
	if _, exists := bro.rooms[roomID]; exists {
		bro.mu.Unlock()
		return fmt.Errorf("room %s already exists", roomID)
	}
	for entityIDStr := range entities {
		entityID := core.EntityID(entityIDStr)
		if indexedRoom, exists := bro.entityRooms[entityID]; exists {
			bro.mu.Unlock()
			return fmt.Errorf("entity %s is already indexed in room %s", entityID, indexedRoom)
		}
	}
	bro.rooms[roomID] = room
	for entityIDStr := range entities {
		bro.entityRooms[core.EntityID(entityIDStr)] = roomID
	}
	topic := bro.roomAdded
	orchestratorID := bro.id.String()
	bro.mu.Unlock()

	if topic != nil {
		_ = topic.Publish(context.Background(), RoomAddedEvent{
			OrchestratorID: orchestratorID,
			RoomID:         roomID.String(),
			RoomType:       roomType,
			AddedAt:        time.Now(),
		})
	}
	return nil
}

// RemoveRoom removes a room from the orchestrator.
func (bro *BasicRoomOrchestrator) RemoveRoom(roomIDStr string) error {
	bro.mu.Lock()
	roomID := RoomID(roomIDStr)
	if _, exists := bro.rooms[roomID]; !exists {
		bro.mu.Unlock()
		return fmt.Errorf("room %s not found", roomID)
	}

	for entityID, mappedRoomID := range bro.entityRooms {
		if mappedRoomID == roomID {
			delete(bro.entityRooms, entityID)
		}
	}
	for connID, conn := range bro.connections {
		if conn.GetFromRoom() == roomIDStr || conn.GetToRoom() == roomIDStr {
			delete(bro.connections, connID)
		}
	}
	delete(bro.rooms, roomID)
	topic := bro.roomRemoved
	orchestratorID := bro.id.String()
	bro.mu.Unlock()

	if topic != nil {
		_ = topic.Publish(context.Background(), RoomRemovedEvent{
			OrchestratorID: orchestratorID,
			RoomID:         roomIDStr,
			Reason:         "removed",
			RemovedAt:      time.Now(),
		})
	}
	return nil
}

// GetRoom retrieves a room by ID
func (bro *BasicRoomOrchestrator) GetRoom(roomIDStr string) (Room, bool) {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	roomID := RoomID(roomIDStr)
	room, exists := bro.rooms[roomID]
	return room, exists
}

// GetAllRooms returns all managed rooms
func (bro *BasicRoomOrchestrator) GetAllRooms() map[string]Room {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	// Return a copy to prevent modification
	result := make(map[string]Room)
	for id, room := range bro.rooms {
		result[id.String()] = room
	}
	return result
}

// AddConnection creates a connection between two rooms.
func (bro *BasicRoomOrchestrator) AddConnection(connection Connection) error {
	if connection == nil {
		return fmt.Errorf("connection cannot be nil")
	}
	connID := ConnectionID(connection.GetID())
	fromRoom := RoomID(connection.GetFromRoom())
	toRoom := RoomID(connection.GetToRoom())
	connectionType := string(connection.GetConnectionType())

	bro.mu.Lock()
	if _, exists := bro.connections[connID]; exists {
		bro.mu.Unlock()
		return fmt.Errorf("connection %s already exists", connID)
	}
	if _, exists := bro.rooms[fromRoom]; !exists {
		bro.mu.Unlock()
		return fmt.Errorf("from room %s does not exist", fromRoom)
	}
	if _, exists := bro.rooms[toRoom]; !exists {
		bro.mu.Unlock()
		return fmt.Errorf("to room %s does not exist", toRoom)
	}
	bro.connections[connID] = connection
	topic := bro.connectionAdded
	orchestratorID := bro.id.String()
	bro.mu.Unlock()

	if topic != nil {
		_ = topic.Publish(context.Background(), ConnectionAddedEvent{
			OrchestratorID: orchestratorID,
			ConnectionID:   connID.String(),
			FromRoom:       fromRoom.String(),
			ToRoom:         toRoom.String(),
			ConnectionType: connectionType,
			AddedAt:        time.Now(),
		})
	}
	return nil
}

// RemoveConnection removes a connection.
func (bro *BasicRoomOrchestrator) RemoveConnection(connectionIDStr string) error {
	bro.mu.Lock()
	connectionID := ConnectionID(connectionIDStr)
	if _, exists := bro.connections[connectionID]; !exists {
		bro.mu.Unlock()
		return fmt.Errorf("connection %s not found", connectionID)
	}
	delete(bro.connections, connectionID)
	topic := bro.connectionRemoved
	orchestratorID := bro.id.String()
	bro.mu.Unlock()

	if topic != nil {
		_ = topic.Publish(context.Background(), ConnectionRemovedEvent{
			OrchestratorID: orchestratorID,
			ConnectionID:   connectionIDStr,
			Reason:         "removed",
			RemovedAt:      time.Now(),
		})
	}
	return nil
}

// GetConnection retrieves a connection by ID
func (bro *BasicRoomOrchestrator) GetConnection(connectionIDStr string) (Connection, bool) {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	connectionID := ConnectionID(connectionIDStr)
	conn, exists := bro.connections[connectionID]
	return conn, exists
}

// GetRoomConnections returns all connections for a specific room
func (bro *BasicRoomOrchestrator) GetRoomConnections(roomIDStr string) []Connection {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	connections := make([]Connection, 0)
	for _, conn := range bro.connections {
		if conn.GetFromRoom() == roomIDStr || (conn.IsReversible() && conn.GetToRoom() == roomIDStr) {
			connections = append(connections, conn)
		}
	}
	return connections
}

// GetAllConnections returns all connections
func (bro *BasicRoomOrchestrator) GetAllConnections() map[string]Connection {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	// Return a copy to prevent modification
	result := make(map[string]Connection)
	for id, conn := range bro.connections {
		result[id.String()] = conn
	}
	return result
}

// MoveEntityBetweenRooms preserves the legacy signature for source
// compatibility. It performs a logical transition only: the entity is removed
// from the source and remains unplaced and unindexed until PlaceEntity chooses
// a destination position. New compositions should call TransitionEntity and
// consume its explicit output.
func (bro *BasicRoomOrchestrator) MoveEntityBetweenRooms(
	entityIDStr, fromRoomStr, toRoomStr, connectionIDStr string,
) error {
	_, err := bro.TransitionEntity(&TransitionEntityInput{
		EntityID:     core.EntityID(entityIDStr),
		FromRoom:     RoomID(fromRoomStr),
		ToRoom:       RoomID(toRoomStr),
		ConnectionID: ConnectionID(connectionIDStr),
	})
	return err
}

// CanMoveEntityBetweenRooms checks whether an indexed, physically present
// entity can depart through the named connection.
func (bro *BasicRoomOrchestrator) CanMoveEntityBetweenRooms(
	entityIDStr, fromRoomStr, toRoomStr, connectionIDStr string,
) bool {
	_, _, _, _, ok := bro.transitionSource(
		core.EntityID(entityIDStr),
		RoomID(fromRoomStr),
		RoomID(toRoomStr),
		ConnectionID(connectionIDStr),
	)
	return ok
}

// GetEntityRoom returns which room contains the entity
func (bro *BasicRoomOrchestrator) GetEntityRoom(entityIDStr string) (string, bool) {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	entityID := core.EntityID(entityIDStr)
	roomID, exists := bro.entityRooms[entityID]
	return roomID.String(), exists
}

// FindPath finds a path between rooms using connections (simple implementation)
func (bro *BasicRoomOrchestrator) FindPath(fromRoom, toRoom string, entity core.Entity) ([]string, error) {
	if fromRoom == toRoom {
		return []string{fromRoom}, nil
	}

	// Simple breadth-first search
	visited := make(map[string]bool)
	queue := [][]string{{fromRoom}}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		currentRoom := path[len(path)-1]
		if visited[currentRoom] {
			continue
		}
		visited[currentRoom] = true

		if currentRoom == toRoom {
			return path, nil
		}

		// Find connections from current room
		connections := bro.GetRoomConnections(currentRoom)
		for _, conn := range connections {
			if !conn.IsPassable(entity) {
				continue
			}

			var nextRoom string
			switch {
			case conn.GetFromRoom() == currentRoom:
				nextRoom = conn.GetToRoom()
			case conn.IsReversible():
				nextRoom = conn.GetFromRoom()
			default:
				continue
			}

			if !visited[nextRoom] {
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = nextRoom
				queue = append(queue, newPath)
			}
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", fromRoom, toRoom)
}

// GetLayout returns the current layout pattern.
func (bro *BasicRoomOrchestrator) GetLayout() LayoutType {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return bro.layout
}

// SetLayout configures the arrangement pattern.
func (bro *BasicRoomOrchestrator) SetLayout(layout LayoutType) error {
	bro.mu.Lock()
	oldLayout := bro.layout
	bro.layout = layout
	topic := bro.layoutChanged
	orchestratorID := bro.id.String()
	bro.mu.Unlock()

	if topic != nil {
		_ = topic.Publish(context.Background(), LayoutChangedEvent{
			OrchestratorID: orchestratorID,
			OldLayout:      string(oldLayout),
			NewLayout:      string(layout),
			ChangedAt:      time.Now(),
		})
	}
	return nil
}
