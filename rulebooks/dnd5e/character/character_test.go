package character

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	categorySkills    = "skills"
	categoryLanguages = "languages"
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
		RaceID:       "human",
		ClassID:      "fighter",
		BackgroundID: "soldier",
		AbilityScores: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 15,
			constants.CON: 14,
			constants.INT: 13,
			constants.WIS: 11,
			constants.CHA: 9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		// Skills and languages from character choices
		Skills: map[string]int{
			"acrobatics":   2, // From class choice
			"athletics":    2, // From class choice and background
			"intimidation": 2, // From background
		},
		Languages: []string{"common", "goblin", "dwarvish"},
		SavingThrows: map[string]int{
			shared.AbilityStrength:     2, // Proficient
			shared.AbilityConstitution: 2,
		},
		Proficiencies: shared.Proficiencies{
			Armor:   []string{"Light", "Medium", "Heavy", "Shield"},
			Weapons: []string{"Simple", "Martial"},
			Tools:   []string{"Gaming set", "Land vehicles"},
		},
		// Choices as shown in the issue - enhanced with ChoiceID field
		Choices: []ChoiceData{
			{
				Category:  categorySkills,            // Standard category
				Source:    "class",                   // Source is class
				ChoiceID:  "fighter_proficiencies_1", // Specific choice identifier
				Selection: []string{"skill-acrobatics", "skill-athletics"},
			},
			{
				Category:  categoryLanguages,  // Standard category
				Source:    "race",             // Source is race
				ChoiceID:  "human_language_1", // Specific choice identifier
				Selection: []string{"goblin"},
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
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAcrobatics],
		"Chosen skill acrobatics should be proficient (from class_fighter_proficiencies_1)")
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAthletics],
		"Chosen skill athletics should be proficient (from class_fighter_proficiencies_1)")

	// Verify background skills are also included
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAthletics],
		"Background skill Athletics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillIntimidation],
		"Background skill Intimidation should be proficient")

	// Verify languages are processed from choices
	s.Assert().Contains(character.languages, constants.LanguageGoblin,
		"Chosen language goblin should be included (from race_human_language_1)")

	// Verify base languages are included
	s.Assert().Contains(character.languages, constants.LanguageCommon,
		"Common should always be included")
	s.Assert().Contains(character.languages, constants.LanguageDwarvish,
		"Background language Dwarvish should be included")

	// Verify ChoiceID tracking - NEW functionality from issue 129
	s.Assert().Len(character.choices, 2, "Should have 2 choice records")

	// Find the skill choice and verify its ChoiceID
	var skillChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == categorySkills {
			skillChoice = &choice
			break
		}
	}
	s.Require().NotNil(skillChoice, "Should find skill choice")
	s.Assert().Equal("fighter_proficiencies_1", skillChoice.ChoiceID,
		"Skill choice should have specific ChoiceID for granular tracking")
	s.Assert().Equal("class", skillChoice.Source,
		"Skill choice should be from class")

	// Find the language choice and verify its ChoiceID
	var languageChoice *ChoiceData
	for _, choice := range character.choices {
		if choice.Category == categoryLanguages {
			languageChoice = &choice
			break
		}
	}
	s.Require().NotNil(languageChoice, "Should find language choice")
	s.Assert().Equal("human_language_1", languageChoice.ChoiceID,
		"Language choice should have specific ChoiceID for granular tracking")
	s.Assert().Equal("race", languageChoice.Source,
		"Language choice should be from race")
}

func (s *CharacterTestSuite) TestLoadCharacterFromData_BackwardsCompatibility() {
	// Test loading a character without choices (old format)
	charData := Data{
		ID:           "test-char-2",
		PlayerID:     "player-123",
		Name:         "Old Format Hero",
		Level:        1,
		RaceID:       "human",
		ClassID:      "fighter",
		BackgroundID: "soldier",
		AbilityScores: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 15,
			constants.CON: 14,
			constants.INT: 13,
			constants.WIS: 11,
			constants.CHA: 9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		// Pre-populated skills and languages (no choices)
		Skills: map[string]int{
			string(constants.SkillAthletics):    2,
			string(constants.SkillIntimidation): 2,
			string(constants.SkillPerception):   2,
			string(constants.SkillSurvival):     2,
		},
		Languages: []string{
			string(constants.LanguageCommon),
			string(constants.LanguageDwarvish),
			string(constants.LanguageElvish),
		},
		SavingThrows: map[string]int{
			string(constants.STR): 2,
			string(constants.CON): 2,
		},
		Proficiencies: shared.Proficiencies{
			Armor:   []string{"Light", "Medium", "Heavy", "Shield"},
			Weapons: []string{"Simple", "Martial"},
			Tools:   []string{"Gaming set", "Land vehicles"},
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
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills[constants.SkillAthletics])
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills[constants.SkillIntimidation])
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills[constants.SkillPerception])
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills[constants.SkillSurvival])

	// Verify languages are preserved
	s.Assert().Contains(character.languages, constants.LanguageCommon)
	s.Assert().Contains(character.languages, constants.LanguageDwarvish)
	s.Assert().Contains(character.languages, constants.LanguageElvish)
}

func (s *CharacterTestSuite) TestLoadCharacterFromData_MixedSelectionTypes() {
	// Test with various selection formats ([]interface{}, []string, string)
	charData := Data{
		ID:           "test-char-3",
		PlayerID:     "player-123",
		Name:         "Mixed Format Hero",
		Level:        1,
		RaceID:       "human",
		ClassID:      "fighter",
		BackgroundID: "soldier",
		AbilityScores: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 15,
			constants.CON: 14,
			constants.INT: 13,
			constants.WIS: 11,
			constants.CHA: 9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		Skills: map[string]int{
			"perception":   2,
			"survival":     2,
			"athletics":    2, // From background
			"intimidation": 2, // From background
		},
		Languages: []string{"common", "orc", "elvish", "draconic", "dwarvish"},
		SavingThrows: map[string]int{
			shared.AbilityStrength:     2,
			shared.AbilityConstitution: 2,
		},
		Proficiencies: shared.Proficiencies{
			Armor:   []string{"Light", "Medium", "Heavy", "Shield"},
			Weapons: []string{"Simple", "Martial"},
			Tools:   []string{"Gaming set", "Land vehicles"},
		},
		Choices: []ChoiceData{
			{
				Category: "skills",
				Source:   "creation",
				// []string format
				Selection: []string{"Perception", "Survival"},
			},
			{
				Category: "extra_language",
				Source:   "creation",
				// single string format
				Selection: "Orcish",
			},
			{
				Category: "bonus_languages",
				Source:   "creation",
				// Simulate []interface{} from JSON unmarshaling
				Selection: []interface{}{"Elvish", "Draconic"},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Load character from data
	character, err := LoadCharacterFromData(charData, s.testRace, s.testClass, s.testBackground)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify skills from []string selection
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillPerception])
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillSurvival])

	// Verify language from string selection
	s.Assert().Contains(character.languages, constants.LanguageOrc)

	// Verify languages from []interface{} selection
	s.Assert().Contains(character.languages, constants.LanguageElvish)
	s.Assert().Contains(character.languages, constants.LanguageDraconic)
}

func TestCharacterTestSuite(t *testing.T) {
	suite.Run(t, new(CharacterTestSuite))
}
