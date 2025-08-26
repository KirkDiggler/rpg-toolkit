// Package spatial provides 2D spatial positioning and movement capabilities for RPG games.
package spatial

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Typed topic definitions for spatial module events
// These are defined at compile-time and connected to event bus at runtime via .On(bus)

var (
	// Entity lifecycle topics
	EntityPlacedTopic  = events.DefineTypedTopic[EntityPlacedEvent]("spatial.entity.placed")
	EntityMovedTopic   = events.DefineTypedTopic[EntityMovedEvent]("spatial.entity.moved")
	EntityRemovedTopic = events.DefineTypedTopic[EntityRemovedEvent]("spatial.entity.removed")

	// Room lifecycle topics
	RoomCreatedTopic = events.DefineTypedTopic[RoomCreatedEvent]("spatial.room.created")
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