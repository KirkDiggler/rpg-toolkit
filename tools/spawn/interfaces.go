package spawn

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnEngine provides entity placement capabilities for game spaces.
// Purpose: Core interface for all entity spawning functionality per ADR-0013.
// Supports split-aware spawning, capacity analysis, and adaptive room scaling.
//
//nolint:revive // type name follows ADR-0013 public API requirements
type SpawnEngine interface {
	// Core spawning - works with single rooms or split room configurations
	PopulateSpace(ctx context.Context, roomOrGroup interface{}, config SpawnConfig) (SpawnResult, error)

	// Legacy single-room interface for backwards compatibility
	PopulateRoom(ctx context.Context, roomID string, config SpawnConfig) (SpawnResult, error)

	// Multi-room spawning for split room scenarios
	PopulateSplitRooms(ctx context.Context, connectedRooms []string, config SpawnConfig) (SpawnResult, error)

	// Configuration validation
	ValidateSpawnConfig(config SpawnConfig) error

	// Room structure analysis for split-awareness
	AnalyzeRoomStructure(roomID string) RoomStructureInfo
}

// SelectablesRegistry manages selection tables for entity spawning.
// Purpose: Interface for registering and accessing entity selection tables.
type SelectablesRegistry interface {
	// RegisterTable registers a selection table for use in spawn configurations
	RegisterTable(tableID string, entities []core.Entity) error

	// GetEntities retrieves entities from a registered table
	GetEntities(tableID string, quantity int) ([]core.Entity, error)

	// ListTables returns the IDs of all registered tables
	ListTables() []string
}

// SpawnResult contains the results of a spawn operation.
// Purpose: Complete outcome of entity placement including successes, failures, and room modifications.
//
//nolint:revive // type name follows ADR-0013 public API requirements
type SpawnResult struct {
	Success              bool               `json:"success"`
	SpawnedEntities      []SpawnedEntity    `json:"spawned_entities"`
	Failures             []SpawnFailure     `json:"failures"`
	RoomModifications    []RoomModification `json:"room_modifications"`
	SplitRecommendations []RoomSplit        `json:"split_recommendations"`
	RoomStructure        RoomStructureInfo  `json:"room_structure"`
}

// SpawnedEntity represents an entity that was successfully placed.
// Purpose: Contains entity reference, final position, and room assignment.
type SpawnedEntity struct {
	Entity   core.Entity      `json:"entity"`
	Position spatial.Position `json:"position"`
	RoomID   string           `json:"room_id"`
}

// SpawnFailure represents an entity that could not be placed.
// Purpose: Error tracking for debugging placement issues and constraint violations.
//
//nolint:revive // type name follows ADR-0013 public API requirements
type SpawnFailure struct {
	EntityType string `json:"entity_type"`
	Reason     string `json:"reason"`
}

// RoomStructureInfo describes the room configuration used for spawning.
// Purpose: Indicates whether spawning used single or split-room configuration.
type RoomStructureInfo struct {
	IsSplit        bool     `json:"is_split"`
	ConnectedRooms []string `json:"connected_rooms"`
	PrimaryRoomID  string   `json:"primary_room_id"`
}

// RoomModification describes changes made to rooms during spawning.
// Purpose: Tracks room scaling and other adaptive changes for debugging and rollback.
type RoomModification struct {
	Type     string      `json:"type"`
	RoomID   string      `json:"room_id"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
	Reason   string      `json:"reason"`
}

// RoomSplit describes a recommended room split configuration.
// Purpose: Passthrough recommendations from environment package for client decision-making.
type RoomSplit struct {
	SuggestedSize      spatial.Dimensions `json:"suggested_size"`
	ConnectionPoints   []spatial.Position `json:"connection_points"`
	SplitReason        string             `json:"split_reason"`
	EntityDistribution map[string]int     `json:"entity_distribution"`
}
