package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// ConditionAppliedEvent is the effect event published when a condition is
// applied to an entity. Maps to the proto StatusApplied wire shape.
type ConditionAppliedEvent struct {
	eventMeta
	encID          core.EncounterID
	seq            uint64
	TargetID       core.EntityID
	SourceID       core.EntityID
	ConditionRef   string
	DurationRounds int
	PerPlayer      map[core.PlayerID]ConditionAppliedSlice
}

// ConditionAppliedSlice is each viewer's projection. Visible says whether
// the player perceived the condition application.
type ConditionAppliedSlice struct {
	Visible bool `json:"visible"`
}

// NewConditionAppliedEvent constructs a ConditionAppliedEvent.
func NewConditionAppliedEvent(
	encID core.EncounterID,
	seq uint64,
	targetID core.EntityID,
	sourceID core.EntityID,
	conditionRef string,
	durationRounds int,
	perPlayer map[core.PlayerID]ConditionAppliedSlice,
) *ConditionAppliedEvent {
	return &ConditionAppliedEvent{
		encID:          encID,
		seq:            seq,
		TargetID:       targetID,
		SourceID:       sourceID,
		ConditionRef:   conditionRef,
		DurationRounds: durationRounds,
		PerPlayer:      perPlayer,
	}
}

func (*ConditionAppliedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *ConditionAppliedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *ConditionAppliedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the condition event,
// derived from PerPlayer keys.
func (e *ConditionAppliedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type conditionAppliedWire struct {
	metaWire
	EncID          core.EncounterID                        `json:"encounter_id"`
	Seq            uint64                                  `json:"sequence"`
	TargetID       core.EntityID                           `json:"target_id"`
	SourceID       core.EntityID                           `json:"source_id"`
	ConditionRef   string                                  `json:"condition_ref"`
	DurationRounds int                                     `json:"duration_rounds"`
	PerPlayer      map[core.PlayerID]ConditionAppliedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *ConditionAppliedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(conditionAppliedWire{
		metaWire:       e.toWire(),
		EncID:          e.encID,
		Seq:            e.seq,
		TargetID:       e.TargetID,
		SourceID:       e.SourceID,
		ConditionRef:   e.ConditionRef,
		DurationRounds: e.DurationRounds,
		PerPlayer:      e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *ConditionAppliedEvent) UnmarshalJSON(b []byte) error {
	var w conditionAppliedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.TargetID = w.TargetID
	e.SourceID = w.SourceID
	e.ConditionRef = w.ConditionRef
	e.DurationRounds = w.DurationRounds
	e.PerPlayer = w.PerPlayer
	return nil
}
