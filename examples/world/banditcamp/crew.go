// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package banditcamp

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

// Crew loads the three guild characters as real D&D 5e sheets.
//
// They are ordinary level-1 characters and the differences between them are
// ordinary too: Rook has expertise in Stealth, Brann has none and a middling
// Dexterity, Sela has a Charisma and the proficiency to use it. Those numbers
// are the *only* thing that separates them at this camp. None of them is
// permitted or forbidden anything.
func Crew(ctx context.Context) (map[journal.EntityID]*character.Character, error) {
	sheets := map[journal.EntityID]*character.Data{
		Rook:  rookSheet(),
		Brann: brannSheet(),
		Sela:  selaSheet(),
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

// rookSheet is a rogue: DEX 16 with expertise in Stealth (+7) and proficient
// Deception on a CHA of 14 (+4).
func rookSheet() *character.Data {
	return &character.Data{
		ID:               string(Rook),
		PlayerID:         "player-rook",
		Name:             "Rook",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.HalfElf,
		ClassID:          classes.Rogue,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16,
			abilities.CON: 12,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 14,
		},
		HitPoints:    9,
		MaxHitPoints: 9,
		ArmorClass:   14,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Stealth:       shared.Expert,
			skills.Deception:     shared.Proficient,
			skills.SleightOfHand: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.DEX: shared.Proficient,
			abilities.INT: shared.Proficient,
		},
	}
}

// brannSheet is a barbarian: DEX 12, no Stealth proficiency (+1). He may still
// sneak, and the camp will still hear him most nights.
func brannSheet() *character.Data {
	return &character.Data{
		ID:               string(Brann),
		PlayerID:         "player-brann",
		Name:             "Brann",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Barbarian,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 12,
			abilities.CON: 16,
			abilities.INT: 8,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		HitPoints:    15,
		MaxHitPoints: 15,
		ArmorClass:   15,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics:    shared.Proficient,
			skills.Intimidation: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
	}
}

// selaSheet is a paladin: CHA 16 with proficient Persuasion (+5).
func selaSheet() *character.Data {
	return &character.Data{
		ID:               string(Sela),
		PlayerID:         "player-sela",
		Name:             "Sela",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Paladin,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14,
			abilities.DEX: 10,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 16,
		},
		HitPoints:    12,
		MaxHitPoints: 12,
		ArmorClass:   16,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Persuasion: shared.Proficient,
			skills.Insight:    shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.WIS: shared.Proficient,
			abilities.CHA: shared.Proficient,
		},
	}
}
