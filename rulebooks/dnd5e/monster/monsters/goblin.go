// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewGoblin creates a standard goblin (CR 1/4, D&D 5e SRD stats).
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

	m.AddAction(mustAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "scimitar",
		AttackBonus: 4,
		Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Slashing, FlatBonus: 2}},
		Reach:       1,
	})))
	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}
