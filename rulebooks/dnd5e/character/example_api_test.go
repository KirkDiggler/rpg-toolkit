package character_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// Example_apiWithTypedConstants demonstrates using the new Draft API with typed constants
func Example_apiWithTypedConstants() {
	// Create a new draft
	draft := &character.Draft{
		ID:       "wizard-001",
		PlayerID: "player-123",
		Name:     "Elara Moonwhisper",
	}

	// Set race with typed constants
	_, _ = draft.SetRace(&character.SetRaceInput{
		RaceID:    races.Elf,
		SubraceID: races.HighElf,
		// High Elf gets a bonus cantrip, but that would be handled separately
	})

	// Set class with typed constants for fighting style (if applicable)
	// For a wizard, we'd select cantrips and spells
	_, _ = draft.SetClass(&character.SetClassInput{
		ClassID: classes.Wizard,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{
				skills.Arcana,
				skills.Investigation,
			},
			// Using typed spell constants!
			Cantrips: []spells.Spell{
				spells.FireBolt,
				spells.MageHand,
				spells.Prestidigitation,
			},
			// Level 1 spells
			Spells: []spells.Spell{
				spells.MagicMissile,
				spells.Shield,
				spells.DetectMagic,
				spells.CharmPerson,
				spells.Sleep,
				spells.Identify,
			},
		},
	})

	// For a Fighter, we'd use fighting style constants
	fighterDraft := &character.Draft{
		ID:       "fighter-001",
		PlayerID: "player-456",
		Name:     "Thorin Ironforge",
	}

	defenseStyle := choices.FightingStyleDefense
	_, _ = fighterDraft.SetClass(&character.SetClassInput{
		ClassID: classes.Fighter,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Survival,
			},
			// Using typed fighting style constant!
			FightingStyle: &defenseStyle,
			// Equipment would use the Equipment interface
			// Equipment: []character.EquipmentSelection{
			//     {
			//         RequirementIndex: 0,
			//         SelectedOption: someWeapon, // implements shared.Equipment
			//     },
			// },
		},
	})

	// For a Dragonborn, we'd use draconic ancestry constants
	dragonbornDraft := &character.Draft{
		ID:       "dragonborn-001",
		PlayerID: "player-789",
		Name:     "Balasar",
	}

	redAncestry := character.AncestryRed
	_, _ = dragonbornDraft.SetRace(&character.SetRaceInput{
		RaceID: races.Dragonborn,
		Choices: character.RaceChoices{
			// Using typed draconic ancestry constant!
			DraconicAncestry: &redAncestry, // Red dragon = fire breath
		},
	})

	// Set background and ability scores as before
	_, _ = draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Sage,
	})

	_, _ = draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: character.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 15,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Method: "standard",
	})

	fmt.Println("Draft created with typed constants!")
	// Output: Draft created with typed constants!
}
