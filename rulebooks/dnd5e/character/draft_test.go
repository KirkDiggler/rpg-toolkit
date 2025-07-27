package character

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type DraftTestSuite struct {
	suite.Suite
	testRace       *race.Data
	testClass      *class.Data
	testBackground *shared.Background
}

func (s *DraftTestSuite) SetupTest() {
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

func (s *DraftTestSuite) TestToCharacter_Success() {
	// Create a complete draft
	draft := &Draft{
		ID:       "test-draft-1",
		PlayerID: "player-123",
		Name:     "Test Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName: "Test Hero",
			shared.ChoiceRace: RaceChoice{
				RaceID: "human",
			},
			shared.ChoiceClass:      "fighter",
			shared.ChoiceBackground: "soldier",
			shared.ChoiceAbilityScores: shared.AbilityScores{
				Strength:     15,
				Dexterity:    14,
				Constitution: 13,
				Intelligence: 12,
				Wisdom:       10,
				Charisma:     8,
			},
			shared.ChoiceSkills: []string{"Perception", "Survival"},
		},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Convert to character
	character, err := draft.ToCharacter(s.testRace, s.testClass, s.testBackground)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify basic info
	s.Assert().Equal("test-draft-1", character.id)
	s.Assert().Equal("player-123", character.playerID)
	s.Assert().Equal("Test Hero", character.name)
	s.Assert().Equal(1, character.level)

	// Verify ability scores (with racial bonuses)
	s.Assert().Equal(16, character.abilityScores.Strength)     // 15 + 1
	s.Assert().Equal(15, character.abilityScores.Dexterity)    // 14 + 1
	s.Assert().Equal(14, character.abilityScores.Constitution) // 13 + 1
	s.Assert().Equal(13, character.abilityScores.Intelligence) // 12 + 1
	s.Assert().Equal(11, character.abilityScores.Wisdom)       // 10 + 1
	s.Assert().Equal(9, character.abilityScores.Charisma)      // 8 + 1

	// Verify HP (10 base + 2 from Con modifier)
	s.Assert().Equal(12, character.maxHitPoints)
	s.Assert().Equal(12, character.hitPoints)

	// Verify physical characteristics from race
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)

	// Verify skills
	s.Assert().Equal(shared.Proficient, character.skills["Perception"])
	s.Assert().Equal(shared.Proficient, character.skills["Survival"])
	s.Assert().Equal(shared.Proficient, character.skills["Athletics"])    // From background
	s.Assert().Equal(shared.Proficient, character.skills["Intimidation"]) // From background

	// Verify languages
	s.Assert().Contains(character.languages, "Common")   // Always included
	s.Assert().Contains(character.languages, "Dwarvish") // From background

	// Verify proficiencies
	s.Assert().Equal(s.testClass.ArmorProficiencies, character.proficiencies.Armor)
	s.Assert().Equal(s.testClass.WeaponProficiencies, character.proficiencies.Weapons)
	s.Assert().Equal(s.testBackground.ToolProficiencies, character.proficiencies.Tools)

	// Verify saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityStrength])
	s.Assert().Equal(shared.Proficient, character.savingThrows[shared.AbilityConstitution])
}

func (s *DraftTestSuite) TestToCharacter_WithSubrace() {
	// Create elf race data with subrace
	elfRace := &race.Data{
		ID:    "elf",
		Name:  "Elf",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[string]int{
			shared.AbilityDexterity: 2,
		},
		Languages: []string{"Common", "Elvish"},
		Subraces: []race.SubraceData{
			{
				ID:   "high-elf",
				Name: "High Elf",
				AbilityScoreIncreases: map[string]int{
					shared.AbilityIntelligence: 1,
				},
			},
		},
	}

	// Create draft with subrace
	draft := &Draft{
		ID:       "test-draft-2",
		PlayerID: "player-123",
		Name:     "Elf Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName: "Elf Hero",
			shared.ChoiceRace: RaceChoice{
				RaceID:    "elf",
				SubraceID: "high-elf",
			},
			shared.ChoiceClass:      "fighter",
			shared.ChoiceBackground: "soldier",
			shared.ChoiceAbilityScores: shared.AbilityScores{
				Strength:     14,
				Dexterity:    15,
				Constitution: 13,
				Intelligence: 12,
				Wisdom:       10,
				Charisma:     8,
			},
		},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(elfRace, s.testClass, s.testBackground)
	s.Require().NoError(err)

	// Verify character was created
	s.Assert().NotNil(character)
	s.Assert().Equal("test-draft-2", character.id)

	// Verify ability scores include racial bonuses
	s.Assert().Equal(17, character.abilityScores.Dexterity)    // 15 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores.Intelligence) // 12 + 1 (high elf)

	// Verify physical characteristics from elf race
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)
}

