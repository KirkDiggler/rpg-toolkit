package spawn

// SpawnConfig specifies how to spawn entities in a room.
// Purpose: Complete configuration for entity placement following ADR-0013 patterns.
type SpawnConfig struct {
	// What to spawn
	EntityGroups []EntityGroup `json:"entity_groups"`

	// How to spawn
	Pattern           SpawnPattern `json:"pattern"`
	TeamConfiguration *TeamConfig  `json:"team_config,omitempty"`

	// Constraints
	SpatialRules SpatialConstraints `json:"spatial_rules"`
	Placement    PlacementRules     `json:"placement"`

	// Behavior
	Strategy        SpawnStrategy  `json:"strategy"`
	AdaptiveScaling *ScalingConfig `json:"adaptive_scaling,omitempty"`

	// Player spawn zones and choices
	PlayerSpawnZones []SpawnZone         `json:"player_spawn_zones,omitempty"`
	PlayerChoices    []PlayerSpawnChoice `json:"player_choices,omitempty"`
}

// EntityGroup represents a group of entities to spawn.
// Purpose: Defines entity type, selection table, and quantity for spawning.
type EntityGroup struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	SelectionTable string       `json:"selection_table"`
	Quantity       QuantitySpec `json:"quantity"`
}

// QuantitySpec specifies how many entities to spawn.
// Purpose: Supports fixed quantities, dice expressions, and ranges for flexible spawning.
type QuantitySpec struct {
	Fixed *int `json:"fixed,omitempty"`
}

// SpawnPattern defines how entities are arranged in space.
// Purpose: Categorizes different spatial arrangement strategies per ADR-0013.
type SpawnPattern string

const (
	// PatternScattered distributes entities randomly across available space
	PatternScattered SpawnPattern = "scattered"
	// PatternFormation uses structured arrangements
	PatternFormation SpawnPattern = "formation"
	// PatternClustered groups entities with spacing
	PatternClustered SpawnPattern = "clustered"
	// PatternTeamBased separates teams into distinct areas
	PatternTeamBased SpawnPattern = "team_based"
	// PatternPlayerChoice allows players to choose positions
	PatternPlayerChoice SpawnPattern = "player_choice"
)

// SpawnStrategy defines the spawning approach.
// Purpose: Controls randomization vs deterministic behavior in entity placement.
type SpawnStrategy string

const (
	// StrategyRandomized uses random placement within constraints
	StrategyRandomized SpawnStrategy = "randomized"
	// StrategyDeterministic produces consistent results
	StrategyDeterministic SpawnStrategy = "deterministic"
	// StrategyBalanced optimizes for gameplay balance
	StrategyBalanced SpawnStrategy = "balanced"
)
