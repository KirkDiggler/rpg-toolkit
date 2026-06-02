package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// ModeChangedEvent is published when the encounter mode flips (e.g.
// FREE_ROAM <-> TURN_BASED). Audience is all players in the encounter.
type ModeChangedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	From      core.EncounterMode
	To        core.EncounterMode
	Reason    string
	PerPlayer map[core.PlayerID]ModeChangedSlice
}

// ModeChangedSlice is each viewer's projection. Visible is always true for
// mode-change events — every player observes mode flips.
type ModeChangedSlice struct {
	Visible bool `json:"visible"`
}

// NewModeChangedEvent constructs a ModeChangedEvent.
func NewModeChangedEvent(
	encID core.EncounterID,
	seq uint64,
	from core.EncounterMode,
	to core.EncounterMode,
	reason string,
	perPlayer map[core.PlayerID]ModeChangedSlice,
) *ModeChangedEvent {
	return &ModeChangedEvent{
		encID:     encID,
		seq:       seq,
		From:      from,
		To:        to,
		Reason:    reason,
		PerPlayer: perPlayer,
	}
}

func (*ModeChangedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *ModeChangedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *ModeChangedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the mode-change,
// derived from PerPlayer keys.
func (e *ModeChangedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type modeChangedWire struct {
	metaWire
	EncID     core.EncounterID                   `json:"encounter_id"`
	Seq       uint64                             `json:"sequence"`
	From      core.EncounterMode                 `json:"from"`
	To        core.EncounterMode                 `json:"to"`
	Reason    string                             `json:"reason,omitempty"`
	PerPlayer map[core.PlayerID]ModeChangedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *ModeChangedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(modeChangedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		From:      e.From,
		To:        e.To,
		Reason:    e.Reason,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *ModeChangedEvent) UnmarshalJSON(b []byte) error {
	var w modeChangedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.From = w.From
	e.To = w.To
	e.Reason = w.Reason
	e.PerPlayer = w.PerPlayer
	return nil
}
