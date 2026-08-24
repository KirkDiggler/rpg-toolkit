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

// NewThug creates a CR 1 thug with a mace component attack and Pack Tactics.
// Multiattack is deferred until a sequence profile and machine exist.
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

	// The component attack remains available until a sequence profile exists.
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.ThugMace(),
		Name: "mace",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Bludgeoning, FlatBonus: 2}},
		},
	})

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
