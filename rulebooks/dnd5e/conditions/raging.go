// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Combat event types that conditions can subscribe to
// These mirror the types in the main events.go but are defined here to avoid circular imports

// AttackEvent represents a combat attack
type AttackEvent struct {
	AttackerID string
	TargetID   string
	WeaponRef  string
	IsMelee    bool
	Damage     int
}

// DamageReceivedEvent represents damage taken
type DamageReceivedEvent struct {
	TargetID   string
	SourceID   string
	Amount     int
	DamageType string
}

// TurnEndEvent represents the end of a character's turn
type TurnEndEvent struct {
	CharacterID string
	Round       int
}

// ConditionRemovedEvent is published when a condition ends
type ConditionRemovedEvent struct {
	CharacterID  string
	ConditionRef string
	Reason       string
}

// RagingConditionInput provides configuration for creating a raging condition
type RagingConditionInput struct {
	CharacterID string // ID of the raging character
	DamageBonus int    // Bonus damage for rage
	Level       int    // Barbarian level
	Source      string // What triggered this (feature ID)
}

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

// Ensure RagingCondition implements ConditionBehavior
var _ ConditionBehavior = (*RagingCondition)(nil)

// Apply subscribes this condition to relevant combat events
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	r.bus = bus

	// Subscribe to attack events to track if we attacked
	attacks := events.DefineTypedTopic[AttackEvent]("dnd5e.combat.attack").On(bus)
	subID1, err := attacks.Subscribe(ctx, r.onAttack)
	if err != nil {
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID1)

	// Subscribe to damage events to track if we were hit
	damages := events.DefineTypedTopic[DamageReceivedEvent]("dnd5e.combat.damage.received").On(bus)
	subID2, err := damages.Subscribe(ctx, r.onDamageReceived)
	if err != nil {
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID2)

	// Subscribe to turn end events to check if rage continues
	turnEnds := events.DefineTypedTopic[TurnEndEvent]("dnd5e.turn.end").On(bus)
	subID3, err := turnEnds.Subscribe(ctx, r.onTurnEnd)
	if err != nil {
		return err
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID3)

	return nil
}

// Remove unsubscribes this condition from events
func (r *RagingCondition) Remove(_ context.Context, bus events.EventBus) error {
	// Unsubscribe from all events we subscribed to in Apply()
	if r.bus == nil {
		return nil // Not applied, nothing to remove
	}

	// We know exactly what we subscribed to, so unsubscribe from those topics
	attackTopic := events.DefineTypedTopic[AttackEvent]("dnd5e.combat.attack").On(bus)
	damageTopic := events.DefineTypedTopic[DamageReceivedEvent]("dnd5e.combat.damage.received").On(bus)
	turnEndTopic := events.DefineTypedTopic[TurnEndEvent]("dnd5e.turn.end").On(bus)

	// TODO: Add Unsubscribe methods to typed topics when events package supports it
	// See issue: https://github.com/KirkDiggler/rpg-toolkit/issues/XXX
	// For now, the event bus will clean up subscriptions when it's destroyed
	_ = attackTopic
	_ = damageTopic
	_ = turnEndTopic

	r.subscriptionIDs = nil
	r.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (r *RagingCondition) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"ref":          "dnd5e:conditions:raging",
		"type":         "raging",
		"character_id": r.CharacterID,
		"damage_bonus": r.DamageBonus,
		"level":        r.Level,
		"source":       r.Source,
	}
	return json.Marshal(data)
}

// onAttack handles attack events to track if we attacked this turn
func (r *RagingCondition) onAttack(_ context.Context, event AttackEvent) error {
	if event.AttackerID != r.CharacterID {
		return nil
	}
	r.DidAttackThisTurn = true
	return nil
}

// onDamageReceived handles damage events to track if we were hit this turn
func (r *RagingCondition) onDamageReceived(_ context.Context, event DamageReceivedEvent) error {
	if event.TargetID != r.CharacterID {
		return nil
	}
	r.WasHitThisTurn = true
	return nil
}

// onTurnEnd handles turn end events to check if rage continues
func (r *RagingCondition) onTurnEnd(ctx context.Context, event TurnEndEvent) error {
	if event.CharacterID != r.CharacterID {
		return nil
	}

	// Increment turns active
	r.TurnsActive++

	// Check if rage ends due to no combat activity
	if !r.DidAttackThisTurn && !r.WasHitThisTurn {
		// Publish condition removed event
		if r.bus != nil {
			removals := events.DefineTypedTopic[ConditionRemovedEvent]("dnd5e.condition.removed").On(r.bus)
			err := removals.Publish(ctx, ConditionRemovedEvent{
				CharacterID:  r.CharacterID,
				ConditionRef: "dnd5e:conditions:raging",
				Reason:       "no_combat_activity",
			})
			if err != nil {
				return err
			}
		}
	}

	// Check if rage ends due to duration (10 rounds = 1 minute)
	if r.TurnsActive >= 10 {
		if r.bus != nil {
			removals := events.DefineTypedTopic[ConditionRemovedEvent]("dnd5e.condition.removed").On(r.bus)
			err := removals.Publish(ctx, ConditionRemovedEvent{
				CharacterID:  r.CharacterID,
				ConditionRef: "dnd5e:conditions:raging",
				Reason:       "duration_expired",
			})
			if err != nil {
				return err
			}
		}
	}

	// Reset combat activity flags for next turn
	r.DidAttackThisTurn = false
	r.WasHitThisTurn = false

	return nil
}

// NewRagingCondition creates a raging condition from input
func NewRagingCondition(input RagingConditionInput) *RagingCondition {
	return &RagingCondition{
		CharacterID: input.CharacterID,
		DamageBonus: input.DamageBonus,
		Level:       input.Level,
		Source:      input.Source,
	}
}
