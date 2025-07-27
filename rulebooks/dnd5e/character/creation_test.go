package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type CreationTestSuite struct {
	suite.Suite
	testRace       *race.Data
	testClass      *class.Data
	testBackground *shared.Background
}

func (s *CreationTestSuite) SetupTest() {
	// Create test race data
	s.testRace = &race.Data{
		ID:    "human",
		Name:  "Human",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[string]int{
			shared.AbilityStrength:     1,
			shared.AbilityDexterity:    1,
			shared.AbilityConstitution: 1,
			shared.AbilityIntelligence: 1,
			shared.AbilityWisdom:       1,
			shared.AbilityCharisma:     1,
		},
		Languages: []string{"Common"},
	}

	// Create test class data
	s.testClass = &class.Data{
		ID:                    "fighter",
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []string{shared.AbilityStrength, shared.AbilityConstitution},
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"Acrobatics", "Animal Handling", "Athletics", "History",
			"Insight", "Intimidation", "Perception", "Survival",
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shield"},
		WeaponProficiencies: []string{"Simple", "Martial"},
		Features: map[int][]class.FeatureData{
			1: {
				{ID: "fighting-style", Name: "Fighting Style", Level: 1},
				{ID: "second-wind", Name: "Second Wind", Level: 1},
			},
		},
	}

	// Create test background data
	s.testBackground = &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []string{"Athletics", "Intimidation"},
		Languages:          []string{"Dwarvish"},
		ToolProficiencies:  []string{"Gaming set", "Land vehicles"},
	}
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
			Strength:     15,
			Dexterity:    14,
			Constitution: 13,
			Intelligence: 12,
			Wisdom:       10,
			Charisma:     8,
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
	s.Assert().Equal(shared.Proficient, character.skills["Acrobatics"], "Chosen skill Acrobatics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills["Animal Handling"],
		"Chosen skill Animal Handling should be proficient")

	// Verify skills from background are included
	s.Assert().Equal(shared.Proficient, character.skills["Athletics"], "Background skill Athletics should be proficient")
	s.Assert().Equal(shared.Proficient, character.skills["Intimidation"],
		"Background skill Intimidation should be proficient")

	// Verify languages from choices are processed
	s.Assert().Contains(character.languages, "Goblin", "Chosen language Goblin should be included")

	// Verify languages from race and background are included
	s.Assert().Contains(character.languages, "Common", "Race language Common should be included")
	s.Assert().Contains(character.languages, "Dwarvish", "Background language Dwarvish should be included")

	// Verify saving throws from class
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityStrength],
		"Strength saving throw should be proficient")
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityConstitution],
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
			Strength:     15,
			Dexterity:    14,
			Constitution: 13,
			Intelligence: 12,
			Wisdom:       10,
			Charisma:     8,
		},
		Choices: map[string]any{}, // Empty choices
	}

	// Create character
	character, err := NewFromCreationData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Should still have background skills
	s.Assert().Equal(shared.Proficient, character.skills["Athletics"])
	s.Assert().Equal(shared.Proficient, character.skills["Intimidation"])

	// Should still have race and background languages
	s.Assert().Contains(character.languages, "Common")
	s.Assert().Contains(character.languages, "Dwarvish")

	// Should have saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityStrength])
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityConstitution])
}

func (s *CreationTestSuite) TestNewFromCreationData_CommonAlwaysIncluded() {
	// Test with a race that doesn't include Common
	nonCommonRace := &race.Data{
		ID:        "exotic",
		Name:      "Exotic Race",
		Size:      "Medium",
		Speed:     30,
		Languages: []string{"Exotic Language"}, // No Common
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
			Strength:     15,
			Dexterity:    14,
			Constitution: 13,
			Intelligence: 12,
			Wisdom:       10,
			Charisma:     8,
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
	s.Assert().Contains(character.languages, "Common", "Common should always be included")
	s.Assert().Contains(character.languages, "Exotic Language", "Race language should be included")
	s.Assert().Contains(character.languages, "Celestial", "Background language should be included")
	s.Assert().Contains(character.languages, "Infernal", "Chosen language should be included")
}

func TestCreationTestSuite(t *testing.T) {
	suite.Run(t, new(CreationTestSuite))
}
