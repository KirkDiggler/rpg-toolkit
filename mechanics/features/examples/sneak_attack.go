// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package examples

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

// SneakAttackListener handles events for the Sneak Attack feature.
type SneakAttackListener struct {
	usedThisTurn bool
}

// EventTypes returns the event types this listener cares about.
func (s *SneakAttackListener) EventTypes() []string {
	return []string{
		events.EventBeforeHit,
		events.EventOnTurnStart,
	}
}

// Priority returns the priority for event handling.
func (s *SneakAttackListener) Priority() int {
	return 150
}

// HandleEvent processes the event for the Sneak Attack feature.
func (s *SneakAttackListener) HandleEvent(_ features.Feature, entity core.Entity, event events.Event) error {
	switch event.Type() {
	case events.EventOnTurnStart:
		// Reset sneak attack usage for the turn
		if event.Source() == entity {
			s.usedThisTurn = false
		}

	case events.EventBeforeHit:
		// Check if we can apply sneak attack
		if !s.usedThisTurn && s.canSneakAttack(entity, event) {
			// Add sneak attack damage
			level := s.getRogueLevel(entity)
			sneakDice := (level + 1) / 2 // 1d6 at level 1, 2d6 at level 3, etc.

			event.Context().AddModifier(events.NewModifier(
				"sneak_attack",
				events.ModifierDamageBonus,
				events.NewDiceValue(sneakDice, 6, "sneak_attack"),
				200,
			))

			s.usedThisTurn = true
		}
	}

	return nil
}

func (s *SneakAttackListener) canSneakAttack(entity core.Entity, event events.Event) bool {
	// Must be the attacker
	if event.Source() != entity {
		return false
	}

	// Must be using a finesse or ranged weapon
	if !s.isFinesseOrRangedWeapon(event) {
		return false
	}

	// Must have advantage OR have an ally adjacent to target
	if s.hasAdvantage(event) || s.hasAllyAdjacent(event) {
		return true
	}

	return false
}

func (s *SneakAttackListener) isFinesseOrRangedWeapon(event events.Event) bool {
	weapon, hasWeapon := event.Context().GetString(events.ContextKeyWeapon)
	if !hasWeapon {
		return false
	}

	// List of finesse and ranged weapons (simplified)
	finesseOrRanged := map[string]bool{
		"dagger":     true,
		"shortsword": true,
		"rapier":     true,
		"scimitar":   true,
		"whip":       true,
		"shortbow":   true,
		"longbow":    true,
		"crossbow":   true,
		"dart":       true,
	}

	return finesseOrRanged[weapon]
}

func (s *SneakAttackListener) hasAdvantage(event events.Event) bool {
	advantage, _ := event.Context().GetBool(events.ContextKeyAdvantage)
	return advantage
}

func (s *SneakAttackListener) hasAllyAdjacent(event events.Event) bool {
	// This would need game state to check positioning
	// For now, we'll check if there's an "ally_adjacent" flag
	allyAdjacent, _ := event.Context().GetBool("ally_adjacent")
	return allyAdjacent
}

func (s *SneakAttackListener) getRogueLevel(_ core.Entity) int {
	// This would need to check the entity's class levels
	// For now, return a default
	return 1
}

// CreateSneakAttackFeature creates the Rogue Sneak Attack feature.
func CreateSneakAttackFeature() features.Feature {
	return features.NewBasicFeature("sneak_attack", "Sneak Attack").
		WithDescription("Once per turn, you can deal extra damage to one creature you hit with an attack "+
			"if you have advantage on the attack roll, or if an ally is within 5 feet of the target.").
		WithType(features.FeatureClass).
		WithLevel(1).
		WithSource("Rogue").
		WithTiming(features.TimingTriggered).
		WithEventListeners(&SneakAttackListener{}).
		WithPrerequisites("class:rogue", "level:1")
}
