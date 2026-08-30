// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package hostagecamp

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// Crew loads the three rescuers as real D&D 5e sheets, one to a company.
//
// They are different classes off different sheets, and deliberately identical
// at the three skills this job turns on: Stealth +4, Insight +3, Persuasion +3.
// Three companies working the same job under the same numbers is what makes the
// isolation assertions mean something — when A's hostage turns and B's does
// not, nothing about the sheets explains it.
func Crew(ctx context.Context) (map[journal.EntityID]*character.Character, error) {
	sheets := map[journal.EntityID]*character.Data{
		Wren:  sheet(Wren, "Wren", classes.Rogue, races.HalfElf),
		Marek: sheet(Marek, "Marek", classes.Paladin, races.Human),
		Sable: sheet(Sable, "Sable", classes.Barbarian, races.Human),
	}

	out := make(map[journal.EntityID]*character.Character, len(sheets))
	for id, data := range sheets {
		loaded, err := character.Load(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", id, err)
		}
		out[id] = loaded
	}

	return out, nil
}

// sheet is one level-1 rescuer: DEX 14 and WIS 12 and CHA 12, proficient in the
// three skills this job asks about.
func sheet(id journal.EntityID, name string, class classes.Class, race races.Race) *character.Data {
	return &character.Data{
		ID:               string(id),
		PlayerID:         "player-" + string(id),
		Name:             name,
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           race,
		ClassID:          class,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 12,
		},
		HitPoints:    11,
		MaxHitPoints: 11,
		ArmorClass:   14,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Stealth:    shared.Proficient,
			skills.Insight:    shared.Proficient,
			skills.Persuasion: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.DEX: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
	}
}

// Rescuers maps each company to the one of them who does the work.
func Rescuers() map[journal.EntityID]journal.EntityID {
	return map[journal.EntityID]journal.EntityID{
		PartyA: Wren,
		PartyB: Marek,
		PartyC: Sable,
	}
}
