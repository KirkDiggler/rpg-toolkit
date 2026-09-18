// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// NewGoblin creates a standard goblin (CR 1/4, D&D 5e SRD stats) carrying a
// scimitar and a shortbow.
func NewGoblin(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Goblin",
		Ref:  refs.Monsters.Goblin(),
		HP:   7,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 8,
			abilities.CHA: 8,
		},
	})

	// Scimitar and shortbow, off the catalog and this goblin's own numbers:
	// DEX 14 beats STR 8 on a finesse blade, and with a +2 proficiency bonus
	// that is the SRD's "+4, 1d6+2" for both (rpg-project#448). Melee FIRST,
	// so a goblin with its back to the wall swings.
	//
	// THE SCIMITAR'S ONE-FOOT REACH IS GONE. It was a hand-typed
	// `ReachFeet: 1` that nothing could correct without correcting it by
	// hand; the catalog says five feet, and the number is no longer authored
	// here to be wrong.
	mustAddWeapon(m, weapons.Scimitar, weapons.Shortbow)

	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}
