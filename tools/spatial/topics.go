// Package spatial provides 2D spatial positioning and movement capabilities for RPG games.
package spatial

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Typed topic definitions for spatial module events
// These are defined at compile-time and connected to event bus at runtime via .On(bus)

var (
	// EntityPlacedTopic publishes events when entities are placed in rooms
	EntityPlacedTopic = events.DefineTypedTopic[EntityPlacedEvent]("spatial.entity.placed")
	// EntityMovedTopic publishes events when entities are moved within or between rooms
	EntityMovedTopic = events.DefineTypedTopic[EntityMovedEvent]("spatial.entity.moved")
	// EntityRemovedTopic publishes events when entities are removed from rooms
	EntityRemovedTopic = events.DefineTypedTopic[EntityRemovedEvent]("spatial.entity.removed")

	// RoomCreatedTopic publishes events when rooms are created
	RoomCreatedTopic = events.DefineTypedTopic[RoomCreatedEvent]("spatial.room.created")

	// RoomAddedTopic publishes events when rooms are added to orchestrators
	RoomAddedTopic = events.DefineTypedTopic[RoomAddedEvent]("spatial.orchestrator.room_added")
	// RoomRemovedTopic publishes events when rooms are removed from orchestrators
	RoomRemovedTopic = events.DefineTypedTopic[RoomRemovedEvent]("spatial.orchestrator.room_removed")
	// ConnectionAddedTopic publishes events when connections are added between rooms
	ConnectionAddedTopic = events.DefineTypedTopic[ConnectionAddedEvent]("spatial.orchestrator.connection_added")
	// ConnectionRemovedTopic publishes events when connections are removed between rooms
	ConnectionRemovedTopic = events.DefineTypedTopic[ConnectionRemovedEvent]("spatial.orchestrator.connection_removed")
	// EntityTransitionBeganTopic publishes events when entity transitions begin
	EntityTransitionBeganTopic = events.DefineTypedTopic[EntityTransitionBeganEvent]("spatial.entity.transition.began")
	// EntityTransitionEndedTopic publishes events when entity transitions complete
	EntityTransitionEndedTopic = events.DefineTypedTopic[EntityTransitionEndedEvent]("spatial.entity.transition.ended")
	// EntityRoomTransitionTopic publishes events when entities transition between rooms
	EntityRoomTransitionTopic = events.DefineTypedTopic[EntityRoomTransitionEvent]("entity.room_transition")
	// LayoutChangedTopic publishes events when orchestrator layouts change
	LayoutChangedTopic = events.DefineTypedTopic[LayoutChangedEvent]("spatial.orchestrator.layout_changed")
)

// EntityPlacedEvent contains data for entity placement events
type EntityPlacedEvent struct {
	EntityID string   `json:"entity_id"`
	Position Position `json:"position"`
	RoomID   string   `json:"room_id"`
	GridType string   `json:"grid_type"` // "square", "hex", "gridless"
}

// EntityMovedEvent contains data for entity movement events
type EntityMovedEvent struct {
	EntityID     string   `json:"entity_id"`
	FromPosition Position `json:"from_position"`
	ToPosition   Position `json:"to_position"`
	RoomID       string   `json:"room_id"`
	MovementType string   `json:"movement_type"` // "normal", "teleport", "forced"
}

// EntityRemovedEvent contains data for entity removal events
type EntityRemovedEvent struct {
	EntityID    string   `json:"entity_id"`
	Position    Position `json:"position"`
	RoomID      string   `json:"room_id"`
	RemovalType string   `json:"removal_type"` // "normal", "destroyed", "teleported"
}

// RoomCreatedEvent contains data for room creation events
type RoomCreatedEvent struct {
	RoomID       string    `json:"room_id"`
	RoomType     string    `json:"room_type"`
	GridType     string    `json:"grid_type"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	CreationTime time.Time `json:"creation_time"`
}

// RoomAddedEvent contains data for room addition to orchestrator events
type RoomAddedEvent struct {
	OrchestratorID string    `json:"orchestrator_id"`
	RoomID         string    `json:"room_id"`
	RoomType       string    `json:"room_type,omitempty"`
	AddedAt        time.Time `json:"added_at"`
}

// RoomRemovedEvent contains data for room removal from orchestrator events
type RoomRemovedEvent struct {
	OrchestratorID string    `json:"orchestrator_id"`
	RoomID         string    `json:"room_id"`
	Reason         string    `json:"reason,omitempty"`
	RemovedAt      time.Time `json:"removed_at"`
}

// ConnectionAddedEvent contains data for connection addition events
type ConnectionAddedEvent struct {
	OrchestratorID string    `json:"orchestrator_id"`
	ConnectionID   string    `json:"connection_id"`
	FromRoom       string    `json:"from_room"`
	ToRoom         string    `json:"to_room"`
	ConnectionType string    `json:"connection_type"`
	AddedAt        time.Time `json:"added_at"`
}

// ConnectionRemovedEvent contains data for connection removal events
type ConnectionRemovedEvent struct {
	OrchestratorID string    `json:"orchestrator_id"`
	ConnectionID   string    `json:"connection_id"`
	Reason         string    `json:"reason,omitempty"`
	RemovedAt      time.Time `json:"removed_at"`
}

// EntityTransitionBeganEvent contains data for entity transition start events
type EntityTransitionBeganEvent struct {
	EntityID       string    `json:"entity_id"`
	FromRoom       string    `json:"from_room"`
	ToRoom         string    `json:"to_room"`
	ConnectionID   string    `json:"connection_id,omitempty"`
	TransitionType string    `json:"transition_type"` // "door", "stairs", "portal", etc.
	BeganAt        time.Time `json:"began_at"`
}

// EntityTransitionEndedEvent contains data for entity transition completion events
type EntityTransitionEndedEvent struct {
	EntityID       string    `json:"entity_id"`
	FromRoom       string    `json:"from_room"`
	ToRoom         string    `json:"to_room"`
	ConnectionID   string    `json:"connection_id,omitempty"`
	TransitionType string    `json:"transition_type"`
	Success        bool      `json:"success"`
	EndedAt        time.Time `json:"ended_at"`
}

// EntityRoomTransitionEvent contains data for entity room transition events
type EntityRoomTransitionEvent struct {
	EntityID  string    `json:"entity_id"`
	FromRoom  string    `json:"from_room"`
	ToRoom    string    `json:"to_room"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// LayoutChangedEvent contains data for orchestrator layout change events
type LayoutChangedEvent struct {
	OrchestratorID string    `json:"orchestrator_id"`
	OldLayout      string    `json:"old_layout,omitempty"`
	NewLayout      string    `json:"new_layout"`
	ChangedAt      time.Time `json:"changed_at"`
}
