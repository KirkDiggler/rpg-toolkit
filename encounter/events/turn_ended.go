package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// TurnEndedEvent is published when an actor's turn ends, before initiative
// advances to the next actor. Audience is all players in the encounter.
type TurnEndedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	ActorID   core.EntityID
	PerPlayer map[core.PlayerID]TurnEndedSlice
}

// TurnEndedSlice is each viewer's projection. Visible is always true for
// turn-state events — every player observes turn boundaries.
type TurnEndedSlice struct {
	Visible bool `json:"visible"`
}

// NewTurnEndedEvent constructs a TurnEndedEvent.
func NewTurnEndedEvent(
	encID core.EncounterID,
	seq uint64,
	actorID core.EntityID,
	perPlayer map[core.PlayerID]TurnEndedSlice,
) *TurnEndedEvent {
	return &TurnEndedEvent{
		encID:     encID,
		seq:       seq,
		ActorID:   actorID,
		PerPlayer: perPlayer,
	}
}

func (*TurnEndedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *TurnEndedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *TurnEndedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the turn-end event,
// derived from PerPlayer keys.
func (e *TurnEndedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type turnEndedWire struct {
	metaWire
	EncID     core.EncounterID                 `json:"encounter_id"`
	Seq       uint64                           `json:"sequence"`
	ActorID   core.EntityID                    `json:"actor_id"`
	PerPlayer map[core.PlayerID]TurnEndedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *TurnEndedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(turnEndedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		ActorID:   e.ActorID,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *TurnEndedEvent) UnmarshalJSON(b []byte) error {
	var w turnEndedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.ActorID = w.ActorID
	e.PerPlayer = w.PerPlayer
	return nil
}
