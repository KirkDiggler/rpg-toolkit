package perception

import "github.com/KirkDiggler/rpg-toolkit/encounter/types"

// PerceptionView is what a single player currently knows about an encounter.
// Persisted on EncounterData; rehydrated on LoadFromData.
//
// Slice 1 only uses Position, SightRange, RevealedHexes. The remaining
// fields are reserved for shape stability — when conditions, senses, and
// entity-knowledge accumulation land in future slices, persisted JSON
// won't need a migration.
type PerceptionView struct {
	PlayerID      types.PlayerID `json:"player_id"`
	Position      types.Hex      `json:"position"`
	SightRange    int            `json:"sight_range"`
	RevealedHexes types.HexSet   `json:"revealed_hexes"`

	// Future-slice fields — emitted as zero values for now.
	KnownEntities map[types.EntityID]EntityKnowledge `json:"known_entities,omitempty"`
	ActiveSenses  []Sense                            `json:"active_senses,omitempty"`
	Conditions    []types.EntityID                   `json:"conditions,omitempty"`
}

// EntityKnowledge is reserved for entity-visibility accumulation in future slices.
type EntityKnowledge struct {
	LastSeenPosition types.Hex `json:"last_seen_position"`
	Identified       bool      `json:"identified"`
}

// Sense is reserved for senses (darkvision, blindsight, ...) in future slices.
type Sense struct {
	Kind  string `json:"kind"`
	Range int    `json:"range"`
}

// NewView constructs a PerceptionView with the empty cumulative reveal set.
func NewView(playerID types.PlayerID, position types.Hex, sightRange int) *PerceptionView {
	return &PerceptionView{
		PlayerID:      playerID,
		Position:      position,
		SightRange:    sightRange,
		RevealedHexes: make(types.HexSet),
	}
}

// ApplyReveal merges newly-revealed hexes into the cumulative set. Idempotent.
func (v *PerceptionView) ApplyReveal(hexes types.HexSet) {
	if v.RevealedHexes == nil {
		v.RevealedHexes = make(types.HexSet)
	}
	for h := range hexes {
		v.RevealedHexes[h] = struct{}{}
	}
}