func (s *DraftTestSuite) TestToCharacter_IncompleteDraft() {
	// Create incomplete draft (missing ability scores)
	draft := &Draft{
		ID:       "test-draft-3",
		PlayerID: "player-123",
		Name:     "Incomplete Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName:       "Incomplete Hero",
			shared.ChoiceRace:       RaceChoice{RaceID: "human"},
			shared.ChoiceClass:      "fighter",
			shared.ChoiceBackground: "soldier",
		},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground,
		},
	}

	// Attempt to convert
	character, err := draft.ToCharacter(s.testRace, s.testClass, s.testBackground)
	s.Assert().Error(err)
	s.Assert().Nil(character)
	s.Assert().Contains(err.Error(), "incomplete")
}

func (s *DraftTestSuite) TestToCharacter_MissingData() {
	draft := &Draft{
		ID: "test-draft-4",
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Test missing race data
	character, err := draft.ToCharacter(nil, s.testClass, s.testBackground)
	s.Assert().Error(err)
	s.Assert().Nil(character)
	s.Assert().Contains(err.Error(), "required")

	// Test missing class data
	character, err = draft.ToCharacter(s.testRace, nil, s.testBackground)
	s.Assert().Error(err)
	s.Assert().Nil(character)

	// Test missing background data
	character, err = draft.ToCharacter(s.testRace, s.testClass, nil)
	s.Assert().Error(err)
	s.Assert().Nil(character)
}

func (s *DraftTestSuite) TestLoadDraftFromData() {
	data := DraftData{
		ID:       "test-draft-5",
		PlayerID: "player-123",
		Name:     "Loaded Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName: "Loaded Hero",
		},
		ProgressFlags: ProgressName,
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		UpdatedAt:     time.Now(),
	}

	draft, err := LoadDraftFromData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(draft)

	s.Assert().Equal(data.ID, draft.ID)
	s.Assert().Equal(data.PlayerID, draft.PlayerID)
	s.Assert().Equal(data.Name, draft.Name)
	s.Assert().Equal(data.ProgressFlags, draft.Progress.flags)
	s.Assert().True(draft.Progress.hasFlag(ProgressName))
}

func (s *DraftTestSuite) TestLoadDraftFromData_NoID() {
	data := DraftData{
		Name: "No ID Hero",
	}

	draft, err := LoadDraftFromData(data)
	s.Assert().Error(err)
	s.Assert().Nil(draft)
	s.Assert().Contains(err.Error(), "ID is required")
}

func (s *DraftTestSuite) TestDraftToData() {
	draft := &Draft{
		ID:       "test-draft-6",
		PlayerID: "player-123",
		Name:     "Test Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName: "Test Hero",
		},
		Progress:  DraftProgress{flags: ProgressName},
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now(),
	}

	data := draft.ToData()

	s.Assert().Equal(draft.ID, data.ID)
	s.Assert().Equal(draft.PlayerID, data.PlayerID)
	s.Assert().Equal(draft.Name, data.Name)
	s.Assert().Equal(draft.Choices, data.Choices)
	s.Assert().Equal(draft.Progress.flags, data.ProgressFlags)
	s.Assert().Equal(draft.CreatedAt, data.CreatedAt)
	s.Assert().Equal(draft.UpdatedAt, data.UpdatedAt)
}

