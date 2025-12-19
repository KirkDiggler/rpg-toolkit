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

// FightingStyleDuelingData is the JSON structure for persisting dueling condition state
type FightingStyleDuelingData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// FightingStyleDuelingCondition grants +2 damage when wielding one-handed melee weapon with no off-hand weapon.
type FightingStyleDuelingCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleDuelingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleDuelingCondition)(nil)

// NewFightingStyleDuelingCondition creates a new Dueling fighting style condition.
func NewFightingStyleDuelingCondition(characterID string) *FightingStyleDuelingCondition {
	return &FightingStyleDuelingCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleDuelingCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to damage chain events.
func (f *FightingStyleDuelingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "dueling fighting style already applied")
	}
	f.bus = bus

	// Subscribe to DamageChain to add +2 damage when eligible
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, f.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleDuelingCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (f *FightingStyleDuelingCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleDuelingData{
		Ref:         refs.Conditions.FightingStyleDueling(),
		CharacterID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads dueling condition state from JSON.
func (f *FightingStyleDuelingCondition) loadJSON(data json.RawMessage) error {
	var duelingData FightingStyleDuelingData
	if err := json.Unmarshal(data, &duelingData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal dueling data")
	}

	f.CharacterID = duelingData.CharacterID
	return nil
}

// onDamageChain adds +2 to damage when wielding one-handed melee weapon with no off-hand weapon.
func (f *FightingStyleDuelingCondition) onDamageChain(
	ctx context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only modify damage for attacks by this character
	if event.AttackerID != f.CharacterID {
		return c, nil
	}

	// Get character registry from context
	registry, ok := gamectx.Characters(ctx)
	if !ok {
		// No character registry available, can't check eligibility
		return c, nil
	}

	// Get character weapons
	weapons := registry.GetCharacterWeapons(f.CharacterID)
	if weapons == nil {
		return c, nil
	}

	// Check Dueling eligibility:
	// 1. Must have a main hand weapon
	// 2. Main hand weapon must be melee and not two-handed
	// 3. Must NOT have an off-hand weapon (shields are OK)
	mainHand := weapons.MainHand()
	if mainHand == nil {
		return c, nil
	}

	if !mainHand.IsMelee {
		return c, nil
	}

	if mainHand.IsTwoHanded {
		return c, nil
	}

	offHand := weapons.OffHand()
	if offHand != nil {
		return c, nil
	}

	// Character is eligible for Dueling bonus - add +2 to damage at StageFeatures
	modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source:            dnd5eEvents.DamageSourceFeature,
			SourceRef:         refs.Conditions.FightingStyleDueling(),
			OriginalDiceRolls: nil,
			FinalDiceRolls:    nil,
			Rerolls:           nil,
			FlatBonus:         2,
			DamageType:        e.DamageType,
			IsCritical:        e.IsCritical,
		})
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "dueling", modifyDamage); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply dueling bonus for character %s", f.CharacterID)
	}

	return c, nil
}
