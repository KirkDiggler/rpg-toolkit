// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spells provides spell management functionality for RPG systems.
// It includes spell casting, concentration tracking, spell slots management,
// and spell list organization for casters.
package spells

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// concentrationType is the type identifier for concentration conditions.
const concentrationType = "concentration"

// ConcentrationCondition tracks concentration on a spell.
type ConcentrationCondition struct {
	*conditions.SimpleCondition
	spell         Spell
	caster        core.Entity
	saveDC        int
	subscriptions []string
}

// NewConcentrationCondition creates a new concentration condition.
func NewConcentrationCondition(caster core.Entity, spell Spell, saveDC int) *ConcentrationCondition {
	cc := &ConcentrationCondition{
		spell:  spell,
		caster: caster,
		saveDC: saveDC,
	}

	// Create the underlying condition
	cc.SimpleCondition = conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     fmt.Sprintf("concentration_%s_%s", spell.GetID(), caster.GetID()),
		Type:   concentrationType,
		Target: caster,
		Source: fmt.Sprintf("spell:%s", spell.GetID()),
		ApplyFunc: func(_ *conditions.SimpleCondition, bus events.EventBus) error {
			return cc.apply(bus)
		},
		RemoveFunc: func(_ *conditions.SimpleCondition, bus events.EventBus) error {
			return cc.remove(bus)
		},
	})

	return cc
}

// GetSpell returns the spell being concentrated on.
func (cc *ConcentrationCondition) GetSpell() Spell {
	return cc.spell
}

// apply sets up event handlers for concentration checks.
func (cc *ConcentrationCondition) apply(bus events.EventBus) error {
	// Listen for damage to trigger concentration saves
	damageHandler := events.HandlerFunc(func(ctx context.Context, event events.Event) error {
		// Check if this is a damage event targeting the caster
		if event.Type() == events.EventOnTakeDamage && event.Target() == cc.caster {
			// Get damage amount from context
			damage, ok := event.Context().GetInt("damage")
			if !ok {
				return nil
			}

			// Calculate DC (10 or half damage, whichever is higher)
			dc := 10
			if damage/2 > dc {
				dc = damage / 2
			}

			// Publish concentration check event
			checkEvent := events.NewGameEvent(
				EventConcentrationCheck,
				cc.caster,
				nil,
			)
			checkEvent.Context().Set("spell", cc.spell)
			checkEvent.Context().Set("dc", dc)
			checkEvent.Context().Set("damage", damage)

			return bus.Publish(ctx, checkEvent)
		}
		return nil
	})

	sub := bus.SubscribeFunc(events.EventOnTakeDamage, 0, damageHandler)
	cc.subscriptions = append(cc.subscriptions, sub)

	return nil
}

// remove cleans up event subscriptions.
func (cc *ConcentrationCondition) remove(bus events.EventBus) error {
	// Unsubscribe from all events
	for _, sub := range cc.subscriptions {
		if err := bus.Unsubscribe(sub); err != nil {
			// Log but don't fail
			continue
		}
	}
	cc.subscriptions = nil

	// Publish concentration broken event
	brokenEvent := events.NewGameEvent(
		EventConcentrationBroken,
		cc.caster,
		nil,
	)
	brokenEvent.Context().Set("spell", cc.spell)

	return bus.Publish(context.TODO(), brokenEvent)
}

// ConcentrationManager helps manage concentration for casters.
type ConcentrationManager struct {
	conditionManager *conditions.ConditionManager
}

// NewConcentrationManager creates a new concentration manager.
func NewConcentrationManager(conditionManager *conditions.ConditionManager) *ConcentrationManager {
	return &ConcentrationManager{
		conditionManager: conditionManager,
	}
}

// StartConcentrating begins concentration on a spell.
func (cm *ConcentrationManager) StartConcentrating(caster core.Entity, spell Spell, saveDC int) error {
	// Check if already concentrating
	existingConditions := cm.conditionManager.GetConditions(caster)
	for _, cond := range existingConditions {
		if cond.GetType() == concentrationType {
			// Break existing concentration
			if err := cm.conditionManager.RemoveCondition(cond); err != nil {
				return fmt.Errorf("failed to break existing concentration: %w", err)
			}
		}
	}

	// Apply new concentration
	concentration := NewConcentrationCondition(caster, spell, saveDC)
	return cm.conditionManager.ApplyCondition(concentration)
}

// StopConcentrating ends concentration.
func (cm *ConcentrationManager) StopConcentrating(caster core.Entity) error {
	conditions := cm.conditionManager.GetConditions(caster)
	for _, cond := range conditions {
		if cond.GetType() == concentrationType {
			return cm.conditionManager.RemoveCondition(cond)
		}
	}
	return nil
}

// IsConcentrating checks if an entity is concentrating.
func (cm *ConcentrationManager) IsConcentrating(caster core.Entity) bool {
	conditions := cm.conditionManager.GetConditions(caster)
	for _, cond := range conditions {
		if cond.GetType() == concentrationType {
			return true
		}
	}
	return false
}

// GetConcentratedSpell returns the spell being concentrated on.
func (cm *ConcentrationManager) GetConcentratedSpell(caster core.Entity) (Spell, bool) {
	conditions := cm.conditionManager.GetConditions(caster)
	for _, cond := range conditions {
		if cc, ok := cond.(*ConcentrationCondition); ok {
			return cc.GetSpell(), true
		}
	}
	return nil, false
}
