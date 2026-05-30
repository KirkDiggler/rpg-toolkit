package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// ResourceChangedEvent is the broker event published when a character's
// resource (e.g. rage charges, ki) changes during an encounter verb.
// Maps to the proto ResourceChanged wire shape.
type ResourceChangedEvent struct {
	encID       core.EncounterID
	seq         uint64
	EntityID    core.EntityID
	ResourceRef string
	NewCurrent  int
	Max         int
	PerPlayer   map[core.PlayerID]ResourceChangedSlice
}

// ResourceChangedSlice is each viewer's projection of the resource change.
type ResourceChangedSlice struct {
	Visible bool `json:"visible"`
}

// NewResourceChangedEvent constructs a ResourceChangedEvent.
func NewResourceChangedEvent(
	encID core.EncounterID,
	seq uint64,
	entityID core.EntityID,
	resourceRef string,
	newCurrent, maxVal int,
	perPlayer map[core.PlayerID]ResourceChangedSlice,
) *ResourceChangedEvent {
	return &ResourceChangedEvent{
		encID:       encID,
		seq:         seq,
		EntityID:    entityID,
		ResourceRef: resourceRef,
		NewCurrent:  newCurrent,
		Max:         maxVal,
		PerPlayer:   perPlayer,
	}
}

func (*ResourceChangedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *ResourceChangedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *ResourceChangedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the resource change,
// derived from PerPlayer keys.
func (e *ResourceChangedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type resourceChangedWire struct {
	EncID       core.EncounterID                       `json:"encounter_id"`
	Seq         uint64                                 `json:"sequence"`
	EntityID    core.EntityID                          `json:"entity_id"`
	ResourceRef string                                 `json:"resource_ref"`
	NewCurrent  int                                    `json:"new_current"`
	Max         int                                    `json:"max"`
	PerPlayer   map[core.PlayerID]ResourceChangedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *ResourceChangedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(resourceChangedWire{
		EncID:       e.encID,
		Seq:         e.seq,
		EntityID:    e.EntityID,
		ResourceRef: e.ResourceRef,
		NewCurrent:  e.NewCurrent,
		Max:         e.Max,
		PerPlayer:   e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *ResourceChangedEvent) UnmarshalJSON(b []byte) error {
	var w resourceChangedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.EntityID = w.EntityID
	e.ResourceRef = w.ResourceRef
	e.NewCurrent = w.NewCurrent
	e.Max = w.Max
	e.PerPlayer = w.PerPlayer
	return nil
}
