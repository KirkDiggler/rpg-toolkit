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

// NewGiantRat creates a CR 1/8 giant rat with bite attack and Pack Tactics
func NewGiantRat(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Giant Rat",
		HP:   7,  // 2d6
		AC:   12, // Natural armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 7,  // -2
			abilities.DEX: 15, // +2
			abilities.CON: 11, // +0
			abilities.INT: 2,  // -4
			abilities.WIS: 10, // +0
			abilities.CHA: 4,  // -3
		},
	})

	// Bite attack
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "bite",
		AttackBonus: 4,       // +2 DEX + 2 proficiency
		DamageDice:  "1d4+2", // 1d4 + DEX
		Reach:       5,
		DamageType:  damage.Piercing,
	}))

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
