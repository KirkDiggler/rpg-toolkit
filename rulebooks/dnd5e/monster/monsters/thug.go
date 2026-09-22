// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// NewThug creates a CR 1 thug carrying a mace and a heavy crossbow, with
// Pack Tactics and the SRD's "makes two melee attacks" Multiattack.
//
// TWO MELEE ATTACKS IS TWO MACE ATTACKS. The SRD line does not name a weapon,
// and the thug carries exactly one melee weapon, so the script names the mace
// twice. The heavy crossbow stays a component of its own: a thug across the
// room still shoots.
func NewThug(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Thug",
		Ref:  refs.Monsters.Thug(),
		HP:   32, // 5d8+10
		AC:   11, // Leather armor
		// 200 is CR 1, the rating THIS FILE claims, not the SRD's. The SRD
		// thug is CR 1/2 and worth 100; these stats are a deliberate variant
		// and the worth follows the variant's rating, not the book's.
		Experience: 200, // CR 1
		AbilityScores: shared.AbilityScores{
			abilities.STR: 15, // +2
			abilities.DEX: 11, // +0
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 10, // +0
			abilities.CHA: 11, // +0
		},
	})

	// Mace and heavy crossbow, off the catalog and this thug's own numbers:
	// STR 15 carries the mace to the SRD's "+4, 1d6+2" and DEX 11 carries the
	// crossbow to "+2, 1d10" — two different abilities, one assembly
	// (rpg-project#448). Melee FIRST, so a thug standing over you swings.
	//
	// Multiattack first, so a driver reading this list in order reaches for
	// the thug's own line. The steps name the mace armed below: a sequence
	// declares what to do with a repertoire, never a new attack of its own.
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.ThugMultiattack(),
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: *refs.Weapons.Mace()},
				{Action: *refs.Weapons.Mace()},
			},
		},
	})

	mustAddWeapon(m, weapons.Mace, weapons.HeavyCrossbow)

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
