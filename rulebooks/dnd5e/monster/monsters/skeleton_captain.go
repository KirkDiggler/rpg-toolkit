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

// NewSkeletonCaptain creates a CR 2 skeleton captain that makes two longsword
// attacks, with vulnerability to bludgeoning and immunity to poison.
//
// This is a skeleton-shaped boss, not a wight: a wight's signature Life Drain
// ability needs a max-HP-reducing attack effect that does not exist in the
// toolkit today (monstertraits/loader.go only knows immunity, vulnerability,
// pack_tactics, and undead_fortitude - and undead_fortitude itself is not
// wired into any HP-clamp path yet). Shipping a wight without Life Drain
// would just be this captain wearing a different name, so the boss stays a
// stat-line upgrade of the existing Skeleton archetype instead.
func NewSkeletonCaptain(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Skeleton Captain",
		Ref:  refs.Monsters.SkeletonCaptain(),
		HP:   45, // 6d8+18
		AC:   16, // Chain shirt + shield
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, // +3
			abilities.DEX: 14, // +2
			abilities.CON: 16, // +3
			abilities.INT: 6,  // -2
			abilities.WIS: 8,  // -1
			abilities.CHA: 5,  // -3
		},
	})

	// Multiattack first, then the component it scripts. This captain is
	// authored rather than SRD — see the type doc above — so its two-swing
	// line is a design decision of this file's, and neither swing is
	// penalised: nothing about a second longsword blow is harder than the
	// first.
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.SkeletonCaptainMultiattack(),
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: *refs.MonsterActions.SkeletonCaptainLongsword()},
				{Action: *refs.MonsterActions.SkeletonCaptainLongsword()},
			},
		},
	})

	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.SkeletonCaptainLongsword(),
		Name: "longsword",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 5,
			Damage:      []damage.Damage{{Dice: "1d8", Type: damage.Slashing, FlatBonus: 3}},
		},
	})

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Add vulnerability to bludgeoning damage (skeleton archetype, D&D 5e SRD)
	m.AddTraitData(monstertraits.MustVulnerabilityJSON(id, damage.Bludgeoning))

	// Add immunity to poison damage (skeleton archetype, D&D 5e SRD)
	m.AddTraitData(monstertraits.MustImmunityJSON(id, damage.Poison))

	return m
}
