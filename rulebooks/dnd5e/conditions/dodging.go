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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DodgingConditionData is the serializable form of the dodging condition.
// This is stored by the game server as an opaque JSON blob.
type DodgingConditionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// DodgingCondition grants disadvantage on attack rolls against the character
// and advantage on DEX saving throws. This condition is applied when a
// character uses the Dodge combat ability and automatically removes itself
// at the start of the character's next turn.
type DodgingCondition struct {
	CharacterID     string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure DodgingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*DodgingCondition)(nil)

// NewDodgingCondition creates a new Dodging condition for the specified character.
// The condition grants disadvantage on attacks targeting this character and
// advantage on DEX saves, and removes itself at the start of the character's next turn.
func NewDodgingCondition(characterID string) *DodgingCondition {
	return &DodgingCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (d *DodgingCondition) IsApplied() bool {
	return d.bus != nil
}

// Apply subscribes this condition to AttackChain, SavingThrowChain, and TurnStart events.
// AttackChain subscription adds disadvantage when this character is targeted.
// SavingThrowChain subscription adds advantage on DEX saves for this character.
// TurnStart subscription removes the condition at the start of the character's next turn.
func (d *DodgingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if d.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "dodging condition already applied")
	}
	d.bus = bus

	// Subscribe to AttackChain to impose disadvantage on attacks targeting this character
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID1, err := attackChain.SubscribeWithChain(ctx, d.onAttackChain)
	if err != nil {
		d.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	d.subscriptionIDs = append(d.subscriptionIDs, subID1)

	// Subscribe to SavingThrowChain to grant advantage on DEX saves
	saveChain := dnd5eEvents.SavingThrowChain.On(bus)
	subID2, err := saveChain.SubscribeWithChain(ctx, d.onSavingThrowChain)
	if err != nil {
		_ = d.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to saving throw chain")
	}
	d.subscriptionIDs = append(d.subscriptionIDs, subID2)

	// Subscribe to TurnStart to remove condition at the start of the character's next turn
	turnStartTopic := dnd5eEvents.TurnStartTopic.On(bus)
	subID3, err := turnStartTopic.Subscribe(ctx, d.onTurnStart)
	if err != nil {
		_ = d.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn start topic")
	}
	d.subscriptionIDs = append(d.subscriptionIDs, subID3)

	return nil
}

// Remove unsubscribes this condition from all events.
func (d *DodgingCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if d.bus == nil {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range d.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	d.subscriptionIDs = nil
	d.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (d *DodgingCondition) ToJSON() (json.RawMessage, error) {
	data := DodgingConditionData{
		Ref:         refs.Conditions.Dodging(),
		CharacterID: d.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads dodging condition state from JSON.
func (d *DodgingCondition) loadJSON(data json.RawMessage) error {
	var dodgingData DodgingConditionData
	if err := json.Unmarshal(data, &dodgingData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal dodging data")
	}

	d.CharacterID = dodgingData.CharacterID
	return nil
}

// onAttackChain handles attack events to impose disadvantage when this character is targeted.
func (d *DodgingCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Only apply when this character is the target
	if event.TargetID != d.CharacterID {
		return c, nil
	}

	// Add disadvantage at the conditions stage
	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.Conditions.Dodging(),
			SourceID:  d.CharacterID,
			Reason:    "Dodging",
		})
		return e, nil
	}

	if err := c.Add(combat.StageConditions, "dodging_disadvantage", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add dodging disadvantage modifier for character %s", d.CharacterID)
	}

	return c, nil
}

// onSavingThrowChain handles saving throw events to grant advantage on DEX saves.
func (d *DodgingCondition) onSavingThrowChain(
	_ context.Context,
	event *dnd5eEvents.SavingThrowChainEvent,
	c chain.Chain[*dnd5eEvents.SavingThrowChainEvent],
) (chain.Chain[*dnd5eEvents.SavingThrowChainEvent], error) {
	// Only apply to this character's saves
	if event.SaverID != d.CharacterID {
		return c, nil
	}

	// Only apply to DEX saves
	if event.Ability != abilities.DEX {
		return c, nil
	}

	// Add advantage at the conditions stage
	modifySave := func(_ context.Context, e *dnd5eEvents.SavingThrowChainEvent) (*dnd5eEvents.SavingThrowChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.SaveModifierSource{
			Name:       "Dodging",
			SourceType: "condition",
			SourceRef:  refs.Conditions.Dodging(),
			EntityID:   d.CharacterID,
		})
		return e, nil
	}

	if err := c.Add(combat.StageConditions, "dodging_dex_advantage", modifySave); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add dodging DEX advantage modifier for character %s", d.CharacterID)
	}

	return c, nil
}

// onTurnStart handles turn start events to remove this condition at the start of the character's next turn.
func (d *DodgingCondition) onTurnStart(ctx context.Context, event dnd5eEvents.TurnStartEvent) error {
	// Only remove on this character's turn start
	if event.CharacterID != d.CharacterID {
		return nil
	}

	if d.bus == nil {
		return nil
	}

	// Publish condition removed event
	removals := dnd5eEvents.ConditionRemovedTopic.On(d.bus)
	err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  d.CharacterID,
		ConditionRef: refs.Conditions.Dodging().String(),
		Reason:       "turn_start",
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish dodging removal for character %s", d.CharacterID)
	}

	// Actually remove the condition (unsubscribe from events)
	return d.Remove(ctx, d.bus)
}
