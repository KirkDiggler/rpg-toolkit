// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package monsters provides factory functions for creating D&D 5e monster stat blocks
package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewBanditMelee creates a CR 1/8 bandit with scimitar
func NewBanditMelee(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Bandit",
		Ref:  refs.Monsters.Bandit(),
		HP:   11, // 2d8+2
		AC:   12, // Leather armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 11, // +0
			abilities.DEX: 12, // +1
			abilities.CON: 12, // +1
			abilities.INT: 10, // +0
			abilities.WIS: 10, // +0
			abilities.CHA: 10, // +0
		},
	})

	// Scimitar melee attack
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.BanditScimitar(),
		Name: "scimitar",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 3, // +1 DEX + 2 proficiency
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Slashing, FlatBonus: 1}},
		},
	})

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}

// NewBanditRanged creates a CR 1/8 bandit with light crossbow
func NewBanditRanged(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Bandit",
		Ref:  refs.Monsters.BanditArcher(),
		HP:   11, // 2d8+2
		AC:   12, // Leather armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 11, // +0
			abilities.DEX: 12, // +1
			abilities.CON: 12, // +1
			abilities.INT: 10, // +0
			abilities.WIS: 10, // +0
			abilities.CHA: 10, // +0
		},
	})

	// Light crossbow ranged attack
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.BanditLightCrossbow(),
		Name: "light crossbow",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}},
			AttackBonus: 3, // +1 DEX + 2 proficiency
			Damage:      []damage.Damage{{Dice: "1d8", Type: damage.Piercing, FlatBonus: 1}},
		},
	})

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}
