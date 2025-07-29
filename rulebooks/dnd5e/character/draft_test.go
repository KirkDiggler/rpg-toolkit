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
		AbilityScoreIncreases: map[constants.Ability]int{
			constants.STR: 1,
			constants.DEX: 1,
			constants.CON: 1,
			constants.INT: 1,
			constants.WIS: 1,
			constants.CHA: 1,
		},
		Languages: []constants.Language{constants.LanguageCommon},
	}

	// Create test class data
	s.testClass = &class.Data{
		ID:                    "fighter",
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []constants.Ability{constants.STR, constants.CON},
		SkillProficiencyCount: 2,
		SkillOptions: []constants.Skill{
			constants.SkillAcrobatics, constants.SkillAnimalHandling, constants.SkillAthletics, constants.SkillHistory,
			constants.SkillInsight, constants.SkillIntimidation, constants.SkillPerception, constants.SkillSurvival,
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shield"},
		WeaponProficiencies: []string{"Simple", "Martial"},
	}

	// Create test background data
	s.testBackground = &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []constants.Skill{constants.SkillAthletics, constants.SkillIntimidation},
		Languages:          []constants.Language{constants.LanguageDwarvish},
		ToolProficiencies:  []string{"Gaming set", "Land vehicles"},
	}
}

func (s *DraftTestSuite) TestToCharacter_Success() {
	// Create a complete draft
	draft := &Draft{
		ID:       "test-draft-1",
		PlayerID: "player-123",
		Name:     "Test Hero",
		RaceChoice: RaceChoice{
			RaceID: constants.RaceHuman,
		},
		ClassChoice: ClassChoice{
			ClassID: constants.ClassFighter,
		},
		BackgroundChoice: constants.BackgroundSoldier,
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		SkillChoices: []constants.Skill{constants.SkillPerception, constants.SkillSurvival},
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
	s.Assert().Equal(16, character.abilityScores[constants.STR]) // 15 + 1
	s.Assert().Equal(15, character.abilityScores[constants.DEX]) // 14 + 1
	s.Assert().Equal(14, character.abilityScores[constants.CON]) // 13 + 1
	s.Assert().Equal(13, character.abilityScores[constants.INT]) // 12 + 1
	s.Assert().Equal(11, character.abilityScores[constants.WIS]) // 10 + 1
	s.Assert().Equal(9, character.abilityScores[constants.CHA])  // 8 + 1

	// Verify HP (10 base + 2 from Con modifier)
	s.Assert().Equal(12, character.maxHitPoints)
	s.Assert().Equal(12, character.hitPoints)

	// Verify physical characteristics from race
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)

	// Verify skills
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillPerception])
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillSurvival])
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillAthletics])    // From background
	s.Assert().Equal(shared.Proficient, character.skills[constants.SkillIntimidation]) // From background

	// Verify languages
	s.Assert().Contains(character.languages, constants.LanguageCommon)   // Always included
	s.Assert().Contains(character.languages, constants.LanguageDwarvish) // From background

	// Verify proficiencies
	s.Assert().Equal(s.testClass.ArmorProficiencies, character.proficiencies.Armor)
	s.Assert().Equal(s.testClass.WeaponProficiencies, character.proficiencies.Weapons)
	s.Assert().Equal(s.testBackground.ToolProficiencies, character.proficiencies.Tools)

	// Verify saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[constants.STR])
	s.Assert().Equal(shared.Proficient, character.savingThrows[constants.CON])
}

func (s *DraftTestSuite) TestToCharacter_WithSubrace() {
	// Create elf race data with subrace
	elfRace := &race.Data{
		ID:    "elf",
		Name:  "Elf",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[constants.Ability]int{
			constants.DEX: 2,
		},
		Languages: []constants.Language{constants.LanguageCommon, constants.LanguageElvish},
		Subraces: []race.SubraceData{
			{
				ID:   "high-elf",
				Name: "High Elf",
				AbilityScoreIncreases: map[constants.Ability]int{
					constants.INT: 1,
				},
			},
		},
	}

	// Create draft with subrace
	draft := &Draft{
		ID:       "test-draft-2",
		PlayerID: "player-123",
		Name:     "Elf Hero",
		RaceChoice: RaceChoice{
			RaceID:    constants.RaceElf,
			SubraceID: constants.SubraceHighElf,
		},
		ClassChoice: ClassChoice{
			ClassID: constants.ClassFighter,
		},
		BackgroundChoice: constants.BackgroundSoldier,
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 14,
			constants.DEX: 15,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
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
	s.Assert().Equal(17, character.abilityScores[constants.DEX]) // 15 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores[constants.INT]) // 12 + 1 (high elf)

	// Verify physical characteristics from elf race
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)
}

