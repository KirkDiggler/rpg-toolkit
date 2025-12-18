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

// FightingStyleDefenseData is the JSON structure for persisting defense condition state
type FightingStyleDefenseData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// FightingStyleDefenseCondition grants +1 to AC while wearing armor.
type FightingStyleDefenseCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleDefenseCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleDefenseCondition)(nil)

// NewFightingStyleDefenseCondition creates a new Defense fighting style condition.
func NewFightingStyleDefenseCondition(characterID string) *FightingStyleDefenseCondition {
	return &FightingStyleDefenseCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleDefenseCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to AC chain events.
func (f *FightingStyleDefenseCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "defense fighting style already applied")
	}
	f.bus = bus

	// Subscribe to ACChain to add +1 AC when wearing armor
	acChain := combat.ACChain.On(bus)
	subID, err := acChain.SubscribeWithChain(ctx, f.onACChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to AC chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleDefenseCondition) Remove(ctx context.Context, bus events.EventBus) error {
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
func (f *FightingStyleDefenseCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleDefenseData{
		Ref:         refs.Conditions.FightingStyleDefense(),
		CharacterID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads defense condition state from JSON.
func (f *FightingStyleDefenseCondition) loadJSON(data json.RawMessage) error {
	var defenseData FightingStyleDefenseData
	if err := json.Unmarshal(data, &defenseData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal defense data")
	}

	f.CharacterID = defenseData.CharacterID
	return nil
}

// onACChain adds +1 to AC when wearing armor.
func (f *FightingStyleDefenseCondition) onACChain(
	_ context.Context,
	event *combat.ACChainEvent,
	c chain.Chain[*combat.ACChainEvent],
) (chain.Chain[*combat.ACChainEvent], error) {
	// Only modify AC for this character
	if event.CharacterID != f.CharacterID {
		return c, nil
	}

	// Only apply bonus when wearing armor
	if !event.HasArmor {
		return c, nil
	}

	// Add +1 to AC at StageFeatures
	modifyAC := func(_ context.Context, e *combat.ACChainEvent) (*combat.ACChainEvent, error) {
		e.Breakdown.AddComponent(combat.ACComponent{
			Type:   combat.ACSourceFeature,
			Source: refs.Conditions.FightingStyleDefense(),
			Value:  1,
		})
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "defense", modifyAC); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply defense bonus for character %s", f.CharacterID)
	}

	return c, nil
}
