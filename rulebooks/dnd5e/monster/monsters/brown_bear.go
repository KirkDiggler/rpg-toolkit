// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewBrownBear creates a CR 1 brown bear boss with multiattack (bite + claws)
func NewBrownBear(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Brown Bear",
		HP:   34, // 4d10+12
		AC:   11, // Natural armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 19, // +4
			abilities.DEX: 10, // +0
			abilities.CON: 16, // +3
			abilities.INT: 2,  // -4
			abilities.WIS: 13, // +1
			abilities.CHA: 7,  // -2
		},
	})

	// Bite attack (part of multiattack)
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "bite",
		AttackBonus: 6,       // +4 STR + 2 proficiency
		DamageDice:  "1d8+4", // 1d8 + STR
		Reach:       5,
		DamageType:  damage.Piercing,
	}))

	// Claw attack (part of multiattack)
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "claw",
		AttackBonus: 6,       // +4 STR + 2 proficiency
		DamageDice:  "2d4+4", // 2d4 + STR
		Reach:       5,
		DamageType:  damage.Slashing,
	}))

	// Multiattack - bite + claws
	m.AddAction(actions.NewMultiattackAction(actions.MultiattackConfig{
		Attacks: []string{"bite", "claw"},
	}))

	// Set movement speed (bears can also climb)
	m.SetSpeed(monster.SpeedData{
		Walk:  40,
		Climb: 30,
	})

	return m
}
