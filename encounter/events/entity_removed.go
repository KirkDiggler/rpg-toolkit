//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EntityRemovedEvent is the effect / state-mutation event published when an
// entity is removed from the encounter's authoritative state. Wave 2.10
// fires it for monsters whose HP hit zero (Reason = "destroyed"); future
// waves use it for "fled", "transformed", etc.
//
// Audience is BROADCAST — every player in the encounter is in PerPlayer
// with Visible: true. State removal is a global concept: even players who
// could not see the death need to drop the entity from their local mirror,
// otherwise their view diverges from server truth.
//
// Maps to the proto EntityRemoved wire shape.
type EntityRemovedEvent struct {
	encID     core.EncounterID
	seq       uint64
	EntityID  core.EntityID
	Reason    string
	PerPlayer map[core.PlayerID]EntityRemovedSlice
}

// EntityRemovedSlice is each viewer's projection. Visible is always true
// for entity-removal events — every player observes state mutations.
type EntityRemovedSlice struct {
	Visible bool `json:"visible"`
}

// NewEntityRemovedEvent constructs an EntityRemovedEvent.
func NewEntityRemovedEvent(
	encID core.EncounterID,
	seq uint64,
	entityID core.EntityID,
	reason string,
	perPlayer map[core.PlayerID]EntityRemovedSlice,
) *EntityRemovedEvent {
	return &EntityRemovedEvent{
		encID:     encID,
		seq:       seq,
		EntityID:  entityID,
		Reason:    reason,
		PerPlayer: perPlayer,
	}
}

func (*EntityRemovedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *EntityRemovedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *EntityRemovedEvent) Sequence() uint64 { return e.seq }

// Audience returns every player who should see the removal — always all
// players in the encounter, since the entity is globally gone.
func (e *EntityRemovedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type entityRemovedWire struct {
	EncID     core.EncounterID                     `json:"encounter_id"`
	Seq       uint64                               `json:"sequence"`
	EntityID  core.EntityID                        `json:"entity_id"`
	Reason    string                               `json:"reason,omitempty"`
	PerPlayer map[core.PlayerID]EntityRemovedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *EntityRemovedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(entityRemovedWire{
		EncID:     e.encID,
		Seq:       e.seq,
		EntityID:  e.EntityID,
		Reason:    e.Reason,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EntityRemovedEvent) UnmarshalJSON(b []byte) error {
	var w entityRemovedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.EntityID = w.EntityID
	e.Reason = w.Reason
	e.PerPlayer = w.PerPlayer
	return nil
}
