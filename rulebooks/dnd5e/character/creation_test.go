package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Choices: map[string]any{
			"skills":    []string{"acrobatics", "animal-handling"},
			"languages": []string{"goblin"},
		},
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify skills from choices are processed
	s.Assert().Equal(shared.Proficient, character.skills[skills.Acrobatics],
		"Chosen skill Acrobatics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills[skills.AnimalHandling],
		"Chosen skill Animal Handling should be proficient")

	// Verify skills from background are included
	s.Assert().Equal(shared.Proficient, character.skills[skills.Athletics],
		"Background skill Athletics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills[skills.Intimidation],
		"Background skill Intimidation should be proficient")

	// Verify languages from choices are processed
	s.Assert().Contains(character.languages, languages.Goblin,
		"Chosen language Goblin should be included")

	// Verify languages from race and background are included
	s.Assert().Contains(character.languages, languages.Common,
		"Race language Common should be included")
	s.Assert().Contains(character.languages, languages.Dwarvish,
		"Background language Dwarvish should be included")

	// Verify saving throws from class
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.STR],
		"Strength saving throw should be proficient")
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.CON],
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
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Choices: map[string]any{}, // Empty choices
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Should still have background skills
	s.Assert().Equal(shared.Proficient, character.skills[skills.Athletics])
	s.Assert().Equal(shared.Proficient, character.skills[skills.Intimidation])

	// Should still have race and background languages
	s.Assert().Contains(character.languages, languages.Common)
	s.Assert().Contains(character.languages, languages.Dwarvish)

	// Should have saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.STR])
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.CON])
}

func (s *CreationTestSuite) TestNewFromCreationData_CommonAlwaysIncluded() {
	// Test with a race that doesn't include Common
	nonCommonRace := &race.Data{
		ID:        "exotic",
		Name:      "Exotic Race",
		Size:      "Medium",
		Speed:     30,
		Languages: []languages.Language{languages.Language(testLanguageExotic)}, // No Common
	}

	// Test background without Common
	exoticBackground := &shared.Background{
		ID:                 "exotic-bg",
		Name:               "Exotic Background",
		SkillProficiencies: []skills.Skill{skills.Arcana},
		Languages:          []languages.Language{languages.Celestial}, // No Common
	}

	data := CreationData{
		ID:             "test-char-no-common",
		PlayerID:       "player-123",
		Name:           "Exotic Hero",
		RaceData:       nonCommonRace,
		ClassData:      s.testClass,
		BackgroundData: exoticBackground,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Choices: map[string]any{
			"languages": []string{"infernal"}, // Also no Common
		},
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify Common is still included
	s.Assert().Contains(character.languages, languages.Common,
		"Common should always be included")
	s.Assert().Contains(character.languages, languages.Language(testLanguageExotic),
		"Race language should be included")
	s.Assert().Contains(character.languages, languages.Celestial,
		"Background language should be included")
	s.Assert().Contains(character.languages, languages.Infernal,
		"Chosen language should be included")
}

func TestCreationTestSuite(t *testing.T) {
	suite.Run(t, new(CreationTestSuite))
}
