//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EncounterEndedEvent is the terminal event published once when the
// encounter-end predicate first goes true. Wave 2.10 emits it with
// Reason = "all_hostiles_defeated"; future waves may add "fled",
// "negotiated", "tpk", "time_out", etc.
//
// Audience is BROADCAST — every player in the encounter is in PerPlayer.
// The terminal-state transition affects everyone uniformly: orchestrator
// stops dispatching turns, UI surfaces an end-state indicator, subsequent
// combat verbs return ErrEncounterEnded.
//
// Maps to the proto EncounterEnded wire shape.
type EncounterEndedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	Reason    string
	PerPlayer map[core.PlayerID]EncounterEndedSlice
}

// EncounterEndedSlice is each viewer's projection. Visible is always true
// for encounter-end events — every player observes the terminal transition.
type EncounterEndedSlice struct {
	Visible bool `json:"visible"`
}

// NewEncounterEndedEvent constructs an EncounterEndedEvent.
func NewEncounterEndedEvent(
	encID core.EncounterID,
	seq uint64,
	reason string,
	perPlayer map[core.PlayerID]EncounterEndedSlice,
) *EncounterEndedEvent {
	return &EncounterEndedEvent{
		encID:     encID,
		seq:       seq,
		Reason:    reason,
		PerPlayer: perPlayer,
	}
}

func (*EncounterEndedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *EncounterEndedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *EncounterEndedEvent) Sequence() uint64 { return e.seq }

// Audience returns every player participating in the encounter, since the
// terminal transition is globally observable.
func (e *EncounterEndedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type encounterEndedWire struct {
	metaWire
	EncID     core.EncounterID                      `json:"encounter_id"`
	Seq       uint64                                `json:"sequence"`
	Reason    string                                `json:"reason,omitempty"`
	PerPlayer map[core.PlayerID]EncounterEndedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *EncounterEndedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(encounterEndedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		Reason:    e.Reason,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EncounterEndedEvent) UnmarshalJSON(b []byte) error {
	var w encounterEndedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.Reason = w.Reason
	e.PerPlayer = w.PerPlayer
	return nil
}
