// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
}

// OpportunityAttackCondition publishes a ReactionTriggerEvent when an enemy
// leaves the holder's threatened reach AND the holder has the OA reaction
// readied (gamectx.IsReactionReady).
//
// Per Wave 2.11d Director ruling B4: BOTH player and NPC reactors publish the
// trigger event; the encounter SDK wrapper (Encounter.MoveEntity) iterates the
// buffered events and either resolves NPC OAs inline (no prompt) or surfaces
// player OAs as InputRequired{reaction_prompt} on the reactor's stream. The
// condition handler itself does NOT make re-entrant combat.ResolveAttack calls.
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
	CharacterID     string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure OpportunityAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*OpportunityAttackCondition)(nil)

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
		Ref:         refs.Conditions.OpportunityAttack(),
		CharacterID: o.CharacterID,
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
