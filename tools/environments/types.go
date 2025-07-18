package environments

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EnvironmentMetadata contains descriptive information about an environment
// Purpose: Allows environments to carry theme, description, and generation info
// without affecting spatial behavior. Games can use this for UI, saving, etc.
type EnvironmentMetadata struct {
	Name        string            `json:"name"`         // Human-readable name
	Description string            `json:"description"`  // Narrative description
	Theme       string            `json:"theme"`        // Visual/thematic style
	Tags        []string          `json:"tags"`         // Searchable tags
	Properties  map[string]string `json:"properties"`   // Custom key-value data
	GeneratedAt time.Time         `json:"generated_at"` // When environment was created
	GeneratedBy string            `json:"generated_by"` // Generator that created it
	Version     string            `json:"version"`      // Environment format version
}

// GenerationConfig specifies how to generate an environment
// Purpose: Provides all parameters needed for generation while keeping
// the generator interface clean. Allows for complex configurations without
// polluting the Generate() method signature.
type GenerationConfig struct {
	// Basic generation parameters
	Type  GenerationType  `json:"type"`  // Graph, Prefab, or Hybrid
	Seed  int64           `json:"seed"`  // Random seed for reproducible generation
	Theme string          `json:"theme"` // Overall environment theme
	Size  EnvironmentSize `json:"size"`  // Small, Medium, Large, Custom

	// Room configuration
	RoomCount   int                `json:"room_count"`    // Number of rooms to generate
	RoomTypes   []string           `json:"room_types"`    // Available room types
	MinRoomSize spatial.Dimensions `json:"min_room_size"` // Minimum room dimensions
	MaxRoomSize spatial.Dimensions `json:"max_room_size"` // Maximum room dimensions

	// Layout configuration
	Layout       LayoutType `json:"layout"`       // Overall layout pattern
	Density      float64    `json:"density"`      // How tightly packed (0.0-1.0)
	Connectivity float64    `json:"connectivity"` // How connected rooms are (0.0-1.0)

	// Component factories for custom room types
	ComponentFactories map[string]ComponentFactory `json:"-"` // Custom component creators

	// Constraints and rules
	Constraints []GenerationConstraint `json:"constraints"` // Generation rules
	Metadata    EnvironmentMetadata    `json:"metadata"`    // Environment metadata
}

// EnvironmentSize represents predefined environment sizes
// Purpose: Provides convenient size presets while allowing custom sizes.
// Games can use standard sizes or define their own through Custom + dimensions.
type EnvironmentSize int

const (
	EnvironmentSizeSmall  EnvironmentSize = iota // ~5-15 rooms
	EnvironmentSizeMedium                        // ~15-50 rooms
	EnvironmentSizeLarge                         // ~50-150 rooms
	EnvironmentSizeCustom                        // Use custom dimensions
)

// LayoutType represents different spatial arrangement patterns
// Purpose: Defines how rooms are connected spatially. Each pattern creates
// different gameplay experiences (linear vs open vs hub-based exploration).
type LayoutType int

const (
	LayoutTypeLinear    LayoutType = iota // Rooms in sequence (classic dungeon crawl)
	LayoutTypeBranching                   // Hub with branches (town with districts)
	LayoutTypeGrid                        // Regular grid pattern (structured facility)
	LayoutTypeOrganic                     // Irregular, natural connections (cave system)
	LayoutTypeCustom                      // Use custom layout algorithm
)

// GenerationConstraint represents a rule that must be satisfied during generation
// Purpose: Allows games to specify requirements like "boss room must be furthest
// from entrance" or "treasure rooms must be connected to main path". Keeps
// generation flexible while ensuring game-specific requirements.
type GenerationConstraint struct {
	Type        ConstraintType `json:"type"`        // What kind of constraint
	Target      string         `json:"target"`      // What it applies to (room type, etc.)
	Requirement string         `json:"requirement"` // The actual requirement
	Priority    int            `json:"priority"`    // Higher priority = more important
}

// ConstraintType categorizes different kinds of generation constraints
// Purpose: Allows the generator to handle different constraint types appropriately.
// Some constraints affect placement, others affect connections, etc.
type ConstraintType int

const (
	ConstraintTypePlacement  ConstraintType = iota // Where rooms can be placed
	ConstraintTypeConnection                       // How rooms can connect
	ConstraintTypeProximity                        // Distance relationships
	ConstraintTypeSequence                         // Order requirements
)

// GeneratorCapabilities describes what a generator can do
// Purpose: Allows runtime discovery of generator features. Games can query
// capabilities to show appropriate UI options or validate configurations.
type GeneratorCapabilities struct {
	SupportedTypes      []GenerationType  `json:"supported_types"`       // What generation types
	SupportedLayouts    []LayoutType      `json:"supported_layouts"`     // What layout patterns
	SupportedSizes      []EnvironmentSize `json:"supported_sizes"`       // What size ranges
	MaxRoomCount        int               `json:"max_room_count"`        // Technical limits
	SupportsConstraints bool              `json:"supports_constraints"`  // Can handle constraints
	SupportsCustomRooms bool              `json:"supports_custom_rooms"` // Can use custom room types
}

