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

// NewZombie creates a CR 1/4 zombie with slam attack and Undead Fortitude
func NewZombie(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Zombie",
		Ref:  refs.Monsters.Zombie(),
		HP:   22, // 3d8+9
		AC:   8,  // No armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 13, // +1
			abilities.DEX: 6,  // -2
			abilities.CON: 16, // +3
			abilities.INT: 3,  // -4
			abilities.WIS: 6,  // -2
			abilities.CHA: 5,  // -3
		},
	})

	// Slam melee attack
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "slam",
		AttackBonus: 3,       // +1 STR + 2 proficiency
		DamageDice:  "1d6+1", // 1d6 + STR
		Reach:       5,
		DamageType:  damage.Bludgeoning,
	}))

	// Set movement speed (zombies are slow)
	m.SetSpeed(monster.SpeedData{Walk: 20})

	// Note: Undead Fortitude trait (CON save to stay at 1 HP when dropped to 0)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.
	// The zombie has CON modifier +3 for the save.

	return m
}
