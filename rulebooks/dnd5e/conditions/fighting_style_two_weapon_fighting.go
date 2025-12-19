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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// FightingStyleTwoWeaponFightingData is the JSON structure for persisting TWF condition state
type FightingStyleTwoWeaponFightingData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// FightingStyleTwoWeaponFightingCondition allows adding ability modifier to off-hand weapon damage.
// Normally when you use two-weapon fighting, you don't add your ability modifier to the damage
// of the bonus action attack. With this fighting style, you can.
type FightingStyleTwoWeaponFightingCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleTwoWeaponFightingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleTwoWeaponFightingCondition)(nil)

// NewFightingStyleTwoWeaponFightingCondition creates a new Two-Weapon Fighting condition.
func NewFightingStyleTwoWeaponFightingCondition(characterID string) *FightingStyleTwoWeaponFightingCondition {
	return &FightingStyleTwoWeaponFightingCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleTwoWeaponFightingCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to damage chain events.
func (f *FightingStyleTwoWeaponFightingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "two-weapon fighting style already applied")
	}
	f.bus = bus

	// Subscribe to DamageChain to add ability modifier to off-hand attacks
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, f.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleTwoWeaponFightingCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (f *FightingStyleTwoWeaponFightingCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleTwoWeaponFightingData{
		Ref:         refs.Conditions.FightingStyleTwoWeaponFighting(),
		CharacterID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads TWF condition state from JSON.
func (f *FightingStyleTwoWeaponFightingCondition) loadJSON(data json.RawMessage) error {
	var twfData FightingStyleTwoWeaponFightingData
	if err := json.Unmarshal(data, &twfData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal two-weapon fighting data")
	}

	f.CharacterID = twfData.CharacterID
	return nil
}

// onDamageChain adds ability modifier to off-hand weapon damage.
func (f *FightingStyleTwoWeaponFightingCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only modify damage for attacks by this character
	if event.AttackerID != f.CharacterID {
		return c, nil
	}

	// Only applies to off-hand attacks
	if !event.IsOffHandAttack {
		return c, nil
	}

	// Check if there's an ability modifier to add
	if event.AbilityModifier == 0 {
		return c, nil
	}

	// Add ability modifier to damage at StageFeatures
	modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source:            dnd5eEvents.DamageSourceFeature,
			SourceRef:         refs.Conditions.FightingStyleTwoWeaponFighting(),
			OriginalDiceRolls: nil,
			FinalDiceRolls:    nil,
			Rerolls:           nil,
			FlatBonus:         e.AbilityModifier,
			DamageType:        e.DamageType,
			IsCritical:        e.IsCritical,
		})
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "two_weapon_fighting", modifyDamage); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply two-weapon fighting for character %s", f.CharacterID)
	}

	return c, nil
}
