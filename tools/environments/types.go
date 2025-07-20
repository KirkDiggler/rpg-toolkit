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
	// EnvironmentSizeSmall represents small environment sizes ~10-20 rooms
	EnvironmentSizeSmall EnvironmentSize = iota
	// EnvironmentSizeMedium is the default size for most environments ~20-50 rooms
	EnvironmentSizeMedium
	// EnvironmentSizeLarge represents large environments ~50-150 rooms
	EnvironmentSizeLarge
	// EnvironmentSizeCustom allows for custom dimensions
	EnvironmentSizeCustom
)

// LayoutType represents different spatial arrangement patterns
// Purpose: Defines how rooms are connected spatially. Each pattern creates
// different gameplay experiences (linear vs open vs hub-based exploration).
type LayoutType int

const (
	// LayoutTypeLinear represents a linear sequence of rooms
	LayoutTypeLinear LayoutType = iota
	// LayoutTypeBranching represents a branching structure with multiple paths
	LayoutTypeBranching
	// LayoutTypeGrid represents a regular grid pattern
	LayoutTypeGrid
	// LayoutTypeOrganic represents irregular, natural connections
	LayoutTypeOrganic
	// LayoutTypeCustom represents a custom layout algorithm // Use custom layout algorithm
	LayoutTypeCustom
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
	// ConstraintTypePlacement represents where rooms can be placed
	ConstraintTypePlacement ConstraintType = iota
	// ConstraintTypeConnection represents how rooms can connect
	ConstraintTypeConnection
	// ConstraintTypeProximity represents distance relationships
	ConstraintTypeProximity
	// ConstraintTypeSequence represents order requirements
	ConstraintTypeSequence
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
	// PathConstraintTypeAvoid represents constraints to avoid certain connections
	PathConstraintTypeAvoid PathConstraintType = iota
	// PathConstraintTypePrefer represents constraints to prefer certain routes
	PathConstraintTypePrefer
	// PathConstraintTypeRequire represents constraints that must be satisfied
	PathConstraintTypeRequire
	// PathConstraintTypeCost represents constraints that consider movement costs
	PathConstraintTypeCost
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

// SpatialIntentProfile translates design concepts into technical parameters
// Purpose: Converts feeling-based design decisions into concrete spatial calculations
type SpatialIntentProfile struct {
	Feeling              SpatialFeeling `json:"feeling"`                // The desired spatial experience
	EntityDensityTarget  float64        `json:"entity_density_target"`  // Target entity density (0.0-1.0)
	MovementFreedomIndex float64        `json:"movement_freedom_index"` // Movement space factor (0.0-1.0)
	VisualScopeIndex     float64        `json:"visual_scope_index"`     // Visual range factor (0.0-1.0)
	TacticalComplexity   float64        `json:"tactical_complexity"`    // Tactical positioning complexity (0.0-1.0)
}

// CapacityQuery represents a query for room capacity analysis
// Purpose: Allows games to analyze room capacity for intelligent design decisions
type CapacityQuery struct {
	RoomID              string               `json:"room_id"`                         // Target room for analysis
	Intent              SpatialIntentProfile `json:"intent"`                          // Desired spatial experience
	EntityCount         int                  `json:"entity_count,omitempty"`          // Number of entities to accommodate
	EntitySizes         []spatial.Dimensions `json:"entity_sizes,omitempty"`          // Sizes of entities to place
	RoomSize            spatial.Dimensions   `json:"room_size,omitempty"`             // Room dimensions for analysis
	Constraints         CapacityConstraints  `json:"constraints,omitempty"`           // Capacity constraints
	ExistingEntityCount int                  `json:"existing_entity_count,omitempty"` // Current entity count
	IncludeSplitOptions bool                 `json:"include_split_options,omitempty"` // Include split recommendations
}

// CapacityQueryResponse contains results from capacity analysis
// Purpose: Provides advisory information about room capacity and recommendations
type CapacityQueryResponse struct {
	Estimate     CapacityEstimate `json:"estimate"`      // Capacity analysis results
	SplitOptions []RoomSplit      `json:"split_options"` // Room splitting recommendations
	Analysis     CapacityAnalysis `json:"analysis"`      // Detailed capacity analysis
	Satisfied    bool             `json:"satisfied"`     // Whether requirements can be met
	Alternatives []string         `json:"alternatives"`  // Alternative approaches
}

// SizingQuery represents a query for optimal room sizing
// Purpose: Helps determine appropriate room dimensions for specific requirements
type SizingQuery struct {
	Intent          SpatialIntentProfile `json:"intent"`                     // Desired spatial experience
	IntentProfile   SpatialIntentProfile `json:"intent_profile"`             // Alternative field name for intent
	EntityCount     int                  `json:"entity_count,omitempty"`     // Number of entities
	EntitySizes     []spatial.Dimensions `json:"entity_sizes,omitempty"`     // Entity dimensions
	Constraints     CapacityConstraints  `json:"constraints,omitempty"`      // Additional constraints
	AdditionalSpace float64              `json:"additional_space,omitempty"` // Extra space multiplier
	MinDimensions   spatial.Dimensions   `json:"min_dimensions,omitempty"`   // Minimum allowed dimensions
	MaxDimensions   spatial.Dimensions   `json:"max_dimensions,omitempty"`   // Maximum allowed dimensions
}

// CapacityConstraints define limits and requirements for capacity calculations
// Purpose: Allows games to specify constraints on room sizing and capacity
type CapacityConstraints struct {
	MinDimensions             spatial.Dimensions `json:"min_dimensions"`              // Minimum room size
	MaxDimensions             spatial.Dimensions `json:"max_dimensions"`              // Maximum room size
	AspectRatio               float64            `json:"aspect_ratio"`                // Preferred width/height ratio
	MaxEntitiesPerRoom        int                `json:"max_entities_per_room"`       // Maximum entities allowed per room
	MinMovementSpace          float64            `json:"min_movement_space"`          // Minimum movement space factor
	WallDensityModifier       float64            `json:"wall_density_modifier"`       // Wall density impact
	RequiredPathwayMultiplier float64            `json:"required_pathway_multiplier"` // Pathway space multiplier
	TargetSpatialFeeling      SpatialFeeling     `json:"target_spatial_feeling"`      // Target spatial experience
	MinEntitySpacing          float64            `json:"min_entity_spacing"`          // Minimum space between entities
}

// CapacityEstimate provides analysis of room capacity
// Purpose: Advisory information about how well a room accommodates entities
type CapacityEstimate struct {
	RecommendedEntityCount int            `json:"recommended_entity_count"` // Recommended entity count
	MaxEntityCount         int            `json:"max_entity_count"`         // Maximum entities that fit comfortably
	UtilizationPercent     float64        `json:"utilization_percent"`      // How much of room would be used
	CrowdingFactor         float64        `json:"crowding_factor"`          // How crowded it would feel (0.0-1.0)
	MovementSpace          float64        `json:"movement_space"`           // Available movement area
	SpatialFeelingActual   SpatialFeeling `json:"spatial_feeling_actual"`   // Actual spatial feeling achieved
	MovementFreedomActual  float64        `json:"movement_freedom_actual"`  // Actual movement freedom achieved
	QualityScore           float64        `json:"quality_score"`            // Overall quality score (0.0-1.0)
	UsableArea             float64        `json:"usable_area"`              // Total usable area for entities
}

// RoomSplit represents a room splitting recommendation
// Purpose: Advisory information about how rooms could be divided
type RoomSplit struct {
	Reason                        string               `json:"reason"`                          // Why split is recommended
	SplitType                     string               `json:"split_type"`                      // Split type
	Dimensions                    []spatial.Dimensions `json:"dimensions"`                      // Room dimensions
	Benefits                      []string             `json:"benefits"`                        // Split advantages
	Complexity                    float64              `json:"complexity"`                      // Complexity rating
	SuggestedSize                 spatial.Dimensions   `json:"suggested_size"`                  // Split room dimensions
	ConnectionPoints              []spatial.Position   `json:"connection_points"`               // Connection points
	RecommendedEntityDistribution map[string]int       `json:"recommended_entity_distribution"` // Entity distribution
	RecommendedConnectionType     string               `json:"recommended_connection_type"`     // Connection type to use
	SplitReason                   string               `json:"split_reason"`                    // Reason for split
	EstimatedCapacityImprovement  float64              `json:"estimated_capacity_improvement"`  // Expected improvement
}

// CapacityAnalysis provides detailed capacity analysis results
// Purpose: In-depth analysis for games that need detailed capacity information
type CapacityAnalysis struct {
	TotalArea               float64          `json:"total_area"`                // Total room area
	UsableArea              float64          `json:"usable_area"`               // Area available for entities
	EntityArea              float64          `json:"entity_area"`               // Area occupied by entities
	MovementArea            float64          `json:"movement_area"`             // Area for movement
	InteractionArea         float64          `json:"interaction_area"`          // Area for entity interactions
	EfficiencyRating        float64          `json:"efficiency_rating"`         // How efficiently space is used
	ComfortRating           float64          `json:"comfort_rating"`            // How comfortable the space feels
	RecommendedActions      []string         `json:"recommended_actions"`       // Suggested improvements
	RoomCapacity            CapacityEstimate `json:"room_capacity"`             // Room capacity analysis
	RequestedEntityCount    int              `json:"requested_entity_count"`    // Number of entities requested
	CapacityUtilization     float64          `json:"capacity_utilization"`      // Capacity utilization ratio
	ResultingSpatialFeeling SpatialFeeling   `json:"resulting_spatial_feeling"` // Resulting spatial experience
	SplitOptions            []RoomSplit      `json:"split_options"`             // Room splitting options
}
