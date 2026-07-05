//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EntityStabilizedEvent is the cause / narrative event published when an
// unconscious entity accumulates 3 successful death saves (rpg-toolkit#741).
// Mirrors EntityDiedEvent's shape — same per-viewer projection template, no
// killer/causer concept since stabilization has no attacker. Maps to the
// proto EntityStabilized wire shape.
type EntityStabilizedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	EntityID  core.EntityID
	PerPlayer map[core.PlayerID]EntityStabilizedSlice
}

// EntityStabilizedSlice is each viewer's projection. Visible says whether
// the player perceived the stabilization.
type EntityStabilizedSlice struct {
	Visible bool `json:"visible"`
}

// NewEntityStabilizedEvent constructs an EntityStabilizedEvent.
func NewEntityStabilizedEvent(
	encID core.EncounterID,
	seq uint64,
	entityID core.EntityID,
	perPlayer map[core.PlayerID]EntityStabilizedSlice,
) *EntityStabilizedEvent {
	return &EntityStabilizedEvent{
		encID:     encID,
		seq:       seq,
		EntityID:  entityID,
		PerPlayer: perPlayer,
	}
}

func (*EntityStabilizedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *EntityStabilizedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *EntityStabilizedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the stabilization,
// derived from PerPlayer keys.
func (e *EntityStabilizedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type entityStabilizedWire struct {
	metaWire
	EncID     core.EncounterID                        `json:"encounter_id"`
	Seq       uint64                                  `json:"sequence"`
	EntityID  core.EntityID                           `json:"entity_id"`
	PerPlayer map[core.PlayerID]EntityStabilizedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *EntityStabilizedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(entityStabilizedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		EntityID:  e.EntityID,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EntityStabilizedEvent) UnmarshalJSON(b []byte) error {
	var w entityStabilizedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.EntityID = w.EntityID
	e.PerPlayer = w.PerPlayer
	return nil
}
