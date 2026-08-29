// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// defaultMeleeReach is the default melee reach for OA eligibility checks,
// in grid units (1 unit = 5ft in D&D 5e). Reach weapons (10ft) are a future
// extension that will read the OA condition's holder's equipped weapon.
const defaultMeleeReach = 1.0

// OpportunityAttackConditionData is the JSON shape used for serialization.
//
// In Wave 2.11d the condition is NOT persisted on character.Data.Conditions
// (it is universal for melee combatants and applied programmatically by the
// orchestrator at character/monster rehydration). The JSON shape exists so the
// loader composes cleanly with the existing pattern and so future per-character
// variants (Sentinel, Polearm Master) can persist their state through the same
// loader switch.
type OpportunityAttackConditionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`

	// UsedThisTurn is persisted rather than kept in memory for the reason
	// SneakAttackData gives for its own copy of this field: every call
	// reconstructs the condition from JSON, so a runtime-only flag is a flag
	// that resets on each RPC and meters nothing at all.
	UsedThisTurn bool `json:"used_this_turn"`
}

// OpportunityAttackCondition publishes a ReactionTriggerEvent when an enemy
// leaves the holder's threatened reach AND the holder has the OA reaction
// readied (gamectx.IsReactionReady).
//
// Per Wave 2.11d Director ruling B4: BOTH player and NPC reactors publish the
// trigger event; the encounter SDK wrapper (Encounter.MoveEntity) iterates the
// buffered events and either resolves NPC OAs inline (no prompt) or surfaces
// player OAs as InputRequired{reaction_prompt} on the reactor's stream. The
// condition handler itself does NOT make re-entrant Strike calls.
//
// Subscribes to MovementChain. Predicate per move event:
//   - Mover is not self (no self-OA).
//   - Move is not OA-prevented (Disengaging short-circuits via OAPreventionSources).
//   - Self threatens the move's FromPosition (within reach).
//   - Self does NOT threaten ToPosition (mover is leaving reach).
//   - gamectx.IsReactionReady(self, OA-ref) returns true.
//
// Reach defaults to 5ft (1 grid unit). Reach weapons + action-economy reaction
// availability are future extensions; the predicate is conservative today.
type OpportunityAttackCondition struct {
	CharacterID string

	// UsedThisTurn is the meter EVERY reactor has, character and monster
	// alike. It is cleared at the start of this reactor's OWN turn, which is
	// when 5e refreshes a spent reaction — see onTurnStart.
	UsedThisTurn bool

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure OpportunityAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*OpportunityAttackCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (o *OpportunityAttackCondition) Ref() *core.Ref { return refs.Conditions.OpportunityAttack() }

// stateChanged reports that the once-per-turn meter moved. See
// [publishStateChanged].
func (o *OpportunityAttackCondition) stateChanged(ctx context.Context) error {
	return publishStateChanged(ctx, o.bus, o.CharacterID, o.Ref())
}

// canReact asks this reactor's own sheet whether it has a reaction to spend,
// in place of the ledger handle a loader used to pass in.
//
// # Where the asymmetry went
//
// The handle carried a fact the cast does not: a monster's SetOwner never
// matched combat.Ledger, so "I hold a purse" meant "I am a character" and the
// gate could be written as "refuse only if an economy says no". The cast hands
// out both kinds through one surface, so that fact moved INTO the answer — a
// character reports its slots, a monster reports true because it has no economy
// to refuse with. Kirk ruled the asymmetry 2026-08-28: "characters have to pay
// for it. the condition can still track it was used but players have a cost."
// Paying is also what keeps this and Protection fighting style mutually
// exclusive, which they are in the rules: both spend the one reaction, and the
// second to ask finds it gone.
//
// # A reactor nobody can look up does NOT react
//
// The lookup has a third answer the handle never had, and this is it. A cast is
// installed by one door on every path that folds anything
// (resolution.installTruth, held structurally by
// TestNoCodePathProducesACastlessInteraction), so a fold with no cast is not a
// monster — it is a fold that was assembled wrong, and there is no sheet to
// ask. Answering "react" there would hand a free reaction to any character
// whose cast went missing, which is precisely the silently-absent-handle
// failure this whole migration removes. RequireRoom below makes the same
// choice for the same reason, and so does Protection.
func (o *OpportunityAttackCondition) canReact(ctx context.Context) bool {
	self, ok := member(ctx, o.CharacterID)
	if !ok {
		return false
	}

	return self.CanReact()
}

// NewOpportunityAttackCondition creates a new OA condition for the given character.
// The condition is universal for melee combatants and applied programmatically
// at encounter setup; it does not require player choice or persistence in
// character.Data.Conditions.
func NewOpportunityAttackCondition(characterID string) *OpportunityAttackCondition {
	return &OpportunityAttackCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied (subscribed).
func (o *OpportunityAttackCondition) IsApplied() bool {
	return o.bus != nil
}

// Apply subscribes the condition to the MovementChain.
func (o *OpportunityAttackCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if o.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "opportunity attack condition already applied")
	}
	o.bus = bus

	movementChain := dnd5eEvents.MovementChain.On(bus)
	subID, err := movementChain.SubscribeWithChain(ctx, o.onMovementChain)
	if err != nil {
		o.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to movement chain")
	}
	o.subscriptionIDs = append(o.subscriptionIDs, subID)

	// Roll the movement subscription back rather than dropping the bus on the
	// floor, which is what DisengagingCondition does at the same seam and for
	// the same reason. Nil-ing o.bus with a live subscription still recorded
	// leaves the WORST of both: IsApplied reports false, Remove early-returns
	// on the nil bus and unsubscribes nothing, and the orphaned handler keeps
	// receiving movement on a bus this condition no longer admits to holding.
	turnStarts := dnd5eEvents.TurnStartTopic.On(bus)
	resetID, err := turnStarts.Subscribe(ctx, o.onTurnStart)
	if err != nil {
		_ = o.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn start")
	}
	o.subscriptionIDs = append(o.subscriptionIDs, resetID)
	return nil
}

// onTurnStart refreshes the spent reaction at the start of the reactor's own
// turn.
//
// TURN START, NOT TURN END, and the difference is the whole point of the
// field. A reaction is spent on somebody ELSE's turn — that is what makes it a
// reaction — so a meter cleared at the end of its holder's turn would be full
// again for the entire window it is supposed to govern. 2014 PHB: "you regain
// a spent reaction at the start of each of your turns."
//
// Only when the flag actually changes, for SneakAttackCondition's stated
// reason: marking unconditionally would flag every combatant dirty at the
// start of every turn they did not react on, and a boundary already runs for
// every participant.
func (o *OpportunityAttackCondition) onTurnStart(ctx context.Context, event dnd5eEvents.TurnStartEvent) error {
	if event.SubjectID == o.CharacterID && o.UsedThisTurn {
		o.UsedThisTurn = false

		return o.stateChanged(ctx)
	}
	return nil
}

// Remove unsubscribes the condition from all events.
func (o *OpportunityAttackCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if o.bus == nil {
		return nil
	}
	total := len(o.subscriptionIDs)
	var errs []error
	for _, id := range o.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", id, err))
		}
	}
	o.subscriptionIDs = nil
	o.bus = nil
	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to its JSON representation.
func (o *OpportunityAttackCondition) ToJSON() (json.RawMessage, error) {
	data := OpportunityAttackConditionData{
		Ref:          refs.Conditions.OpportunityAttack(),
		CharacterID:  o.CharacterID,
		UsedThisTurn: o.UsedThisTurn,
	}
	return json.Marshal(data)
}

// loadJSON loads OA condition state from JSON.
func (o *OpportunityAttackCondition) loadJSON(data json.RawMessage) error {
	var oaData OpportunityAttackConditionData
	if err := json.Unmarshal(data, &oaData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal opportunity attack data")
	}
	o.CharacterID = oaData.CharacterID
	o.UsedThisTurn = oaData.UsedThisTurn
	return nil
}

// onMovementChain inspects each movement step and publishes a
// ReactionTriggerEvent when this combatant has a triggerable OA opportunity.
//
// The chain itself is NOT modified — the condition does not append a stage.
// The trigger event is published on the encounter bus; the orchestrator
// (encounter SDK wrapper) drains it after MoveEntity returns.
func (o *OpportunityAttackCondition) onMovementChain(
	ctx context.Context,
	event *dnd5eEvents.MovementChainEvent,
	c chain.Chain[*dnd5eEvents.MovementChainEvent],
) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
	// Don't OA your own movement.
	if event.EntityID == o.CharacterID {
		return c, nil
	}

	// Disengaging (or any other source) prevented OAs for this step.
	if event.IsOAPrevented() {
		return c, nil
	}

	// Already reacted. Cleared at the start of this reactor's own turn.
	if o.UsedThisTurn {
		return c, nil
	}

	// A reactor with an economy PAYS; a monster has none to pay from and is
	// metered by UsedThisTurn alone. See canReact for why that asymmetry is
	// the rule rather than a gap.
	if !o.canReact(ctx) {
		return c, nil
	}

	// Readiness gate — opt-in at the orchestrator level. If unreadied,
	// no trigger fires and the move proceeds single-phase.
	if !gamectx.IsReactionReady(ctx, o.CharacterID, refs.Conditions.OpportunityAttack().String()) {
		return c, nil
	}

	// Need spatial data for the leave-reach geometry check.
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		// No room → cannot evaluate geometry; skip silently. This matches
		// SneakAttack's behavior when gamectx isn't fully populated.
		return c, nil //nolint:nilerr // missing context = condition no-op
	}

	if !o.isLeavingMyThreatRange(room, event) {
		return c, nil
	}

	// Predicate matched — publish the trigger event for the orchestrator.
	triggerTopic := dnd5eEvents.ReactionTriggerTopic.On(o.bus)
	if pubErr := triggerTopic.Publish(ctx, dnd5eEvents.ReactionTriggerEvent{
		ReactorID:    o.CharacterID,
		ConditionRef: refs.Conditions.OpportunityAttack().String(),
		TriggerKind:  dnd5eEvents.TriggerKindMovementOA,
		SourceEntity: event.EntityID,
		Payload: dnd5eEvents.MovementChainEvent{
			EntityID:     event.EntityID,
			EntityType:   event.EntityType,
			FromPosition: event.FromPosition,
			ToPosition:   event.ToPosition,
		},
	}); pubErr != nil {
		return c, rpgerr.Wrap(pubErr, "failed to publish OA reaction trigger event")
	}

	// Spend AFTER the publish, never before: a trigger that failed to reach
	// the orchestrator is a reaction that did not happen, and charging for it
	// would leave the reactor unable to react to the next mover for a swing
	// nobody made.
	//
	// The bill goes out unconditionally now, where it used to be guarded by
	// whether a purse had been handed over. Nothing here decides who pays:
	// a keeper holding an economy debits it, and a monster's keeper has no row
	// for the topic at all, so the request truthfully passes it by. That is the
	// same asymmetry, moved from a nil check in this condition to which
	// subscriptions each sheet keeper's table holds.
	o.UsedThisTurn = true
	if err := publishSpendRequested(
		ctx, o.bus, o.CharacterID, coreCombat.ActionReaction, 1, o.Ref(),
	); err != nil {
		return c, rpgerr.Wrap(err, "failed to publish opportunity attack reaction spend")
	}
	if err := o.stateChanged(ctx); err != nil {
		return c, rpgerr.Wrap(err, "failed to publish opportunity attack meter change")
	}

	return c, nil
}

// isLeavingMyThreatRange returns true if the moving entity (event.EntityID)
// was within this combatant's reach at FromPosition AND is outside reach at
// ToPosition. Returns false if this combatant cannot be located in the room
// (defensive: the OA condition holder must be in the same room as the move).
func (o *OpportunityAttackCondition) isLeavingMyThreatRange(
	room spatial.Room,
	event *dnd5eEvents.MovementChainEvent,
) bool {
	threatenerPos, found := room.GetEntityPosition(o.CharacterID)
	if !found {
		return false
	}
	fromPos := spatial.Position{X: event.FromPosition.X, Y: event.FromPosition.Y}
	toPos := spatial.Position{X: event.ToPosition.X, Y: event.ToPosition.Y}

	grid := room.GetGrid()
	distFrom := grid.Distance(threatenerPos, fromPos)
	distTo := grid.Distance(threatenerPos, toPos)

	reach := o.reach()
	return distFrom <= reach && distTo > reach
}

// reach returns the threatener's melee reach in grid units. Defaults to 5ft
// (1 grid unit). Future: read the holder's equipped weapon for reach-weapon
// support (10ft for glaives/halberds), and check incapacitated/prone state.
func (o *OpportunityAttackCondition) reach() float64 {
	// Reference combat.DefaultMeleeReach indirectly through the local constant
	// to avoid creating an import-cycle expectation across the conditions
	// package and combat. The two should match.
	_ = combat.DefaultMeleeReach // compile-time witness that the constants align
	return defaultMeleeReach
}
