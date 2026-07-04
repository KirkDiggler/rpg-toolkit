package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// ConditionRemovedEvent is the effect event published when a condition is
// removed from an entity. Maps to the proto StatusRemoved wire shape.
// Mirrors ConditionAppliedEvent's structure exactly — see condition_applied.go.
type ConditionRemovedEvent struct {
	eventMeta
	encID        core.EncounterID
	seq          uint64
	TargetID     core.EntityID
	ConditionRef string
	Reason       string
	PerPlayer    map[core.PlayerID]ConditionRemovedSlice
}

// ConditionRemovedSlice is each viewer's projection. Visible says whether
// the player perceived the condition removal.
type ConditionRemovedSlice struct {
	Visible bool `json:"visible"`
}

// NewConditionRemovedEvent constructs a ConditionRemovedEvent.
func NewConditionRemovedEvent(
	encID core.EncounterID,
	seq uint64,
	targetID core.EntityID,
	conditionRef string,
	reason string,
	perPlayer map[core.PlayerID]ConditionRemovedSlice,
) *ConditionRemovedEvent {
	return &ConditionRemovedEvent{
		encID:        encID,
		seq:          seq,
		TargetID:     targetID,
		ConditionRef: conditionRef,
		Reason:       reason,
		PerPlayer:    perPlayer,
	}
}

func (*ConditionRemovedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *ConditionRemovedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *ConditionRemovedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the condition removal,
// derived from PerPlayer keys.
func (e *ConditionRemovedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type conditionRemovedWire struct {
	metaWire
	EncID        core.EncounterID                        `json:"encounter_id"`
	Seq          uint64                                  `json:"sequence"`
	TargetID     core.EntityID                           `json:"target_id"`
	ConditionRef string                                  `json:"condition_ref"`
	Reason       string                                  `json:"reason"`
	PerPlayer    map[core.PlayerID]ConditionRemovedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *ConditionRemovedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(conditionRemovedWire{
		metaWire:     e.toWire(),
		EncID:        e.encID,
		Seq:          e.seq,
		TargetID:     e.TargetID,
		ConditionRef: e.ConditionRef,
		Reason:       e.Reason,
		PerPlayer:    e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *ConditionRemovedEvent) UnmarshalJSON(b []byte) error {
	var w conditionRemovedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.TargetID = w.TargetID
	e.ConditionRef = w.ConditionRef
	e.Reason = w.Reason
	e.PerPlayer = w.PerPlayer
	return nil
}
