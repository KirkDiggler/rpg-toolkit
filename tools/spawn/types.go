package spawn

// SpawnConfig specifies how to spawn entities in a room
// Phase 1: Basic configuration for scattered spawning only
type SpawnConfig struct {
	EntityGroups []EntityGroup `json:"entity_groups"`
	Pattern      SpawnPattern  `json:"pattern"`
}

// EntityGroup represents a group of entities to spawn
type EntityGroup struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	SelectionTable string       `json:"selection_table"`
	Quantity       QuantitySpec `json:"quantity"`
}

// QuantitySpec specifies how many entities to spawn
type QuantitySpec struct {
	Fixed *int `json:"fixed,omitempty"`
}

// SpawnPattern defines how entities are arranged in space
type SpawnPattern string

const (
	// PatternScattered distributes entities randomly across available space
	PatternScattered SpawnPattern = "scattered"
)
