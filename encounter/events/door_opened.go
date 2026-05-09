//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// DoorOpenedEvent is published when an entity opens a door in the encounter.
//
// Vision changes (newly revealed hexes through the door) ride on a parallel
// HexRevealedEvent published alongside this one — see the decoupled
// cause/effect decision.
type DoorOpenedEvent struct {
	encID     core.EncounterID
	seq       uint64
	DoorID    core.EntityID
	OpenedBy  core.EntityID
	PerPlayer map[core.PlayerID]DoorOpenedPlayerSlice
}

// DoorOpenedPlayerSlice is each viewer's projection. Visible says whether
// the player perceived the door at all (via their own LoS).
type DoorOpenedPlayerSlice struct {
	Visible bool `json:"visible"`
}

// NewDoorOpenedEvent constructs a DoorOpenedEvent.
func NewDoorOpenedEvent(
	encID core.EncounterID,
	seq uint64,
	door core.EntityID,
	openedBy core.EntityID,
	perPlayer map[core.PlayerID]DoorOpenedPlayerSlice,
) *DoorOpenedEvent {
	return &DoorOpenedEvent{
		encID:     encID,
		seq:       seq,
		DoorID:    door,
		OpenedBy:  openedBy,
		PerPlayer: perPlayer,
	}
}

func (*DoorOpenedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *DoorOpenedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *DoorOpenedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the open-door event,
// derived from PerPlayer keys.
func (e *DoorOpenedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type doorOpenedWire struct {
	EncID     core.EncounterID                        `json:"encounter_id"`
	Seq       uint64                                  `json:"sequence"`
	DoorID    core.EntityID                           `json:"door_id"`
	OpenedBy  core.EntityID                           `json:"opened_by"`
	PerPlayer map[core.PlayerID]DoorOpenedPlayerSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *DoorOpenedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(doorOpenedWire{
		EncID: e.encID, Seq: e.seq,
		DoorID: e.DoorID, OpenedBy: e.OpenedBy,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *DoorOpenedEvent) UnmarshalJSON(b []byte) error {
	var w doorOpenedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.DoorID = w.DoorID
	e.OpenedBy = w.OpenedBy
	e.PerPlayer = w.PerPlayer
	return nil
}
