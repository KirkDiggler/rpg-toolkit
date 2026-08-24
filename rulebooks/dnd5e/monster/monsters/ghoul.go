// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewGhoul creates a CR 1 ghoul with bite and claw component attacks.
// Multiattack is deferred until a sequence profile and machine exist.
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

	// Component attacks remain available until a sequence profile exists.
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.GhoulBite(),
		Name: "bite",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "2d6", Type: damage.Piercing, FlatBonus: 2}},
		},
	})
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.GhoulClaw(),
		Name: "claw",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "2d4", Type: damage.Slashing, FlatBonus: 2}},
		},
	})

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Paralyzing touch effect (CON save DC 10 or paralyzed until end of next turn)
	// would be implemented as a condition effect in the full combat system.
	// A paralyzing condition declaration belongs here when that condition's
	// parameter and lifecycle contract is ready.

	return m
}
