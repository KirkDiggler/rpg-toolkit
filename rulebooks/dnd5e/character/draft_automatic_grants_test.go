package character

import (
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDraft_AutomaticGrantDetection(t *testing.T) {
	t.Run("Half-Orc Intimidation Warning", func(t *testing.T) {
		// The rulebook knows that Half-Orc grants Intimidation automatically
		// and Folk Hero grants Animal Handling and Survival

		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Fighter",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.HalfOrc,
			},
			BackgroundChoice: backgrounds.FolkHero,
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "fighter_skills",
					SkillSelection: []skills.Skill{
						skills.Survival,     // Already granted by Folk Hero background
						skills.Intimidation, // Already granted by Half-Orc race
					},
				},
			},
		}

		// Now just use ValidateChoices - the rulebook knows what these grant
		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should be able to save (warnings don't prevent saving)
		assert.True(t, result.CanSave, "Should be able to save with warnings")

		// Should have warnings for redundant choices
		assert.NotEmpty(t, result.Warnings, "Should have warnings for redundant choices")

		// Check for specific warnings
		foundSurvivalWarning := false
		foundIntimidationWarning := false

		for _, warning := range result.Warnings {
			// Check for Survival warning (granted by Folk Hero background)
			if strings.Contains(warning.Message, "survival") &&
				strings.Contains(warning.Message, "already granted automatically by background") {
				foundSurvivalWarning = true
			}
			// Check for Intimidation warning (granted by Half-Orc race)
			if strings.Contains(warning.Message, "intimidation") &&
				strings.Contains(warning.Message, "already granted automatically by race") {
				foundIntimidationWarning = true
			}
		}

		assert.True(t, foundSurvivalWarning, "Should warn about Survival being redundant")
		assert.True(t, foundIntimidationWarning, "Should warn about Intimidation being redundant")

		// Verify draft state was updated
		assert.NotEmpty(t, draft.ValidationWarnings, "Draft should have warnings")
		assert.False(t, draft.CanFinalize, "Should not finalize without equipment and fighting style")
	})

	t.Run("High Elf Language Warning", func(t *testing.T) {
		// The rulebook knows that High Elf grants Elvish automatically
		// and Noble grants History and Persuasion skills

		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Wizard",
			ClassChoice: ClassChoice{
				ClassID: classes.Wizard,
			},
			RaceChoice: RaceChoice{
				RaceID: races.HighElf,
			},
			BackgroundChoice: backgrounds.Noble,
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceLanguages,
					Source:   shared.SourceRace,
					ChoiceID: "high_elf_bonus_language",
					LanguageSelection: []languages.Language{
						languages.Elvish, // Already granted by High Elf race
					},
				},
			},
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should have warning for redundant language choice
		foundLanguageWarning := false
		for _, warning := range result.Warnings {
			if assert.Contains(t, warning.Message, "elvish") &&
				assert.Contains(t, warning.Message, "already granted automatically by race") {
				foundLanguageWarning = true
			}
		}
		assert.True(t, foundLanguageWarning, "Should warn about Elvish being redundant")
	})

	t.Run("No False Warnings for Valid Choices", func(t *testing.T) {
		// The rulebook knows that Human doesn't grant any skills automatically
		// and Acolyte grants Insight and Religion

		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Fighter",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Acolyte,
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "fighter_skills",
					SkillSelection: []skills.Skill{
						skills.Athletics,    // Not granted by anything
						skills.Intimidation, // Not granted by anything
					},
				},
			},
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should have no warnings for these valid choices
		for _, warning := range result.Warnings {
			// Shouldn't warn about Athletics or Intimidation
			assert.NotContains(t, warning.Message, "athletics")
			assert.NotContains(t, warning.Message, "intimidation")
		}

		assert.True(t, result.CanSave, "Should be able to save valid choices")
	})

	t.Run("ValidateChoicesWithData Deprecated", func(t *testing.T) {
		// Test that the deprecated method still works but just calls ValidateChoices
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Fighter",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.HalfOrc,
			},
			BackgroundChoice: backgrounds.Soldier,
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "fighter_skills",
					SkillSelection: []skills.Skill{
						skills.Athletics,
						skills.Intimidation,
					},
				},
			},
		}

		// The deprecated method should still work but now detects redundancy
		result, err := draft.ValidateChoicesWithData(nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should HAVE warnings now since we use rulebook knowledge
		assert.NotEmpty(t, result.Warnings, "Should have warnings for redundant choices")

		assert.True(t, result.CanSave, "Should be able to save")
		assert.False(t, result.CanFinalize, "Should not finalize without equipment and fighting style")
	})
}
