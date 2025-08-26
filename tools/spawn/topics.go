// Package spawn provides intelligent entity placement and spawning capabilities.
package spawn

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Typed topic definitions for spawn module events
// These are defined at compile-time and connected to event bus at runtime via .On(bus)

var (
	// Entity operations topics
	EntitySpawnedTopic = events.DefineTypedTopic[EntitySpawnedEvent]("spawn.entity.spawned")
	
	// Capacity management topics
	SplitRecommendedTopic = events.DefineTypedTopic[SplitRecommendedEvent]("spawn.split.recommended")
	RoomScaledTopic       = events.DefineTypedTopic[RoomScaledEvent]("spawn.room.scaled")
)

// EntitySpawnedEvent contains data for entity spawning events
type EntitySpawnedEvent struct {
	EntityID     string            `json:"entity_id"`
	EntityType   string            `json:"entity_type,omitempty"`
	Position     spatial.Position  `json:"position"`
	RoomID       string            `json:"room_id"`
	SpawnType    string            `json:"spawn_type"` // "scattered", "formation", "player_choice", "clustered"
	SpawnGroupID string            `json:"spawn_group_id,omitempty"`
	Constraints  []string          `json:"constraints,omitempty"`
	SpawnedAt    time.Time         `json:"spawned_at"`
}

// SplitRecommendedEvent indicates when room splitting is recommended for capacity
type SplitRecommendedEvent struct {
	RoomID             string    `json:"room_id"`
	CurrentCapacity    int       `json:"current_capacity"`
	RequiredCapacity   int       `json:"required_capacity"`
	RecommendationType string    `json:"recommendation_type"` // "split", "expand", "redistribute"
	Reason             string    `json:"reason"`
	Timestamp          time.Time `json:"timestamp"`
}

// RoomScaledEvent indicates when a room's capacity has been scaled
type RoomScaledEvent struct {
	RoomID      string    `json:"room_id"`
	OldCapacity int       `json:"old_capacity"`
	NewCapacity int       `json:"new_capacity"`
	ScaleReason string    `json:"scale_reason"`
	ScaleType   string    `json:"scale_type"` // "expand", "contract", "optimize"
	ScaledAt    time.Time `json:"scaled_at"`
}