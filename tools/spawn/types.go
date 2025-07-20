package spawn

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SpawnConfig specifies how to spawn entities in a space
// Purpose: Comprehensive configuration for spawn operations supporting
// team coordination, player choice, and spatial constraints
type SpawnConfig struct {
	// What to spawn
	EntityGroups []EntityGroup `json:"entity_groups"`

	// How to spawn
	Pattern          SpawnPattern        `json:"pattern"`
	TeamConfiguration *TeamConfig        `json:"team_config,omitempty"`

	// Constraints
	SpatialRules SpatialConstraints `json:"spatial_rules"`
	Placement    PlacementRules     `json:"placement"`

	// Behavior
	Strategy        SpawnStrategy   `json:"strategy"`
	AdaptiveScaling *ScalingConfig  `json:"adaptive_scaling,omitempty"`

	// Player spawn zones and choices
	PlayerSpawnZones []SpawnZone         `json:"player_spawn_zones,omitempty"`
	PlayerChoices    []PlayerSpawnChoice `json:"player_choices,omitempty"`
}

// EntityGroup represents a group of entities to spawn
// Purpose: Groups entities by type with selection and quantity rules
type EntityGroup struct {
	ID             string              `json:"id"`               // Group identifier
	Type           string              `json:"type"`             // "player", "enemy", "treasure", etc.
	SelectionTable string              `json:"selection_table"`  // selectables table ID
	Entities       []core.Entity       `json:"entities"`         // Pre-provided entities (alternative to selection)
	Quantity       QuantitySpec        `json:"quantity"`
	Priority       int                 `json:"priority"`         // for conflict resolution
	TeamID         string              `json:"team_id,omitempty"` // for team-based spawning
}

// QuantitySpec specifies how many entities to spawn
// Purpose: Flexible quantity determination using fixed, dice, or range
type QuantitySpec struct {
	Fixed    *int      `json:"fixed,omitempty"`     // Exact number
	DiceRoll *string   `json:"dice_roll,omitempty"` // "2d6+1"
	MinMax   *MinMax   `json:"min_max,omitempty"`   // Random range
}

// MinMax represents a numeric range
type MinMax struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// SpawnPattern represents different spawn arrangement patterns
type SpawnPattern string

const (
	// PatternFormation arranges entities in structured formations
	PatternFormation SpawnPattern = "formation"
	// PatternClustered groups entities with spacing
	PatternClustered SpawnPattern = "clustered"
	// PatternScattered distributes entities randomly
	PatternScattered SpawnPattern = "scattered"
	// PatternTeamBased separates teams into distinct areas
	PatternTeamBased SpawnPattern = "team_based"
	// PatternPlayerChoice allows players to choose positions
	PatternPlayerChoice SpawnPattern = "player_choice"
	// PatternCustom uses user-defined patterns
	PatternCustom SpawnPattern = "custom"
)

// SpawnStrategy represents spawn decision-making approaches
type SpawnStrategy string

const (
	// StrategyDeterministic produces same result each time
	StrategyDeterministic SpawnStrategy = "deterministic"
	// StrategyRandomized uses random placement within constraints
	StrategyRandomized SpawnStrategy = "randomized"
	// StrategyBalanced optimizes for gameplay balance
	StrategyBalanced SpawnStrategy = "balanced"
)

// TeamConfig configures team-based spawning behavior
// Purpose: Enables team cohesion and separation for tactical gameplay
type TeamConfig struct {
	Teams           []Team                `json:"teams"`
	CohesionRules   TeamCohesionRules     `json:"cohesion_rules"`
	SeparationRules SeparationConstraints `json:"separation_rules"`
	SpawnZones      []SpawnZone           `json:"spawn_zones,omitempty"`
}

// Team represents a group of allied entities
// Purpose: Defines team membership and formation preferences
type Team struct {
	ID               string                `json:"id"`                // "friendlies", "enemies", "neutrals"
	EntityTypes      []string              `json:"entity_types"`      // Entity types for this team
	Formation        *FormationPattern     `json:"formation,omitempty"` // How team members are arranged
	PreferredZone    string                `json:"preferred_zone,omitempty"` // Spawn zone for this team
	Cohesion         float64               `json:"cohesion"`          // How tightly grouped (0.0-1.0)
}

// TeamCohesionRules configures team clustering behavior
// Purpose: Controls whether teams stay together or mix
type TeamCohesionRules struct {
	KeepFriendliesTogether bool    `json:"keep_friendlies_together"`
	KeepEnemiesTogether    bool    `json:"keep_enemies_together"`
	MinTeamSeparation      float64 `json:"min_team_separation"`
}