// EntityQuery represents a query for entities across the environment
// Purpose: Provides structured way to query entities across multiple rooms.
// More powerful than spatial queries because it can aggregate across rooms
// and apply environment-specific filters.
type EntityQuery struct {
	// Spatial criteria
	Center  *spatial.Position `json:"center,omitempty"`   // Center point for range queries
	Radius  float64           `json:"radius,omitempty"`   // Search radius
	RoomIDs []string          `json:"room_ids,omitempty"` // Specific rooms to search

	// Entity criteria
	EntityTypes []string `json:"entity_types,omitempty"` // Entity types to include
	ExcludeIDs  []string `json:"exclude_ids,omitempty"`  // Entity IDs to exclude

	// Environment-specific criteria
	InTheme    string `json:"in_theme,omitempty"`    // Only in rooms with theme
	HasFeature string `json:"has_feature,omitempty"` // Only in rooms with feature

	// Result limits
	Limit int `json:"limit,omitempty"` // Max results to return
}

// RoomQuery represents a query for rooms in the environment
// Purpose: Allows complex room searches like "find all treasure rooms within
// 3 connections of the entrance" or "find rooms with theme X and feature Y".
type RoomQuery struct {
	// Basic criteria
	RoomTypes []string `json:"room_types,omitempty"` // Room types to include
	Themes    []string `json:"themes,omitempty"`     // Themes to include
	Features  []string `json:"features,omitempty"`   // Required features

	// Spatial criteria
	NearPosition *spatial.Position `json:"near_position,omitempty"` // Near this position
	MaxDistance  int               `json:"max_distance,omitempty"`  // Max connections away

	// Connection criteria
	ConnectedTo    string `json:"connected_to,omitempty"`    // Must connect to this room
	MinConnections int    `json:"min_connections,omitempty"` // Minimum number of connections
	MaxConnections int    `json:"max_connections,omitempty"` // Maximum number of connections

	// Result limits
	Limit int `json:"limit,omitempty"` // Max results to return
}

// PathQuery represents a pathfinding query across the environment
// Purpose: Enables pathfinding that spans multiple rooms, considering
// connection types, requirements, and movement costs.
type PathQuery struct {
	From        spatial.Position `json:"from"`                  // Starting position
	To          spatial.Position `json:"to"`                    // Destination position
	EntityID    string           `json:"entity_id,omitempty"`   // Entity making the journey
	Constraints []PathConstraint `json:"constraints,omitempty"` // Movement constraints
	MaxCost     float64          `json:"max_cost,omitempty"`    // Maximum total cost
}

// PathConstraint represents a constraint on pathfinding
// Purpose: Allows complex pathfinding rules like "avoid locked doors unless
// entity has key" or "prefer routes through well-lit rooms".
type PathConstraint struct {
	Type      PathConstraintType `json:"type"`      // What kind of constraint
	Parameter string             `json:"parameter"` // Constraint parameter
	Weight    float64            `json:"weight"`    // How much to weight this (0.0-1.0)
}

// PathConstraintType categorizes different pathfinding constraints
// Purpose: Allows the pathfinder to handle different constraint types.
type PathConstraintType int

const (
	PathConstraintTypeAvoid   PathConstraintType = iota // Avoid certain connection types
	PathConstraintTypePrefer                            // Prefer certain routes
	PathConstraintTypeRequire                           // Require certain capabilities
	PathConstraintTypeCost                              // Consider movement costs
)

// Feature represents an environmental feature that can be added to rooms
// Purpose: Provides extensible way to add game-specific elements to rooms
// without coupling the environment system to specific game mechanics.
type Feature struct {
	Type       string                 `json:"type"`       // Feature type (trap, chest, etc.)
	Name       string                 `json:"name"`       // Display name
	Position   *spatial.Position      `json:"position"`   // Where in the room (if specific)
	Properties map[string]interface{} `json:"properties"` // Feature-specific data
}

// Layout represents a spatial arrangement pattern for rooms
// Purpose: Defines how rooms should be arranged spatially. Can be used
// by generators to create different types of environments.
type Layout struct {
	Type        LayoutType             `json:"type"`        // Layout pattern type
	Parameters  map[string]interface{} `json:"parameters"`  // Layout-specific parameters
	Connections []LayoutConnection     `json:"connections"` // Connection specifications
}

// LayoutConnection represents a connection specification in a layout
// Purpose: Defines how rooms should be connected in a layout pattern.
// More abstract than spatial connections - gets converted during generation.
type LayoutConnection struct {
	FromType       string  `json:"from_type"`       // Source room type
	ToType         string  `json:"to_type"`         // Destination room type
	ConnectionType string  `json:"connection_type"` // Type of connection
	Probability    float64 `json:"probability"`     // Chance this connection exists
	Required       bool    `json:"required"`        // Must this connection exist
}

// RoomPrefab represents a pre-designed room template
// Purpose: Allows designers to create reusable room templates that can be
// instantiated during generation. Supports both exact layouts and parameterized templates.
type RoomPrefab struct {
	Name        string                 `json:"name"`        // Prefab identifier
	Type        string                 `json:"type"`        // Room type this creates
	Theme       string                 `json:"theme"`       // Visual theme
	Size        spatial.Dimensions     `json:"size"`        // Room dimensions
	Features    []Feature              `json:"features"`    // Features to place
	Connections []PrefabConnection     `json:"connections"` // Connection points
	Parameters  map[string]interface{} `json:"parameters"`  // Customization parameters
}

// PrefabConnection represents a connection point in a room prefab
// Purpose: Defines where connections can be made to this prefab room.
// Allows prefabs to specify valid connection points and types.
type PrefabConnection struct {
	Position spatial.Position `json:"position"` // Where the connection attaches
	Type     string           `json:"type"`     // What type of connection
	Required bool             `json:"required"` // Must this connection exist
	Name     string           `json:"name"`     // Connection identifier
}
