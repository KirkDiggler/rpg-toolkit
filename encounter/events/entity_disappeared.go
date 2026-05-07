package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EntityDisappearedEvent is published when a moving entity leaves one or more
// viewers' lines of sight during a move. This event lets consumers clear or
// freeze entity markers at each viewer's last-known position.
//
// PerPlayer maps each affected viewer's PlayerID to the last hex at which that
// viewer saw the entity. Different viewers may have last seen the entity at
// different hexes (e.g. two viewers in different positions during a pass-through
// move), so per-viewer last-known position is required. For movement-driven
// disappearance this is the last hex of that viewer's SeenSegments.
type EntityDisappearedEvent struct {
	encID     core.EncounterID
	seq       uint64
	Entity    core.EntityID
	PerPlayer map[core.PlayerID]core.Hex
}

// NewEntityDisappearedEvent constructs an EntityDisappearedEvent. The encounter
// is responsible for stamping the encounter ID and sequence number.
func NewEntityDisappearedEvent(
	encID core.EncounterID,
	seq uint64,
	entity core.EntityID,
	perPlayer map[core.PlayerID]core.Hex,
) *EntityDisappearedEvent {
	return &EntityDisappearedEvent{
		encID:     encID,
		seq:       seq,
		Entity:    entity,
		PerPlayer: perPlayer,
	}
}

func (*EntityDisappearedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *EntityDisappearedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *EntityDisappearedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who lost sight of the entity, derived
// from the keys of PerPlayer.
func (e *EntityDisappearedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

// entityDisappearedWire is the on-wire shape — used only by MarshalJSON / UnmarshalJSON.
type entityDisappearedWire struct {
	EncID     core.EncounterID           `json:"encounter_id"`
	Seq       uint64                     `json:"sequence"`
	Entity    core.EntityID              `json:"entity"`
	PerPlayer map[core.PlayerID]core.Hex `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *EntityDisappearedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(entityDisappearedWire{
		EncID:     e.encID,
		Seq:       e.seq,
		Entity:    e.Entity,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EntityDisappearedEvent) UnmarshalJSON(b []byte) error {
	var w entityDisappearedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.Entity = w.Entity
	e.PerPlayer = w.PerPlayer
	return nil
}
