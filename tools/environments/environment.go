// Package environments provides procedural generation of game environments using graph algorithms.
// This package creates environments by building abstract graphs of rooms and connections,
// then placing them spatially while integrating with the spatial module for obstacle detection.
package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BasicEnvironment is the core implementation of the Environment interface
// Purpose: Wraps a spatial orchestrator with environment-specific functionality
// like metadata, queries, and event integration. Acts as the primary client
// interface to the spatial infrastructure.
type BasicEnvironment struct {
	// Core identity following toolkit patterns
	id  string
	typ string

	// Spatial infrastructure - we are a CLIENT of spatial, not replacement
	orchestrator spatial.RoomOrchestrator

	// Environment-specific data
	metadata EnvironmentMetadata
	theme    string

	// Unified coordinate system - room origins in dungeon-absolute coordinates
	// Enables GetBlockedHexes, GetRoomAt, and GetRoomBounds methods
	roomPositions map[string]spatial.CubeCoordinate

	// Cached blocked hexes in dungeon-absolute coordinates
	// Calculated during generation from wall placements
	blockedHexes map[spatial.CubeCoordinate]bool

	// Event integration following toolkit patterns - typed topics
	environmentEntityAddedTopic     events.TypedTopic[EnvironmentEntityAddedEvent]
	environmentEntityMovedTopic     events.TypedTopic[EnvironmentEntityMovedEvent]
	environmentRoomAddedTopic       events.TypedTopic[EnvironmentRoomAddedEvent]
	themeChangedTopic               events.TypedTopic[ThemeChangedEvent]
	environmentMetadataChangedTopic events.TypedTopic[EnvironmentMetadataChangedEvent]

	// Query delegation - delegates to spatial, adds environment-level aggregation
	queryHandler QueryHandler

	// Thread safety following toolkit patterns
	mutex sync.RWMutex

	// Subscription management for cleanup
	subscriptions []string
}

// BasicEnvironmentConfig follows the toolkit's config pattern
// Purpose: Provides clean dependency injection and configuration
type BasicEnvironmentConfig struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Theme    string              `json:"theme"`
	Metadata EnvironmentMetadata `json:"metadata"`
	// EventBus removed - ConnectToEventBus pattern used instead
	Orchestrator spatial.RoomOrchestrator `json:"-"` // Not serializable
	QueryHandler QueryHandler             `json:"-"` // Not serializable
	// RoomPositions maps room IDs to their origin in dungeon-absolute coordinates
	// Enables unified coordinate system across the entire environment
	RoomPositions map[string]spatial.CubeCoordinate `json:"-"` // Not serializable
	// BlockedHexes contains all blocked positions in dungeon-absolute coordinates
	// Calculated during generation from wall placements
	BlockedHexes map[spatial.CubeCoordinate]bool `json:"-"` // Not serializable
}

// NewBasicEnvironment creates a new BasicEnvironment following toolkit patterns
// Purpose: Standard constructor with config struct, proper initialization
func NewBasicEnvironment(config BasicEnvironmentConfig) *BasicEnvironment {
	roomPositions := config.RoomPositions
	if roomPositions == nil {
		roomPositions = make(map[string]spatial.CubeCoordinate)
	}

	blockedHexes := config.BlockedHexes
	if blockedHexes == nil {
		blockedHexes = make(map[spatial.CubeCoordinate]bool)
	}

	env := &BasicEnvironment{
		id:            config.ID,
		typ:           config.Type,
		theme:         config.Theme,
		metadata:      config.Metadata,
		orchestrator:  config.Orchestrator,
		queryHandler:  config.QueryHandler,
		roomPositions: roomPositions,
		blockedHexes:  blockedHexes,
		subscriptions: make([]string, 0),
	}

	// Note: Event subscriptions handled via ConnectToEventBus method

	return env
}

// GetID returns the unique identifier for this environment
func (e *BasicEnvironment) GetID() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.id
}

// GetType returns the type of this environment
func (e *BasicEnvironment) GetType() core.EntityType {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return core.EntityType(e.typ)
}

// Environment interface implementation - delegates to spatial infrastructure

// GetOrchestrator returns the room orchestrator managing spatial layout
func (e *BasicEnvironment) GetOrchestrator() spatial.RoomOrchestrator {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.orchestrator
}

// GetRooms returns all rooms in this environment
func (e *BasicEnvironment) GetRooms() []spatial.Room {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to spatial orchestrator - we don't duplicate this logic
	roomMap := e.orchestrator.GetAllRooms()
	rooms := make([]spatial.Room, 0, len(roomMap))
	for _, room := range roomMap {
		rooms = append(rooms, room)
	}
	return rooms
}

// GetRoom returns a specific room by ID
func (e *BasicEnvironment) GetRoom(roomID string) (spatial.Room, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to spatial orchestrator
	return e.orchestrator.GetRoom(roomID)
}

// GetConnections returns all connections in this environment
func (e *BasicEnvironment) GetConnections() []spatial.Connection {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to spatial orchestrator
	connMap := e.orchestrator.GetAllConnections()
	connections := make([]spatial.Connection, 0, len(connMap))
	for _, conn := range connMap {
		connections = append(connections, conn)
	}
	return connections
}

