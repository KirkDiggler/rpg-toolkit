// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// PoisonedCondition causes disadvantage on attack rolls.
type PoisonedCondition struct {
	*BaseCondition
	subscriptionID string
}

// NewPoisonedCondition creates a new poisoned condition.
func NewPoisonedCondition(id, source string, sourceEntity core.Entity, duration Duration) *PoisonedCondition {
	return &PoisonedCondition{
		BaseCondition: NewCondition(id, "poisoned", source, sourceEntity, duration),
	}
}

func (c *PoisonedCondition) OnApply(bus events.EventBus, target core.Entity) error {
	// Subscribe to attack roll events to add disadvantage
	c.subscriptionID = bus.SubscribeFunc(events.EventAttackRoll, 100, func(ctx context.Context, e events.Event) error {
		// Check if the attacker has this condition
		if e.Source() != nil && target != nil && e.Source().GetID() == target.GetID() {
			// Add disadvantage modifier
			e.Context().AddModifier(events.NewModifier(
				"poisoned",
				events.ModifierDisadvantage,
				true,
				100,
			))
		}
		return nil
	})
	return nil
}

func (c *PoisonedCondition) OnRemove(bus events.EventBus, target core.Entity) error {
	// Unsubscribe from events
	if c.subscriptionID != "" {
		bus.Unsubscribe(c.subscriptionID)
	}
	return nil
}

// BlessedCondition adds a bonus to attack rolls and saving throws.
type BlessedCondition struct {
	*BaseCondition
	attackSubID string
	saveSubID   string
}

// NewBlessedCondition creates a new blessed condition.
func NewBlessedCondition(id, source string, sourceEntity core.Entity, duration Duration) *BlessedCondition {
	return &BlessedCondition{
		BaseCondition: NewCondition(id, "blessed", source, sourceEntity, duration),
	}
}

func (c *BlessedCondition) OnApply(bus events.EventBus, target core.Entity) error {
	// Subscribe to attack rolls
	c.attackSubID = bus.SubscribeFunc(events.EventAttackRoll, 50, func(ctx context.Context, e events.Event) error {
		if e.Source() != nil && target != nil && e.Source().GetID() == target.GetID() {
			// Add d4 bonus (we'll use 2.5 average for simplicity)
			e.Context().AddModifier(events.NewModifier(
				"blessed",
				events.ModifierAttackBonus,
				3, // Average of d4
				50,
			))
		}
		return nil
	})

	// Subscribe to saving throws
	c.saveSubID = bus.SubscribeFunc(events.EventSavingThrow, 50, func(ctx context.Context, e events.Event) error {
		if e.Source() != nil && target != nil && e.Source().GetID() == target.GetID() {
			e.Context().AddModifier(events.NewModifier(
				"blessed",
				events.ModifierSaveBonus,
				3, // Average of d4
				50,
			))
		}
		return nil
	})

	return nil
}

func (c *BlessedCondition) OnRemove(bus events.EventBus, target core.Entity) error {
	if c.attackSubID != "" {
		bus.Unsubscribe(c.attackSubID)
	}
	if c.saveSubID != "" {
		bus.Unsubscribe(c.saveSubID)
	}
	return nil
}

// StunnedCondition prevents actions and reactions.
type StunnedCondition struct {
	*BaseCondition
	actionSubID string
}

// NewStunnedCondition creates a new stunned condition.
func NewStunnedCondition(id, source string, sourceEntity core.Entity, duration Duration) *StunnedCondition {
	return &StunnedCondition{
		BaseCondition: NewCondition(id, "stunned", source, sourceEntity, duration),
	}
}

func (c *StunnedCondition) OnApply(bus events.EventBus, target core.Entity) error {
	// Subscribe to action attempts
	c.actionSubID = bus.SubscribeFunc("before_action", 0, func(ctx context.Context, e events.Event) error {
		if e.Source() != nil && target != nil && e.Source().GetID() == target.GetID() {
			// Prevent the action
			e.Context().Set("prevented", true)
			e.Context().Set("reason", "stunned")
		}
		return nil
	})
	return nil
}

func (c *StunnedCondition) OnRemove(bus events.EventBus, target core.Entity) error {
	if c.actionSubID != "" {
		bus.Unsubscribe(c.actionSubID)
	}
	return nil
}

