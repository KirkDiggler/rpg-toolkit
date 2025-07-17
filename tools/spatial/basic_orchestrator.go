package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// BasicRoomOrchestrator implements the RoomOrchestrator interface
type BasicRoomOrchestrator struct {
	id          string
	entityType  string
	eventBus    events.EventBus
	rooms       map[string]Room
	connections map[string]Connection
	entityRooms map[string]string // entityID -> roomID mapping
	layout      LayoutType
}

// BasicRoomOrchestratorConfig holds configuration for creating a basic room orchestrator
type BasicRoomOrchestratorConfig struct {
	ID       string
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

	return &BasicRoomOrchestrator{
		id:          config.ID,
		entityType:  config.Type,
		eventBus:    config.EventBus,
		rooms:       make(map[string]Room),
		connections: make(map[string]Connection),
		entityRooms: make(map[string]string),
		layout:      layout,
	}
}

// GetID returns the orchestrator ID
func (bro *BasicRoomOrchestrator) GetID() string {
	return bro.id
}

// GetType returns the entity type
func (bro *BasicRoomOrchestrator) GetType() string {
	return bro.entityType
}

// SetEventBus sets the event bus for the orchestrator
func (bro *BasicRoomOrchestrator) SetEventBus(bus events.EventBus) {
	bro.eventBus = bus
}

// GetEventBus returns the current event bus
func (bro *BasicRoomOrchestrator) GetEventBus() events.EventBus {
	return bro.eventBus
}

// AddRoom adds a room to the orchestrator
func (bro *BasicRoomOrchestrator) AddRoom(room Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	roomID := room.GetID()
	if _, exists := bro.rooms[roomID]; exists {
		return fmt.Errorf("room %s already exists", roomID)
	}

	bro.rooms[roomID] = room

	// Track all entities currently in this room
	entities := room.GetAllEntities()
	for entityID := range entities {
		bro.entityRooms[entityID] = roomID
	}

	// Subscribe to room events to track entity movements
	if bro.eventBus != nil {
		// Subscribe to entity placement events for this room
		bro.eventBus.SubscribeFunc(EventEntityPlaced, 0, bro.handleEntityPlaced)
		bro.eventBus.SubscribeFunc(EventEntityMoved, 0, bro.handleEntityMoved)
		bro.eventBus.SubscribeFunc(EventEntityRemoved, 0, bro.handleEntityRemoved)
	}

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventRoomAdded, bro, nil)
		event.Context().Set("orchestrator_id", bro.id)
		event.Context().Set("room", room)
		_ = bro.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// RemoveRoom removes a room from the orchestrator
func (bro *BasicRoomOrchestrator) RemoveRoom(roomID string) error {
	_, exists := bro.rooms[roomID]
	if !exists {
		return fmt.Errorf("room %s not found", roomID)
	}

	// Remove all entity mappings for this room
	for entityID, mappedRoomID := range bro.entityRooms {
		if mappedRoomID == roomID {
			delete(bro.entityRooms, entityID)
		}
	}

	// Remove connections that reference this room
	toRemove := make([]string, 0)
	for connID, conn := range bro.connections {
		if conn.GetFromRoom() == roomID || conn.GetToRoom() == roomID {
			toRemove = append(toRemove, connID)
		}
	}
	for _, connID := range toRemove {
		delete(bro.connections, connID)
	}

	delete(bro.rooms, roomID)

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventRoomRemoved, bro, nil)
		event.Context().Set("orchestrator_id", bro.id)
		event.Context().Set("room_id", roomID)
		_ = bro.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// GetRoom retrieves a room by ID
func (bro *BasicRoomOrchestrator) GetRoom(roomID string) (Room, bool) {
	room, exists := bro.rooms[roomID]
	return room, exists
}

// GetAllRooms returns all managed rooms
func (bro *BasicRoomOrchestrator) GetAllRooms() map[string]Room {
	// Return a copy to prevent modification
	result := make(map[string]Room)
	for id, room := range bro.rooms {
		result[id] = room
	}
	return result
}