// SeparationConstraints defines team separation requirements
// Purpose: Ensures teams maintain tactical distances
type SeparationConstraints struct {
	MinTeamDistance  float64               `json:"min_team_distance"`  // Minimum distance between teams
	BufferZones      []BufferZone          `json:"buffer_zones"`       // No-spawn areas between teams
	TeamPlacement    TeamPlacementStrategy `json:"team_placement"`     // "corners", "opposite_sides", "random"
}

// TeamPlacementStrategy represents team positioning approaches
type TeamPlacementStrategy string

const (
	// TeamPlacementCorners places teams in room corners
	TeamPlacementCorners TeamPlacementStrategy = "corners"
	// TeamPlacementOppositeSides places teams on opposite sides
	TeamPlacementOppositeSides TeamPlacementStrategy = "opposite_sides"
	// TeamPlacementRandom places teams randomly with separation
	TeamPlacementRandom TeamPlacementStrategy = "random"
)

// SpawnZone represents a designated spawning area
// Purpose: Defines where specific entity types can spawn
type SpawnZone struct {
	ID          string            `json:"id"`           // "player_zone_north"
	Area        spatial.Rectangle `json:"area"`         // Zone boundaries
	EntityTypes []string          `json:"entity_types"` // Allowed entity types
	MaxEntities int               `json:"max_entities"` // Maximum entities in zone
	TeamID      string            `json:"team_id,omitempty"` // Restricted to specific team
}

// PlayerSpawnChoice represents a player's spawn position choice
// Purpose: Enables player agency in spawn positioning
type PlayerSpawnChoice struct {
	PlayerID string            `json:"player_id"`
	ZoneID   string            `json:"zone_id"`     // Which zone they picked
	Position spatial.Position  `json:"position"`    // Exact spot in zone
}

// BufferZone represents a no-spawn area
// Purpose: Creates separation between teams
type BufferZone struct {
	Area        spatial.Rectangle `json:"area"`
	Description string            `json:"description"`
}

// FormationPattern represents structured entity arrangements
// Purpose: Defines how entities are positioned relative to each other
type FormationPattern struct {
	Name         string                  `json:"name"`
	Positions    []RelativePosition      `json:"positions"`
	Scaling      FormationScaling        `json:"scaling"`
	Constraints  FormationConstraints    `json:"constraints"`
}

// RelativePosition represents a position relative to formation center
type RelativePosition struct {
	X        float64 `json:"x"`        // Relative X offset
	Y        float64 `json:"y"`        // Relative Y offset
	Priority int     `json:"priority"` // Fill order priority
}

// FormationScaling configures formation flexibility
type FormationScaling struct {
	AllowRotation   bool `json:"allow_rotation"`
	AllowStretching bool `json:"allow_stretching"`
	PreserveRatios  bool `json:"preserve_ratios"`
}

// FormationConstraints defines formation placement rules
type FormationConstraints struct {
	MinSpacing      float64 `json:"min_spacing"`
	MaxSpacing      float64 `json:"max_spacing"`
	RequiredClear   bool    `json:"required_clear"`   // Must have clear formation area
	AdaptToTerrain  bool    `json:"adapt_to_terrain"` // Adjust for obstacles
}

// SpatialConstraints defines spatial placement rules
// Purpose: Controls entity spacing and positioning requirements
type SpatialConstraints struct {
	MinDistance      map[string]float64 `json:"min_distance"`       // Between entity types
	LineOfSight      LineOfSightRules   `json:"line_of_sight"`
	AreaOfEffect     map[string]float64 `json:"area_of_effect"`     // Buffer zones
	PathingRules     PathingConstraints `json:"pathing"`
	WallProximity    float64            `json:"wall_proximity"`     // Min distance from walls
}

// LineOfSightRules defines visibility requirements
type LineOfSightRules struct {
	RequiredSight []EntityPair `json:"required_sight"` // Must see each other
	BlockedSight  []EntityPair `json:"blocked_sight"`  // Must NOT see each other
	CheckWalls    bool         `json:"check_walls"`    // Consider wall blocking
}

// EntityPair represents a relationship between entity types
type EntityPair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PathingConstraints defines movement and accessibility rules
type PathingConstraints struct {
	MaintainExitAccess   bool     `json:"maintain_exit_access"`
	MinPathWidth         float64  `json:"min_path_width"`
	RequiredConnections  []string `json:"required_connections"` // Connection IDs
}

