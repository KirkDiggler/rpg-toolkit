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

// NewThug creates a CR 1 thug boss with multiattack (2x mace) and Pack Tactics
func NewThug(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Thug",
		Ref:  refs.Monsters.Thug(),
		HP:   32, // 5d8+10
		AC:   11, // Leather armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 15, // +2
			abilities.DEX: 11, // +0
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 10, // +0
			abilities.CHA: 11, // +0
		},
	})

	// Mace attack (part of multiattack)
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "mace",
		AttackBonus: 4,       // +2 STR + 2 proficiency
		DamageDice:  "1d6+2", // 1d6 + STR
		Reach:       5,
		DamageType:  damage.Bludgeoning,
	}))

	// Multiattack - 2x mace
	m.AddAction(actions.NewMultiattackAction(actions.MultiattackConfig{
		Attacks: []string{"mace", "mace"},
	}))

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
