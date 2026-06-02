//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EntityDiedEvent is the cause / narrative event published when an entity's
// HP reaches zero. Wave 2.10 fires it for any entity (player or monster),
// though only monsters are auto-removed (see EntityRemovedEvent). KillerID
// carries the entity that landed the killing blow when known; it is empty
// for environmental damage / poison / future indirect-kill sources that
// don't track a single attacker. Per-viewer projection: a viewer is in
// PerPlayer iff they have LoS to the dying entity OR the killer.
//
// Maps to the proto EntityDied wire shape. Cause-only — see
// EntityRemovedEvent for the state mutation.
type EntityDiedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	EntityID  core.EntityID
	KillerID  core.EntityID
	PerPlayer map[core.PlayerID]EntityDiedSlice
}

// EntityDiedSlice is each viewer's projection. Visible says whether the
// player perceived the death (LoS to dying entity OR killer).
type EntityDiedSlice struct {
	Visible bool `json:"visible"`
}

// NewEntityDiedEvent constructs an EntityDiedEvent. KillerID may be empty
// when the cause has no single attributable attacker.
func NewEntityDiedEvent(
	encID core.EncounterID,
	seq uint64,
	entityID core.EntityID,
	killerID core.EntityID,
	perPlayer map[core.PlayerID]EntityDiedSlice,
) *EntityDiedEvent {
	return &EntityDiedEvent{
		encID:     encID,
		seq:       seq,
		EntityID:  entityID,
		KillerID:  killerID,
		PerPlayer: perPlayer,
	}
}

func (*EntityDiedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *EntityDiedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *EntityDiedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the death event,
// derived from PerPlayer keys.
func (e *EntityDiedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type entityDiedWire struct {
	metaWire
	EncID     core.EncounterID                  `json:"encounter_id"`
	Seq       uint64                            `json:"sequence"`
	EntityID  core.EntityID                     `json:"entity_id"`
	KillerID  core.EntityID                     `json:"killer_id,omitempty"`
	PerPlayer map[core.PlayerID]EntityDiedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *EntityDiedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(entityDiedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		EntityID:  e.EntityID,
		KillerID:  e.KillerID,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EntityDiedEvent) UnmarshalJSON(b []byte) error {
	var w entityDiedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.EntityID = w.EntityID
	e.KillerID = w.KillerID
	e.PerPlayer = w.PerPlayer
	return nil
}
