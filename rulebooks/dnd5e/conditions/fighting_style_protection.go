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
)

// FightingStyleProtectionData is the JSON structure for persisting protection condition state
type FightingStyleProtectionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// protectionOwner is the minimal, structurally-satisfied view of the
// protector's OWN live sheet this condition needs — shield equipped, and
// the same action-economy ledger [combat.Pay]/[combat.CanPay] already read
// (rpg-toolkit#1178). A *character.Character satisfies this without
// `conditions` importing `character` (which would cycle — character already
// imports conditions to load them); see [dnd5eEvents.OwnerAware].
type protectionOwner interface {
	combat.Ledger
	HasShieldEquipped() bool
}

// FightingStyleProtectionCondition imposes disadvantage on attacks against adjacent allies.
// When a creature you can see attacks a target other than you that is within 5 feet of you,
// you can use your reaction to impose disadvantage on the attack roll. You must be wielding a shield.
type FightingStyleProtectionCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
	owner           protectionOwner
}

// Ensure FightingStyleProtectionCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleProtectionCondition)(nil)

// Ensure FightingStyleProtectionCondition implements dnd5eEvents.OwnerAware
var _ dnd5eEvents.OwnerAware = (*FightingStyleProtectionCondition)(nil)

// SetOwner hands this condition its own character's live sheet, read the
// same way [combat.Pay] already does, in place of a context-installed
// gamectx registry (rpg-toolkit#1178). An owner that does not satisfy
// [protectionOwner] is ignored: the condition simply never finds itself
// eligible, the same "nothing to do" default every other unmet check here
// already takes.
func (f *FightingStyleProtectionCondition) SetOwner(owner any) {
	if o, ok := owner.(protectionOwner); ok {
		f.owner = o
	}
}

// NewFightingStyleProtectionCondition creates a new Protection fighting style condition.
func NewFightingStyleProtectionCondition(characterID string) *FightingStyleProtectionCondition {
	return &FightingStyleProtectionCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleProtectionCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to attack chain events.
func (f *FightingStyleProtectionCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "protection fighting style already applied")
	}
	f.bus = bus

	// Subscribe to AttackChain to impose disadvantage when eligible
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, f.onAttackChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleProtectionCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if f.bus == nil {
		return nil
	}

	total := len(f.subscriptionIDs)
	var errs []error
	for _, subID := range f.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (f *FightingStyleProtectionCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleProtectionData{
		Ref:         refs.Conditions.FightingStyleProtection(),
		CharacterID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads protection condition state from JSON.
func (f *FightingStyleProtectionCondition) loadJSON(data json.RawMessage) error {
	var protectionData FightingStyleProtectionData
	if err := json.Unmarshal(data, &protectionData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal protection data")
	}

	f.CharacterID = protectionData.CharacterID
	return nil
}

// onAttackChain imposes disadvantage on attacks against nearby allies when using shield and reaction.
//
// rpg-toolkit#1178: this used to fire on the protector's OWN outgoing
// attacks too, because it excluded only "target is me" and never "attacker
// is me" — but Protection is a REACTION to someone else's attack (the doc
// comment above says so: "a creature ... attacks a target other than
// you"), never a response to your own swing. That gap put every armed
// attack by a Protection-wielding character on the code path below, which
// depends on gamectx.RequireCharacters — a live registry the session stack
// never installs — turning an unrelated eligibility bug into a crash on
// every one of that character's own attacks.
func (f *FightingStyleProtectionCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Never a reaction to my own attack.
	if event.AttackerID == f.CharacterID {
		return c, nil
	}

	// Only triggers for attacks on OTHER creatures (not self)
	if event.TargetID == f.CharacterID {
		return c, nil
	}

	// Only triggers for melee attacks
	if !event.IsMelee {
		return c, nil
	}

	// Must be wielding a shield — read off this character's own live sheet,
	// handed over at attach time (rpg-toolkit#1178), never a
	// context-installed registry.
	if f.owner == nil || !f.owner.HasShieldEquipped() {
		return c, nil
	}

	// Must have a reaction available — the same ledger combat.Pay/CanPay
	// already read, so "can I react" is answered the identical way
	// everywhere in this rulebook it is asked.
	if f.owner.SlotsLeft(coreCombat.ActionReaction) <= 0 {
		return c, nil
	}

	// Check if we're within 5 feet of the target. Positions are genuinely
	// world state no single character's sheet carries, so this stays on
	// [gamectx.RequireRoom] — the one registry resolution.Resolve DOES
	// install (for prone's range predicate), and the only one this
	// condition still needs.
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return c, err
	}

	// Get positions of fighter and target
	fighterPos, fighterExists := room.GetEntityPosition(f.CharacterID)
	targetPos, targetExists := room.GetEntityPosition(event.TargetID)
	if !fighterExists || !targetExists {
		return c, nil
	}

	// Check if within 5 feet (adjacent on grid = distance 1)
	grid := room.GetGrid()
	distance := grid.Distance(fighterPos, targetPos)
	if distance > 1 {
		return c, nil
	}

	// All conditions met - add modifier to impose disadvantage at StageFeatures
	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceID:  f.CharacterID,
			SourceRef: refs.Conditions.FightingStyleProtection(),
			Reason:    "protection_fighting_style",
		})

		// Record that reaction was consumed
		e.ReactionsConsumed = append(e.ReactionsConsumed, dnd5eEvents.ReactionConsumption{
			CharacterID: f.CharacterID,
			FeatureRef:  refs.Conditions.FightingStyleProtection(),
			Reason:      "protection_fighting_style",
		})

		// Actually consume the reaction. SpendSlots never refuses — the
		// eligibility check above already ran, and Ledger's own contract
		// (combat/gate.go) is that a debit past a passed check cannot fail.
		f.owner.SpendSlots(coreCombat.ActionReaction, 1)

		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "protection", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply protection for character %s", f.CharacterID)
	}

	return c, nil
}
