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

// FightingStyleArcheryData is the JSON structure for persisting archery condition state
type FightingStyleArcheryData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// FightingStyleArcheryCondition grants +2 to attack rolls with ranged weapons.
type FightingStyleArcheryCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleArcheryCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleArcheryCondition)(nil)

// NewFightingStyleArcheryCondition creates a new Archery fighting style condition.
func NewFightingStyleArcheryCondition(characterID string) *FightingStyleArcheryCondition {
	return &FightingStyleArcheryCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleArcheryCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to attack chain events.
func (f *FightingStyleArcheryCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "archery fighting style already applied")
	}
	f.bus = bus

	// Subscribe to AttackChain to add +2 bonus for ranged attacks
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, f.onAttackChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleArcheryCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (f *FightingStyleArcheryCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleArcheryData{
		Ref:         refs.Conditions.FightingStyleArchery(),
		CharacterID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads archery condition state from JSON.
func (f *FightingStyleArcheryCondition) loadJSON(data json.RawMessage) error {
	var archeryData FightingStyleArcheryData
	if err := json.Unmarshal(data, &archeryData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal archery data")
	}

	f.CharacterID = archeryData.CharacterID
	return nil
}

// onAttackChain adds +2 to attack rolls for ranged weapons.
func (f *FightingStyleArcheryCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Only modify attacks by this character
	if event.AttackerID != f.CharacterID {
		return c, nil
	}

	// Only apply to ranged attacks
	if event.IsMelee {
		return c, nil
	}

	// Add +2 to attack bonus at StageFeatures
	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AttackBonus += 2
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "archery", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply archery bonus for character %s", f.CharacterID)
	}

	return c, nil
}
