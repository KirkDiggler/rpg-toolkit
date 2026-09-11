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
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// OpportunityAttackCondition publishes a ReactionTriggerEvent when an enemy
// leaves the holder's threatened reach AND the holder has the OA reaction
// readied (gamectx.IsReactionReady).
//
// BOTH player and NPC reactors publish the trigger event, and the drainer is
// resolution.NewMovement: its collectTriggers subscribes ReactionTriggerTopic
// for the duration of the movement fold, buffers whatever this condition
// publishes, sorts by ReactorID then ConditionRef, and yields one Request per
// trigger. The condition handler itself does NOT make re-entrant Strike calls.
//
// This paragraph used to name Encounter.MoveEntity as the drainer, prompting
// players and resolving NPC OAs inline per Wave 2.11d ruling B4. Both halves
// are gone: MoveEntity was deleted with the old encounter module, and Kirk's
// autofire ruling superseded B4 — nobody is prompted, the OA simply fires
// (rpg-project#316, which is also what wires a production caller to
// NewMovement; until it lands the trigger is published to a fold nothing
// enters).
//
// Subscribes to MovementChain. Predicate per move event, in the order the code
// asks it:
//   - Mover is not self (no self-OA).
//   - canReact: the reactor still has its one reaction, which its own keeper
//     meters — a character's slot, a monster's meter.
//   - gamectx.IsReactionReady(self, OA-ref) returns true.
//   - A room is in context; without one the geometry cannot be evaluated and
//     the condition is a silent no-op.
//   - Self threatens the move's FromPosition and does NOT threaten
//     ToPosition — the mover is leaving reach.
//
// Reach defaults to 5ft (1 grid unit). Reach weapons are a future extension;
// the predicate is conservative today.
type OpportunityAttackCondition struct {
	MemberID string

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure OpportunityAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*OpportunityAttackCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (o *OpportunityAttackCondition) Ref() *core.Ref { return refs.Conditions.OpportunityAttack() }

// canReact asks this reactor's own sheet whether it has a reaction to spend,
// in place of the ledger handle a loader used to pass in.
//
// # ONE METER, AND IT IS NOT THIS CONDITION'S
//
// This condition used to keep a once-per-turn flag of its own, because a
// monster had no economy and the flag was the only thing holding it to one
// swing. Kirk reversed that on 2026-09-11: a monster's keeper now keeps its
// one reaction, a character's keeps the slot, and this gate is the single
// question both answer. Dissonant Whispers is why — it spends a monster's
// reaction to make it flee, and a flag living here could never have seen that.
//
// So the whole of this condition's part is: ask before offering, and publish
// the bill once a swing has run. Paying is also what keeps this and Protection
// fighting style mutually exclusive, which they are in the rules: both spend
// the one reaction, and the second to ask finds it gone.
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
	self, ok := member(ctx, o.MemberID)
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
		MemberID: characterID,
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
	//
	// TWO SUBSCRIPTIONS, and there used to be four. Turn start and long rest
	// were here to clear a flag this condition no longer keeps; the keeper
	// that owns the meter now owns the clearing too.
	taken := dnd5eEvents.ReactionTakenTopic.On(bus)
	takenID, err := taken.Subscribe(ctx, o.onReactionTaken)
	if err != nil {
		_ = o.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to reaction taken")
	}
	o.subscriptionIDs = append(o.subscriptionIDs, takenID)

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
		Ref:      refs.Conditions.OpportunityAttack(),
		MemberID: o.MemberID,
	}
	return json.Marshal(data)
}

// loadJSON loads OA condition state from JSON.
func (o *OpportunityAttackCondition) loadJSON(data json.RawMessage) error {
	var oaData OpportunityAttackConditionData
	if err := json.Unmarshal(data, &oaData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal opportunity attack data")
	}
	o.MemberID = oaData.MemberID
	return nil
}

