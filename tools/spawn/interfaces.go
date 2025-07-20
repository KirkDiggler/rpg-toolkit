package spawn

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnEngine defines the interface for entity spawning and placement
// Purpose: Provides comprehensive entity placement capabilities while maintaining
// split-awareness and clean separation of concerns with environment package
type SpawnEngine interface {
	core.Entity

	// Core spawning - works with single rooms or split room configurations
	PopulateSpace(ctx context.Context, roomOrGroup interface{}, config SpawnConfig) (SpawnResult, error)

	// Legacy single-room interface for backwards compatibility
	PopulateRoom(ctx context.Context, roomID string, config SpawnConfig) (SpawnResult, error)

	// Multi-room spawning for split room scenarios
	PopulateSplitRooms(ctx context.Context, connectedRooms []string, config SpawnConfig) (SpawnResult, error)

	// Room transition handling
	HandleRoomTransition(ctx context.Context, entityID, fromRoom, toRoom, connectionID string) (spatial.Position, error)

	// Configuration validation
	ValidateSpawnConfig(config SpawnConfig) error

	// Room structure analysis for split-awareness
	AnalyzeRoomStructure(roomID string) RoomStructureInfo

	// Helper configuration support
	PopulateSpaceWithHelper(ctx context.Context, roomOrGroup interface{}, entities []core.Entity, helperConfig HelperConfig) (SpawnResult, error)
}

// SelectablesRegistry defines the interface for managing selection tables
// Purpose: Manages selectables integration for entity selection
type SelectablesRegistry interface {
	// RegisterTable registers a selection table with the spawn engine
	RegisterTable(tableID string, table selectables.SelectionTable[core.Entity]) error

	// GetTable retrieves a registered selection table
	GetTable(tableID string) (selectables.SelectionTable[core.Entity], error)

	// ListTables returns all registered table IDs
	ListTables() []string

	// RemoveTable removes a selection table
	RemoveTable(tableID string) error
}

//go:generate mockgen -destination=mock/mock_spawn_engine.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/spawn SpawnEngine
//go:generate mockgen -destination=mock/mock_selectables_registry.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/spawn SelectablesRegistry

// SpawnEnvironmentQueryHandler extends environments.QueryHandler with spawn-specific queries
// Purpose: Provides capacity analysis and sizing queries needed for spawn engine functionality
type SpawnEnvironmentQueryHandler interface {
	environments.QueryHandler
	
	// HandleCapacityQuery processes capacity analysis queries
	HandleCapacityQuery(ctx context.Context, query interface{}) (interface{}, error)
	
	// HandleSizingQuery processes room sizing queries
	HandleSizingQuery(ctx context.Context, query interface{}) (spatial.Dimensions, error)
}