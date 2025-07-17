package spatial

import (
	"context"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicRoomOrchestrator implements the RoomOrchestrator interface
type BasicRoomOrchestrator struct {
	mu            sync.RWMutex
	id            OrchestratorID
	entityType    string
	eventBus      events.EventBus
	rooms         map[RoomID]Room
	connections   map[ConnectionID]Connection
	entityRooms   map[EntityID]RoomID // entityID -> roomID mapping
	layout        LayoutType
	subscriptions []string // Track event subscription IDs
}

// BasicRoomOrchestratorConfig holds configuration for creating a basic room orchestrator
type BasicRoomOrchestratorConfig struct {
	ID       OrchestratorID // Optional: if empty, will auto-generate
	Type     string
	EventBus events.EventBus
	Layout   LayoutType
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
		eventBus:      config.EventBus,
		rooms:         make(map[RoomID]Room),
		connections:   make(map[ConnectionID]Connection),
		entityRooms:   make(map[EntityID]RoomID),
		layout:        layout,
		subscriptions: make([]string, 0),
	}
}

// GetID returns the orchestrator ID
func (bro *BasicRoomOrchestrator) GetID() string {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return bro.id.String()
}

// GetType returns the entity type
func (bro *BasicRoomOrchestrator) GetType() string {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return bro.entityType
}

// SetEventBus sets the event bus for the orchestrator
func (bro *BasicRoomOrchestrator) SetEventBus(bus events.EventBus) {
	bro.mu.Lock()
	defer bro.mu.Unlock()
	bro.eventBus = bus
}

// GetEventBus returns the current event bus
func (bro *BasicRoomOrchestrator) GetEventBus() events.EventBus {
	bro.mu.RLock()
	defer bro.mu.RUnlock()
	return bro.eventBus
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

	// Subscribe to room events to track entity movements (only once)
	if bro.eventBus != nil && len(bro.subscriptions) == 0 {
		// Subscribe to entity placement events for this room
		sub1 := bro.eventBus.SubscribeFunc(EventEntityPlaced, 0, bro.handleEntityPlaced)
		sub2 := bro.eventBus.SubscribeFunc(EventEntityMoved, 0, bro.handleEntityMoved)
		sub3 := bro.eventBus.SubscribeFunc(EventEntityRemoved, 0, bro.handleEntityRemoved)
		bro.subscriptions = append(bro.subscriptions, sub1, sub2, sub3)
	}

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventRoomAdded, bro, nil)
		event.Context().Set("orchestrator_id", bro.id.String())
		event.Context().Set("room", room)
		_ = bro.eventBus.Publish(context.Background(), event)
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
	if len(bro.rooms) == 0 && bro.eventBus != nil {
		for _, subID := range bro.subscriptions {
			_ = bro.eventBus.Unsubscribe(subID)
		}
		bro.subscriptions = nil
	}

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventRoomRemoved, bro, nil)
		event.Context().Set("orchestrator_id", bro.id.String())
		event.Context().Set("room_id", roomIDStr)
		_ = bro.eventBus.Publish(context.Background(), event)
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

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventConnectionAdded, bro, nil)
		event.Context().Set("orchestrator_id", bro.id.String())
		event.Context().Set("connection", connection)
		_ = bro.eventBus.Publish(context.Background(), event)
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

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventConnectionRemoved, bro, nil)
		event.Context().Set("orchestrator_id", bro.id.String())
		event.Context().Set("connection_id", connectionIDStr)
		_ = bro.eventBus.Publish(context.Background(), event)
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

