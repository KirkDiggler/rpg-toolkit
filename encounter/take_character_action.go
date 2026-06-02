package encounter

// Beat-1 of the TakeAction wave (rpg-project #54 / #697 chunk 2).
//
// General (non-attack) action dispatch. The attack ref keeps its dedicated
// two-phase resolver path in TakeActionPhased (it carries reaction prompts and
// damage resolution); every OTHER action ref flows through here, delegating to
// the held character's own rules engine. There is no per-ref logic in the
// encounter beyond "is this an ability or a granted-capacity action" — the
// character package owns the catalog (ActivateAbility routes combat abilities +
// features; ExecuteAction routes the granted-capacity strikes / move). This is
// the "default to one system" unification: the encounter deletes its hardcoded
// attack gate and consults the menu/economy engine that already exists.
//
// The action runs on the held *character.Character (the LoadFromData-cascade
// instance, #689) — never a re-load. It captures any condition the action
// applies (e.g. Dodge → Dodging) and bridges it onto the broker, diffs the
// economy pre/post to report what was consumed, and emits the first-class
// ActionResolvedEvent (Invariant 9) so every action — not just attacks — leaves
// a canonical record on the spine.

import (
	"context"
	"fmt"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// takeCharacterAction dispatches a non-attack action ref to the held
// character's rules engine and publishes the resolved-action event. It returns
// a Resolved outcome on success (these actions never pause for reactions in
// Beat-1).
//
// The actor must have a hydrated character (a flat stat-snapshot seat cannot
// take menu actions — it has no economy/menu); ErrNonCombatant otherwise. An
// unknown ref, or one the character's engine reports it cannot take, returns an
// error so the caller surfaces it as a structural failure (the menu's
// available=false pre-empts the legal cases; this guards the illegal ones).
func (e *Encounter) takeCharacterAction(
	ctx context.Context, player *PlayerData, ref ActionRef, target ActionTarget,
) (*TakeActionOutcome, error) {
	char := e.heldCharacter(player.EntityID)
	if char == nil {
		return nil, fmt.Errorf("%w: player %q has no hydrated character for action %q",
			ErrNonCombatant, player.ID, ref.ID)
	}

	// Beat-scope guard (D17): a ref this build defers (e.g. move) is surfaced in
	// the menu as available=false; the verb must agree, so reject it here rather
	// than letting it no-op-succeed. Menu and verb stay consistent — the menu
	// never promises what the verb won't do.
	for _, d := range deferredActionRefs {
		if ref.ID == d.id {
			return nil, fmt.Errorf("%w: %q: %s", ErrActionDeferred, ref.ID, d.reason)
		}
	}

	tRef := toolkitRef(ref)

	// Snapshot economy before so we can report what the action consumed.
	preEconomy := snapshotEconomy(char.GetActionEconomy())

	// Capture any condition the action applies so it bridges to the broker
	// (mirrors ActivateFeature). Subscribe before activating.
	capturedCond, unsubCond, err := subscribeConditions(ctx, e.bus)
	if err != nil {
		return nil, fmt.Errorf("subscribe conditions: %w", err)
	}
	defer func() { _ = unsubCond() }()

	if err := e.dispatchCharacterAction(ctx, char, ref, &tRef); err != nil {
		return nil, err
	}

	// Bridge captured dnd5e conditions → broker events (audience-projected).
	if err := e.applyActivatedConditions(player, player.EntityID, *capturedCond); err != nil {
		return nil, fmt.Errorf("bridge conditions: %w", err)
	}

	// Diff the economy to report consumption, then publish the resolved-action
	// event for this action (Invariant 9 — every action emits one, not just
	// attacks).
	consumed := economyConsumedDiff(preEconomy, char.GetActionEconomy())
	corrID, err := e.publishActionResolved(player.EntityID, target.EntityID, tRef.String(), consumed)
	if err != nil {
		return nil, fmt.Errorf("publish action resolved: %w", err)
	}

	// Push the post-action turn state so the menu/economy refreshes live
	// (Invariant 12), correlated to the action that caused it (Invariant 8).
	if err := e.publishTurnStateChanged(player.EntityID, corrID); err != nil {
		return nil, fmt.Errorf("publish turn-state changed: %w", err)
	}

	return &TakeActionOutcome{Resolved: true}, nil
}

// dispatchCharacterAction routes the ref to the character engine: an ability
// (combat ability or feature) goes through ActivateAbility; a granted-capacity
// action (strike, move) goes through ExecuteAction. Membership is decided by
// the character's own menu, so no ref is enumerated here.
func (e *Encounter) dispatchCharacterAction(
	ctx context.Context, char *dnd5eCharacter.Character, ref ActionRef, tRef *toolkitcore.Ref,
) error {
	if characterHasAbility(char, ref.ID) {
		out, err := char.ActivateAbility(ctx, &dnd5eCharacter.ActivateAbilityInput{AbilityRef: tRef})
		if err != nil {
			return fmt.Errorf("activate ability %q: %w", ref.ID, err)
		}
		if !out.Success {
			return fmt.Errorf("%w: %q: %s", ErrUnsupportedAction, ref.ID, out.Error)
		}
		return nil
	}

	if characterHasAction(char, ref.ID) {
		out, err := char.ExecuteAction(ctx, &dnd5eCharacter.ExecuteActionInput{ActionRef: tRef})
		if err != nil {
			return fmt.Errorf("execute action %q: %w", ref.ID, err)
		}
		if !out.Success {
			return fmt.Errorf("%w: %q: %s", ErrUnsupportedAction, ref.ID, out.Error)
		}
		return nil
	}

	return fmt.Errorf("%w: %q", ErrUnsupportedAction, ref.ID)
}

// characterHasAbility reports whether the ref matches one of the character's
// available abilities (combat abilities + features).
func characterHasAbility(char *dnd5eCharacter.Character, refID string) bool {
	for _, a := range char.AvailableAbilities() {
		if a.Ref != nil && a.Ref.ID == refID {
			return true
		}
	}
	return false
}

// characterHasAction reports whether the ref matches one of the character's
// available granted-capacity actions (strikes, move).
func characterHasAction(char *dnd5eCharacter.Character, refID string) bool {
	for _, a := range char.AvailableActions() {
		if a.Ref != nil && a.Ref.ID == refID {
			return true
		}
	}
	return false
}

// economySnapshot is the minimal economy shape we diff to compute what an
// action consumed.
type economySnapshot struct {
	actions      int
	bonusActions int
	reactions    int
	movement     int
	granted      map[string]int
}

// snapshotEconomy captures the action economy counters (nil-safe).
func snapshotEconomy(ae *dnd5eCharacter.ActionEconomyData) economySnapshot {
	if ae == nil {
		return economySnapshot{}
	}
	granted := make(map[string]int, len(ae.Granted))
	for k, v := range ae.Granted {
		granted[string(k)] = v
	}
	return economySnapshot{
		actions:      ae.ActionsRemaining,
		bonusActions: ae.BonusActionsRemaining,
		reactions:    ae.ReactionsRemaining,
		movement:     ae.MovementRemaining,
		granted:      granted,
	}
}

// economyConsumedDiff reports what an action spent: the positive drop in each
// primary counter plus the positive drop in each granted-capacity key. A
// negative delta (the action GRANTED capacity, e.g. Attack adds attacks) is not
// "consumed" and is omitted.
func economyConsumedDiff(pre economySnapshot, postAE *dnd5eCharacter.ActionEconomyData) events.EconomyConsumed {
	post := snapshotEconomy(postAE)
	consumed := events.EconomyConsumed{
		Actions:      positiveDrop(pre.actions, post.actions),
		BonusActions: positiveDrop(pre.bonusActions, post.bonusActions),
		Reactions:    positiveDrop(pre.reactions, post.reactions),
		Movement:     positiveDrop(pre.movement, post.movement),
	}
	var granted map[string]int
	for k, preV := range pre.granted {
		if drop := positiveDrop(preV, post.granted[k]); drop > 0 {
			if granted == nil {
				granted = map[string]int{}
			}
			granted[k] = drop
		}
	}
	consumed.GrantedConsumed = granted
	return consumed
}

// positiveDrop returns pre-post when positive, else 0 (a grant, not a spend).
func positiveDrop(pre, post int) int {
	if d := pre - post; d > 0 {
		return d
	}
	return 0
}

// publishActionResolved emits the first-class ActionResolvedEvent for a
// non-attack action (Invariant 9). Audience is per-viewer LoS to the actor.
// Unlike the attack path this publishes only the umbrella event — there is no
// AttackResolved/Damage for a non-attack action.
//
// Returns the correlation id stamped on the event so the caller can tie the
// follow-on turn-state refresh to the same action (Invariant 8).
func (e *Encounter) publishActionResolved(
	actorID, targetID core.EntityID, actionRef string, consumed events.EconomyConsumed,
) (core.CorrelationID, error) {
	actor := e.findPlayerByEntityID(actorID)
	perPlayer := make(map[core.PlayerID]events.ActionResolvedSlice)
	for viewerID, viewer := range e.data.Players {
		if viewer == nil || viewer.View == nil || actor == nil || actor.View == nil {
			continue
		}
		if !e.viewerCanSee(viewer, actor.View.Position) {
			continue
		}
		perPlayer[viewerID] = events.ActionResolvedSlice{Visible: true}
	}

	seq := e.nextSeq()
	corrID := e.correlationFor(seq)
	evt := events.NewActionResolvedEvent(
		e.data.ID, seq, actorID, actionRef, targetID, consumed, perPlayer,
	)
	return corrID, e.publishCorrelated(evt, corrID)
}
