// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Human is the rulebook's human base block (rpg-project#555 R5): the stat
// block every authored castle NPC derives from until a second base exists.
//
// It is a [monster.Template], not a constructor, so a base is assembled by the
// same [monster.FromTemplate] as every block derived from it. An ordinary
// person: every score 10, one d8 of hit dice, nothing worn, fists, the
// floor proficiency bonus, and worth nothing on the fall.
//
// Treat it as read-only content. [monster.Template.Merge] copies what it
// takes, so deriving from it never mutates it.
var Human = monster.Template{
	Base: refs.Monsters.Human(),
	Name: "Human",
	Abilities: map[abilities.Ability]int{
		abilities.STR: 10,
		abilities.DEX: 10,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	},
	HitDice:     "1d8",
	Proficiency: 2,
	Actions:     []weapons.WeaponID{weapons.UnarmedStrike},
	Speed:       monster.SpeedData{Walk: 30},
	Experience:  0,
}