// MoveEntityBetweenRooms moves an entity from one room to another
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

	toRoomObj, exists := bro.rooms[toRoom]
	if !exists {
		return fmt.Errorf("to room %s not found", toRoom)
	}

	// Get the connection
	connection, exists := bro.connections[connectionID]
	if !exists {
		return fmt.Errorf("connection %s not found", connectionID)
	}

	// Get entity position in from room
	entity, exists := fromRoomObj.GetAllEntities()[entityIDStr]
	if !exists {
		return fmt.Errorf("entity %s not found in room %s", entityID, fromRoom)
	}

	// Remove from source room
	err := fromRoomObj.RemoveEntity(entityIDStr)
	if err != nil {
		return fmt.Errorf("failed to remove entity from source room: %w", err)
	}

	// Place in destination room at connection position
	var targetPosition Position
	if connection.GetFromRoom() == fromRoomStr {
		targetPosition = connection.GetToPosition()
	} else {
		targetPosition = connection.GetFromPosition()
	}

	err = toRoomObj.PlaceEntity(entity, targetPosition)
	if err != nil {
		// Try to restore entity to original room
		_ = fromRoomObj.PlaceEntity(entity, connection.GetFromPosition())
		return fmt.Errorf("failed to place entity in destination room: %w", err)
	}

	// Update entity room mapping
	bro.entityRooms[entityID] = toRoom

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

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventLayoutChanged, bro, nil)
		event.Context().Set("orchestrator_id", bro.id)
		event.Context().Set("old_layout", oldLayout)
		event.Context().Set("new_layout", layout)
		event.Context().Set("metrics", bro.calculateMetrics())
		_ = bro.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// calculateMetrics calculates layout metrics
func (bro *BasicRoomOrchestrator) calculateMetrics() LayoutMetrics {
	totalRooms := len(bro.rooms)
	totalConnections := len(bro.connections)

	var connectivity float64
	if totalRooms > 0 {
		connectivity = float64(totalConnections) / float64(totalRooms)
	}

	return LayoutMetrics{
		TotalRooms:       totalRooms,
		TotalConnections: totalConnections,
		AverageDistance:  0.0, // TODO: Implement distance calculation
		MaxDistance:      0.0, // TODO: Implement distance calculation
		Connectivity:     connectivity,
		RoomPositions:    make(map[string]Position), // TODO: Implement room positioning
		LayoutType:       bro.layout,
	}
}

// handleEntityPlaced handles entity placement events to track entity locations
func (bro *BasicRoomOrchestrator) handleEntityPlaced(_ context.Context, event events.Event) error {
	roomID, ok := event.Context().Get("room_id")
	if !ok {
		return nil
	}

	roomIDStr, ok := roomID.(string)
	if !ok {
		return nil
	}

	// Only track if this room is managed by this orchestrator
	roomTypedID := RoomID(roomIDStr)
	if _, exists := bro.rooms[roomTypedID]; !exists {
		return nil
	}

	if event.Source() != nil {
		entityID := EntityID(event.Source().GetID())
		bro.entityRooms[entityID] = roomTypedID
	}

	return nil
}

// handleEntityMoved handles entity movement events to update tracking
func (bro *BasicRoomOrchestrator) handleEntityMoved(_ context.Context, event events.Event) error {
	roomID, ok := event.Context().Get("room_id")
	if !ok {
		return nil
	}

	roomIDStr, ok := roomID.(string)
	if !ok {
		return nil
	}

	// Only track if this room is managed by this orchestrator
	roomTypedID := RoomID(roomIDStr)
	if _, exists := bro.rooms[roomTypedID]; !exists {
		return nil
	}

	if event.Source() != nil {
		entityID := EntityID(event.Source().GetID())
		bro.entityRooms[entityID] = roomTypedID
	}

	return nil
}

// handleEntityRemoved handles entity removal events to update tracking
func (bro *BasicRoomOrchestrator) handleEntityRemoved(_ context.Context, event events.Event) error {
	roomID, ok := event.Context().Get("room_id")
	if !ok {
		return nil
	}

	roomIDStr, ok := roomID.(string)
	if !ok {
		return nil
	}

	// Only track if this room is managed by this orchestrator
	roomTypedID := RoomID(roomIDStr)
	if _, exists := bro.rooms[roomTypedID]; !exists {
		return nil
	}

	if event.Source() != nil {
		entityID := EntityID(event.Source().GetID())
		delete(bro.entityRooms, entityID)
	}

	return nil
}
