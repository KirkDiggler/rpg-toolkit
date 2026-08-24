// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewZombie creates a CR 1/4 zombie with slam attack, immunity to poison, and Undead Fortitude
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
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.ZombieSlam(),
		Name: "slam",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 3,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Bludgeoning, FlatBonus: 1}},
		},
	})

	// Set movement speed (zombies are slow)
	m.SetSpeed(monster.SpeedData{Walk: 20})

	// Add immunity to poison damage (D&D 5e SRD)
	m.AddTraitData(monstertraits.MustImmunityJSON(id, damage.Poison))

	// Note: Undead Fortitude trait (CON save to stay at 1 HP when dropped to 0)
	// requires a dice roller and is applied when the monster is loaded into combat
	// via LoadFromData with LoadMonsterConditions. The zombie has CON modifier +3.

	return m
}
