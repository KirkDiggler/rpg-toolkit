// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewSkeleton creates a CR 1/4 skeleton with shortsword, shortbow, vulnerability to bludgeoning, and immunity to poison
func NewSkeleton(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Skeleton",
		Ref:  refs.Monsters.Skeleton(),
		HP:   13, // 2d8+4
		AC:   13, // Armor scraps
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 14, // +2
			abilities.CON: 15, // +2
			abilities.INT: 6,  // -2
			abilities.WIS: 8,  // -1
			abilities.CHA: 5,  // -3
		},
	})

	// Shortsword melee attack
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.SkeletonShortsword(),
		Name: "shortsword",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing, FlatBonus: 2}},
		},
	})

	// Shortbow ranged attack
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.SkeletonShortbow(),
		Name: "shortbow",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing, FlatBonus: 2}},
		},
	})

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Add vulnerability to bludgeoning damage (D&D 5e SRD)
	m.AddTraitData(monstertraits.MustVulnerabilityJSON(id, damage.Bludgeoning))

	// Add immunity to poison damage (D&D 5e SRD)
	m.AddTraitData(monstertraits.MustImmunityJSON(id, damage.Poison))

	return m
}
