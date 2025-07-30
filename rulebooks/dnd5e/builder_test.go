package dnd5e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Test data helpers
func createTestRaceData() dnd5e.RaceData {
	return dnd5e.RaceData{
		ID:    constants.RaceHuman,
		Name:  "Human",
		Size:  "medium",
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
}

func createTestClassData() dnd5e.ClassData {
	return dnd5e.ClassData{
		ID:                    constants.ClassFighter,
		Name:                  "Fighter",
		HitDice:               10,
		HitPointsPerLevel:     6,
		SkillProficiencyCount: 2,
		SkillOptions: []constants.Skill{
			constants.SkillAcrobatics, constants.SkillAthletics, constants.SkillHistory, constants.SkillInsight,
			constants.SkillIntimidation, constants.SkillPerception, constants.SkillSurvival,
		},
		SavingThrows:        []constants.Ability{constants.STR, constants.CON},
		ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies: []string{"simple", "martial"},
	}
}

func createTestBackgroundData() dnd5e.Background {
	return dnd5e.Background{
		ID:                 constants.BackgroundSoldier,
		Name:               "Soldier",
		SkillProficiencies: []constants.Skill{constants.SkillAthletics, constants.SkillIntimidation},
		Languages:          []constants.Language{},
	}
}

func TestCharacterBuilder(t *testing.T) {
	builder, err := dnd5e.NewCharacterBuilder("test-draft-id")
	require.NoError(t, err)

	// Test name setting
	err = builder.SetName("Test Character")
	require.NoError(t, err)

	// Test empty name validation
	err = builder.SetName("")
	assert.Error(t, err)

	// Test race setting
	raceData := createTestRaceData()
	err = builder.SetRaceData(raceData, "")
	require.NoError(t, err)

	// Test class setting
	classData := createTestClassData()
	err = builder.SetClassData(classData, "")
	require.NoError(t, err)

	// Test background setting
	backgroundData := createTestBackgroundData()
	err = builder.SetBackgroundData(backgroundData)
	require.NoError(t, err)

	// Test progress tracking
	progress := builder.Progress()
	assert.Greater(t, progress.PercentComplete, float32(0))
	assert.Contains(t, progress.CompletedSteps, "name")
	assert.Contains(t, progress.CompletedSteps, "race")
	assert.Contains(t, progress.CompletedSteps, "class")
	assert.Contains(t, progress.CompletedSteps, "background")
}

func TestCharacterCreationFlow(t *testing.T) {
	// Create a builder and complete character creation
	builder, err := dnd5e.NewCharacterBuilder("test-draft-id")
	require.NoError(t, err)

	// Set name
	err = builder.SetName("Ragnar")
	require.NoError(t, err)

	// Set race
	raceData := createTestRaceData()
	err = builder.SetRaceData(raceData, "")
	require.NoError(t, err)

	// Set class
	classData := createTestClassData()
	err = builder.SetClassData(classData, "")
	require.NoError(t, err)

	// Set background
	backgroundData := createTestBackgroundData()
	err = builder.SetBackgroundData(backgroundData)
	require.NoError(t, err)

	// Set ability scores
	scores, err := shared.NewAbilityScores(&shared.AbilityScoreConfig{
		STR: 15,
		DEX: 14,
		CON: 13,
		INT: 12,
		WIS: 10,
		CHA: 8,
	})
	require.NoError(t, err)
	err = builder.SetAbilityScores(scores)
	require.NoError(t, err)

	// Select skills
	skills := []string{"perception", "survival"}
	err = builder.SelectSkills(skills)
	require.NoError(t, err)

	// Check progress
	progress := builder.Progress()
	assert.True(t, progress.CanBuild)

	// Build character
	character, err := builder.Build()
	require.NoError(t, err)
	require.NotNil(t, character)

	// Convert to data
	charData := character.ToData()
	assert.Equal(t, "Ragnar", charData.Name)
	assert.Equal(t, 1, charData.Level)
	assert.Equal(t, constants.RaceHuman, charData.RaceID)
	assert.Equal(t, constants.ClassFighter, charData.ClassID)
	assert.Equal(t, constants.BackgroundSoldier, charData.BackgroundID)

	// Check ability scores include racial bonuses
	assert.Equal(t, 16, charData.AbilityScores[constants.STR]) // 15 + 1 racial
	assert.Equal(t, 15, charData.AbilityScores[constants.DEX]) // 14 + 1 racial

	// Check HP calculation
	// Base 10 + CON modifier ((14 - 10) / 2 = 2)
	assert.Equal(t, 12, charData.MaxHitPoints) // 10 base + 2 from CON modifier

	// Save draft for later
	draftData := builder.ToData()
	assert.NotEmpty(t, draftData.Choices)

	// Load draft and continue
	builder2, err := dnd5e.LoadDraft(draftData)
	require.NoError(t, err)

	// The loaded builder doesn't have the race/class/background data objects,
	// so it can't calculate context-aware progress. It should still be able to build.
	progress2 := builder2.Progress()
	assert.True(t, progress2.CanBuild)
	assert.True(t, progress2.PercentComplete >= 100) // May be over 100% without context
}