// PlacementRules defines general placement behavior
type PlacementRules struct {
	AllowOverlap      bool    `json:"allow_overlap"`
	RequireLineOfSight bool   `json:"require_line_of_sight"`
	MaxAttempts       int     `json:"max_attempts"`
	TimeoutSeconds    int     `json:"timeout_seconds"`
	FallbackStrategy  string  `json:"fallback_strategy"`
}

// ScalingConfig configures adaptive room scaling
type ScalingConfig struct {
	Enabled         bool    `json:"enabled"`
	MaxAttempts     int     `json:"max_attempts"`
	ScalingFactor   float64 `json:"scaling_factor"`
	PreserveAspect  bool    `json:"preserve_aspect"`
	EmitEvents      bool    `json:"emit_events"`
}

// SpawnResult contains the results of a spawn operation
// Purpose: Comprehensive spawn operation results with split-awareness
type SpawnResult struct {
	Success              bool                      `json:"success"`
	SpawnedEntities      []SpawnedEntity           `json:"spawned_entities"`
	Failures             []SpawnFailure            `json:"failures"`
	RoomModifications    []RoomModification        `json:"room_modifications"`
	SplitRecommendations []RoomSplit  `json:"split_recommendations"`  // Passthrough from environment
	RoomStructure        RoomStructureInfo         `json:"room_structure"`         // What structure was used
	Metadata             SpawnMetadata             `json:"metadata"`
}

// SpawnedEntity represents a successfully spawned entity
// Purpose: Tracks where entities were placed with full context
type SpawnedEntity struct {
	Entity      core.Entity       `json:"entity"`
	Position    spatial.Position  `json:"position"`
	RoomID      string            `json:"room_id"`      // Which room in split configuration
	GroupID     string            `json:"group_id"`
	TeamID      string            `json:"team_id,omitempty"`
	SpawnReason string            `json:"spawn_reason"`
}

// SpawnFailure represents a failed entity placement
type SpawnFailure struct {
	EntityType        string                `json:"entity_type"`
	GroupID           string                `json:"group_id"`
	Reason            string                `json:"reason"`
	AttemptedPosition *spatial.Position     `json:"attempted_position,omitempty"`
	ConstraintsFailed []string              `json:"constraints_failed,omitempty"`
	Suggestions       []string              `json:"suggestions"`
}

// RoomModification represents changes made to room structure
type RoomModification struct {
	Type        string      `json:"type"`        // "scaled", "split", "modified"
	RoomID      string      `json:"room_id"`
	OldValue    interface{} `json:"old_value"`
	NewValue    interface{} `json:"new_value"`
	Reason      string      `json:"reason"`
	Timestamp   time.Time   `json:"timestamp"`
}

// RoomStructureInfo provides information about room configuration
// Purpose: Enables split-aware spawning logic
type RoomStructureInfo struct {
	IsSplit         bool     `json:"is_split"`
	ConnectedRooms  []string `json:"connected_rooms"`
	ConnectionTypes []string `json:"connection_types"`
	TotalCapacity   int      `json:"total_capacity"`
	PrimaryRoomID   string   `json:"primary_room_id"`
}

// SpawnMetadata contains operational information about spawn process
type SpawnMetadata struct {
	ExecutionTime     time.Duration   `json:"execution_time"`
	AttemptCounts     map[string]int  `json:"attempt_counts"`     // Per entity type
	ConstraintStats   ConstraintStats `json:"constraint_stats"`
	PerformanceMetrics map[string]float64 `json:"performance_metrics"`
}

// ConstraintStats tracks constraint satisfaction during spawning
type ConstraintStats struct {
	TotalConstraints    int     `json:"total_constraints"`
	SatisfiedConstraints int    `json:"satisfied_constraints"`
	ViolatedConstraints []string `json:"violated_constraints"`
	QualityScore        float64  `json:"quality_score"` // 0.0-1.0
}

// HelperConfig provides convenience configuration for common spawn patterns
// Purpose: Optional helper configs for common patterns without factory pattern
type HelperConfig struct {
	Purpose         string  `json:"purpose"`          // "combat", "exploration", "boss"
	Difficulty      int     `json:"difficulty"`       // 1-5 scale
	SpatialFeeling  string  `json:"spatial_feeling"`  // "tight", "normal", "vast"
	TeamSeparation  bool    `json:"team_separation"`  // Keep teams separated
	PlayerChoice    bool    `json:"player_choice"`    // Allow player positioning
	AutoScale       bool    `json:"auto_scale"`       // Scale room if needed
}