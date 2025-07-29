package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Test constants for non-standard languages used in tests
const (
	testLanguageExotic = "exotic language"
)

type CreationTestSuite struct {
	suite.Suite
	testRace       *race.Data
	testClass      *class.Data
	testBackground *shared.Background
}

func (s *CreationTestSuite) SetupTest() {
	s.testRace = createTestRaceData()
	s.testClass = createTestClassData()
	s.testBackground = createTestBackgroundData()
}

func (s *CreationTestSuite) TestNewFromCreationData_ProcessesChoices() {
	// Create creation data with choices as mentioned in the issue
	data := CreationData{
		ID:             "test-char-1",
		PlayerID:       "player-123",
		Name:           "Test Hero",
		RaceData:       s.testRace,
		ClassData:      s.testClass,
		BackgroundData: s.testBackground,
		AbilityScores: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		Choices: map[string]any{
			"skills":    []string{"Acrobatics", "Animal Handling"},
			"languages": []string{"Goblin"},
		},
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify skills from choices are processed
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAcrobatics],
		"Chosen skill Acrobatics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAnimalHandling],
		"Chosen skill Animal Handling should be proficient")

	// Verify skills from background are included
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAthletics],
		"Background skill Athletics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillIntimidation],
		"Background skill Intimidation should be proficient")

	// Verify languages from choices are processed
	s.Assert().Contains(character.languages, constants.LanguageGoblin,
		"Chosen language Goblin should be included")

	// Verify languages from race and background are included
	s.Assert().Contains(character.languages, constants.LanguageCommon,
		"Race language Common should be included")
	s.Assert().Contains(character.languages, constants.LanguageDwarvish,
		"Background language Dwarvish should be included")

	// Verify saving throws from class
	s.Assert().Equal(shared.Proficient, character.savingThrows[constants.STR],
		"Strength saving throw should be proficient")
	s.Assert().Equal(shared.Proficient, character.savingThrows[constants.CON],
		"Constitution saving throw should be proficient")

	// Verify proficiencies from class and background
	s.Assert().Equal(s.testClass.ArmorProficiencies, character.proficiencies.Armor,
		"Armor proficiencies should match class")
	s.Assert().Equal(s.testClass.WeaponProficiencies, character.proficiencies.Weapons,
		"Weapon proficiencies should match class")
	s.Assert().Equal(s.testBackground.ToolProficiencies, character.proficiencies.Tools,
		"Tool proficiencies should match background")
}

func (s *CreationTestSuite) TestNewFromCreationData_EmptyChoices() {
	// Test with no choices to ensure defaults work
	data := CreationData{
		ID:             "test-char-2",
		PlayerID:       "player-123",
		Name:           "Test Hero 2",
		RaceData:       s.testRace,
		ClassData:      s.testClass,
		BackgroundData: s.testBackground,
		AbilityScores: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		Choices: map[string]any{}, // Empty choices
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Should still have background skills
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAthletics])
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillIntimidation])

	// Should still have race and background languages
	s.Assert().Contains(character.languages, constants.LanguageCommon)
	s.Assert().Contains(character.languages, constants.LanguageDwarvish)

	// Should have saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[constants.STR])
	s.Assert().Equal(shared.Proficient, character.savingThrows[constants.CON])
}

func (s *CreationTestSuite) TestNewFromCreationData_CommonAlwaysIncluded() {
	// Test with a race that doesn't include Common
	nonCommonRace := &race.Data{
		ID:        "exotic",
		Name:      "Exotic Race",
		Size:      "Medium",
		Speed:     30,
		Languages: []string{testLanguageExotic}, // No Common
	}

	// Test background without Common
	exoticBackground := &shared.Background{
		ID:                 "exotic-bg",
		Name:               "Exotic Background",
		SkillProficiencies: []string{"Arcana"},
		Languages:          []string{"Celestial"}, // No Common
	}

	data := CreationData{
		ID:             "test-char-no-common",
		PlayerID:       "player-123",
		Name:           "Exotic Hero",
		RaceData:       nonCommonRace,
		ClassData:      s.testClass,
		BackgroundData: exoticBackground,
		AbilityScores: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		Choices: map[string]any{
			"languages": []string{"Infernal"}, // Also no Common
		},
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify Common is still included
	s.Assert().Contains(character.languages, constants.LanguageCommon,
		"Common should always be included")
	s.Assert().Contains(character.languages, constants.Language(testLanguageExotic),
		"Race language should be included")
	s.Assert().Contains(character.languages, constants.LanguageCelestial,
		"Background language should be included")
	s.Assert().Contains(character.languages, constants.LanguageInfernal,
		"Chosen language should be included")
}

func TestCreationTestSuite(t *testing.T) {
	suite.Run(t, new(CreationTestSuite))
}
