package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// DeathSaveRolledEvent is the cause / narrative event published for every
// death save roll (or automatic damage-while-unconscious failure) an
// unconscious entity takes (rpg-toolkit#741). Unlike EntityDiedEvent /
// EntityStabilizedEvent, which fire once at a terminal transition, this
// fires on EVERY roll so a client (or playtest harness log) can show death
// saves resolving in real time. Roll is 0 when the failure came from taking
// damage while unconscious rather than an actual d20 roll (mirrors
// rulebooks/dnd5e/events.DeathSaveRolledEvent's own Roll==0 convention).
// Same per-viewer projection template as EntityDiedEvent (LoS to the
// rolling entity) — no killer/causer concept. Maps to the proto
// DeathSaveRolled wire shape.
type DeathSaveRolledEvent struct {
	eventMeta
	encID                 core.EncounterID
	seq                   uint64
	EntityID              core.EntityID
	Roll                  int
	Successes             int
	Failures              int
	IsCriticalFail        bool
	IsCriticalSuccess     bool
	Stabilized            bool
	Dead                  bool
	RegainedConsciousness bool
	HPRestored            int
	PerPlayer             map[core.PlayerID]DeathSaveRolledSlice
}

// DeathSaveRolledSlice is each viewer's projection. Visible says whether
// the player perceived the roll.
type DeathSaveRolledSlice struct {
	Visible bool `json:"visible"`
}

// NewDeathSaveRolledEventInput bundles DeathSaveRolledEvent's fields —
// enough distinct int/bool fields that a positional constructor would be
// error-prone to call correctly at each bridge call site.
type NewDeathSaveRolledEventInput struct {
	EncID                 core.EncounterID
	Seq                   uint64
	EntityID              core.EntityID
	Roll                  int
	Successes             int
	Failures              int
	IsCriticalFail        bool
	IsCriticalSuccess     bool
	Stabilized            bool
	Dead                  bool
	RegainedConsciousness bool
	HPRestored            int
	PerPlayer             map[core.PlayerID]DeathSaveRolledSlice
}

// NewDeathSaveRolledEvent constructs a DeathSaveRolledEvent.
func NewDeathSaveRolledEvent(in *NewDeathSaveRolledEventInput) *DeathSaveRolledEvent {
	return &DeathSaveRolledEvent{
		encID:                 in.EncID,
		seq:                   in.Seq,
		EntityID:              in.EntityID,
		Roll:                  in.Roll,
		Successes:             in.Successes,
		Failures:              in.Failures,
		IsCriticalFail:        in.IsCriticalFail,
		IsCriticalSuccess:     in.IsCriticalSuccess,
		Stabilized:            in.Stabilized,
		Dead:                  in.Dead,
		RegainedConsciousness: in.RegainedConsciousness,
		HPRestored:            in.HPRestored,
		PerPlayer:             in.PerPlayer,
	}
}

func (*DeathSaveRolledEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *DeathSaveRolledEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *DeathSaveRolledEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the roll, derived
// from PerPlayer keys.
func (e *DeathSaveRolledEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type deathSaveRolledWire struct {
	metaWire
	EncID                 core.EncounterID                       `json:"encounter_id"`
	Seq                   uint64                                 `json:"sequence"`
	EntityID              core.EntityID                          `json:"entity_id"`
	Roll                  int                                    `json:"roll"`
	Successes             int                                    `json:"successes"`
	Failures              int                                    `json:"failures"`
	IsCriticalFail        bool                                   `json:"is_critical_fail"`
	IsCriticalSuccess     bool                                   `json:"is_critical_success"`
	Stabilized            bool                                   `json:"stabilized"`
	Dead                  bool                                   `json:"dead"`
	RegainedConsciousness bool                                   `json:"regained_consciousness"`
	HPRestored            int                                    `json:"hp_restored"`
	PerPlayer             map[core.PlayerID]DeathSaveRolledSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *DeathSaveRolledEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(deathSaveRolledWire{
		metaWire:              e.toWire(),
		EncID:                 e.encID,
		Seq:                   e.seq,
		EntityID:              e.EntityID,
		Roll:                  e.Roll,
		Successes:             e.Successes,
		Failures:              e.Failures,
		IsCriticalFail:        e.IsCriticalFail,
		IsCriticalSuccess:     e.IsCriticalSuccess,
		Stabilized:            e.Stabilized,
		Dead:                  e.Dead,
		RegainedConsciousness: e.RegainedConsciousness,
		HPRestored:            e.HPRestored,
		PerPlayer:             e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *DeathSaveRolledEvent) UnmarshalJSON(b []byte) error {
	var w deathSaveRolledWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.EntityID = w.EntityID
	e.Roll = w.Roll
	e.Successes = w.Successes
	e.Failures = w.Failures
	e.IsCriticalFail = w.IsCriticalFail
	e.IsCriticalSuccess = w.IsCriticalSuccess
	e.Stabilized = w.Stabilized
	e.Dead = w.Dead
	e.RegainedConsciousness = w.RegainedConsciousness
	e.HPRestored = w.HPRestored
	e.PerPlayer = w.PerPlayer
	return nil
}
