package spawn

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// SpawnConfig specifies how to spawn entities in a room.
// Purpose: Complete configuration for entity placement following ADR-0013 patterns.
//
//nolint:revive // type name follows ADR-0013 public API requirements
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

	// Seed makes this call's position search reproducible: the same room
	// state plus the same Seed always yields the same searched positions
	// (both the unconstrained and constraint-aware paths, including
	// gridless sampling). Nil (default) uses the engine's own
	// non-deterministic random source. Does not affect SelectablesRegistry
	// entity-selection order, which is a separate random source
	// (rpg-toolkit#760).
	Seed *int64 `json:"seed,omitempty"`
}

// EntityGroup represents a group of entities to spawn.
// Purpose: Defines entity type, selection table, and quantity for spawning.
type EntityGroup struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	SelectionTable string       `json:"selection_table"`
	Quantity       QuantitySpec `json:"quantity"`

	// FixedPositions optionally supplies exact positions to use for this
	// group's entities, in order, bypassing the search entirely. A fixed
	// position is still checked against the real room (Room.CanPlaceEntity)
	// before use; an invalid one is reported as a SpawnFailure rather than
	// silently placing the entity on a wall or out of bounds. If shorter
	// than the group's quantity, the remaining entities fall through to the
	// normal search (respecting PositionOracle and SpatialRules as usual).
	FixedPositions []spatial.Position `json:"fixed_positions,omitempty"`

	// PositionOracle optionally supplies a caller-defined predicate that a
	// searched candidate position must satisfy, in addition to room-derived
	// validity (bounds and walls/occupancy via Room.CanPlaceEntity) and any
	// SpatialRules. This lets a caller express a placement requirement the
	// constraint vocabulary doesn't cover yet — e.g. "outside these specific
	// viewers' sight" — directly in the search, instead of discarding the
	// engine's chosen position and recomputing placement itself
	// (rpg-toolkit#760). Not serializable to JSON; nil means no extra
	// filter. Applies only to entities in this group that fall through to
	// search (i.e. beyond len(FixedPositions)).
	PositionOracle PositionOracle `json:"-"`
}

// PositionOracle is a caller-supplied predicate for accepting or rejecting a
// candidate position during search. It is ANDed with room-derived validity
// (Room.CanPlaceEntity) and any SpatialRules — it composes with them, it
// does not replace them.
type PositionOracle func(pos spatial.Position) bool

// QuantitySpec specifies how many entities to spawn.
// Purpose: Supports fixed quantities, dice expressions, and ranges for flexible spawning.
type QuantitySpec struct {
	Fixed *int `json:"fixed,omitempty"`
}

// SpawnPattern defines how entities are arranged in space.
// Purpose: Categorizes different spatial arrangement strategies per ADR-0013.
//
//nolint:revive // type name follows ADR-0013 public API requirements
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
//
//nolint:revive // type name follows ADR-0013 public API requirements
type SpawnStrategy string

const (
	// StrategyRandomized uses random placement within constraints
	StrategyRandomized SpawnStrategy = "randomized"
	// StrategyDeterministic produces consistent results
	StrategyDeterministic SpawnStrategy = "deterministic"
	// StrategyBalanced optimizes for gameplay balance
	StrategyBalanced SpawnStrategy = "balanced"
)
