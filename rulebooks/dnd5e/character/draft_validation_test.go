package character

import (
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDraft_ValidateChoices(t *testing.T) {
	t.Run("Validation Updates Draft State", func(t *testing.T) {
		// Test that validation warnings and errors are stored on the draft
		// Note: For now, we're not detecting automatic grants from race/background
		// That will come with the individual validator updates
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Fighter",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Noble,
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "fighter_skills",
					SkillSelection: []skills.Skill{
						skills.Athletics,
						skills.Athletics, // Duplicate within same choice
					},
				},
			},
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check that draft state was updated
		assert.NotEmpty(t, draft.ValidationErrors, "Should have errors for duplicate skill and missing requirements")
		assert.False(t, draft.CanFinalize, "Should not be able to finalize with errors")

		// Check for duplicate error
		foundDuplicateError := false
		for _, err := range draft.ValidationErrors {
			if strings.Contains(err, "Duplicate selection") {
				foundDuplicateError = true
			}
		}
		assert.True(t, foundDuplicateError, "Should have error for duplicate skill selection")
	})

	t.Run("Valid Fighter Choices", func(t *testing.T) {
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Fighter",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
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

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Debug output
		if !result.CanSave {
			for _, err := range result.Errors {
				t.Logf("Error: %s", err.Message)
			}
		}

		// Fighter requires 2 skills, fighting style, and equipment
		// We only provided skills, so should have incomplete for missing choices
		assert.True(t, result.CanSave, "Should be able to save with valid skill choices")
		assert.False(t, result.CanFinalize, "Should not finalize without fighting style and equipment")
	})

	t.Run("Invalid Class in LoadDraftFromData", func(t *testing.T) {
		data := DraftData{
			ID: "test-draft",
			ClassChoice: ClassChoice{
				ClassID: classes.Class("invalid-class"),
			},
		}

		_, err := LoadDraftFromData(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid class")
	})

	t.Run("Duplicate Skills Detection", func(t *testing.T) {
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Character",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.HalfElf,
			},
			BackgroundChoice: backgrounds.Soldier,
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "fighter_skills",
					SkillSelection: []skills.Skill{
						skills.Athletics,
						skills.Athletics, // Duplicate!
					},
				},
			},
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should have error for duplicate skill
		assert.False(t, result.CanSave)
		assert.NotEmpty(t, result.Errors)
	})
}

func TestDraft_GetValidationStatus(t *testing.T) {
	draft := &Draft{
		ID:   "test-draft",
		Name: "Test Character",
		ValidationWarnings: []string{
			"skill 'athletics' chosen from class but already granted by background",
		},
		ValidationErrors: []string{
			"Must choose exactly 1 fighting style, got 0",
		},
		CanFinalize: false,
	}

	warnings, errors, canFinalize := draft.GetValidationStatus()

	assert.Equal(t, draft.ValidationWarnings, warnings)
	assert.Equal(t, draft.ValidationErrors, errors)
	assert.Equal(t, draft.CanFinalize, canFinalize)
	assert.True(t, draft.HasValidationIssues())
}

func TestLoadDraftFromData_Validation(t *testing.T) {
	t.Run("Valid Data", func(t *testing.T) {
		data := DraftData{
			ID: "test-draft",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Soldier,
		}

		draft, err := LoadDraftFromData(data)
		require.NoError(t, err)
		assert.Equal(t, classes.Fighter, draft.ClassChoice.ClassID)
		assert.Equal(t, races.Human, draft.RaceChoice.RaceID)
		assert.Equal(t, backgrounds.Soldier, draft.BackgroundChoice)
	})

	t.Run("Invalid Race", func(t *testing.T) {
		data := DraftData{
			ID: "test-draft",
			RaceChoice: RaceChoice{
				RaceID: races.Race("goblin"), // Not a valid player race
			},
		}

		_, err := LoadDraftFromData(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid race")
	})

	t.Run("Invalid Background", func(t *testing.T) {
		data := DraftData{
			ID:               "test-draft",
			BackgroundChoice: backgrounds.Background("invalid-background"), // Not defined
		}

		_, err := LoadDraftFromData(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid background")
	})

	t.Run("Validation State Persists", func(t *testing.T) {
		// Create draft with validation state
		originalDraft := &Draft{
			ID: "test-draft",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Soldier,
			ValidationWarnings: []string{
				"skill 'athletics' chosen from class but already granted by background",
			},
			ValidationErrors: []string{
				"Must choose exactly 1 fighting style, got 0",
			},
			CanFinalize: false,
		}

		// Convert to data
		data := originalDraft.ToData()

		// Verify data contains validation state
		assert.Equal(t, originalDraft.ValidationWarnings, data.ValidationWarnings)
		assert.Equal(t, originalDraft.ValidationErrors, data.ValidationErrors)
		assert.Equal(t, originalDraft.CanFinalize, data.CanFinalize)

		// Load from data
		loadedDraft, err := LoadDraftFromData(data)
		require.NoError(t, err)

		// Verify loaded draft has same validation state
		assert.Equal(t, originalDraft.ValidationWarnings, loadedDraft.ValidationWarnings)
		assert.Equal(t, originalDraft.ValidationErrors, loadedDraft.ValidationErrors)
		assert.Equal(t, originalDraft.CanFinalize, loadedDraft.CanFinalize)
	})
}
