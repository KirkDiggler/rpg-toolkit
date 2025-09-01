package character

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type CharacterTestSuite struct {
	suite.Suite
	testRace       *race.Data
	testClass      *class.Data
	testBackground *shared.Background
}

func (s *CharacterTestSuite) SetupTest() {
	s.testRace = createTestRaceData()
	s.testClass = createTestClassData()
	s.testBackground = createTestBackgroundData()
}

func (s *CharacterTestSuite) TestLoadCharacterFromData_WithChoices() {
	// Create character data with choices as shown in the issue
	charData := Data{
		ID:           "test-char-1",
		PlayerID:     "player-123",
		Name:         "Test Hero",
		Level:        1,
		RaceID:       races.Human,
		ClassID:      classes.Fighter,
		BackgroundID: backgrounds.Soldier,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 15,
			abilities.CON: 14,
			abilities.INT: 13,
			abilities.WIS: 11,
			abilities.CHA: 9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		// Skills and languages from character choices
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Acrobatics:   shared.Proficient, // From class choice
			skills.Athletics:    shared.Proficient, // From class choice and background
			skills.Intimidation: shared.Proficient, // From background
		},
		Languages: []string{"common", "goblin", "dwarvish"},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
		Proficiencies: shared.Proficiencies{
			Armor: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorHeavy,
				proficiencies.ArmorShields,
			},
			Weapons: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
			Tools: []proficiencies.Tool{
				proficiencies.ToolDiceSet,
				proficiencies.ToolVehicleLand,
			},
		},
		// Choices as shown in the issue - enhanced with ChoiceID field
		Choices: []ChoiceData{
			{
				Category:       shared.ChoiceSkills,       // Standard category
				Source:         shared.SourceClass,        // Source is class
				ChoiceID:       "fighter_proficiencies_1", // Specific choice identifier
				SkillSelection: []skills.Skill{skills.Acrobatics, skills.Athletics},
			},
			{
				Category:          shared.ChoiceLanguages, // Standard category
				Source:            shared.SourceRace,      // Source is race
				ChoiceID:          "human_language_1",     // Specific choice identifier
				LanguageSelection: []languages.Language{languages.Goblin},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Load character from data
	character, err := LoadCharacterFromData(charData, s.testRace, s.testClass, s.testBackground)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify skills are processed from choices
	s.Assert().Equal(shared.Proficient, character.skills[skills.Acrobatics],
		"Chosen skill acrobatics should be proficient (from class_fighter_proficiencies_1)")
	s.Assert().Equal(shared.Proficient, character.skills[skills.Athletics],
		"Chosen skill athletics should be proficient (from class_fighter_proficiencies_1)")

	// Verify background skills are also included
	s.Assert().Equal(shared.Proficient, character.skills[skills.Athletics],
		"Background skill Athletics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills[skills.Intimidation],
		"Background skill Intimidation should be proficient")

	// Verify languages are processed from choices
	s.Assert().Contains(character.languages, languages.Goblin,
		"Chosen language goblin should be included (from race_human_language_1)")

	// Verify base languages are included
	s.Assert().Contains(character.languages, languages.Common,
		"Common should always be included")
	s.Assert().Contains(character.languages, languages.Dwarvish,
		"Background language Dwarvish should be included")

	// Verify ChoiceID tracking - NEW functionality from issue 129
	s.Assert().Len(character.choices, 2, "Should have 2 choice records")

	// Find the skill choice and verify its ChoiceID
	var skillChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == shared.ChoiceSkills {
			skillChoice = &choice
			break
		}
	}
	s.Require().NotNil(skillChoice, "Should find skill choice")
	s.Assert().Equal("fighter_proficiencies_1", skillChoice.ChoiceID,
		"Skill choice should have specific ChoiceID for granular tracking")
	s.Assert().Equal(shared.SourceClass, skillChoice.Source,
		"Skill choice should be from class")

	// Find the language choice and verify its ChoiceID
	var languageChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == shared.ChoiceLanguages {
			languageChoice = &choice
			break
		}
	}
	s.Require().NotNil(languageChoice, "Should find language choice")
	s.Assert().Equal("human_language_1", languageChoice.ChoiceID,
		"Language choice should have specific ChoiceID for granular tracking")
	s.Assert().Equal(shared.SourceRace, languageChoice.Source,
		"Language choice should be from race")
}

func (s *CharacterTestSuite) TestLoadCharacterFromData_BackwardsCompatibility() {
	// Test loading a character without choices (old format)
	charData := Data{
		ID:           "test-char-2",
		PlayerID:     "player-123",
		Name:         "Old Format Hero",
		Level:        1,
		RaceID:       races.Human,
		ClassID:      classes.Fighter,
		BackgroundID: backgrounds.Soldier,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 15,
			abilities.CON: 14,
			abilities.INT: 13,
			abilities.WIS: 11,
			abilities.CHA: 9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		// Pre-populated skills and languages (no choices)
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics:    shared.Proficient,
			skills.Intimidation: shared.Proficient,
			skills.Perception:   shared.Proficient,
			skills.Survival:     shared.Proficient,
		},
		Languages: []string{
			string(languages.Common),
			string(languages.Dwarvish),
			string(languages.Elvish),
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
		Proficiencies: shared.Proficiencies{
			Armor: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorHeavy,
				proficiencies.ArmorShields,
			},
			Weapons: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
			Tools: []proficiencies.Tool{
				proficiencies.ToolDiceSet,
				proficiencies.ToolVehicleLand,
			},
		},
		// No choices
		Choices:   []ChoiceData{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Load character from data
	character, err := LoadCharacterFromData(charData, s.testRace, s.testClass, s.testBackground)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify skills are preserved
	s.Assert().Equal(shared.Proficient, character.skills[skills.Athletics])
	s.Assert().Equal(shared.Proficient, character.skills[skills.Intimidation])
	s.Assert().Equal(shared.Proficient, character.skills[skills.Perception])
	s.Assert().Equal(shared.Proficient, character.skills[skills.Survival])

	// Verify languages are preserved
	s.Assert().Contains(character.languages, languages.Common)
	s.Assert().Contains(character.languages, languages.Dwarvish)
	s.Assert().Contains(character.languages, languages.Elvish)
}

func TestCharacterTestSuite(t *testing.T) {
	suite.Run(t, new(CharacterTestSuite))
}
