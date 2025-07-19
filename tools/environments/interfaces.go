package environments

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// GenerationType represents different environment generation approaches
type GenerationType int

const (
	// GenerationTypeGraph represents graph-based generation
	GenerationTypeGraph GenerationType = iota
	// GenerationTypePrefab represents prefab-based generation
	GenerationTypePrefab
	// GenerationTypeHybrid represents hybrid generation combining graph and prefab
	GenerationTypeHybrid
)

// Environment defines the interface for generated environments
type Environment interface {
	core.Entity

	// GetOrchestrator returns the underlying spatial orchestrator
	GetOrchestrator() spatial.RoomOrchestrator

	// GetRooms returns all rooms in the environment
	GetRooms() []spatial.Room

	// GetRoom returns a specific room by ID
	GetRoom(roomID string) (spatial.Room, bool)

	// GetConnections returns all connections in the environment
	GetConnections() []spatial.Connection

	// GetConnection returns a specific connection by ID
	GetConnection(connectionID string) (spatial.Connection, bool)

	// GetTheme returns the environment's theme
	GetTheme() string

	// GetMetadata returns environment metadata
	GetMetadata() EnvironmentMetadata

	// QueryEntities performs multi-room entity queries
	QueryEntities(ctx context.Context, query EntityQuery) ([]core.Entity, error)

	// QueryRooms performs room-based queries
	QueryRooms(ctx context.Context, query RoomQuery) ([]spatial.Room, error)

	// FindPath finds a path between two positions across rooms
	FindPath(from, to spatial.Position) ([]spatial.Position, error)

	// Export exports the environment to a serializable format
	Export() ([]byte, error)
}

// EnvironmentGenerator defines the interface for environment generation
type EnvironmentGenerator interface {
	core.Entity

	// Generate creates a new environment
	Generate(ctx context.Context, config GenerationConfig) (Environment, error)

	// GetGenerationType returns the generation type
	GetGenerationType() GenerationType

	// Validate validates a generation configuration
	Validate(config GenerationConfig) error

	// GetCapabilities returns generator capabilities
	GetCapabilities() GeneratorCapabilities
}

// RoomBuilder defines the interface for component-based room building
type RoomBuilder interface {
	// WithSize sets the room dimensions
	WithSize(width, height int) RoomBuilder

	// WithTheme sets the room theme
	WithTheme(theme string) RoomBuilder

	// WithFeatures adds features to the room
	WithFeatures(features ...Feature) RoomBuilder

	// WithLayout sets the room layout
	WithLayout(layout Layout) RoomBuilder

	// WithPrefab applies a prefab template
	WithPrefab(prefab RoomPrefab) RoomBuilder

	// WithWallPattern sets the wall pattern
	WithWallPattern(pattern string) RoomBuilder

	// WithWallDensity sets the wall density
	WithWallDensity(density float64) RoomBuilder

	// WithDestructibleRatio sets the destructible wall ratio
	WithDestructibleRatio(ratio float64) RoomBuilder

	// WithMaterial sets the wall material
	WithMaterial(material string) RoomBuilder

	// WithShape sets the room shape by name
	WithShape(shapeName string) RoomBuilder

	// WithRandomSeed sets the random seed
	WithRandomSeed(seed int64) RoomBuilder

	// WithSafety sets the path safety parameters
	WithSafety(safety PathSafetyParams) RoomBuilder

	// Build creates the room
	Build() (spatial.Room, error)
}

// ComponentFactory defines the interface for custom component creation
type ComponentFactory interface {
	// CreateComponent creates a component from a specification
	CreateComponent(spec interface{}) (interface{}, error)

	// GetComponentType returns the component type this factory creates
	GetComponentType() string

	// Validate validates a component specification
	Validate(spec interface{}) error
}

// QueryHandler defines the interface for environment queries
type QueryHandler interface {
	// HandleEntityQuery processes entity queries
	HandleEntityQuery(ctx context.Context, query EntityQuery) ([]core.Entity, error)

	// HandleRoomQuery processes room queries
	HandleRoomQuery(ctx context.Context, query RoomQuery) ([]spatial.Room, error)

	// HandlePathQuery processes pathfinding queries
	HandlePathQuery(ctx context.Context, query PathQuery) ([]spatial.Position, error)
}

//go:generate mockgen -destination=mock/mock_environment.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/environments Environment
//go:generate mockgen -destination=mock/mock_generator.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/environments EnvironmentGenerator
//go:generate mockgen -destination=mock/mock_room_builder.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/environments RoomBuilder
//go:generate mockgen -destination=mock/mock_component_factory.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/environments ComponentFactory
//go:generate mockgen -destination=mock/mock_query_handler.go -package=mock github.com/KirkDiggler/rpg-toolkit/tools/environments QueryHandler
