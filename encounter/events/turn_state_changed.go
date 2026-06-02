package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// TurnStateChangedEvent is the push-refresh event for an actor's turn state
// (North-Star Invariant 12: "economy/menu refresh is pushed as a delta event —
// the menu never goes silently stale"). The encounter publishes it whenever the
// actor's economy / available menu mutates — turn start (seeding) and every
// action taken (economy deducted) — so the client updates the menu live off the
// stream instead of polling.
//
// It carries a full snapshot (not a diff): the actor's current economy plus the
// available abilities/actions menu. rpg-api projects this verbatim onto the
// proto TurnStateChanged (envelope field 45); the web renders it without knowing
// rules. The toolkit owns the snapshot (Inv 11) — the API authors nothing.
//
// The snapshot is flattened to primitives (strings/ints), NOT rulebook types:
// the encounter spine stays rulebook-agnostic, exactly as EconomyConsumed does.
// The encounter builds the snapshot from its rulebook-aware ActorTurnState at
// publish time.
type TurnStateChangedEvent struct {
	eventMeta
	encID core.EncounterID
	seq   uint64
	// ActorID is the entity whose turn state this snapshot describes.
	ActorID core.EntityID
	// State is the actor's current turn-state snapshot (economy + menu).
	State TurnStateSnapshot
	// PerPlayer is the per-viewer projection. Turn state is the actor's own
	// private "what can I do now" view, so the audience is the actor's
	// controlling player.
	PerPlayer map[core.PlayerID]TurnStateChangedSlice
}

// TurnStateSnapshot is the rulebook-agnostic, wire-ready snapshot of an actor's
// turn state: the action economy plus the available menu. All fields are
// primitives so the encounter spine carries no rulebook types.
type TurnStateSnapshot struct {
	// InCombat reports whether the actor has an active economy this turn. When
	// false the economy fields are zero and Menu is empty (e.g. the actor has no
	// hydrated character, or is not in combat).
	InCombat bool `json:"in_combat"`
	// TurnNumber is the economy's turn counter (0 when not in combat).
	TurnNumber int `json:"turn_number"`
	// ActionsRemaining / BonusActionsRemaining / ReactionsRemaining /
	// MovementRemaining are the current economy counters.
	ActionsRemaining      int `json:"actions_remaining"`
	BonusActionsRemaining int `json:"bonus_actions_remaining"`
	ReactionsRemaining    int `json:"reactions_remaining"`
	MovementRemaining     int `json:"movement_remaining"`
	// Menu is the available abilities + actions the actor can take right now,
	// flattened into one ordered list (abilities first, then granted-capacity
	// actions). Each entry is the menu data the UI renders.
	Menu []MenuEntry `json:"menu"`
}

// MenuEntry is one option in an actor's action menu, flattened to primitives.
// Mirrors the toolkit-authored AvailableAbility / AvailableAction fields the
// game server projects field-for-field (Inv 11): ref, name, economy slot,
// target kind, and effective availability + reason.
type MenuEntry struct {
	// Ref is the canonical ref string of the option, e.g.
	// "dnd5e:combat_abilities:attack" / "dnd5e:actions:strike".
	Ref string `json:"ref"`
	// Name is the display name, e.g. "Attack".
	Name string `json:"name"`
	// EconomySlot is which slot the option draws from (menu grouping):
	// "action" / "bonus_action" / "reaction" / "movement" / "free".
	EconomySlot string `json:"economy_slot"`
	// TargetKind is what target the UI must prompt for:
	// "self" / "single_entity" / "position" / "area" / "none".
	TargetKind string `json:"target_kind"`
	// Available is the effective takeability ("should the UI offer this now?").
	Available bool `json:"available"`
	// Reason is why Available is false (empty when available).
	Reason string `json:"reason,omitempty"`
}

// TurnStateChangedSlice is each viewer's projection. Visible says whether the
// player receives this turn-state refresh (the actor's own controlling player).
type TurnStateChangedSlice struct {
	Visible bool `json:"visible"`
}

// NewTurnStateChangedEvent constructs a TurnStateChangedEvent. The encounter
// stamps the encounter id and sequence; OccurredAt + CorrelationID are stamped
// at publish via Stamp.
func NewTurnStateChangedEvent(
	encID core.EncounterID,
	seq uint64,
	actorID core.EntityID,
	state TurnStateSnapshot,
	perPlayer map[core.PlayerID]TurnStateChangedSlice,
) *TurnStateChangedEvent {
	return &TurnStateChangedEvent{
		encID:     encID,
		seq:       seq,
		ActorID:   actorID,
		State:     state,
		PerPlayer: perPlayer,
	}
}

func (*TurnStateChangedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *TurnStateChangedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *TurnStateChangedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who receive this turn-state refresh,
// derived from PerPlayer keys.
func (e *TurnStateChangedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

type turnStateChangedWire struct {
	metaWire
	EncID     core.EncounterID                        `json:"encounter_id"`
	Seq       uint64                                  `json:"sequence"`
	ActorID   core.EntityID                           `json:"actor_id"`
	State     TurnStateSnapshot                       `json:"state"`
	PerPlayer map[core.PlayerID]TurnStateChangedSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *TurnStateChangedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(turnStateChangedWire{
		metaWire:  e.toWire(),
		EncID:     e.encID,
		Seq:       e.seq,
		ActorID:   e.ActorID,
		State:     e.State,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *TurnStateChangedEvent) UnmarshalJSON(b []byte) error {
	var w turnStateChangedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.ActorID = w.ActorID
	e.State = w.State
	e.PerPlayer = w.PerPlayer
	return nil
}