func (s *DraftTestSuite) TestIsComplete() {
	testCases := []struct {
		name     string
		flags    uint32
		expected bool
	}{
		{
			name:     "all required flags",
			flags:    ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
			expected: true,
		},
		{
			name:     "missing name",
			flags:    ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
			expected: false,
		},
		{
			name:     "missing ability scores",
			flags:    ProgressName | ProgressRace | ProgressClass | ProgressBackground,
			expected: false,
		},
		{
			name: "with extra optional flags",
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground |
				ProgressAbilityScores | ProgressSkills | ProgressLanguages,
			expected: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			draft := &Draft{
				Progress: DraftProgress{flags: tc.flags},
			}
			s.Assert().Equal(tc.expected, draft.IsComplete())
		})
	}
}

func (s *DraftTestSuite) TestToCharacter_WithLanguageChoices() {
	// Create a complete draft with language choices
	draft := &Draft{
		ID:       "test-draft-lang",
		PlayerID: "player-123",
		Name:     "Multilingual Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName: "Multilingual Hero",
			shared.ChoiceRace: RaceChoice{
				RaceID: "human",
			},
			shared.ChoiceClass:      "fighter",
			shared.ChoiceBackground: "soldier",
			shared.ChoiceAbilityScores: shared.AbilityScores{
				Strength:     15,
				Dexterity:    14,
				Constitution: 13,
				Intelligence: 12,
				Wisdom:       10,
				Charisma:     8,
			},
			shared.ChoiceLanguages: []string{"Elvish", "Goblin", "Draconic"},
		},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Convert to character
	character, err := draft.ToCharacter(s.testRace, s.testClass, s.testBackground)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify languages include all sources
	s.Assert().Contains(character.languages, "Common", "Common should always be included")
	s.Assert().Contains(character.languages, "Dwarvish", "Background language should be included")

	// Verify chosen languages
	s.Assert().Contains(character.languages, "Elvish", "Chosen language Elvish should be included")
	s.Assert().Contains(character.languages, "Goblin", "Chosen language Goblin should be included")
	s.Assert().Contains(character.languages, "Draconic", "Chosen language Draconic should be included")

	// Verify no duplicates (set behavior)
	languageCount := make(map[string]int)
	for _, lang := range character.languages {
		languageCount[lang]++
	}
	for lang, count := range languageCount {
		s.Assert().Equal(1, count, "Language %s should appear only once", lang)
	}
}

func (s *DraftTestSuite) TestToCharacter_CommonAlwaysIncluded() {
	// Test with a race that doesn't include Common
	nonCommonRace := &race.Data{
		ID:        "exotic",
		Name:      "Exotic Race",
		Size:      "Medium",
		Speed:     30,
		Languages: []string{"Exotic Language"}, // No Common
	}

	draft := &Draft{
		ID:       "test-draft-no-common",
		PlayerID: "player-123",
		Name:     "Exotic Hero",
		Choices: map[shared.ChoiceCategory]any{
			shared.ChoiceName: "Exotic Hero",
			shared.ChoiceRace: RaceChoice{
				RaceID: "exotic",
			},
			shared.ChoiceClass:      "fighter",
			shared.ChoiceBackground: "soldier",
			shared.ChoiceAbilityScores: shared.AbilityScores{
				Strength:     15,
				Dexterity:    14,
				Constitution: 13,
				Intelligence: 12,
				Wisdom:       10,
				Charisma:     8,
			},
		},
		Progress: DraftProgress{
			flags: ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores,
		},
	}

	// Convert to character
	character, err := draft.ToCharacter(nonCommonRace, s.testClass, s.testBackground)
	s.Require().NoError(err)
	s.Assert().NotNil(character)

	// Verify Common is still included
	s.Assert().Contains(character.languages, "Common", "Common should always be included even if race doesn't have it")
}

func TestDraftTestSuite(t *testing.T) {
	suite.Run(t, new(DraftTestSuite))
}
