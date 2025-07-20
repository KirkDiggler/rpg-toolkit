package spawn

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnEngine provides entity placement capabilities for game spaces
// Phase 1: Basic interface with core functionality only
type SpawnEngine interface {
	// PopulateRoom places entities in a single room using the specified configuration
	PopulateRoom(ctx context.Context, roomID string, config SpawnConfig) (SpawnResult, error)

	// ValidateSpawnConfig validates a spawn configuration before use
	ValidateSpawnConfig(config SpawnConfig) error
}

// SelectablesRegistry manages selection tables for entity spawning
type SelectablesRegistry interface {
	// RegisterTable registers a selection table for use in spawn configurations
	RegisterTable(tableID string, entities []core.Entity) error

	// GetEntities retrieves entities from a registered table
	GetEntities(tableID string, quantity int) ([]core.Entity, error)

	// ListTables returns the IDs of all registered tables
	ListTables() []string
}

// SpawnResult contains the results of a spawn operation
type SpawnResult struct {
	Success         bool            `json:"success"`
	SpawnedEntities []SpawnedEntity `json:"spawned_entities"`
	Failures        []SpawnFailure  `json:"failures"`
}

// SpawnedEntity represents an entity that was successfully placed
type SpawnedEntity struct {
	Entity   core.Entity      `json:"entity"`
	Position spatial.Position `json:"position"`
	RoomID   string           `json:"room_id"`
}

// SpawnFailure represents an entity that could not be placed
type SpawnFailure struct {
	EntityType string `json:"entity_type"`
	Reason     string `json:"reason"`
}
