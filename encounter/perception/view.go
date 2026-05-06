package perception

import "github.com/KirkDiggler/rpg-toolkit/encounter/core"

// View is what a single player currently knows about an encounter.
// Persisted on EncounterData; rehydrated on LoadFromData.
//
// Slice 1 only uses Position, SightRange, RevealedHexes. The remaining
// fields are reserved for shape stability — when conditions, senses, and
// entity-knowledge accumulation land in future slices, persisted JSON
// won't need a migration.
type View struct {
	PlayerID      core.PlayerID `json:"player_id"`
	Position      core.Hex      `json:"position"`
	SightRange    int           `json:"sight_range"`
	RevealedHexes core.HexSet   `json:"revealed_hexes"`

	// Future-slice fields — emitted as zero values for now.
	KnownEntities map[core.EntityID]EntityKnowledge `json:"known_entities,omitempty"`
	ActiveSenses  []Sense                           `json:"active_senses,omitempty"`
	Conditions    []core.EntityID                   `json:"conditions,omitempty"`
}

// EntityKnowledge is reserved for entity-visibility accumulation in future slices.
type EntityKnowledge struct {
	LastSeenPosition core.Hex `json:"last_seen_position"`
	Identified       bool     `json:"identified"`
}

// Sense is reserved for senses (darkvision, blindsight, ...) in future slices.
type Sense struct {
	Kind  string `json:"kind"`
	Range int    `json:"range"`
}

// NewView constructs a View with the empty cumulative reveal set.
func NewView(playerID core.PlayerID, position core.Hex, sightRange int) *View {
	return &View{
		PlayerID:      playerID,
		Position:      position,
		SightRange:    sightRange,
		RevealedHexes: make(core.HexSet),
	}
}

// ApplyReveal merges newly-revealed hexes into the cumulative set. Idempotent.
func (v *View) ApplyReveal(hexes core.HexSet) {
	if v.RevealedHexes == nil {
		v.RevealedHexes = make(core.HexSet)
	}
	for h := range hexes {
		v.RevealedHexes[h] = struct{}{}
	}
}
