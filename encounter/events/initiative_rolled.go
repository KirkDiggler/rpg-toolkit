//nolint:dupl // Event scaffold intentionally mirrors other concretes in this package.
package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// InitiativeRolledEvent is published once, immediately after ModeChangedEvent,
// when SetMode rolls initiative on a FREE_ROAM->TURN_BASED transition
// (rpg-toolkit#765). It carries the freshly-rolled turn order so consumers
// never need to read it back from persisted state — see SetMode's doc
// comment for the full ordering contract (ModeChanged -> InitiativeRolled ->
// TurnStarted) and why InitiativeRolled is not re-published on later,
// per-turn TurnStartedEvents. Audience is all players in the encounter — the
// roster is shared knowledge among combatants, not fog-of-war-gated.
type InitiativeRolledEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	Order     []core.EntityID
	PerPlayer map[core.PlayerID]InitiativeRolledSlice
}

// InitiativeRolledSlice is each viewer's projection. Visible is always true —
// every player in the encounter observes the rolled turn order.
type InitiativeRolledSlice struct {
	Visible bool `json:"visible"`
}

// NewInitiativeRolledEvent constructs an InitiativeRolledEvent.
func NewInitiativeRolledEvent(
	encID core.EncounterID,
	seq uint64,
	order []core.EntityID,
	perPlayer map[core.PlayerID]InitiativeRolledSlice,
) *InitiativeRolledEvent {
	return &InitiativeRolledEvent{
		encID:     encID,
		seq:       seq,
		Order:     order,
		PerPlayer: perPlayer,
	}
}

func (*InitiativeRolledEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *InitiativeRolledEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *InitiativeRolledEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the rolled roster,
// derived from PerPlayer keys.
func (e *InitiativeRolledEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type initiativeRolledWire struct {
	metaWire
	EncID     core.EncounterID                        `json:"encounter_id"`
	Seq       uint64                                  `json:"sequence"`
	Order     []core.EntityID                         `json:"order"`
	PerPlayer map[core.PlayerID]InitiativeRolledSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *InitiativeRolledEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(initiativeRolledWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		Order:     e.Order,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *InitiativeRolledEvent) UnmarshalJSON(b []byte) error {
	var w initiativeRolledWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.Order = w.Order
	e.PerPlayer = w.PerPlayer
	return nil
}
