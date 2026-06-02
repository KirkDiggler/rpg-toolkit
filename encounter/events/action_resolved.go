package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// ActionResolvedEvent is the first-class "an action was taken" event
// (North-Star Invariant 9). Every player-facing action — attack, Dodge, Dash,
// the Monk bonus-action strike — emits one of these as the canonical record of
// the action itself, distinct from the scattered effect events (damage,
// condition, resource) it may cause. Those effects carry the same
// CorrelationID (via the embedded eventMeta) so the toolkit-owned combat log
// reassembles "this damage came from that action."
//
// It is intentionally action-shaped, not attack-shaped: ActionRef names what
// was done and EconomyConsumed reports what the action cost off the actor's
// turn economy. Attack-specific roll detail (hit / crit / roll / AC) stays on
// the parallel AttackResolvedEvent for an attack action; ActionResolvedEvent is
// the umbrella beat that every action shares.
type ActionResolvedEvent struct {
	eventMeta
	encID core.EncounterID
	seq   uint64
	// ActorID is the entity that took the action.
	ActorID core.EntityID
	// ActionRef is the canonical ref of the action taken, e.g.
	// "dnd5e:actions:strike" / "dnd5e:combat_abilities:dodge". String form on
	// the wire so the encounter SDK stays free of rulebook ref types.
	ActionRef string
	// TargetID is the primary target, when the action has one. Empty for
	// self / no-target actions (Dodge, Dash).
	TargetID core.EntityID
	// EconomyConsumed reports what this action spent off the turn economy.
	EconomyConsumed EconomyConsumed
	// PerPlayer is the per-viewer visibility projection.
	PerPlayer map[core.PlayerID]ActionResolvedSlice
}

// EconomyConsumed reports the turn-economy cost an action paid. Each field is
// the count consumed by THIS action (0 when the action did not touch that
// slot). The toolkit owns the deduction (Invariant 2/3); this is the faithful
// record of what was spent, which rpg-api projects verbatim and the web renders
// as the economy decrementing.
type EconomyConsumed struct {
	// Actions is the number of standard actions consumed (0 or 1 today).
	Actions int `json:"actions"`
	// BonusActions is the number of bonus actions consumed (0 or 1 today) —
	// the field the Monk Martial Arts bonus strike exercises.
	BonusActions int `json:"bonus_actions"`
	// Reactions is the number of reactions consumed.
	Reactions int `json:"reactions"`
	// Movement is the movement consumed, in the encounter's movement unit.
	Movement int `json:"movement"`
	// GrantedConsumed maps a granted-capacity key (e.g. "attacks",
	// "martial_arts_bonus") to the count this action drew from it. Nil when the
	// action consumed no granted capacity. String keys keep the encounter SDK
	// free of rulebook capacity-key types.
	GrantedConsumed map[string]int `json:"granted_consumed,omitempty"`
}

// ActionResolvedSlice is each viewer's projection. Visible says whether the
// player perceived the action at all (via LoS to the actor or target).
type ActionResolvedSlice struct {
	Visible bool `json:"visible"`
}

// NewActionResolvedEvent constructs an ActionResolvedEvent. The encounter
// stamps the encounter id and sequence; OccurredAt + CorrelationID are stamped
// at publish via Stamp.
func NewActionResolvedEvent(
	encID core.EncounterID,
	seq uint64,
	actorID core.EntityID,
	actionRef string,
	targetID core.EntityID,
	consumed EconomyConsumed,
	perPlayer map[core.PlayerID]ActionResolvedSlice,
) *ActionResolvedEvent {
	return &ActionResolvedEvent{
		encID:           encID,
		seq:             seq,
		ActorID:         actorID,
		ActionRef:       actionRef,
		TargetID:        targetID,
		EconomyConsumed: consumed,
		PerPlayer:       perPlayer,
	}
}

func (*ActionResolvedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *ActionResolvedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *ActionResolvedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive the action, derived
// from PerPlayer keys.
func (e *ActionResolvedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type actionResolvedWire struct {
	metaWire
	EncID           core.EncounterID                      `json:"encounter_id"`
	Seq             uint64                                `json:"sequence"`
	ActorID         core.EntityID                         `json:"actor_id"`
	ActionRef       string                                `json:"action_ref"`
	TargetID        core.EntityID                         `json:"target_id,omitempty"`
	EconomyConsumed EconomyConsumed                       `json:"economy_consumed"`
	PerPlayer       map[core.PlayerID]ActionResolvedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *ActionResolvedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(actionResolvedWire{
		metaWire:        e.toWire(),
		EncID:           e.encID,
		Seq:             e.seq,
		ActorID:         e.ActorID,
		ActionRef:       e.ActionRef,
		TargetID:        e.TargetID,
		EconomyConsumed: e.EconomyConsumed,
		PerPlayer:       e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *ActionResolvedEvent) UnmarshalJSON(b []byte) error {
	var w actionResolvedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.ActorID = w.ActorID
	e.ActionRef = w.ActionRef
	e.TargetID = w.TargetID
	e.EconomyConsumed = w.EconomyConsumed
	e.PerPlayer = w.PerPlayer
	return nil
}
