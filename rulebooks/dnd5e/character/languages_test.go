package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// LanguagesSuite tests language population during character creation and serialization
type LanguagesSuite struct {
	suite.Suite
	eventBus events.EventBus
}

func (s *LanguagesSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

// TestHumanWithLanguageChoice verifies Human gets Common + chosen language
func (s *LanguagesSuite) TestHumanWithLanguageChoice() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-lang-1",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)

	// Set up a complete Human character with Elvish as chosen language
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Human"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 12, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	// Finalize and check languages
	char, err := draft.ToCharacter(context.Background(), "char-lang-1", s.eventBus)
	s.Require().NoError(err)

	data := char.ToData()
	s.Require().Len(data.Languages, 2, "Human should have 2 languages (Common + chosen)")
	s.Contains(data.Languages, languages.Common, "Human should have Common")
	s.Contains(data.Languages, languages.Elvish, "Human should have chosen Elvish")
}

// TestElfDefaultLanguages verifies Elf gets Common + Elvish automatically
func (s *LanguagesSuite) TestElfDefaultLanguages() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-lang-2",
		PlayerID: "player-2",
	})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Elf"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Elf,
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(context.Background(), "char-lang-2", s.eventBus)
	s.Require().NoError(err)

	data := char.ToData()
	s.Require().Len(data.Languages, 2, "Elf should have 2 languages")
	s.Contains(data.Languages, languages.Common, "Elf should have Common")
	s.Contains(data.Languages, languages.Elvish, "Elf should have Elvish")
}

// TestDwarfDefaultLanguages verifies Dwarf gets Common + Dwarvish
func (s *LanguagesSuite) TestDwarfDefaultLanguages() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-lang-3",
		PlayerID: "player-3",
	})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Dwarf"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:    races.Dwarf,
		SubraceID: races.HillDwarf,
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 16,
			abilities.INT: 8, abilities.WIS: 14, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(context.Background(), "char-lang-3", s.eventBus)
	s.Require().NoError(err)

	data := char.ToData()
	s.Require().Len(data.Languages, 2, "Dwarf should have 2 languages")
	s.Contains(data.Languages, languages.Common, "Dwarf should have Common")
	s.Contains(data.Languages, languages.Dwarvish, "Dwarf should have Dwarvish")
}

// TestLanguageRoundTrip verifies languages serialize and deserialize correctly
func (s *LanguagesSuite) TestLanguageRoundTrip() {
	// Create and finalize a character
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-lang-rt",
		PlayerID: "player-rt",
	})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Round Trip Test"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Draconic},
		},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 12, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(context.Background(), "char-rt", s.eventBus)
	s.Require().NoError(err)

	// Serialize to Data
	data := char.ToData()
	s.Require().Len(data.Languages, 2)
	s.Contains(data.Languages, languages.Common)
	s.Contains(data.Languages, languages.Draconic)

	// Clean up original character
	s.Require().NoError(char.Cleanup(context.Background()))

	// Deserialize from Data
	newBus := events.NewEventBus()
	restored, err := LoadFromData(context.Background(), data, newBus)
	s.Require().NoError(err)
	defer func() { _ = restored.Cleanup(context.Background()) }()

	// Verify languages survived the round trip
	restoredData := restored.ToData()
	s.Require().Len(restoredData.Languages, 2, "Languages should survive round trip")
	s.Contains(restoredData.Languages, languages.Common, "Common should survive round trip")
	s.Contains(restoredData.Languages, languages.Draconic, "Draconic should survive round trip")
}

// TestHalfElfLanguages verifies Half-Elf gets Common + Elvish + chosen language
func (s *LanguagesSuite) TestHalfElfLanguages() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-lang-he",
		PlayerID: "player-he",
	})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Half-Elf"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID: races.HalfElf,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Dwarvish},
			Skills:    []skills.Skill{skills.Perception, skills.Stealth},
		},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Soldier}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 16,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(context.Background(), "char-he", s.eventBus)
	s.Require().NoError(err)

	data := char.ToData()
	s.Require().Len(data.Languages, 3, "Half-Elf should have 3 languages")
	s.Contains(data.Languages, languages.Common, "Half-Elf should have Common")
	s.Contains(data.Languages, languages.Elvish, "Half-Elf should have Elvish")
	s.Contains(data.Languages, languages.Dwarvish, "Half-Elf should have chosen Dwarvish")
}

func TestLanguagesSuite(t *testing.T) {
	suite.Run(t, new(LanguagesSuite))
}
