package spawn

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnEngine provides the primary interface for entity spawning and room population
// Purpose: Handles the complex logic of placing entities in rooms according to
// patterns, constraints, and game rules while integrating with existing toolkit modules
type SpawnEngine interface {
	// PopulateRoom places entities in a room according to the given configuration
	PopulateRoom(roomID string, config SpawnConfig) (SpawnResult, error)

	// HandleRoomTransition places an entity when moving between connected rooms
	// Ensures placement near appropriate entrance/exit points
	HandleRoomTransition(entityID, fromRoom, toRoom, connectionID string) (spatial.Position, error)

	// ValidateSpawnConfig checks if a spawn configuration is valid before execution
	ValidateSpawnConfig(config SpawnConfig) error

	// GetSupportedPatterns returns the spawn patterns supported by this engine
	GetSupportedPatterns() []SpawnPattern

	// GetRoomCapacity estimates how many entities can fit in a room with given constraints
	GetRoomCapacity(roomID string, constraints SpatialConstraints) (int, error)
}

// SpawnPattern defines different approaches to entity placement
type SpawnPattern string

const (
	// PatternFormation places entities in structured arrangements (line, circle, wedge, etc.)
	PatternFormation SpawnPattern = "formation"

	// PatternClustered creates groups of entities with internal spacing rules
	PatternClustered SpawnPattern = "clustered"

	// PatternScattered distributes entities randomly within constraints
	PatternScattered SpawnPattern = "scattered"

	// PatternTeamBased separates entities into team zones (red vs blue, players vs enemies)
	PatternTeamBased SpawnPattern = "team_based"

	// PatternCustom allows user-defined placement patterns
	PatternCustom SpawnPattern = "custom"
)

// SpawnStrategy controls the deterministic vs random nature of spawning
type SpawnStrategy string

const (
	// StrategyDeterministic produces the same result each time with same config
	StrategyDeterministic SpawnStrategy = "deterministic"

	// StrategyRandomized uses random placement within constraints
	StrategyRandomized SpawnStrategy = "randomized"

	// StrategyBalanced optimizes placement for gameplay balance
	StrategyBalanced SpawnStrategy = "balanced"
)

// SpawnConfig defines all parameters for a room population operation
type SpawnConfig struct {
	// What to spawn
	EntityGroups []EntityGroup `json:"entity_groups"`
	LootTables   []LootTable   `json:"loot_tables"`

	// How to spawn
	Pattern           SpawnPattern     `json:"pattern"`
	TeamConfiguration *TeamConfig      `json:"team_config,omitempty"`
	FormationConfig   *FormationConfig `json:"formation_config,omitempty"`

	// Constraints and rules
	SpatialRules SpatialConstraints `json:"spatial_rules"`
	Placement    PlacementRules     `json:"placement"`

	// Behavior control
	Strategy        SpawnStrategy  `json:"strategy"`
	AdaptiveScaling *ScalingConfig `json:"adaptive_scaling,omitempty"`

	// Context for decision making
	RoomContext map[string]interface{} `json:"room_context,omitempty"`
}

