// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ApplyConditionInput defines the input for applying a condition
type ApplyConditionInput struct {
	Target       core.Entity
	Condition    Condition
	EventBus     *events.Bus
	OverrideData map[string]interface{} // Optional overrides for condition metadata
}

// ApplyConditionOutput defines the output when applying a condition
type ApplyConditionOutput struct {
	Applied     bool
	PreviousRef *Condition // Reference to previous condition if it was replaced
}

// RemoveConditionInput defines the input for removing a condition
type RemoveConditionInput struct {
	Target      core.Entity
	Type        ConditionType
	Source      string // Optional: only remove if from this source
	EventBus    *events.Bus
}

// RemoveConditionOutput defines the output when removing a condition
type RemoveConditionOutput struct {
	Removed   bool
	Condition *Condition // The condition that was removed
}

// TickDurationInput defines the input for ticking condition durations
type TickDurationInput struct {
	Target       core.Entity
	DurationType DurationType
	Amount       int // How many units to tick (usually 1)
}

// TickDurationOutput defines the output when ticking durations
type TickDurationOutput struct {
	ExpiredConditions []Condition // Conditions that expired
}

// Manager handles condition lifecycle and interactions
type Manager interface {
	// ApplyCondition applies a condition to a target entity
	ApplyCondition(ctx context.Context, input *ApplyConditionInput) (*ApplyConditionOutput, error)

	// RemoveCondition removes a condition from a target
	RemoveCondition(ctx context.Context, input *RemoveConditionInput) (*RemoveConditionOutput, error)

	// TickDuration decrements duration for time-based conditions
	TickDuration(ctx context.Context, input *TickDurationInput) (*TickDurationOutput, error)

	// GetConditions returns all conditions on a target
	GetConditions(ctx context.Context, target core.Entity) ([]Condition, error)

	// HasCondition checks if target has a specific condition
	HasCondition(ctx context.Context, target core.Entity, conditionType ConditionType) (bool, error)
}

// RagingConditionHandler handles the specific mechanics of the raging condition
type RagingConditionHandler struct {
	bus *events.Bus
}

// NewRagingConditionHandler creates a new handler for raging condition
func NewRagingConditionHandler(bus *events.Bus) *RagingConditionHandler {
	return &RagingConditionHandler{bus: bus}
}

// OnApply is called when the raging condition is applied
func (h *RagingConditionHandler) OnApply(ctx context.Context, target core.Entity, condition *Condition) error {
	// Subscribe to attack events for damage bonus
	attackSub, err := h.bus.SubscribeWithFilter(
		dnd5e.EventRefAttack,
		h.onAttack(target, condition),
		func(e events.Event) bool {
			if attack, ok := e.(*dnd5e.AttackEvent); ok {
				return attack.Attacker == target
			}
			return false
		},
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to attack events: %w", err)
	}

	// Subscribe to damage events for resistance
	damageSub, err := h.bus.SubscribeWithFilter(
		dnd5e.EventRefDamageReceived,
		h.onDamageReceived(target, condition),
		func(e events.Event) bool {
			if damage, ok := e.(*dnd5e.DamageReceivedEvent); ok {
				return damage.Target == target
			}
			return false
		},
	)
	if err != nil {
		// Clean up attack subscription
		_ = h.bus.Unsubscribe(attackSub)
		return fmt.Errorf("failed to subscribe to damage events: %w", err)
	}

	// Store subscription IDs in condition metadata for cleanup
	if condition.Metadata == nil {
		condition.Metadata = make(map[string]interface{})
	}
	condition.Metadata["subscriptions"] = []string{attackSub, damageSub}

	// Publish rage started event
	damageBonus := h.getDamageBonus(condition)
	_ = h.bus.Publish(&dnd5e.RageStartedEvent{
		Owner:       target,
		DamageBonus: damageBonus,
	})

	return nil
}

// OnRemove is called when the raging condition is removed
func (h *RagingConditionHandler) OnRemove(ctx context.Context, target core.Entity, condition *Condition) error {
	// Unsubscribe from events
	if subs, ok := condition.Metadata["subscriptions"].([]string); ok {
		for _, sub := range subs {
			_ = h.bus.Unsubscribe(sub)
		}
	}

	// Publish rage ended event
	_ = h.bus.Publish(&dnd5e.RageEndedEvent{
		Owner: target,
	})

	return nil
}

// getDamageBonus calculates the rage damage bonus from condition metadata
func (h *RagingConditionHandler) getDamageBonus(condition *Condition) int {
	if condition.Metadata == nil {
		return 2 // Default to level 1-8 bonus
	}

	if level, ok := condition.Metadata["barbarian_level"].(int); ok {
		if level >= 16 {
			return 4
		} else if level >= 9 {
			return 3
		}
	}
	return 2
}

// onAttack handles attack events to add damage bonus
func (h *RagingConditionHandler) onAttack(target core.Entity, condition *Condition) func(interface{}) error {
	return func(e interface{}) error {
		attack := e.(*dnd5e.AttackEvent)

		// Only add bonus to Strength-based melee attacks
		if attack.IsMelee && attack.Ability == dnd5e.AbilityStrength {
			damageBonus := h.getDamageBonus(condition)
			ctx := attack.Context()
			ctx.AddModifier(events.NewSimpleModifier(
				dnd5e.ModifierSourceRage,
				dnd5e.ModifierTypeAdditive,
				dnd5e.ModifierTargetDamage,
				200, // priority (after base damage)
				damageBonus,
			))
		}

		return nil
	}
}

// onDamageReceived handles damage events to apply resistance
func (h *RagingConditionHandler) onDamageReceived(target core.Entity, condition *Condition) func(interface{}) error {
	return func(e interface{}) error {
		damage := e.(*dnd5e.DamageReceivedEvent)

		// Apply resistance to physical damage
		if damage.DamageType == dnd5e.DamageTypeBludgeoning ||
			damage.DamageType == dnd5e.DamageTypePiercing ||
			damage.DamageType == dnd5e.DamageTypeSlashing {
			// Add resistance modifier (halves damage)
			ctx := damage.Context()
			ctx.AddModifier(events.NewSimpleModifier(
				dnd5e.ModifierSourceRage,
				dnd5e.ModifierTypeResistance,
				dnd5e.ModifierTargetDamage,
				100, // priority (apply early)
				0.5, // multiplier for half damage
			))
		}

		return nil
	}
}