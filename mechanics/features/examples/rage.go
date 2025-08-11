// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package examples

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// RageListener handles events for the Rage feature.
type RageListener struct{}

// EventTypes returns the event types this listener cares about.
func (r RageListener) EventTypes() []string {
	return []string{
		events.EventOnDamageRoll,
		events.EventBeforeTakeDamage,
		events.EventOnTurnEnd,
	}
}

// Priority returns the priority for event handling.
func (r RageListener) Priority() int {
	return 100
}

// HandleEvent processes the event for the Rage feature.
func (r RageListener) HandleEvent(_ features.Feature, _ core.Entity, event events.Event) error {
	switch event.Type() {
	case events.EventOnDamageRoll:
		// Add rage damage bonus to melee STR-based attacks
		if r.isMeleeStrengthAttack(event) {
			event.Context().AddModifier(events.NewModifier(
				"rage",
				events.ModifierDamageBonus,
				events.NewRawValue(2, "rage"),
				150,
			))
		}

	case events.EventBeforeTakeDamage:
		// Add resistance to physical damage
		if r.isPhysicalDamage(event) {
			// Add resistance modifier (halve damage)
			event.Context().AddModifier(events.NewModifier(
				"rage_resistance",
				"damage_resistance",
				events.NewRawValue(50, "rage"), // 50% reduction
				200,
			))
		}

	case events.EventOnTurnEnd:
		// Check if rage should end (no attack or damage taken)
		// This would need additional tracking in a real implementation
	}

	return nil
}

func (r RageListener) isMeleeStrengthAttack(event events.Event) bool {
	// Check if this is a melee attack using STR
	weapon, hasWeapon := event.Context().GetString(events.ContextKeyWeapon)
	if !hasWeapon {
		return false
	}

	// Check if it's a melee weapon (simplified)
	return weapon != "shortbow" && weapon != "longbow" && weapon != "crossbow"
}

func (r RageListener) isPhysicalDamage(event events.Event) bool {
	// Check if damage type is physical
	damageType, hasDamageType := event.Context().GetString(events.ContextKeyDamageType)
	if !hasDamageType {
		return false
	}

	// Physical damage types
	return damageType == "slashing" || damageType == "piercing" || damageType == "bludgeoning"
}

// CreateRageFeature creates the Barbarian Rage feature.
func CreateRageFeature() features.Feature {
	// Create the rage resource (2 uses per long rest at level 1)
	rageResource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:              "rage_uses_barbarian",
		Type:            resources.ResourceTypeAbilityUse,
		Key:             "rage_uses",
		Current:         2,
		Maximum:         2,
		LongRestRestore: -1, // Restore to full on long rest
		RestoreType:     resources.RestoreLongRest,
	})

	return features.NewBasicFeature(core.MustNewRef("rage", "dnd5e", "class_feature"), "Rage").
		WithDescription("In battle, you fight with primal ferocity. On your turn, you can enter a rage as a bonus action.").
		WithType(features.FeatureClass).
		WithLevel(1).
		WithSource(&core.Source{Category: core.SourceClass, Name: "Barbarian"}).
		WithTiming(features.TimingActivated).
		WithResources(rageResource).
		WithEventListeners(RageListener{}).
		WithPrerequisites("class:barbarian", "level:1")
}

// RageActivationExample shows how rage might be activated in a game.
// This is just an example - actual implementation would be game-specific.
func RageActivationExample(character features.FeatureHolder, eventBus events.EventBus) error {
	// When a player uses a bonus action to rage
	return character.ActivateFeature("rage", eventBus)
}
