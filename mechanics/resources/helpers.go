// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// CreateSpellSlots creates a set of spell slot resources for a caster.
// The slots map should have level (1-9) as keys and max slots as values.
func CreateSpellSlots(owner core.Entity, slots map[int]int) []Resource {
	var resources []Resource

	for level := 1; level <= 9; level++ {
		if maxSlots, exists := slots[level]; exists && maxSlots > 0 {
			resource := NewSimpleResource(SimpleResourceConfig{
				ID:              fmt.Sprintf("%s-spell-slots-%d", owner.GetID(), level),
				Type:            ResourceTypeSpellSlot,
				Owner:           owner,
				Key:             fmt.Sprintf("spell_slots_%d", level),
				Current:         maxSlots,
				Maximum:         maxSlots,
				RestoreType:     RestoreLongRest,
				LongRestRestore: -1, // Full restore on long rest
			})
			resources = append(resources, resource)
		}
	}

	return resources
}

// CreateAbilityUse creates a resource for tracking ability uses.
func CreateAbilityUse(owner core.Entity, abilityName string, maxUses int, restoreType RestorationType) Resource {
	var shortRestore, longRestore int

	switch restoreType {
	case RestoreShortRest:
		shortRestore = -1 // Full restore
		longRestore = -1  // Also restore on long rest
	case RestoreLongRest:
		longRestore = -1 // Full restore
	case RestoreTurn:
		// No automatic restoration
	}

	return NewSimpleResource(SimpleResourceConfig{
		ID:               fmt.Sprintf("%s-%s-uses", owner.GetID(), abilityName),
		Type:             ResourceTypeAbilityUse,
		Owner:            owner,
		Key:              fmt.Sprintf("%s_uses", abilityName),
		Current:          maxUses,
		Maximum:          maxUses,
		RestoreType:      restoreType,
		ShortRestRestore: shortRestore,
		LongRestRestore:  longRestore,
	})
}

// CreateHitDice creates hit dice resources for a character.
func CreateHitDice(owner core.Entity, hitDieType string, level int) Resource {
	return NewSimpleResource(SimpleResourceConfig{
		ID:              fmt.Sprintf("%s-hit-dice-%s", owner.GetID(), hitDieType),
		Type:            ResourceTypeHitDice,
		Owner:           owner,
		Key:             fmt.Sprintf("hit_dice_%s", hitDieType),
		Current:         level,
		Maximum:         level,
		RestoreType:     RestoreLongRest,
		LongRestRestore: level / 2, // Restore half level (minimum 1) on long rest
	})
}

// CreateActionEconomy creates action economy resources for combat.
func CreateActionEconomy(owner core.Entity) []Resource {
	resources := []Resource{
		// Standard action
		NewSimpleResource(SimpleResourceConfig{
			ID:          fmt.Sprintf("%s-action", owner.GetID()),
			Type:        ResourceTypeAction,
			Owner:       owner,
			Key:         "action",
			Current:     1,
			Maximum:     1,
			RestoreType: RestoreTurn,
		}),
		// Bonus action
		NewSimpleResource(SimpleResourceConfig{
			ID:          fmt.Sprintf("%s-bonus-action", owner.GetID()),
			Type:        ResourceTypeBonusAction,
			Owner:       owner,
			Key:         "bonus_action",
			Current:     1,
			Maximum:     1,
			RestoreType: RestoreTurn,
		}),
		// Reaction
		NewSimpleResource(SimpleResourceConfig{
			ID:          fmt.Sprintf("%s-reaction", owner.GetID()),
			Type:        ResourceTypeReaction,
			Owner:       owner,
			Key:         "reaction",
			Current:     1,
			Maximum:     1,
			RestoreType: RestoreTurn,
		}),
	}

	return resources
}

// CreateRageUses creates a resource for barbarian rage uses.
func CreateRageUses(owner core.Entity, level int) Resource {
	// Calculate rage uses based on level
	var uses int
	switch {
	case level < 3:
		uses = 2
	case level < 6:
		uses = 3
	case level < 12:
		uses = 4
	case level < 17:
		uses = 5
	case level < 20:
		uses = 6
	default:
		uses = -1 // Unlimited at level 20
	}

	// At level 20, barbarians have unlimited rages
	if uses == -1 {
		return NewSimpleResource(SimpleResourceConfig{
			ID:          fmt.Sprintf("%s-rage-uses", owner.GetID()),
			Type:        ResourceTypeAbilityUse,
			Owner:       owner,
			Key:         "rage_uses",
			Current:     999, // Effectively unlimited
			Maximum:     999,
			RestoreType: RestoreNever, // Doesn't need restoration
		})
	}

	return CreateAbilityUse(owner, "rage", uses, RestoreLongRest)
}

// CreateKiPoints creates a resource for monk ki points.
func CreateKiPoints(owner core.Entity, level int) Resource {
	if level < 2 {
		return nil // Monks don't get ki until level 2
	}

	return NewSimpleResource(SimpleResourceConfig{
		ID:               fmt.Sprintf("%s-ki-points", owner.GetID()),
		Type:             ResourceTypeAbilityUse,
		Owner:            owner,
		Key:              "ki_points",
		Current:          level,
		Maximum:          level,
		RestoreType:      RestoreShortRest,
		ShortRestRestore: -1, // Full restore on short rest
		LongRestRestore:  -1, // Also restore on long rest
	})
}
