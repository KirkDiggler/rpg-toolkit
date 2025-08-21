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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
		AbilityScoreIncreases: map[abilities.Ability]int{
			abilities.STR: 1,
			abilities.DEX: 1,
			abilities.CON: 1,
			abilities.INT: 1,
			abilities.WIS: 1,
			abilities.CHA: 1,
		},
		Languages: []languages.Language{languages.Common},
	}

	// Create test class data
	s.testClass = &class.Data{
		ID:                    classes.Fighter,
		Name:                  "Fighter",
		HitDice:               10,
		SavingThrows:          []abilities.Ability{abilities.STR, abilities.CON},
		SkillProficiencyCount: 2,
		SkillOptions: []skills.Skill{
			skills.Acrobatics, skills.AnimalHandling, skills.Athletics, skills.History,
			skills.Insight, skills.Intimidation, skills.Perception, skills.Survival,
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shield"},
		WeaponProficiencies: []string{"Simple", "Martial"},
	}

	// Create test background data
	s.testBackground = &shared.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []skills.Skill{skills.Athletics, skills.Intimidation},
		Languages:          []languages.Language{languages.Dwarvish},
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
			RaceID: races.Human,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Choices: []ChoiceData{
			{
				Category:       shared.ChoiceSkills,
				Source:         shared.SourceClass,
				ChoiceID:       "fighter_skill_proficiencies",
				SkillSelection: []skills.Skill{skills.Perception, skills.Survival},
			},
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
	s.Assert().Equal(16, character.abilityScores[abilities.STR]) // 15 + 1
	s.Assert().Equal(15, character.abilityScores[abilities.DEX]) // 14 + 1
	s.Assert().Equal(14, character.abilityScores[abilities.CON]) // 13 + 1
	s.Assert().Equal(13, character.abilityScores[abilities.INT]) // 12 + 1
	s.Assert().Equal(11, character.abilityScores[abilities.WIS]) // 10 + 1
	s.Assert().Equal(9, character.abilityScores[abilities.CHA])  // 8 + 1

	// Verify HP (10 base + 2 from Con modifier)
	s.Assert().Equal(12, character.maxHitPoints)
	s.Assert().Equal(12, character.hitPoints)

	// Verify physical characteristics from race
	s.Assert().Equal(30, character.speed)
	s.Assert().Equal("Medium", character.size)

	// Verify skills
	s.Assert().Equal(shared.Proficient, character.skills[skills.Perception])
	s.Assert().Equal(shared.Proficient, character.skills[skills.Survival])
	s.Assert().Equal(shared.Proficient, character.skills[skills.Athletics])    // From background
	s.Assert().Equal(shared.Proficient, character.skills[skills.Intimidation]) // From background

	// Verify languages
	s.Assert().Contains(character.languages, languages.Common)   // Always included
	s.Assert().Contains(character.languages, languages.Dwarvish) // From background

	// Verify proficiencies
	s.Assert().Equal(s.testClass.ArmorProficiencies, character.proficiencies.Armor)
	s.Assert().Equal(s.testClass.WeaponProficiencies, character.proficiencies.Weapons)
	s.Assert().Equal(s.testBackground.ToolProficiencies, character.proficiencies.Tools)

	// Verify saving throws
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.STR])
	s.Assert().Equal(shared.Proficient, character.savingThrows[abilities.CON])
}

func (s *DraftTestSuite) TestToCharacter_WithSubrace() {
	// Create elf race data with subrace
	elfRace := &race.Data{
		ID:    "elf",
		Name:  "Elf",
		Size:  "Medium",
		Speed: 30,
		AbilityScoreIncreases: map[abilities.Ability]int{
			abilities.DEX: 2,
		},
		Languages: []languages.Language{languages.Common, languages.Elvish},
		Subraces: []race.SubraceData{
			{
				ID:   "high-elf",
				Name: "High Elf",
				AbilityScoreIncreases: map[abilities.Ability]int{
					abilities.INT: 1,
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
			RaceID:    races.Elf,
			SubraceID: races.HighElf,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 14,
			abilities.DEX: 15,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
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
	s.Assert().Equal(17, character.abilityScores[abilities.DEX]) // 15 + 2 (elf)
	s.Assert().Equal(13, character.abilityScores[abilities.INT]) // 12 + 1 (high elf)

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
		RaceChoice: RaceChoice{RaceID: races.Human},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
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
			RaceID: races.Human,
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
			RaceID: races.Human,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		Choices: []ChoiceData{
			{
				Category:       shared.ChoiceSkills,
				Source:         shared.SourceClass,
				ChoiceID:       "fighter_skill_proficiencies",
				SkillSelection: []skills.Skill{skills.Athletics, skills.Perception},
			},
		},
		Progress:  DraftProgress{flags: ProgressName | ProgressRace | ProgressClass},
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now(),
	}

	data := draft.ToData()

	s.Assert().Equal(draft.ID, data.ID)
	s.Assert().Equal(draft.PlayerID, data.PlayerID)
	s.Assert().Equal(draft.Name, data.Name)
	s.Assert().Equal(draft.RaceChoice, data.RaceChoice)
	s.Assert().Equal(draft.ClassChoice, data.ClassChoice)
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
		RaceChoice: RaceChoice{
			RaceID: races.Human,
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
		},
		Choices: []ChoiceData{
			{
				Category: shared.ChoiceLanguages,
				Source:   shared.SourceRace,
				ChoiceID: "human_languages",
				LanguageSelection: []languages.Language{
					languages.Elvish,
					languages.Goblin,
					languages.Draconic,
				},
			},
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
	s.Assert().Contains(character.languages, languages.Common, "Common should always be included")
	s.Assert().Contains(character.languages, languages.Dwarvish, "Background language should be included")

	// Verify chosen languages
	s.Assert().Contains(character.languages, languages.Elvish, "Chosen language Elvish should be included")
	s.Assert().Contains(character.languages, languages.Goblin, "Chosen language Goblin should be included")
	s.Assert().Contains(character.languages, languages.Draconic, "Chosen language Draconic should be included")

	// Verify no duplicates (set behavior)
	languageCount := make(map[languages.Language]int)
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
		Languages: []languages.Language{languages.Language(testLanguageExotic)}, // No Common
	}

	draft := &Draft{
		ID:       "test-draft-no-common",
		PlayerID: "player-123",
		Name:     "Exotic Hero",
		RaceChoice: RaceChoice{
			RaceID: races.Race("exotic"),
		},
		ClassChoice: ClassChoice{
			ClassID: classes.Fighter,
		},
		BackgroundChoice: backgrounds.Soldier,
		AbilityScoreChoice: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14,
			abilities.CON: 13,
			abilities.INT: 12,
			abilities.WIS: 10,
			abilities.CHA: 8,
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
	s.Assert().Contains(character.languages, languages.Common,
		"Common should always be included even if race doesn't have it")
}

func TestDraftTestSuite(t *testing.T) {
	suite.Run(t, new(DraftTestSuite))
}
