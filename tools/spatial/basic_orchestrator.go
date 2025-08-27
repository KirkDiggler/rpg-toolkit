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
	roomAdded             events.TypedTopic[RoomAddedEvent]
	roomRemoved           events.TypedTopic[RoomRemovedEvent]
	connectionAdded       events.TypedTopic[ConnectionAddedEvent]
	connectionRemoved     events.TypedTopic[ConnectionRemovedEvent]
	entityTransitionBegan events.TypedTopic[EntityTransitionBeganEvent]
	entityTransitionEnded events.TypedTopic[EntityTransitionEndedEvent]
	entityRoomTransition  events.TypedTopic[EntityRoomTransitionEvent]
	layoutChanged         events.TypedTopic[LayoutChangedEvent]

	// Entity event subscriptions
	entityPlacements events.TypedTopic[EntityPlacedEvent]
	entityMovements  events.TypedTopic[EntityMovedEvent]
	entityRemovals   events.TypedTopic[EntityRemovedEvent]

	rooms         map[RoomID]Room
	connections   map[ConnectionID]Connection
	entityRooms   map[EntityID]RoomID // entityID -> roomID mapping
	layout        LayoutType
	subscriptions []string // Track event subscription IDs for entity events
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
		id:            id,
		entityType:    config.Type,
		rooms:         make(map[RoomID]Room),
		connections:   make(map[ConnectionID]Connection),
		entityRooms:   make(map[EntityID]RoomID),
		layout:        layout,
		subscriptions: make([]string, 0),
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
	bro.entityTransitionBegan = EntityTransitionBeganTopic.On(bus)
	bro.entityTransitionEnded = EntityTransitionEndedTopic.On(bus)
	bro.entityRoomTransition = EntityRoomTransitionTopic.On(bus)
	bro.layoutChanged = LayoutChangedTopic.On(bus)

	// Connect entity event subscriptions
	bro.entityPlacements = EntityPlacedTopic.On(bus)
	bro.entityMovements = EntityMovedTopic.On(bus)
	bro.entityRemovals = EntityRemovedTopic.On(bus)
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

// AddRoom adds a room to the orchestrator
func (bro *BasicRoomOrchestrator) AddRoom(room Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	bro.mu.Lock()
	defer bro.mu.Unlock()

	roomID := RoomID(room.GetID())
	if _, exists := bro.rooms[roomID]; exists {
		return fmt.Errorf("room %s already exists", roomID)
	}

	bro.rooms[roomID] = room

	// Track all entities currently in this room
	entities := room.GetAllEntities()
	for entityIDStr := range entities {
		entityID := EntityID(entityIDStr)
		bro.entityRooms[entityID] = roomID
	}

	// Subscribe to entity events using typed subscriptions (only once)
	if len(bro.subscriptions) == 0 && bro.entityPlacements != nil {
		ctx := context.Background()
		sub1, _ := bro.entityPlacements.Subscribe(ctx, bro.handleEntityPlacedTyped)
		sub2, _ := bro.entityMovements.Subscribe(ctx, bro.handleEntityMovedTyped)
		sub3, _ := bro.entityRemovals.Subscribe(ctx, bro.handleEntityRemovedTyped)
		bro.subscriptions = append(bro.subscriptions, sub1, sub2, sub3)
	}

	// Emit typed event
	if bro.roomAdded != nil {
		event := RoomAddedEvent{
			OrchestratorID: bro.id.String(),
			RoomID:         room.GetID(),
			RoomType:       string(room.GetType()),
			AddedAt:        time.Now(),
		}
		_ = bro.roomAdded.Publish(context.Background(), event)
	}

	return nil
}

