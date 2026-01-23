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

// NewWolf creates a CR 1/4 wolf with bite (knockdown), Pack Tactics, and TargetLowestHP
func NewWolf(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Wolf",
		Ref:  refs.Monsters.Wolf(),
		HP:   11, // 2d8+2
		AC:   13, // Natural armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, // +1
			abilities.DEX: 15, // +2
			abilities.CON: 12, // +1
			abilities.INT: 3,  // -4
			abilities.WIS: 12, // +1
			abilities.CHA: 6,  // -2
		},
	})

	// Bite attack with knockdown (DC 11 STR save or prone)
	m.AddAction(actions.NewBiteAction(actions.BiteConfig{
		AttackBonus: 4,       // +2 DEX + 2 proficiency
		DamageDice:  "2d4+2", // 2d4 + STR
		KnockdownDC: 11,      // DC 11 STR save
		DamageType:  damage.Piercing,
	}))

	// Set movement speed (wolves are fast)
	m.SetSpeed(monster.SpeedData{Walk: 40})

	// Set targeting strategy - wolves focus wounded prey
	m.SetTargeting(monster.TargetLowestHP)

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
