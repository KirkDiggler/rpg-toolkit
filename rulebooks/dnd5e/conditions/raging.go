// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// RagingCondition represents the barbarian rage state.
// It implements the Condition interface.
type RagingCondition struct {
	CharacterID       string          `json:"character_id"`
	DamageBonus       int             `json:"damage_bonus"`
	Level             int             `json:"level"`
	Source            string          `json:"source"`
	TurnsActive       int             `json:"turns_active"`
	WasHitThisTurn    bool            `json:"was_hit_this_turn"`
	DidAttackThisTurn bool            `json:"did_attack_this_turn"`
	subscriptionIDs   []string        `json:"-"` // Don't persist subscription IDs
	bus               events.EventBus `json:"-"` // Don't persist bus reference
}

// Ensure RagingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*RagingCondition)(nil)

// Apply subscribes this condition to relevant combat events
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	r.bus = bus

	// Subscribe to attack events to track if we attacked
	attacks := dnd5eEvents.AttackTopic.On(bus)
	subID1, err := attacks.Subscribe(ctx, r.onAttack)
	if err != nil {
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID1)

	// Subscribe to damage events to track if we were hit
	damages := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID2, err := damages.Subscribe(ctx, r.onDamageReceived)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID2)

	// Subscribe to turn end events to check if rage continues
	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	subID3, err := turnEnds.Subscribe(ctx, r.onTurnEnd)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID3)

	// Subscribe to condition applied events to check for unconscious
	conditions := dnd5eEvents.ConditionAppliedTopic.On(bus)
	subID4, err := conditions.Subscribe(ctx, r.onConditionApplied)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID4)

	// Subscribe to damage chain to add rage damage bonus
	damageChain := combat.DamageChain.On(bus)
	subID5, err := damageChain.SubscribeWithChain(ctx, r.onDamageChain)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = r.Remove(ctx, bus)
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID5)

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
	data := map[string]interface{}{
		"ref":                  "dnd5e:conditions:raging",
		"type":                 "raging",
		"character_id":         r.CharacterID,
		"damage_bonus":         r.DamageBonus,
		"level":                r.Level,
		"source":               r.Source,
		"turns_active":         r.TurnsActive,
		"was_hit_this_turn":    r.WasHitThisTurn,
		"did_attack_this_turn": r.DidAttackThisTurn,
	}
	return json.Marshal(data)
}

// onAttack handles attack events to track if we attacked this turn
func (r *RagingCondition) onAttack(_ context.Context, event dnd5eEvents.AttackEvent) error {
	if event.AttackerID != r.CharacterID {
		return nil
	}
	r.DidAttackThisTurn = true
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
		// Publish condition removed event
		if r.bus != nil {
			removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
			err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				CharacterID:  r.CharacterID,
				ConditionRef: "dnd5e:conditions:raging",
				Reason:       "no_combat_activity",
			})
			if err != nil {
				return rpgerr.Wrapf(err, "error removing raging for character id %s", r.CharacterID)
			}
		}
	}

	// Check if rage ends due to duration (10 rounds = 1 minute)
	if r.TurnsActive >= 10 {
		if r.bus != nil {
			removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
			err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				CharacterID:  r.CharacterID,
				ConditionRef: "dnd5e:conditions:raging",
				Reason:       "duration_expired",
			})
			if err != nil {
				return rpgerr.Wrapf(err, "error removing raging for character id %s", r.CharacterID)
			}
		}
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
		// End rage immediately
		if r.bus != nil {
			removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
			err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
				CharacterID:  r.CharacterID,
				ConditionRef: "dnd5e:conditions:raging",
				Reason:       "unconscious",
			})
			if err != nil {
				return rpgerr.Wrapf(err, "error removing raging for character id %s", r.CharacterID)
			}
		}
	}
	return nil
}

// onDamageChain adds rage damage bonus to attacks made by the raging character
func (r *RagingCondition) onDamageChain(
	_ context.Context,
	event *combat.DamageChainEvent,
	c chain.Chain[*combat.DamageChainEvent],
) (chain.Chain[*combat.DamageChainEvent], error) {
	// Only add bonus if we're the attacker
	if event.AttackerID != r.CharacterID {
		return c, nil
	}

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