// RemoveRoom removes a room from the orchestrator
func (bro *BasicRoomOrchestrator) RemoveRoom(roomIDStr string) error {
	bro.mu.Lock()
	defer bro.mu.Unlock()

	roomID := RoomID(roomIDStr)
	_, exists := bro.rooms[roomID]
	if !exists {
		return fmt.Errorf("room %s not found", roomID)
	}

	// Remove all entity mappings for this room - collect keys first
	toRemoveEntities := make([]EntityID, 0)
	for entityID, mappedRoomID := range bro.entityRooms {
		if mappedRoomID == roomID {
			toRemoveEntities = append(toRemoveEntities, entityID)
		}
	}
	for _, entityID := range toRemoveEntities {
		delete(bro.entityRooms, entityID)
	}

	// Remove connections that reference this room - collect keys first
	toRemoveConnections := make([]ConnectionID, 0)
	for connID, conn := range bro.connections {
		if conn.GetFromRoom() == roomIDStr || conn.GetToRoom() == roomIDStr {
			toRemoveConnections = append(toRemoveConnections, connID)
		}
	}
	for _, connID := range toRemoveConnections {
		delete(bro.connections, connID)
	}

	delete(bro.rooms, roomID)

	// Clean up subscriptions when last room is removed
	if len(bro.rooms) == 0 && bro.entityPlacements != nil {
		ctx := context.Background()
		for _, subID := range bro.subscriptions {
			_ = bro.entityPlacements.Unsubscribe(ctx, subID)
		}
		bro.subscriptions = nil
	}

	// Emit typed event
	if bro.roomRemoved != nil {
		event := RoomRemovedEvent{
			OrchestratorID: bro.id.String(),
			RoomID:         roomIDStr,
			Reason:         "removed",
			RemovedAt:      time.Now(),
		}
		_ = bro.roomRemoved.Publish(context.Background(), event)
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

// AddConnection creates a connection between two rooms
func (bro *BasicRoomOrchestrator) AddConnection(connection Connection) error {
	if connection == nil {
		return fmt.Errorf("connection cannot be nil")
	}

	bro.mu.Lock()
	defer bro.mu.Unlock()

	connID := ConnectionID(connection.GetID())
	if _, exists := bro.connections[connID]; exists {
		return fmt.Errorf("connection %s already exists", connID)
	}

	// Validate that both rooms exist
	fromRoom := RoomID(connection.GetFromRoom())
	toRoom := RoomID(connection.GetToRoom())

	if _, exists := bro.rooms[fromRoom]; !exists {
		return fmt.Errorf("from room %s does not exist", fromRoom)
	}

	if _, exists := bro.rooms[toRoom]; !exists {
		return fmt.Errorf("to room %s does not exist", toRoom)
	}

	bro.connections[connID] = connection

	// Emit typed event
	if bro.connectionAdded != nil {
		event := ConnectionAddedEvent{
			OrchestratorID: bro.id.String(),
			ConnectionID:   connection.GetID(),
			FromRoom:       connection.GetFromRoom(),
			ToRoom:         connection.GetToRoom(),
			ConnectionType: string(connection.GetConnectionType()),
			AddedAt:        time.Now(),
		}
		_ = bro.connectionAdded.Publish(context.Background(), event)
	}

	return nil
}

// RemoveConnection removes a connection
func (bro *BasicRoomOrchestrator) RemoveConnection(connectionIDStr string) error {
	bro.mu.Lock()
	defer bro.mu.Unlock()

	connectionID := ConnectionID(connectionIDStr)
	if _, exists := bro.connections[connectionID]; !exists {
		return fmt.Errorf("connection %s not found", connectionID)
	}

	delete(bro.connections, connectionID)

	// Emit typed event
	if bro.connectionRemoved != nil {
		event := ConnectionRemovedEvent{
			OrchestratorID: bro.id.String(),
			ConnectionID:   connectionIDStr,
			Reason:         "removed",
			RemovedAt:      time.Now(),
		}
		_ = bro.connectionRemoved.Publish(context.Background(), event)
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

// MoveEntityBetweenRooms moves an entity from one room to another (ADR-0015: Abstract Connections)
func (bro *BasicRoomOrchestrator) MoveEntityBetweenRooms(
	entityIDStr, fromRoomStr, toRoomStr, connectionIDStr string,
) error {
	bro.mu.Lock()
	defer bro.mu.Unlock()

	entityID := EntityID(entityIDStr)
	fromRoom := RoomID(fromRoomStr)
	toRoom := RoomID(toRoomStr)
	connectionID := ConnectionID(connectionIDStr)

	if !bro.canMoveEntityBetweenRoomsUnsafe(entityID, fromRoom, toRoom, connectionID) {
		return fmt.Errorf("cannot move entity %s from %s to %s via %s", entityID, fromRoom, toRoom, connectionID)
	}

	// Get the rooms
	fromRoomObj, exists := bro.rooms[fromRoom]
	if !exists {
		return fmt.Errorf("from room %s not found", fromRoom)
	}

	// Verify entity exists in source room
	entities := fromRoomObj.GetAllEntities()
	if _, exists := entities[entityIDStr]; !exists {
		return fmt.Errorf("entity %s not found in room %s", entityID, fromRoom)
	}

	// Remove from source room
	err := fromRoomObj.RemoveEntity(entityIDStr)
	if err != nil {
		return fmt.Errorf("failed to remove entity from source room: %w", err)
	}

	// Update entity room mapping
	bro.entityRooms[entityID] = toRoom

	// Emit typed transition event for game to handle positioning (ADR-0015)
	if bro.entityRoomTransition != nil {
		event := EntityRoomTransitionEvent{
			EntityID:  entityIDStr,
			FromRoom:  fromRoomStr,
			ToRoom:    toRoomStr,
			Reason:    fmt.Sprintf("connection:%s", connectionIDStr),
			Timestamp: time.Now(),
		}
		_ = bro.entityRoomTransition.Publish(context.Background(), event)
	}

	return nil
}

// CanMoveEntityBetweenRooms checks if entity movement is possible
func (bro *BasicRoomOrchestrator) CanMoveEntityBetweenRooms(
	entityIDStr, fromRoomStr, toRoomStr, connectionIDStr string,
) bool {
	bro.mu.RLock()
	defer bro.mu.RUnlock()

	entityID := EntityID(entityIDStr)
	fromRoom := RoomID(fromRoomStr)
	toRoom := RoomID(toRoomStr)
	connectionID := ConnectionID(connectionIDStr)

	return bro.canMoveEntityBetweenRoomsUnsafe(entityID, fromRoom, toRoom, connectionID)
}

// canMoveEntityBetweenRoomsUnsafe is the internal version that doesn't acquire locks
func (bro *BasicRoomOrchestrator) canMoveEntityBetweenRoomsUnsafe(
	entityID EntityID, fromRoom, toRoom RoomID, connectionID ConnectionID,
) bool {
	// Check if entity is in the from room
	if currentRoom, exists := bro.entityRooms[entityID]; !exists || currentRoom != fromRoom {
		return false
	}

	// Check if connection exists
	connection, exists := bro.connections[connectionID]
	if !exists {
		return false
	}

	// Check if connection links the specified rooms (forward or reverse direction)
	forwardDirection := connection.GetFromRoom() == fromRoom.String() && connection.GetToRoom() == toRoom.String()
	reverseDirection := connection.IsReversible() && connection.GetFromRoom() == toRoom.String() &&
		connection.GetToRoom() == fromRoom.String()

	if !forwardDirection && !reverseDirection {
		return false
	}

	// Check if connection is passable
	fromRoomObj, exists := bro.rooms[fromRoom]
	if !exists {
		return false
	}

	entities := fromRoomObj.GetAllEntities()
	entity, exists := entities[entityID.String()]
	if !exists {
		return false
	}

	return connection.IsPassable(entity)
}

// GetEntityRoom returns which room contains the entity
func (bro *BasicRoomOrchestrator) GetEntityRoom(entityIDStr string) (string, bool) {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	entityID := EntityID(entityIDStr)
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

// GetLayout returns the current layout pattern
func (bro *BasicRoomOrchestrator) GetLayout() LayoutType {
	return bro.layout
}

// SetLayout configures the arrangement pattern
func (bro *BasicRoomOrchestrator) SetLayout(layout LayoutType) error {
	oldLayout := bro.layout
	bro.layout = layout

	// Emit typed event
	if bro.layoutChanged != nil {
		event := LayoutChangedEvent{
			OrchestratorID: bro.id.String(),
			OldLayout:      string(oldLayout),
			NewLayout:      string(layout),
			ChangedAt:      time.Now(),
		}
		_ = bro.layoutChanged.Publish(context.Background(), event)
	}

	return nil
}

// handleEntityPlacedTyped handles typed entity placement events to track entity locations
func (bro *BasicRoomOrchestrator) handleEntityPlacedTyped(_ context.Context, event EntityPlacedEvent) error {
	// Only track if this room is managed by this orchestrator
	roomTypedID := RoomID(event.RoomID)
	if _, exists := bro.rooms[roomTypedID]; !exists {
		return nil
	}

	entityID := EntityID(event.EntityID)
	bro.entityRooms[entityID] = roomTypedID

	return nil
}

// handleEntityMovedTyped handles typed entity movement events to update tracking
func (bro *BasicRoomOrchestrator) handleEntityMovedTyped(_ context.Context, event EntityMovedEvent) error {
	// Only track if this room is managed by this orchestrator
	roomTypedID := RoomID(event.RoomID)
	if _, exists := bro.rooms[roomTypedID]; !exists {
		return nil
	}

	entityID := EntityID(event.EntityID)
	bro.entityRooms[entityID] = roomTypedID

	return nil
}

// handleEntityRemovedTyped handles typed entity removal events to update tracking
func (bro *BasicRoomOrchestrator) handleEntityRemovedTyped(_ context.Context, event EntityRemovedEvent) error {
	// Only track if this room is managed by this orchestrator
	roomTypedID := RoomID(event.RoomID)
	if _, exists := bro.rooms[roomTypedID]; !exists {
		return nil
	}

	entityID := EntityID(event.EntityID)
	delete(bro.entityRooms, entityID)

	return nil
}
