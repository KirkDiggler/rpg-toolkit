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

// NewBrownBear creates a CR 1 brown bear with bite and claw component attacks.
// Multiattack is deferred until a sequence profile and machine exist.
func NewBrownBear(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Brown Bear",
		Ref:  refs.Monsters.BrownBear(),
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

	// Component attacks remain available until a sequence profile exists.
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.BrownBearBite(),
		Name: "bite",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 6,
			Damage:      []damage.Damage{{Dice: "1d8", Type: damage.Piercing, FlatBonus: 4}},
		},
	})
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.BrownBearClaw(),
		Name: "claw",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 6,
			Damage:      []damage.Damage{{Dice: "2d4", Type: damage.Slashing, FlatBonus: 4}},
		},
	})

	// Set movement speed (bears can also climb)
	m.SetSpeed(monster.SpeedData{
		Walk:  40,
		Climb: 30,
	})

	return m
}
