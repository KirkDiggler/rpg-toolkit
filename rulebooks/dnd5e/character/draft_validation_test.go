package character

import (
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
			ID: "test-draft",
			BackgroundChoice: backgrounds.Background("pirate"), // Not defined
		}

		_, err := LoadDraftFromData(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid background")
	})
}