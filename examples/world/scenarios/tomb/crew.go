// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package tomb

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// Crew loads Finch as a real D&D 5e sheet.
//
// Finch is the only one of this cast who ever makes a contested check: both
// [Search] and [Open] ask about her. Bram and Thane need no sheet — Bram
// never attempts anything, by design, and Defeat and Loot are declared
// outcomes that never touch a resolver.
func Crew(ctx context.Context) (map[journal.EntityID]*character.Character, error) {
	data := &character.Data{
		ID:               string(Finch),
		PlayerID:         "player-finch",
		Name:             "Finch",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.HalfElf,
		ClassID:          classes.Rogue,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 12,
			abilities.INT: 14,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		HitPoints:    9,
		MaxHitPoints: 9,
		ArmorClass:   13,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Investigation: shared.Proficient,
			skills.SleightOfHand: shared.Proficient,
		},
	}

	loaded, err := character.Load(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("loading %q: %w", Finch, err)
	}

	return map[journal.EntityID]*character.Character{Finch: loaded}, nil
}
