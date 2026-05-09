//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// TurnStartedEvent is published when initiative advances to a new actor.
// Audience is all players in the encounter.
type TurnStartedEvent struct {
	encID     core.EncounterID
	seq       uint64
	ActorID   core.EntityID
	Round     int
	PerPlayer map[core.PlayerID]TurnStartedSlice
}

// TurnStartedSlice is each viewer's projection. Visible is always true for
// turn-state events — every player observes whose turn it is.
type TurnStartedSlice struct {
	Visible bool `json:"visible"`
}

// NewTurnStartedEvent constructs a TurnStartedEvent.
func NewTurnStartedEvent(
	encID core.EncounterID,
	seq uint64,
	actorID core.EntityID,
	round int,
	perPlayer map[core.PlayerID]TurnStartedSlice,
) *TurnStartedEvent {
	return &TurnStartedEvent{
		encID:     encID,
		seq:       seq,
		ActorID:   actorID,
		Round:     round,
		PerPlayer: perPlayer,
	}
}

func (*TurnStartedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *TurnStartedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *TurnStartedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the turn-start event,
// derived from PerPlayer keys.
func (e *TurnStartedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type turnStartedWire struct {
	EncID     core.EncounterID                   `json:"encounter_id"`
	Seq       uint64                             `json:"sequence"`
	ActorID   core.EntityID                      `json:"actor_id"`
	Round     int                                `json:"round"`
	PerPlayer map[core.PlayerID]TurnStartedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *TurnStartedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(turnStartedWire{
		EncID:     e.encID,
		Seq:       e.seq,
		ActorID:   e.ActorID,
		Round:     e.Round,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *TurnStartedEvent) UnmarshalJSON(b []byte) error {
	var w turnStartedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.ActorID = w.ActorID
	e.Round = w.Round
	e.PerPlayer = w.PerPlayer
	return nil
}
