// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// NewThug creates a CR 1 thug carrying a mace and a heavy crossbow, with
// Pack Tactics.
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

	// Mace and heavy crossbow, off the catalog and this thug's own numbers:
	// STR 15 carries the mace to the SRD's "+4, 1d6+2" and DEX 11 carries the
	// crossbow to "+2, 1d10" — two different abilities, one assembly
	// (rpg-project#448). Melee FIRST, so a thug standing over you swings.
	//
	// Multiattack is still deferred until a sequence profile and machine
	// exist; these are the component attacks.
	mustAddWeapon(m, weapons.Mace, weapons.HeavyCrossbow)

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	// Note: Pack Tactics trait (advantage when ally adjacent to target)
	// is applied when the monster is loaded into combat via LoadFromData with an event bus.

	return m
}
