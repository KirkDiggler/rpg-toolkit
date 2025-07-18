// Package environments provides procedural generation of game environments using graph algorithms.
// This package creates environments by building abstract graphs of rooms and connections,
// then placing them spatially while integrating with the spatial module for obstacle detection.
package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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

	// Event integration following toolkit patterns
	eventBus events.EventBus

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
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Theme        string                   `json:"theme"`
	Metadata     EnvironmentMetadata      `json:"metadata"`
	EventBus     events.EventBus          `json:"-"` // Not serializable
	Orchestrator spatial.RoomOrchestrator `json:"-"` // Not serializable
	QueryHandler QueryHandler             `json:"-"` // Not serializable
}

// NewBasicEnvironment creates a new BasicEnvironment following toolkit patterns
// Purpose: Standard constructor with config struct, proper initialization
func NewBasicEnvironment(config BasicEnvironmentConfig) *BasicEnvironment {
	env := &BasicEnvironment{
		id:            config.ID,
		typ:           config.Type,
		theme:         config.Theme,
		metadata:      config.Metadata,
		orchestrator:  config.Orchestrator,
		eventBus:      config.EventBus,
		queryHandler:  config.QueryHandler,
		subscriptions: make([]string, 0),
	}

	// Subscribe to relevant events for reactive behavior
	if env.eventBus != nil {
		env.setupEventSubscriptions()
	}

	return env
}

// GetID returns the unique identifier for this environment
func (e *BasicEnvironment) GetID() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.id
}

// GetType returns the type of this environment
func (e *BasicEnvironment) GetType() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.typ
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

func (e *BasicEnvironment) QueryEntities(ctx context.Context, query EntityQuery) ([]core.Entity, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to our query handler which will aggregate across rooms
	if e.queryHandler == nil {
		return nil, fmt.Errorf("no query handler configured")
	}

	return e.queryHandler.HandleEntityQuery(ctx, query)
}

func (e *BasicEnvironment) QueryRooms(ctx context.Context, query RoomQuery) ([]spatial.Room, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Delegate to our query handler for environment-level room queries
	if e.queryHandler == nil {
		return nil, fmt.Errorf("no query handler configured")
	}

	return e.queryHandler.HandleRoomQuery(ctx, query)
}

func (e *BasicEnvironment) FindPath(_ spatial.Position, to spatial.Position) ([]spatial.Position, error) {
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

func (e *BasicEnvironment) setupEventSubscriptions() {
	// Subscribe to spatial events to react to changes
	if e.eventBus != nil {
		// React to spatial entity changes
		sub1 := e.eventBus.SubscribeFunc("spatial.entity.placed", 0, events.HandlerFunc(e.handleEntityPlaced))
		sub2 := e.eventBus.SubscribeFunc("spatial.entity.moved", 0, events.HandlerFunc(e.handleEntityMoved))
		sub3 := e.eventBus.SubscribeFunc("spatial.room.created", 0, events.HandlerFunc(e.handleRoomCreated))

		e.subscriptions = append(e.subscriptions, sub1, sub2, sub3)
	}
}

func (e *BasicEnvironment) handleEntityPlaced(ctx context.Context, event events.Event) error {
	// React to entities being placed in our environment
	// This is where we could apply environment effects, update tracking, etc.

	// Publish environment-specific event
	if e.eventBus != nil {
		envEvent := events.NewGameEvent(EventEnvironmentEntityAdded, e, event.Source())
		envEvent.Context().Set("original_event", event)
		envEvent.Context().Set("environment_theme", e.theme)
		_ = e.eventBus.Publish(ctx, envEvent)
	}
	return nil
}

func (e *BasicEnvironment) handleEntityMoved(ctx context.Context, event events.Event) error {
	// React to entity movement within our environment
	// Could trigger environment transitions, hazards, etc.

	// Publish environment-specific event if needed
	if e.eventBus != nil {
		envEvent := events.NewGameEvent(EventEnvironmentEntityMoved, e, event.Source())
		envEvent.Context().Set("original_event", event)
		envEvent.Context().Set("environment_theme", e.theme)
		_ = e.eventBus.Publish(ctx, envEvent)
	}
	return nil
}

func (e *BasicEnvironment) handleRoomCreated(ctx context.Context, event events.Event) error {
	// React to new rooms being added to our environment
	// Could apply theme, add default features, etc.

	if e.eventBus != nil {
		envEvent := events.NewGameEvent(EventEnvironmentRoomAdded, e, event.Source())
		envEvent.Context().Set("original_event", event)
		envEvent.Context().Set("environment_theme", e.theme)
		_ = e.eventBus.Publish(ctx, envEvent)
	}
	return nil
}

// Cleanup method for proper resource management
func (e *BasicEnvironment) Cleanup() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Unsubscribe from all events
	if e.eventBus != nil {
		for _, subID := range e.subscriptions {
			_ = e.eventBus.Unsubscribe(subID)
		}
		e.subscriptions = nil
	}
}

// Theme management - environment-specific functionality

func (e *BasicEnvironment) SetTheme(newTheme string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	oldTheme := e.theme
	e.theme = newTheme

	// Publish theme change event
	if e.eventBus != nil {
		event := events.NewGameEvent(EventThemeChanged, e, nil)
		event.Context().Set("old_theme", oldTheme)
		event.Context().Set("new_theme", newTheme)
		event.Context().Set("affected_rooms", e.getAllRoomIDsUnsafe())
		_ = e.eventBus.Publish(context.Background(), event)
	}
}

func (e *BasicEnvironment) UpdateMetadata(metadata EnvironmentMetadata) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.metadata = metadata

	// Publish metadata change event
	if e.eventBus != nil {
		event := events.NewGameEvent(EventEnvironmentMetadataChanged, e, nil)
		event.Context().Set("metadata", metadata)
		_ = e.eventBus.Publish(context.Background(), event)
	}
}

// Helper methods

func (e *BasicEnvironment) getAllRoomIDsUnsafe() []string {
	// Unsafe version for internal use when already holding lock
	var roomIDs []string
	for _, room := range e.orchestrator.GetAllRooms() {
		roomIDs = append(roomIDs, room.GetID())
	}
	return roomIDs
}