// GetConnection returns a specific connection by ID
func (e *BasicEnvironment) GetConnection(connectionID string) (spatial.Connection, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to spatial orchestrator
	return e.orchestrator.GetConnection(connectionID)
}

// GetTheme returns the theme of this environment
func (e *BasicEnvironment) GetTheme() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.theme
}

// GetMetadata returns metadata about this environment
func (e *BasicEnvironment) GetMetadata() EnvironmentMetadata {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.metadata
}

// Environment-specific functionality - this is where we add value beyond spatial

// QueryEntities searches for entities within the environment based on the provided query criteria.
func (e *BasicEnvironment) QueryEntities(ctx context.Context, query EntityQuery) ([]core.Entity, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to our query handler which will aggregate across rooms
	if e.queryHandler == nil {
		return nil, fmt.Errorf("no query handler configured")
	}

	return e.queryHandler.HandleEntityQuery(ctx, query)
}

// QueryRooms searches for rooms within the environment based on the provided query criteria.
func (e *BasicEnvironment) QueryRooms(ctx context.Context, query RoomQuery) ([]spatial.Room, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to our query handler for environment-level room queries
	if e.queryHandler == nil {
		return nil, fmt.Errorf("no query handler configured")
	}

	return e.queryHandler.HandleRoomQuery(ctx, query)
}

// FindPath finds a path between two positions within the environment.
func (e *BasicEnvironment) FindPath(_ spatial.Position, _ spatial.Position) ([]spatial.Position, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// For now, delegate to spatial orchestrator pathfinding
	// Later we can enhance with environment-specific pathfinding logic
	if e.orchestrator == nil {
		return nil, fmt.Errorf("no orchestrator configured")
	}

	// Note: This is a simplified implementation
	// The spatial orchestrator currently doesn't have position-to-position pathfinding
	// This would need to be enhanced or we'd implement it in the query handler
	return nil, fmt.Errorf("position-to-position pathfinding not yet implemented")
}

// FindPathCube finds a path between cube coordinates using A* algorithm.
// This is the primary pathfinding method for hex grid environments.
func (e *BasicEnvironment) FindPathCube(input *FindPathCubeInput) (*FindPathCubeOutput, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	pathfinder := spatial.NewSimplePathFinder()
	path := pathfinder.FindPath(input.From, input.To, input.Blocked)

	return &FindPathCubeOutput{
		Path:          path,
		TotalDistance: len(path),
		Found:         len(path) > 0 || input.From == input.To,
	}, nil
}

// Export serializes the environment to a byte array for storage or transmission.
func (e *BasicEnvironment) Export() ([]byte, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Export environment to JSON for persistence/serialization
	// This includes metadata and references to spatial components
	exportData := struct {
		ID            string              `json:"id"`
		Type          string              `json:"type"`
		Theme         string              `json:"theme"`
		Metadata      EnvironmentMetadata `json:"metadata"`
		RoomIDs       []string            `json:"room_ids"`
		ConnectionIDs []string            `json:"connection_ids"`
	}{
		ID:       e.id,
		Type:     e.typ,
		Theme:    e.theme,
		Metadata: e.metadata,
	}

	// Collect room and connection IDs for reference
	for _, room := range e.orchestrator.GetAllRooms() {
		exportData.RoomIDs = append(exportData.RoomIDs, room.GetID())
	}

	for _, conn := range e.orchestrator.GetAllConnections() {
		exportData.ConnectionIDs = append(exportData.ConnectionIDs, conn.GetID())
	}

	return json.Marshal(exportData)
}

// Event integration following toolkit patterns

// ConnectToEventBus connects the environment's typed topics to the event bus
func (e *BasicEnvironment) ConnectToEventBus(bus events.EventBus) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Connect typed topics to event bus
	e.environmentEntityAddedTopic = EnvironmentEntityAddedTopic.On(bus)
	e.environmentEntityMovedTopic = EnvironmentEntityMovedTopic.On(bus)
	e.environmentRoomAddedTopic = EnvironmentRoomAddedTopic.On(bus)
	e.themeChangedTopic = ThemeChangedTopic.On(bus)
	e.environmentMetadataChangedTopic = EnvironmentMetadataChangedTopic.On(bus)

	// TODO: Subscribe to spatial typed events once integration is completed
	// e.g.: e.spatialEntityPlacedSub = spatial.EntityPlacedTopic.Subscribe(ctx, e.handleEntityPlaced)
	// e.g.: e.spatialEntityMovedSub = spatial.EntityMovedTopic.Subscribe(ctx, e.handleEntityMoved)
	// e.g.: e.spatialRoomCreatedSub = spatial.RoomCreatedTopic.Subscribe(ctx, e.handleRoomCreated)
}

// Cleanup method for proper resource management
func (e *BasicEnvironment) Cleanup() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Cleanup typed topic connections
	// Note: With typed topics, cleanup is handled automatically by event bus
	e.subscriptions = nil
}

