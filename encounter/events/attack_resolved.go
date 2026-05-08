package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// AttackResolvedEvent is the cause/narration event published when an attack
// resolves. It carries enough detail for narration / animation hooks but
// does not itself describe the HP outcome — that lives on the parallel
// DamageDealtEvent published alongside on hit.
type AttackResolvedEvent struct {
	encID       core.EncounterID
	seq         uint64
	AttackerID  core.EntityID
	TargetID    core.EntityID
	Hit         bool
	Critical    bool
	AttackRoll  int
	AttackBonus int
	TargetAC    int
	PerPlayer   map[core.PlayerID]AttackResolvedSlice
}

// AttackResolvedSlice is each viewer's projection. Visible says whether the
// player perceived the attack at all (via their LoS to attacker or target).
type AttackResolvedSlice struct {
	Visible bool `json:"visible"`
}

// NewAttackResolvedEvent constructs an AttackResolvedEvent.
func NewAttackResolvedEvent(
	encID core.EncounterID,
	seq uint64,
	attackerID core.EntityID,
	targetID core.EntityID,
	hit bool,
	critical bool,
	attackRoll int,
	attackBonus int,
	targetAC int,
	perPlayer map[core.PlayerID]AttackResolvedSlice,
) *AttackResolvedEvent {
	return &AttackResolvedEvent{
		encID:       encID,
		seq:         seq,
		AttackerID:  attackerID,
		TargetID:    targetID,
		Hit:         hit,
		Critical:    critical,
		AttackRoll:  attackRoll,
		AttackBonus: attackBonus,
		TargetAC:    targetAC,
		PerPlayer:   perPlayer,
	}
}

func (*AttackResolvedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *AttackResolvedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *AttackResolvedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the attack-resolution event,
// derived from PerPlayer keys.
func (e *AttackResolvedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type attackResolvedWire struct {
	EncID       core.EncounterID                      `json:"encounter_id"`
	Seq         uint64                                `json:"sequence"`
	AttackerID  core.EntityID                         `json:"attacker_id"`
	TargetID    core.EntityID                         `json:"target_id"`
	Hit         bool                                  `json:"hit"`
	Critical    bool                                  `json:"critical"`
	AttackRoll  int                                   `json:"attack_roll"`
	AttackBonus int                                   `json:"attack_bonus"`
	TargetAC    int                                   `json:"target_ac"`
	PerPlayer   map[core.PlayerID]AttackResolvedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *AttackResolvedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(attackResolvedWire{
		EncID:       e.encID,
		Seq:         e.seq,
		AttackerID:  e.AttackerID,
		TargetID:    e.TargetID,
		Hit:         e.Hit,
		Critical:    e.Critical,
		AttackRoll:  e.AttackRoll,
		AttackBonus: e.AttackBonus,
		TargetAC:    e.TargetAC,
		PerPlayer:   e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *AttackResolvedEvent) UnmarshalJSON(b []byte) error {
	var w attackResolvedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.AttackerID = w.AttackerID
	e.TargetID = w.TargetID
	e.Hit = w.Hit
	e.Critical = w.Critical
	e.AttackRoll = w.AttackRoll
	e.AttackBonus = w.AttackBonus
	e.TargetAC = w.TargetAC
	e.PerPlayer = w.PerPlayer
	return nil
}