// AddConnection creates a connection between two rooms
func (bro *BasicRoomOrchestrator) AddConnection(connection Connection) error {
	if connection == nil {
		return fmt.Errorf("connection cannot be nil")
	}

	connID := connection.GetID()
	if _, exists := bro.connections[connID]; exists {
		return fmt.Errorf("connection %s already exists", connID)
	}

	// Validate that both rooms exist
	fromRoom := connection.GetFromRoom()
	toRoom := connection.GetToRoom()

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
		event.Context().Set("orchestrator_id", bro.id)
		event.Context().Set("connection", connection)
		_ = bro.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// RemoveConnection removes a connection
func (bro *BasicRoomOrchestrator) RemoveConnection(connectionID string) error {
	if _, exists := bro.connections[connectionID]; !exists {
		return fmt.Errorf("connection %s not found", connectionID)
	}

	delete(bro.connections, connectionID)

	// Emit event
	if bro.eventBus != nil {
		event := events.NewGameEvent(EventConnectionRemoved, bro, nil)
		event.Context().Set("orchestrator_id", bro.id)
		event.Context().Set("connection_id", connectionID)
		_ = bro.eventBus.Publish(context.Background(), event)
	}

	return nil
}

// GetConnection retrieves a connection by ID
func (bro *BasicRoomOrchestrator) GetConnection(connectionID string) (Connection, bool) {
	conn, exists := bro.connections[connectionID]
	return conn, exists
}

// GetRoomConnections returns all connections for a specific room
func (bro *BasicRoomOrchestrator) GetRoomConnections(roomID string) []Connection {
	connections := make([]Connection, 0)
	for _, conn := range bro.connections {
		if conn.GetFromRoom() == roomID || (conn.IsReversible() && conn.GetToRoom() == roomID) {
			connections = append(connections, conn)
		}
	}
	return connections
}

// GetAllConnections returns all connections
func (bro *BasicRoomOrchestrator) GetAllConnections() map[string]Connection {
	// Return a copy to prevent modification
	result := make(map[string]Connection)
	for id, conn := range bro.connections {
		result[id] = conn
	}
	return result
}

// MoveEntityBetweenRooms moves an entity from one room to another
func (bro *BasicRoomOrchestrator) MoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) error {
	if !bro.CanMoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID) {
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
	entity, exists := fromRoomObj.GetAllEntities()[entityID]
	if !exists {
		return fmt.Errorf("entity %s not found in room %s", entityID, fromRoom)
	}

	// Remove from source room
	err := fromRoomObj.RemoveEntity(entityID)
	if err != nil {
		return fmt.Errorf("failed to remove entity from source room: %w", err)
	}

	// Place in destination room at connection position
	var targetPosition Position
	if connection.GetFromRoom() == fromRoom {
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
func (bro *BasicRoomOrchestrator) CanMoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) bool {
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
	forwardDirection := connection.GetFromRoom() == fromRoom && connection.GetToRoom() == toRoom
	reverseDirection := connection.IsReversible() && connection.GetFromRoom() == toRoom && connection.GetToRoom() == fromRoom
	
	if !forwardDirection && !reverseDirection {
		return false
	}

	// Check if connection is passable
	fromRoomObj, exists := bro.rooms[fromRoom]
	if !exists {
		return false
	}

	entities := fromRoomObj.GetAllEntities()
	entity, exists := entities[entityID]
	if !exists {
		return false
	}

	return connection.IsPassable(entity)
}

// GetEntityRoom returns which room contains the entity
func (bro *BasicRoomOrchestrator) GetEntityRoom(entityID string) (string, bool) {
	roomID, exists := bro.entityRooms[entityID]
	return roomID, exists
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
	if _, exists := bro.rooms[roomIDStr]; !exists {
		return nil
	}

	if event.Source() != nil {
		bro.entityRooms[event.Source().GetID()] = roomIDStr
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
	if _, exists := bro.rooms[roomIDStr]; !exists {
		return nil
	}

	if event.Source() != nil {
		bro.entityRooms[event.Source().GetID()] = roomIDStr
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
	if _, exists := bro.rooms[roomIDStr]; !exists {
		return nil
	}

	if event.Source() != nil {
		delete(bro.entityRooms, event.Source().GetID())
	}

	return nil
}