// Theme management - environment-specific functionality

// SetTheme changes the visual and atmospheric theme of the environment.
func (e *BasicEnvironment) SetTheme(newTheme string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	oldTheme := e.theme
	e.theme = newTheme

	// Publish typed theme change event
	event := ThemeChangedEvent{
		EnvironmentID: e.id,
		OldTheme:      oldTheme,
		NewTheme:      newTheme,
		AffectedRooms: e.getAllRoomIDsUnsafe(),
		ChangedAt:     time.Now(),
	}
	_ = e.themeChangedTopic.Publish(context.Background(), event)
}

// UpdateMetadata updates the environment's metadata with new information.
func (e *BasicEnvironment) UpdateMetadata(metadata EnvironmentMetadata) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.metadata = metadata

	// Publish typed metadata change event
	event := EnvironmentMetadataChangedEvent{
		EnvironmentID: e.id,
		Metadata:      metadata,
		ChangedAt:     time.Now(),
	}
	_ = e.environmentMetadataChangedTopic.Publish(context.Background(), event)
}

// Helper methods

func (e *BasicEnvironment) getAllRoomIDsUnsafe() []string {
	// Unsafe version for internal use when already holding lock
	allRooms := e.orchestrator.GetAllRooms()
	roomIDs := make([]string, 0, len(allRooms))
	for _, room := range allRooms {
		roomIDs = append(roomIDs, room.GetID())
	}
	return roomIDs
}

// Unified coordinate system methods

// GetBlockedHexes returns all blocked hexes in dungeon-absolute coordinates.
// This returns the pre-calculated blocked hexes from generation.
func (e *BasicEnvironment) GetBlockedHexes() map[spatial.CubeCoordinate]bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[spatial.CubeCoordinate]bool, len(e.blockedHexes))
	for k, v := range e.blockedHexes {
		result[k] = v
	}
	return result
}

// GetRoomAt returns which room contains the given dungeon-absolute coordinate.
func (e *BasicEnvironment) GetRoomAt(coord spatial.CubeCoordinate) (string, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	for roomID, room := range e.orchestrator.GetAllRooms() {
		roomPos, hasPos := e.roomPositions[roomID]
		if !hasPos {
			roomPos = spatial.CubeCoordinate{}
		}

		// Convert absolute to room-local
		localX := coord.X - roomPos.X
		localZ := coord.Z - roomPos.Z

		// Check if within room grid bounds
		grid := room.GetGrid()
		if grid == nil {
			continue
		}

		dims := grid.GetDimensions()
		width := int(dims.Width)
		height := int(dims.Height)

		// Room uses cube coords starting at 0,0,0
		// Width extends in +X direction, height extends in -Z direction
		if localX >= 0 && localX < width && localZ <= 0 && localZ > -height {
			return roomID, true
		}
	}

	return "", false
}

// GetRoomBounds returns the min/max coordinates for a room in dungeon-absolute space.
func (e *BasicEnvironment) GetRoomBounds(roomID string) (spatial.CubeCoordinate, spatial.CubeCoordinate, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	room, exists := e.orchestrator.GetRoom(roomID)
	if !exists {
		return spatial.CubeCoordinate{}, spatial.CubeCoordinate{}, fmt.Errorf("room %s not found", roomID)
	}

	roomPos, hasPos := e.roomPositions[roomID]
	if !hasPos {
		roomPos = spatial.CubeCoordinate{}
	}

	grid := room.GetGrid()
	if grid == nil {
		return spatial.CubeCoordinate{}, spatial.CubeCoordinate{}, fmt.Errorf("room %s has no grid", roomID)
	}

	// Get dimensions and calculate bounds
	dims := grid.GetDimensions()
	width := int(dims.Width)
	height := int(dims.Height)

	// Room-local bounds: x from 0 to width-1, z from 0 to -(height-1)
	// Min is bottom-left (x=0, z=-(height-1)), max is top-right (x=width-1, z=0)
	minX := 0
	maxX := width - 1
	minZ := -(height - 1)
	maxZ := 0

	// Y is derived: y = -x - z
	minLocal := spatial.CubeCoordinate{X: minX, Y: -minX - minZ, Z: minZ}
	maxLocal := spatial.CubeCoordinate{X: maxX, Y: -maxX - maxZ, Z: maxZ}

	// Convert to absolute
	minAbs := spatial.CubeCoordinate{
		X: roomPos.X + minLocal.X,
		Y: roomPos.Y + minLocal.Y,
		Z: roomPos.Z + minLocal.Z,
	}
	maxAbs := spatial.CubeCoordinate{
		X: roomPos.X + maxLocal.X,
		Y: roomPos.Y + maxLocal.Y,
		Z: roomPos.Z + maxLocal.Z,
	}

	return minAbs, maxAbs, nil
}

// GetRoomPosition returns a room's origin in dungeon-absolute coordinates.
func (e *BasicEnvironment) GetRoomPosition(roomID string) (spatial.CubeCoordinate, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	pos, exists := e.roomPositions[roomID]
	return pos, exists
}
