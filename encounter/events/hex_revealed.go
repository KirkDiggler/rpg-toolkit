package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// HexRevealedEvent is published whenever a player's vision gains hexes
// (or, eventually, newly visible entities), regardless of cause. Move,
// OpenDoor, LightChanged, ConditionRemoved (blind wearing off), etc. all
// emit HexRevealedEvent alongside their own action event.
//
// The cause stays in the parallel action event; this event describes the
// effect on perception with the same shape across all causes.
type HexRevealedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	PerPlayer map[core.PlayerID]HexRevealedSlice
}

// HexRevealedSlice is each viewer's projection — newly visible hexes
// and (in future slices) entities for that player.
//
// Slice 1 emits Hexes only; Entities is reserved for shape stability so
// future slices can add entity-visibility accumulation without a JSON
// migration.
type HexRevealedSlice struct {
	Hexes    core.HexSet        `json:"hexes"`
	Entities []EntityVisibility `json:"entities,omitempty"`
}

// EntityVisibility names an entity that has become visible to a player.
// Reserved for future slices; not emitted in slice 1.
type EntityVisibility struct {
	EntityID core.EntityID `json:"entity_id"`
	Position core.Hex      `json:"position"`
}

// NewHexRevealedEvent constructs a HexRevealedEvent.
func NewHexRevealedEvent(
	encID core.EncounterID,
	seq uint64,
	perPlayer map[core.PlayerID]HexRevealedSlice,
) *HexRevealedEvent {
	return &HexRevealedEvent{
		encID:     encID,
		seq:       seq,
		PerPlayer: perPlayer,
	}
}

func (*HexRevealedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *HexRevealedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *HexRevealedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players whose vision changed, derived from PerPlayer keys.
func (e *HexRevealedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type hexRevealedWire struct {
	metaWire
	EncID     core.EncounterID                   `json:"encounter_id"`
	Seq       uint64                             `json:"sequence"`
	PerPlayer map[core.PlayerID]HexRevealedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *HexRevealedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(hexRevealedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *HexRevealedEvent) UnmarshalJSON(b []byte) error {
	var w hexRevealedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.PerPlayer = w.PerPlayer
	return nil
}