func (s *DraftTestSuite) TestToCharacter_IncompleteDraft() {
	// Create incomplete draft (missing ability scores)
	draft := &Draft{
		ID:         "test-draft-3",
		PlayerID:   "player-123",
		Name:       "Incomplete Hero",
		RaceChoice: RaceChoice{RaceID: constants.RaceHuman},
		ClassChoice: ClassChoice{
			ClassID: constants.ClassFighter,
		},
		BackgroundChoice: constants.BackgroundSoldier,
		// AbilityScoreChoice is missing
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
		RaceChoice: RaceChoice{
			RaceID: constants.RaceHuman,
		},
		ProgressFlags: ProgressName | ProgressRace,
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		UpdatedAt:     time.Now(),
	}

	draft, err := LoadDraftFromData(data)
	s.Require().NoError(err)
	s.Assert().NotNil(draft)

	s.Assert().Equal(data.ID, draft.ID)
	s.Assert().Equal(data.PlayerID, draft.PlayerID)
	s.Assert().Equal(data.Name, draft.Name)
	s.Assert().Equal(data.RaceChoice, draft.RaceChoice)
	s.Assert().Equal(data.ProgressFlags, draft.Progress.flags)
	s.Assert().True(draft.Progress.hasFlag(ProgressName))
	s.Assert().True(draft.Progress.hasFlag(ProgressRace))
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
		RaceChoice: RaceChoice{
			RaceID: constants.RaceHuman,
		},
		ClassChoice: ClassChoice{
			ClassID: constants.ClassFighter,
		},
		SkillChoices: []constants.Skill{constants.SkillAthletics, constants.SkillPerception},
		Progress:     DraftProgress{flags: ProgressName | ProgressRace | ProgressClass},
		CreatedAt:    time.Now().Add(-1 * time.Hour),
		UpdatedAt:    time.Now(),
	}

	data := draft.ToData()

	s.Assert().Equal(draft.ID, data.ID)
	s.Assert().Equal(draft.PlayerID, data.PlayerID)
	s.Assert().Equal(draft.Name, data.Name)
	s.Assert().Equal(draft.RaceChoice, data.RaceChoice)
	s.Assert().Equal(draft.ClassChoice, data.ClassChoice)
	s.Assert().Equal(draft.SkillChoices, data.SkillChoices)
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
		RaceChoice: RaceChoice{
			RaceID: constants.RaceHuman,
		},
		ClassChoice: ClassChoice{
			ClassID: constants.ClassFighter,
		},
		BackgroundChoice: constants.BackgroundSoldier,
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		LanguageChoices: []constants.Language{constants.LanguageElvish, constants.LanguageGoblin, constants.LanguageDraconic},
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
	s.Assert().Contains(character.languages, constants.LanguageCommon, "Common should always be included")
	s.Assert().Contains(character.languages, constants.LanguageDwarvish, "Background language should be included")

	// Verify chosen languages
	s.Assert().Contains(character.languages, constants.LanguageElvish, "Chosen language Elvish should be included")
	s.Assert().Contains(character.languages, constants.LanguageGoblin, "Chosen language Goblin should be included")
	s.Assert().Contains(character.languages, constants.LanguageDraconic, "Chosen language Draconic should be included")

	// Verify no duplicates (set behavior)
	languageCount := make(map[constants.Language]int)
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
		Languages: []constants.Language{constants.Language(testLanguageExotic)}, // No Common
	}

	draft := &Draft{
		ID:       "test-draft-no-common",
		PlayerID: "player-123",
		Name:     "Exotic Hero",
		RaceChoice: RaceChoice{
			RaceID: constants.Race("exotic"),
		},
		ClassChoice: ClassChoice{
			ClassID: constants.ClassFighter,
		},
		BackgroundChoice: constants.BackgroundSoldier,
		AbilityScoreChoice: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
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
	s.Assert().Contains(character.languages, constants.LanguageCommon,
		"Common should always be included even if race doesn't have it")
}

func TestDraftTestSuite(t *testing.T) {
	suite.Run(t, new(DraftTestSuite))
}
