// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package monsters provides factory functions for creating D&D 5e monster stat blocks
package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// NewBanditMelee creates a CR 1/8 bandit carrying a scimitar and a light
// crossbow — the SRD bandit, blade first
func NewBanditMelee(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Bandit",
		Ref:  refs.Monsters.Bandit(),
		HP:   11, // 2d8+2
		AC:   12, // Leather armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 11, // +0
			abilities.DEX: 12, // +1
			abilities.CON: 12, // +1
			abilities.INT: 10, // +0
			abilities.WIS: 10, // +0
			abilities.CHA: 10, // +0
		},
	})

	// Scimitar and light crossbow, off the catalog and this bandit's own
	// numbers: DEX 12 and a +2 proficiency bonus give the SRD's "+3, 1d6+1"
	// and "+3, 1d8+1" (rpg-project#448). Melee FIRST, so a bandit you have
	// closed on draws the blade.
	mustAddWeapon(m, weapons.Scimitar, weapons.LightCrossbow)

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}

// NewBanditRanged creates a CR 1/8 bandit carrying a light crossbow alone
func NewBanditRanged(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Bandit",
		Ref:  refs.Monsters.BanditArcher(),
		HP:   11, // 2d8+2
		AC:   12, // Leather armor
		AbilityScores: shared.AbilityScores{
			abilities.STR: 11, // +0
			abilities.DEX: 12, // +1
			abilities.CON: 12, // +1
			abilities.INT: 10, // +0
			abilities.WIS: 10, // +0
			abilities.CHA: 10, // +0
		},
	})

	// The bow and nothing else. This is the SAME stat block as NewBanditMelee
	// above, with one word dropped from its list — which is exactly the thing
	// a placement's `actions` now says without a second constructor
	// (rpg-project#448). Kept as it stands because content may name
	// `dnd5e:monsters:bandit-archer` today; it is a candidate for deletion
	// once nothing does.
	mustAddWeapon(m, weapons.LightCrossbow)

	// Set movement speed
	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}
