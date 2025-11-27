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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// RagingData is the JSON structure for persisting raging condition state
type RagingData struct {
	Ref               core.Ref `json:"ref"`
	CharacterID       string   `json:"character_id"`
	DamageBonus       int      `json:"damage_bonus"`
	Level             int      `json:"level"`
	Source            string   `json:"source"`
	TurnsActive       int      `json:"turns_active"`
	WasHitThisTurn    bool     `json:"was_hit_this_turn"`
	DidAttackThisTurn bool     `json:"did_attack_this_turn"`
}

// RagingCondition represents the barbarian rage state.
// It implements the Condition interface.
type RagingCondition struct {
	CharacterID       string
	DamageBonus       int
	Level             int
	Source            string
	TurnsActive       int
	WasHitThisTurn    bool
	DidAttackThisTurn bool
	subscriptionIDs   []string
	bus               events.EventBus
}

// Ensure RagingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*RagingCondition)(nil)

// Apply subscribes this condition to relevant combat events
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	r.bus = bus

	// Subscribe to damage events to track if we were hit
	damages := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID1, err := damages.Subscribe(ctx, r.onDamageReceived)
	if err != nil {
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID1)

	// Subscribe to turn end events to check if rage continues
	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	subID2, err := turnEnds.Subscribe(ctx, r.onTurnEnd)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID2)

	// Subscribe to condition applied events to check for unconscious
	conditions := dnd5eEvents.ConditionAppliedTopic.On(bus)
	subID3, err := conditions.Subscribe(ctx, r.onConditionApplied)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID3)

	// Subscribe to damage chain to add rage damage bonus and track successful hits
	damageChain := combat.DamageChain.On(bus)
	subID4, err := damageChain.SubscribeWithChain(ctx, r.onDamageChain)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID4)

	return nil
}

// Remove unsubscribes this condition from events
func (r *RagingCondition) Remove(ctx context.Context, bus events.EventBus) error {
	// Unsubscribe from all events we subscribed to in Apply()
	if r.bus == nil {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range r.subscriptionIDs {
		err := bus.Unsubscribe(ctx, subID)
		if err != nil {
			return err
		}
	}

	r.subscriptionIDs = nil
	r.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (r *RagingCondition) ToJSON() (json.RawMessage, error) {
	data := RagingData{
		Ref: core.Ref{
			Module: "dnd5e",
			Type:   "conditions",
			Value:  "raging",
		},
		CharacterID:       r.CharacterID,
		DamageBonus:       r.DamageBonus,
		Level:             r.Level,
		Source:            r.Source,
		TurnsActive:       r.TurnsActive,
		WasHitThisTurn:    r.WasHitThisTurn,
		DidAttackThisTurn: r.DidAttackThisTurn,
	}
	return json.Marshal(data)
}

// loadJSON loads raging condition state from JSON
func (r *RagingCondition) loadJSON(data json.RawMessage) error {
	var ragingData RagingData
	if err := json.Unmarshal(data, &ragingData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal raging data")
	}

	r.CharacterID = ragingData.CharacterID
	r.DamageBonus = ragingData.DamageBonus
	r.Level = ragingData.Level
	r.Source = ragingData.Source
	r.TurnsActive = ragingData.TurnsActive
	r.WasHitThisTurn = ragingData.WasHitThisTurn
	r.DidAttackThisTurn = ragingData.DidAttackThisTurn

	return nil
}

// onDamageReceived handles damage events to track if we were hit this turn
func (r *RagingCondition) onDamageReceived(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	if event.TargetID != r.CharacterID {
		return nil
	}
	r.WasHitThisTurn = true
	return nil
}

// onTurnEnd handles turn end events to check if rage continues
func (r *RagingCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.CharacterID != r.CharacterID {
		return nil
	}

	// Increment turns active
	r.TurnsActive++

	// Check if rage ends due to no combat activity
	if !r.DidAttackThisTurn && !r.WasHitThisTurn {
		return r.endRage(ctx, "no_combat_activity")
	}

	// Check if rage ends due to duration (10 rounds = 1 minute)
	if r.TurnsActive >= 10 {
		return r.endRage(ctx, "duration_expired")
	}

	// Reset combat activity flags for next turn
	r.DidAttackThisTurn = false
	r.WasHitThisTurn = false

	return nil
}

// onConditionApplied handles condition applied events to check for unconscious
func (r *RagingCondition) onConditionApplied(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
	// Check if unconscious was applied to us
	if event.Type == dnd5eEvents.ConditionUnconscious && event.Target.GetID() == r.CharacterID {
		return r.endRage(ctx, "unconscious")
	}
	return nil
}

// endRage publishes the removal event and unsubscribes from all events
func (r *RagingCondition) endRage(ctx context.Context, reason string) error {
	if r.bus == nil {
		return nil
	}

	// Publish condition removed event
	removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
	err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  r.CharacterID,
		ConditionRef: "dnd5e:conditions:raging",
		Reason:       reason,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "error publishing rage removal for character id %s", r.CharacterID)
	}

	// Actually remove the condition (unsubscribe from events)
	return r.Remove(ctx, r.bus)
}

// onDamageChain adds rage damage bonus to attacks made by the raging character
// and tracks that we successfully hit an enemy this turn (for rage maintenance)
func (r *RagingCondition) onDamageChain(
	_ context.Context,
	event *combat.DamageChainEvent,
	c chain.Chain[*combat.DamageChainEvent],
) (chain.Chain[*combat.DamageChainEvent], error) {
	// Only add bonus if we're the attacker
	if event.AttackerID != r.CharacterID {
		return c, nil
	}

	// Track that we successfully hit an enemy this turn
	// (damage chain only fires when an attack hits)
	r.DidAttackThisTurn = true

	// Add rage damage modifier in the StageFeatures stage
	modifyDamage := func(_ context.Context, e *combat.DamageChainEvent) (*combat.DamageChainEvent, error) {
		// Append rage damage component
		e.Components = append(e.Components, combat.DamageComponent{
			Source:            combat.DamageSourceRage,
			OriginalDiceRolls: nil, // No dice
			FinalDiceRolls:    nil,
			Rerolls:           nil,
			FlatBonus:         r.DamageBonus,
			DamageType:        e.DamageType, // Same as weapon damage type
			IsCritical:        e.IsCritical,
		})
		return e, nil
	}
	err := c.Add(dnd5e.StageFeatures, "rage", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrapf(err, "error applying raging for character id %s", r.CharacterID)
	}

	return c, nil
}
