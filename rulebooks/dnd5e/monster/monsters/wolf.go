// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
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

	// Bite attack: "if the target is a creature, it must succeed on a DC 11
	// Strength saving throw or be knocked prone" — declared as a gate rather
	// than as a bare DC, so the stat block says what can be contested and how
	// (ADR-0039).
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.WolfBite(),
		Name: "Bite",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}},
			OnHit: []combatActions.ConditionApplication{{
				Ref:  *refs.Conditions.Prone(),
				Save: saves.NewSaveGate(abilities.STR, 11),
			}},
		},
	})

	// Set movement speed (wolves are fast)
	m.SetSpeed(monster.SpeedData{Walk: 40})

	// Set targeting strategy - wolves focus wounded prey
	m.SetTargeting(monster.TargetLowestHP)

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
