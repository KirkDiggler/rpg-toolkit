package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// DamageDealtEvent is the effect event published when an entity takes damage
// from an attack. Maps to the proto EntityDamaged wire shape.
type DamageDealtEvent struct {
	eventMeta
	encID      core.EncounterID
	seq        uint64
	TargetID   core.EntityID
	SourceID   core.EntityID
	Amount     int
	DamageType string
	HPAfter    int
	MaxHP      int
	PerPlayer  map[core.PlayerID]DamageDealtSlice
	// Components is the optional per-source breakdown forwarded from the
	// combat resolver's AttackOutcome.Components. Nil means no breakdown.
	Components []core.DamageComponent
}

// DamageDealtSlice is each viewer's projection. Visible says whether the
// player perceived the damage event (via LoS to the target).
type DamageDealtSlice struct {
	Visible bool `json:"visible"`
}

// NewDamageDealtEvent constructs a DamageDealtEvent.
func NewDamageDealtEvent(
	encID core.EncounterID,
	seq uint64,
	targetID core.EntityID,
	sourceID core.EntityID,
	amount int,
	damageType string,
	hpAfter int,
	maxHP int,
	perPlayer map[core.PlayerID]DamageDealtSlice,
) *DamageDealtEvent {
	return &DamageDealtEvent{
		encID:      encID,
		seq:        seq,
		TargetID:   targetID,
		SourceID:   sourceID,
		Amount:     amount,
		DamageType: damageType,
		HPAfter:    hpAfter,
		MaxHP:      maxHP,
		PerPlayer:  perPlayer,
	}
}

func (*DamageDealtEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *DamageDealtEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *DamageDealtEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the damage event,
// derived from PerPlayer keys.
func (e *DamageDealtEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type damageDealtWire struct {
	metaWire
	EncID      core.EncounterID                   `json:"encounter_id"`
	Seq        uint64                             `json:"sequence"`
	TargetID   core.EntityID                      `json:"target_id"`
	SourceID   core.EntityID                      `json:"source_id"`
	Amount     int                                `json:"amount"`
	DamageType string                             `json:"damage_type"`
	HPAfter    int                                `json:"hp_after"`
	MaxHP      int                                `json:"max_hp"`
	PerPlayer  map[core.PlayerID]DamageDealtSlice `json:"per_player"`
	Components []core.DamageComponent             `json:"components,omitempty"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *DamageDealtEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(damageDealtWire{
		metaWire:   e.toWire(),
		EncID:      e.encID,
		Seq:        e.seq,
		TargetID:   e.TargetID,
		SourceID:   e.SourceID,
		Amount:     e.Amount,
		DamageType: e.DamageType,
		HPAfter:    e.HPAfter,
		MaxHP:      e.MaxHP,
		PerPlayer:  e.PerPlayer,
		Components: e.Components,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *DamageDealtEvent) UnmarshalJSON(b []byte) error {
	var w damageDealtWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.TargetID = w.TargetID
	e.SourceID = w.SourceID
	e.Amount = w.Amount
	e.DamageType = w.DamageType
	e.HPAfter = w.HPAfter
	e.MaxHP = w.MaxHP
	e.PerPlayer = w.PerPlayer
	e.Components = w.Components
	return nil
}
