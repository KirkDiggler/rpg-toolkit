package character

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
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
			Strength:     16,
			Dexterity:    15,
			Constitution: 14,
			Intelligence: 13,
			Wisdom:       11,
			Charisma:     9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		// Empty skills and languages - should be rebuilt from choices
		Skills:    map[string]int{},
		Languages: []string{},
		SavingThrows: map[string]int{
			shared.AbilityStrength:     2, // Proficient
			shared.AbilityConstitution: 2,
		},
		Proficiencies: shared.Proficiencies{
			Armor:   []string{"Light", "Medium", "Heavy", "Shield"},
			Weapons: []string{"Simple", "Martial"},
			Tools:   []string{"Gaming set", "Land vehicles"},
		},
		// Choices as shown in the issue
		Choices: []ChoiceData{
			{
				Category:  "class_fighter_proficiencies_1",
				Source:    "creation",
				Selection: []string{"skill-acrobatics", "skill-athletics"},
			},
			{
				Category:  "race_human_language_1",
				Source:    "creation",
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
	s.Assert().Equal(shared.Proficient, character.skills["acrobatics"],
		"Chosen skill acrobatics should be proficient (from class_fighter_proficiencies_1)")
	s.Assert().Equal(shared.Proficient, character.skills["athletics"],
		"Chosen skill athletics should be proficient (from class_fighter_proficiencies_1)")

	// Verify background skills are also included
	s.Assert().Equal(shared.Proficient, character.skills["Athletics"],
		"Background skill Athletics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills["Intimidation"],
		"Background skill Intimidation should be proficient")

	// Verify languages are processed from choices
	s.Assert().Contains(character.languages, "goblin",
		"Chosen language goblin should be included (from race_human_language_1)")

	// Verify base languages are included
	s.Assert().Contains(character.languages, "Common",
		"Common should always be included")
	s.Assert().Contains(character.languages, "Dwarvish",
		"Background language Dwarvish should be included")
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
			Strength:     16,
			Dexterity:    15,
			Constitution: 14,
			Intelligence: 13,
			Wisdom:       11,
			Charisma:     9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		// Pre-populated skills and languages (no choices)
		Skills: map[string]int{
			"Athletics":    2,
			"Intimidation": 2,
			"Perception":   2,
			"Survival":     2,
		},
		Languages: []string{"Common", "Dwarvish", "Elvish"},
		SavingThrows: map[string]int{
			shared.AbilityStrength:     2,
			shared.AbilityConstitution: 2,
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
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills["Athletics"])
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills["Intimidation"])
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills["Perception"])
	s.Assert().Equal(shared.ProficiencyLevel(2), character.skills["Survival"])

	// Verify languages are preserved
	s.Assert().Contains(character.languages, "Common")
	s.Assert().Contains(character.languages, "Dwarvish")
	s.Assert().Contains(character.languages, "Elvish")
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
			Strength:     16,
			Dexterity:    15,
			Constitution: 14,
			Intelligence: 13,
			Wisdom:       11,
			Charisma:     9,
		},
		Speed:        30,
		Size:         "Medium",
		HitPoints:    12,
		MaxHitPoints: 12,
		Skills:       map[string]int{},
		Languages:    []string{},
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
	s.Assert().Equal(shared.Proficient, character.skills["Perception"])
	s.Assert().Equal(shared.Proficient, character.skills["Survival"])

	// Verify language from string selection
	s.Assert().Contains(character.languages, "Orcish")

	// Verify languages from []interface{} selection
	s.Assert().Contains(character.languages, "Elvish")
	s.Assert().Contains(character.languages, "Draconic")
}

func TestCharacterTestSuite(t *testing.T) {
	suite.Run(t, new(CharacterTestSuite))
}