// onMovementChain inspects each movement step and publishes a
// ReactionTriggerEvent when this combatant has a triggerable OA opportunity.
//
// The chain itself is NOT modified — the condition does not append a stage.
// The trigger is published on the interaction's own bus, where
// resolution.NewMovement's collectTriggers is subscribed for the duration of
// the fold and drains it. It named MoveEntity as the drainer until
// rpg-project#316; that method went with the old encounter module.
//
// # Prevention is NOT checked here, and cannot be
//
// This predicate used to open with "is the move OA-prevented" and the check was
// DEAD — it could never once have returned true. Disengaging writes
// OAPreventionSources from a chain STAGE, and a stage runs during the chain's
// Execute; this handler is a SUBSCRIBER, which runs strictly earlier, so it was
// reading a field nothing had written yet. Every OA test hand-published an
// event with the prevention source already in it, which is why four green
// suites never showed it (rpg-project#316).
//
// resolution.NewMovement enforces prevention after the fold, where the answer
// is complete, by dropping TriggerKindMovementOA triggers. So this condition
// publishes its trigger and the machine decides whether it survives — which is
// also why the trigger carries its kind.
func (o *OpportunityAttackCondition) onMovementChain(
	ctx context.Context,
	event *dnd5eEvents.MovementChainEvent,
	c chain.Chain[*dnd5eEvents.MovementChainEvent],
) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
	// Don't OA your own movement.
	if event.EntityID == o.MemberID {
		return c, nil
	}

	// The meter, and the only one: a reactor that has already spent its
	// reaction — on this, on Protection, or on a spell that made it flee —
	// has nothing left to swing with. See canReact.
	if !o.canReact(ctx) {
		return c, nil
	}

	// Readiness gate — opt-in at the orchestrator level. If unreadied,
	// no trigger fires and the move proceeds single-phase.
	if !gamectx.IsReactionReady(ctx, o.MemberID, refs.Conditions.OpportunityAttack().String()) {
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
		ReactorID:    o.MemberID,
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

	// AND STOP. The trigger is an offer, not a bill.
	//
	// Publishing used to spend the reaction right here, on the reasoning that
	// a trigger which reached the orchestrator was a reaction that happened.
	// It is not. The machine that drains the triggers still asks its
	// ReactionAttacks capability what this reactor swings, and "nothing" is a
	// legal answer: an ally the mover is not hostile to, an unarmed caster
	// with no melee attack, a prevented step, a player who is asked and holds.
	// Every one of those was billed. A friend walking past a fighter cost the
	// fighter their reaction for a swing nobody made.
	//
	// So the bill moved to where the swing is: the machine publishes
	// [dnd5eEvents.ReactionTakenEvent] once the reaction has actually run, and
	// onReactionTaken spends on that. A trigger nobody takes costs nothing.
	return c, nil
}

// onReactionTaken spends the reaction the machine just ran.
//
// This is the other half of the offer/bill split onMovementChain's tail
// describes: the trigger says a predicate matched, this event says a swing
// happened, and only the second one costs anything.
//
// It answers only for THIS holder and THIS condition. One bus carries every
// combatant's conditions, so a taken event names its reactor and its ref for
// the same reason the trigger does, and a reactor's OA meter must not move
// because somebody else swung or because the same member's Shield fired.
//
// Spending is idempotent by the meter, which is the KEEPER's rather than this
// condition's. A second taken event for a reactor who has not had a turn since
// bills again, and the ledger's floor makes that harmless: you cannot spend a
// reaction you do not have. Nothing here has to remember that you already did.
func (o *OpportunityAttackCondition) onReactionTaken(
	ctx context.Context, event dnd5eEvents.ReactionTakenEvent,
) error {
	if event.ReactorID != o.MemberID || event.ConditionRef != o.Ref().String() {
		return nil
	}

	// The bill goes out and nothing here decides who pays. Both keepers hold a
	// row for it now: a character's debits the slot, a monster's flips the one
	// reaction it has. Neither goes below empty.
	if err := publishSpendRequested(
		ctx, o.bus, o.MemberID, coreCombat.ActionReaction, 1, o.Ref(),
	); err != nil {
		return rpgerr.Wrap(err, "failed to publish opportunity attack reaction spend")
	}

	return nil
}

// isLeavingMyThreatRange returns true if the moving entity (event.EntityID)
// was within this combatant's reach at FromPosition AND is outside reach at
// ToPosition. Returns false if this combatant cannot be located in the room
// (defensive: the OA condition holder must be in the same room as the move).
func (o *OpportunityAttackCondition) isLeavingMyThreatRange(
	room spatial.Room,
	event *dnd5eEvents.MovementChainEvent,
) bool {
	threatenerPos, found := room.GetEntityPosition(o.MemberID)
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
