// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package monsters provides factory functions for creating D&D 5e monster stat blocks
package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
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
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "scimitar",
		AttackBonus: 3,       // +1 DEX + 2 proficiency
		DamageDice:  "1d6+1", // 1d6 + DEX
		Reach:       5,
		DamageType:  damage.Slashing,
	}))

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
	m.AddAction(actions.NewRangedAction(actions.RangedConfig{
		Name:        "light crossbow",
		AttackBonus: 3,       // +1 DEX + 2 proficiency
		DamageDice:  "1d8+1", // 1d8 + DEX
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	}))

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}
