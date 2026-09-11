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

// NewAnimatedArmor creates a CR 1 animated armor with a slam attack and
// immunity to poison and psychic damage.
//
// The first monster here carrying TWO damage immunities. That works because
// AddTraitData appends to a slice and the loader builds a fresh condition per
// blob — the two immunity traits share a ref but are separate conditions, each
// subscribing to the bus on its own. TestAnimatedArmorTraitsIncludedInData
// asserts both survive, since one clobbering the other is exactly the failure
// a single-immunity roster could never have caught.
//
// Deliberately NOT represented, each because the mechanism does not exist —
// not because the SRD was skimmed:
//
//   - Multiattack (two slams). Deferred until a sequence profile and machine
//     exist, the same sentence brown_bear/ghoul/thug/skeleton_captain carry.
//     One slam is registered; the armor hits once.
//   - Condition immunities (blinded, charmed, deafened, exhaustion,
//     frightened, paralyzed, petrified, poisoned). monstertraits offers
//     Immunity/Vulnerability/PackTactics/UndeadFortitude — damage immunity
//     only. There is no condition-immunity trait to attach, and inventing one
//     for a monster nothing currently charms would be a mechanism without a
//     use case.
//   - Blindsight 60 ft. monster.Data has a Blindsight field, but nothing
//     reads it (only Darkvision has a reader) and monster.Config exposes no
//     way to set senses at all. Writing an inert number would claim a sense
//     the engine does not honor.
//   - Antimagic Susceptibility and False Appearance. No antimagic field and
//     no "indistinguishable while motionless" concept exist to hang them on.
func NewAnimatedArmor(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Animated Armor",
		Ref:  refs.Monsters.AnimatedArmor(),
		HP:   33, // 6d8+6
		AC:   18, // Natural armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, // +2
			abilities.DEX: 11, // +0
			abilities.CON: 13, // +1
			abilities.INT: 1,  // -5
			abilities.WIS: 3,  // -4
			abilities.CHA: 1,  // -5
		},
	})

	// Slam melee attack
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.AnimatedArmorSlam(),
		Name: "slam",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Bludgeoning, FlatBonus: 2}},
		},
	})

	// Set movement speed (a suit of armor walks slower than a person)
	m.SetSpeed(monster.SpeedData{Walk: 25})

	// Add immunity to poison and psychic damage (D&D 5e SRD) — an empty suit
	// has nothing to poison and no mind to assail.
	m.AddTraitData(monstertraits.MustImmunityJSON(id, damage.Poison))
	m.AddTraitData(monstertraits.MustImmunityJSON(id, damage.Psychic))

	return m
}
