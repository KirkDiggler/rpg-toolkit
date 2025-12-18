// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

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

// FightingStyleProtectionCondition imposes disadvantage on attacks against adjacent allies.
// When a creature you can see attacks a target other than you that is within 5 feet of you,
// you can use your reaction to impose disadvantage on the attack roll. You must be wielding a shield.
type FightingStyleProtectionCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleProtectionCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleProtectionCondition)(nil)

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

	for _, subID := range f.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil
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
func (f *FightingStyleProtectionCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Only triggers for attacks on OTHER creatures (not self)
	if event.TargetID == f.CharacterID {
		return c, nil
	}

	// Only triggers for melee attacks
	if !event.IsMelee {
		return c, nil
	}

	// Check if we have shield equipped
	registry, ok := gamectx.Characters(ctx)
	if !ok {
		return c, nil
	}

	weapons := registry.GetCharacterWeapons(f.CharacterID)
	if weapons == nil {
		return c, nil
	}

	// Must be wielding a shield
	if !weapons.HasShield() {
		return c, nil
	}

	// Check if we have reaction available
	actionEconomy := registry.GetCharacterActionEconomy(f.CharacterID)
	if actionEconomy == nil || !actionEconomy.CanUseReaction() {
		return c, nil
	}

	// Check if we're within 5 feet of the target
	room, ok := gamectx.Room(ctx)
	if !ok {
		return c, nil
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

		// Actually consume the reaction
		if useErr := actionEconomy.UseReaction(); useErr != nil {
			return e, rpgerr.Wrap(useErr, "failed to consume reaction for protection")
		}

		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "protection", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply protection for character %s", f.CharacterID)
	}

	return c, nil
}