// EntityGroup defines a group of related entities to spawn
type EntityGroup struct {
	// Identification
	ID   string `json:"id"`
	Type string `json:"type"` // "player", "enemy", "treasure", "environmental"

	// Selection rules
	SelectionTable string       `json:"selection_table"` // selectables table ID
	Quantity       QuantitySpec `json:"quantity"`
	Priority       int          `json:"priority"` // for conflict resolution

	// Group behavior
	Cohesion    float64                `json:"cohesion"` // how tightly grouped (0-1)
	Constraints GroupConstraints       `json:"constraints"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// QuantitySpec defines how many entities to spawn
type QuantitySpec struct {
	Fixed      *int     `json:"fixed,omitempty"`      // exact count
	DiceRoll   *string  `json:"dice_roll,omitempty"`  // "1d4", "2d6+1", etc.
	MinMax     *Range   `json:"min_max,omitempty"`    // random between min/max
	Percentage *float64 `json:"percentage,omitempty"` // percentage of room capacity
}

// Range defines a min/max range for quantities
type Range struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// LootTable connects spawn config to selectables module tables
type LootTable struct {
	ID       string                 `json:"id"`
	TableID  string                 `json:"table_id"` // selectables table ID
	Context  map[string]interface{} `json:"context"`  // context for selection
	Priority int                    `json:"priority"`
}

// SpatialConstraints defines spatial rules for entity placement
type SpatialConstraints struct {
	// Distance requirements
	MinDistance   map[string]float64 `json:"min_distance"`   // between entity types
	WallProximity float64            `json:"wall_proximity"` // min distance from walls

	// Line of sight rules
	LineOfSight LineOfSightRules `json:"line_of_sight"`

	// Area control
	AreaOfEffect map[string]float64 `json:"area_of_effect"` // buffer zones by entity type

	// Movement and pathing
	PathingRules PathingConstraints `json:"pathing"`

	// Room boundaries
	RoomBoundaries BoundaryConstraints `json:"room_boundaries"`
}

// LineOfSightRules controls visibility between entities
type LineOfSightRules struct {
	RequiredSight []EntityPair `json:"required_sight"` // must see each other
	BlockedSight  []EntityPair `json:"blocked_sight"`  // must NOT see each other
	CheckWalls    bool         `json:"check_walls"`    // whether walls block sight
}

// EntityPair represents a relationship between two entity types
type EntityPair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PathingConstraints ensures movement paths remain clear
type PathingConstraints struct {
	MaintainExitAccess  bool     `json:"maintain_exit_access"`
	MinPathWidth        float64  `json:"min_path_width"`
	RequiredConnections []string `json:"required_connections"` // connection IDs that must remain accessible
}

// BoundaryConstraints controls placement near room edges
type BoundaryConstraints struct {
	MinDistanceFromEdge float64 `json:"min_distance_from_edge"`
	AllowEdgePlacement  bool    `json:"allow_edge_placement"`
	CornerAvoidance     float64 `json:"corner_avoidance"`
}

// PlacementRules defines general placement behavior
type PlacementRules struct {
	AllowOverlap    bool   `json:"allow_overlap"`
	MaxAttempts     int    `json:"max_attempts"`     // per entity placement
	RetryStrategy   string `json:"retry_strategy"`   // "backtrack", "skip", "relax_constraints"
	FailureHandling string `json:"failure_handling"` // "abort", "continue", "partial"
	Timeout         int    `json:"timeout"`          // seconds
}

// GroupConstraints defines rules specific to entity groups
type GroupConstraints struct {
	MaxSpacing      float64 `json:"max_spacing"`      // maximum distance between group members
	RequiredSpacing float64 `json:"required_spacing"` // minimum distance between group members
	Formation       string  `json:"formation"`        // formation pattern ID
	LeaderBased     bool    `json:"leader_based"`     // whether group has a designated leader
}

// SpawnResult contains the outcome of a spawn operation
type SpawnResult struct {
	Success           bool               `json:"success"`
	SpawnedEntities   []SpawnedEntity    `json:"spawned_entities"`
	Failures          []SpawnFailure     `json:"failures"`
	RoomModifications []RoomModification `json:"room_modifications"`
	Metadata          SpawnMetadata      `json:"metadata"`
	Warnings          []string           `json:"warnings,omitempty"`
}

// SpawnedEntity represents a successfully placed entity
type SpawnedEntity struct {
	Entity      core.Entity            `json:"entity"`
	Position    spatial.Position       `json:"position"`
	GroupID     string                 `json:"group_id"`
	SpawnReason string                 `json:"spawn_reason"`
	Rotation    float64                `json:"rotation,omitempty"` // facing direction in degrees
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SpawnFailure represents an entity that couldn't be placed
type SpawnFailure struct {
	EntityType        string            `json:"entity_type"`
	GroupID           string            `json:"group_id"`
	Reason            string            `json:"reason"`
	AttemptedPosition *spatial.Position `json:"attempted_position,omitempty"`
	Suggestions       []string          `json:"suggestions"`
	ConstraintsFailed []string          `json:"constraints_failed"`
}

// RoomModification represents changes made to the room during spawning
type RoomModification struct {
	Type       string                 `json:"type"` // "resize", "add_feature", etc.
	Reason     string                 `json:"reason"`
	OldValue   interface{}            `json:"old_value"`
	NewValue   interface{}            `json:"new_value"`
	Reversible bool                   `json:"reversible"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SpawnMetadata provides detailed information about the spawn operation
type SpawnMetadata struct {
	TotalAttempts        int                    `json:"total_attempts"`
	SuccessfulPlacements int                    `json:"successful_placements"`
	FailedPlacements     int                    `json:"failed_placements"`
	ExecutionTime        int64                  `json:"execution_time_ms"`
	ConstraintChecks     int                    `json:"constraint_checks"`
	RoomUtilization      float64                `json:"room_utilization"`   // percentage of room used
	PatternEfficiency    float64                `json:"pattern_efficiency"` // how well pattern was achieved
	Details              map[string]interface{} `json:"details,omitempty"`
}

// Advanced Configuration Types

// TeamConfig defines team-based spawning rules
type TeamConfig struct {
	Teams           []Team                `json:"teams"`
	SeparationRules SeparationConstraints `json:"separation_rules"`
	SpawnZones      []SpawnZone           `json:"spawn_zones"`
}

// Team represents a group of entities that should be spawned together
type Team struct {
	ID            string            `json:"id"`
	EntityTypes   []string          `json:"entity_types"`
	Formation     *FormationPattern `json:"formation,omitempty"`
	PreferredZone string            `json:"preferred_zone"`
	Leadership    *LeadershipConfig `json:"leadership,omitempty"`
}

// SeparationConstraints controls distance between teams
type SeparationConstraints struct {
	MinTeamDistance float64 `json:"min_team_distance"`
	PreferOpposite  bool    `json:"prefer_opposite"` // place teams on opposite sides
	NeutralZone     float64 `json:"neutral_zone"`    // empty space between teams
}

// SpawnZone defines areas within a room for specific types of spawning
type SpawnZone struct {
	ID          string             `json:"id"`
	Type        string             `json:"type"`         // "team", "neutral", "special"
	Boundaries  []spatial.Position `json:"boundaries"`   // polygon defining zone
	EntityTypes []string           `json:"entity_types"` // what can spawn here
	MaxCapacity int                `json:"max_capacity"`
	Priority    int                `json:"priority"`
}

// FormationConfig defines structured placement patterns
type FormationConfig struct {
	Patterns   []FormationPattern  `json:"patterns"`
	AllowMixed bool                `json:"allow_mixed"` // multiple patterns in same room
	Adaptation FormationAdaptation `json:"adaptation"`
}

// FormationPattern defines a specific arrangement of entities
type FormationPattern struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Positions   []RelativePosition   `json:"positions"`
	Scaling     FormationScaling     `json:"scaling"`
	Constraints FormationConstraints `json:"constraints"`
}

