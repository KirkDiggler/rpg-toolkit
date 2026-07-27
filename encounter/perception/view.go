package perception

import "github.com/KirkDiggler/rpg-toolkit/encounter/core"

// View is what a single player currently knows about an encounter.
// Persisted on EncounterData; rehydrated on LoadFromData.
//
// rpg-toolkit#851: Memory replaces the old RevealedHexes (core.HexSet — a
// set of COORDINATES with no payload, recording only THAT a hex was seen,
// never WHAT was there). That was the single reason a Remembered
// observation could not be produced at all. Memory keys are exactly the
// hexes RevealedHexes used to hold; the payload is now a full
// HexObservation instead of nothing.
//
// The remaining fields are reserved for shape stability — when conditions,
// senses, and entity-knowledge accumulation land in future slices,
// persisted JSON won't need a migration.
type View struct {
	PlayerID   core.PlayerID `json:"player_id"`
	Position   core.Hex      `json:"position"`
	SightRange int           `json:"sight_range"`
	Memory     Memory        `json:"memory"`

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

// NewView constructs a View with an empty memory.
func NewView(playerID core.PlayerID, position core.Hex, sightRange int) *View {
	return &View{
		PlayerID:   playerID,
		Position:   position,
		SightRange: sightRange,
		Memory:     NewMemory(),
	}
}

// Knows reports whether h has ever been observed by this viewer — the same
// "is this hex known at all, regardless of current visibility" question
// RevealedHexes.Has used to answer. Safe on a nil View or nil Memory.
func (v *View) Knows(h core.Hex) bool {
	if v == nil {
		return false
	}
	return v.Memory.Knows(h)
}

// KnownHexSet returns the set of hex positions this viewer has ever
// observed — a core.HexSet view of Memory's keys, for callers that need
// plain membership/diffing (e.g. the wire's sticky "first reveal" delta)
// without the full per-hex observation payload. Safe on a nil View.
func (v *View) KnownHexSet() core.HexSet {
	if v == nil {
		return nil
	}
	out := make(core.HexSet, len(v.Memory))
	for h := range v.Memory {
		out[h] = struct{}{}
	}
	return out
}

// Observe records obs as this viewer's latest authorized observation of
// obs.Position, replacing whatever was there wholesale. See Memory.Observe.
func (v *View) Observe(obs HexObservation) {
	if v.Memory == nil {
		v.Memory = NewMemory()
	}
	v.Memory.Observe(obs)
}
