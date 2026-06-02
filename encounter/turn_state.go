package encounter

// Beat-1 of the TakeAction wave (rpg-project #54 / #697 chunk 2).
//
// "What can this actor do" exposed as data. North-Star Invariant 11: the
// toolkit computes the action menu (availability, reasons, slot, target kind)
// and the economy; the game server (rpg-api) projects it field-for-field; the
// web renders it. This query is that read surface — it returns toolkit domain
// types built from the held character's own rules engine, so rpg-api never
// computes availability or authors a reason string.
//
// It reads the held *character.Character (the LoadFromData-cascade instance,
// #689) and calls the character's existing AvailableAbilities / AvailableActions
// / GetActionEconomy — the same two-level menu the character package already
// computes. No menu logic lives here; this only shapes the character's output
// into an encounter-level, per-actor view.

import (
	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// deferredActionRef pairs an action ref id this build does not resolve yet with
// the user-facing reason the menu shows for it. The encounter — which owns the
// beat scope — composes "effective takeability" on top of the character's
// honest rules menu (D17): the character correctly says the action is allowed
// (e.g. a level-1 character CAN move), but this build does not resolve it yet,
// so the wire `available` flag (which means "should the UI offer this now?",
// not raw rules-permission) is false with an honest reason. Beat 2 removes an
// entry from this set in one line — it does not touch the character menu.
type deferredActionRef struct {
	id     string
	reason string
}

// deferredActionRefs is the set of action refs this build surfaces in the menu
// but cannot take yet. Move is the only Beat-1 member (movement resolution is
// Beat 2 — wave decision D5). Expressed as a set rather than a hardcoded
// `if id == "move"` so Beat 2 is a one-line removal and the mechanism
// generalizes to any future beat-deferred ref.
var deferredActionRefs = []deferredActionRef{
	{id: refs.Actions.Move().ID, reason: "movement lands in Beat 2"},
}

// ActorTurnState is the per-actor turn view: the action economy and the menu
// (abilities + actions) the actor can take right now. It is the toolkit domain
// shape rpg-api projects onto the wire TurnState (economy + available_actions).
//
// Abilities and Actions are the two halves of the dnd5e two-level model:
// abilities spend a primary economy slot (action / bonus / reaction) and grant
// capacity or a condition; actions spend granted capacity (the strikes an
// attack grants, movement). Both carry the menu fields rpg-api needs verbatim:
// ref, name, availability + reason, economy slot, target kind.
type ActorTurnState struct {
	// ActorID is the entity whose turn view this is.
	ActorID core.EntityID
	// Economy is the actor's current action economy (nil when the actor has no
	// hydrated character or is not in combat).
	Economy *character.ActionEconomyData
	// Abilities are the action-economy-spending menu entries.
	Abilities []character.AvailableAbility
	// Actions are the granted-capacity-spending menu entries.
	Actions []character.AvailableAction
}

// ActorTurnState returns the menu + economy for the given actor, computed by the
// held character's own rules engine. Returns a zero-valued ActorTurnState (with
// the ActorID set) when the actor has no hydrated character — a flat
// stat-snapshot seat or an NPC carries no character menu/economy.
//
// The caller (rpg-api) projects the returned domain types onto the wire
// TurnState. The character menu is taken verbatim for ability/action membership,
// names, slots, target kinds, and rules availability; the encounter then
// composes "effective takeability" for beat-deferred refs (D17) — see
// applyDeferredActions. It authors no rules verdict, only the beat-scope it owns.
func (e *Encounter) ActorTurnState(actorID core.EntityID) ActorTurnState {
	char := e.heldCharacter(actorID)
	if char == nil {
		return ActorTurnState{ActorID: actorID}
	}
	return ActorTurnState{
		ActorID:   actorID,
		Economy:   char.GetActionEconomy(),
		Abilities: char.AvailableAbilities(),
		Actions:   applyDeferredActions(char.AvailableActions()),
	}
}

// publishTurnStateChanged emits a TurnStateChangedEvent for the actor whose
// turn state just mutated (North-Star Invariant 12: the menu/economy refresh is
// pushed, never silently stale). Called after every turn-state mutation: turn
// start (seeding) and each action taken (economy deducted).
//
// The snapshot is built from the actor's own rules engine via ActorTurnState
// and flattened to the rulebook-agnostic events.TurnStateSnapshot. Audience is
// the actor's controlling player — turn state is that player's private "what can
// I do now" view. A no-op for actors with no controlling player seat (NPCs) or
// no hydrated character (stat-snapshot seats): there is no menu to push.
//
// corrID ties a post-action refresh to the action that caused it (Invariant 8);
// pass an empty corrID for turn-start refreshes (not caused by an action).
func (e *Encounter) publishTurnStateChanged(actorID core.EntityID, corrID core.CorrelationID) error {
	actor := e.findPlayerByEntityID(actorID)
	if actor == nil {
		return nil // NPC or unknown — no controlling player to refresh.
	}
	char := e.heldCharacter(actorID)
	if char == nil {
		return nil // Stat-snapshot seat — no character menu/economy to push.
	}

	snapshot := buildTurnStateSnapshot(e.ActorTurnState(actorID))
	evt := events.NewTurnStateChangedEvent(
		e.data.ID, e.nextSeq(), actorID, snapshot,
		map[core.PlayerID]events.TurnStateChangedSlice{actor.ID: {Visible: true}},
	)
	return e.publishCorrelated(evt, corrID)
}

// buildTurnStateSnapshot flattens an ActorTurnState (which holds rulebook types)
// into the rulebook-agnostic events.TurnStateSnapshot the spine carries. The
// menu is one ordered list: abilities first, then granted-capacity actions —
// each flattened to ref/name/slot/target-kind/availability primitives.
func buildTurnStateSnapshot(ts ActorTurnState) events.TurnStateSnapshot {
	snap := events.TurnStateSnapshot{}
	if ts.Economy != nil {
		snap.InCombat = true
		snap.TurnNumber = ts.Economy.TurnNumber
		snap.ActionsRemaining = ts.Economy.ActionsRemaining
		snap.BonusActionsRemaining = ts.Economy.BonusActionsRemaining
		snap.ReactionsRemaining = ts.Economy.ReactionsRemaining
		snap.MovementRemaining = ts.Economy.MovementRemaining
	}
	menu := make([]events.MenuEntry, 0, len(ts.Abilities)+len(ts.Actions))
	for _, a := range ts.Abilities {
		menu = append(menu, abilityMenuEntry(a))
	}
	for _, a := range ts.Actions {
		menu = append(menu, actionMenuEntry(a))
	}
	snap.Menu = menu
	return snap
}

// abilityMenuEntry flattens an AvailableAbility to the agnostic MenuEntry.
func abilityMenuEntry(a character.AvailableAbility) events.MenuEntry {
	return events.MenuEntry{
		Ref:         refString(a.Ref),
		Name:        a.Name,
		EconomySlot: string(a.EconomySlot),
		TargetKind:  string(a.TargetKind),
		Available:   a.CanUse,
		Reason:      a.Reason,
	}
}

// actionMenuEntry flattens an AvailableAction to the agnostic MenuEntry.
func actionMenuEntry(a character.AvailableAction) events.MenuEntry {
	return events.MenuEntry{
		Ref:         refString(a.Ref),
		Name:        a.Name,
		EconomySlot: string(a.EconomySlot),
		TargetKind:  string(a.TargetKind),
		Available:   a.CanUse,
		Reason:      a.Reason,
	}
}

// refString renders a *core.Ref to its canonical "module:type:id" string, or
// "" when nil.
func refString(ref *toolkitcore.Ref) string {
	if ref == nil {
		return ""
	}
	return ref.String()
}

// applyDeferredActions composes the encounter's beat-scope onto the character's
// honest action menu: any action ref in deferredActionRefs is forced
// unavailable with its reason, even though the character's rules engine reports
// it usable (D17). The character menu is not mutated — this returns an adjusted
// copy. Actions not in the deferred set pass through unchanged.
func applyDeferredActions(actions []character.AvailableAction) []character.AvailableAction {
	if len(actions) == 0 {
		return actions
	}
	out := make([]character.AvailableAction, len(actions))
	copy(out, actions)
	for i := range out {
		if out[i].Ref == nil {
			continue
		}
		for _, d := range deferredActionRefs {
			if out[i].Ref.ID == d.id {
				out[i].CanUse = false
				out[i].Reason = d.reason
			}
		}
	}
	return out
}