// RelativePosition defines position relative to formation center
type RelativePosition struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Role     string  `json:"role,omitempty"`     // "leader", "support", "flanker"
	Optional bool    `json:"optional,omitempty"` // can be skipped if needed
}

// FormationScaling controls how formations adapt to different situations
type FormationScaling struct {
	AllowRotation   bool    `json:"allow_rotation"`
	AllowStretching bool    `json:"allow_stretching"`
	PreserveRatios  bool    `json:"preserve_ratios"`
	MinScale        float64 `json:"min_scale"`
	MaxScale        float64 `json:"max_scale"`
}

// FormationConstraints defines requirements for formation use
type FormationConstraints struct {
	MinEntities   int      `json:"min_entities"`
	MaxEntities   int      `json:"max_entities"`
	RequiredRoles []string `json:"required_roles,omitempty"`
	MinRoomSize   float64  `json:"min_room_size"`
}

// FormationAdaptation controls how formations adapt to constraints
type FormationAdaptation struct {
	AllowPartial    bool   `json:"allow_partial"`    // use partial formation if full doesn't fit
	FallbackPattern string `json:"fallback_pattern"` // what to use if primary fails
	AdaptiveSpacing bool   `json:"adaptive_spacing"` // adjust spacing to fit room
}

// LeadershipConfig defines leader-follower relationships
type LeadershipConfig struct {
	HasLeader     bool    `json:"has_leader"`
	LeaderType    string  `json:"leader_type,omitempty"`
	FollowerRules string  `json:"follower_rules"` // "stay_close", "formation", "guard"
	MaxDistance   float64 `json:"max_distance"`   // max distance from leader
}

// ScalingConfig controls adaptive room scaling
type ScalingConfig struct {
	Enabled        bool    `json:"enabled"`
	MaxAttempts    int     `json:"max_attempts"`
	ScalingFactor  float64 `json:"scaling_factor"`  // how much to scale each attempt
	PreserveAspect bool    `json:"preserve_aspect"` // maintain room proportions
	EmitEvents     bool    `json:"emit_events"`     // publish scaling events
	MaxScale       float64 `json:"max_scale"`       // maximum scale multiplier
}
