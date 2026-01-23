// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewGhoul creates a CR 1 ghoul boss with multiattack (bite + claws) and paralyzing touch
func NewGhoul(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Ghoul",
		Ref:  refs.Monsters.Ghoul(),
		HP:   22, // 5d8
		AC:   12, // Natural armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 13, // +1
			abilities.DEX: 15, // +2
			abilities.CON: 10, // +0
			abilities.INT: 7,  // -2
			abilities.WIS: 10, // +0
			abilities.CHA: 6,  // -2
		},
	})

	// Bite attack (part of multiattack)
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "bite",
		AttackBonus: 4,       // +2 DEX + 2 proficiency
		DamageDice:  "2d6+2", // 2d6 + DEX
		Reach:       5,
		DamageType:  damage.Piercing,
	}))

	// Claw attack (part of multiattack, has paralyzing touch)
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "claw",
		AttackBonus: 4,       // +2 DEX + 2 proficiency
		DamageDice:  "2d4+2", // 2d4 + DEX
		Reach:       5,
		DamageType:  damage.Slashing,
	}))

	// Multiattack - bite + claws
	m.AddAction(actions.NewMultiattackAction(actions.MultiattackConfig{
		Attacks: []string{"bite", "claw"},
	}))

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Paralyzing touch effect (CON save DC 10 or paralyzed until end of next turn)
	// would be implemented as a condition effect in the full combat system.
	// For now, the ghoul has the claw attack as part of multiattack.

	return m
}
