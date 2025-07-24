package dnd5e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// Test data helpers
func createTestRaceData() dnd5e.RaceData {
	return dnd5e.RaceData{
		ID:    "human",
		Name:  "Human",
		Size:  "medium",
		Speed: 30,
		AbilityScoreIncreases: map[string]int{
			"strength":     1,
			"dexterity":    1,
			"constitution": 1,
			"intelligence": 1,
			"wisdom":       1,
			"charisma":     1,
		},
		Languages: []string{"common"},
	}
}

func createTestClassData() dnd5e.ClassData {
	return dnd5e.ClassData{
		ID:                    "fighter",
		Name:                  "Fighter",
		HitDice:               10,
		HitPointsAt1st:        10,
		HitPointsPerLevel:     6,
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"acrobatics", "athletics", "history", "insight",
			"intimidation", "perception", "survival",
		},
		SavingThrows:        []string{"strength", "constitution"},
		ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies: []string{"simple", "martial"},
	}
}

func createTestBackgroundData() dnd5e.Background {
	return dnd5e.Background{
		ID:                 "soldier",
		Name:               "Soldier",
		SkillProficiencies: []string{"athletics", "intimidation"},
		Languages:          []string{},
	}
}

func TestCharacterBuilder(t *testing.T) {
	builder := dnd5e.NewCharacterBuilder()

	// Test name setting
	err := builder.SetName("Test Character")
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
	err = builder.SetClassData(classData)
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
	builder := dnd5e.NewCharacterBuilder()

	// Set name
	err := builder.SetName("Ragnar")
	require.NoError(t, err)

	// Set race
	raceData := createTestRaceData()
	err = builder.SetRaceData(raceData, "")
	require.NoError(t, err)

	// Set class
	classData := createTestClassData()
	err = builder.SetClassData(classData)
	require.NoError(t, err)

	// Set background
	backgroundData := createTestBackgroundData()
	err = builder.SetBackgroundData(backgroundData)
	require.NoError(t, err)

	// Set ability scores
	scores := dnd5e.AbilityScores{
		Strength:     15,
		Dexterity:    14,
		Constitution: 13,
		Intelligence: 12,
		Wisdom:       10,
		Charisma:     8,
	}
	err = builder.SetAbilityScores(scores)
	require.NoError(t, err)

	// Select skills
	skills := []string{"athletics", "intimidation"}
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
	assert.Equal(t, "human", charData.RaceID)
	assert.Equal(t, "fighter", charData.ClassID)
	assert.Equal(t, "soldier", charData.BackgroundID)

	// Check ability scores include racial bonuses
	assert.Equal(t, 16, charData.AbilityScores.Strength)  // 15 + 1 racial
	assert.Equal(t, 15, charData.AbilityScores.Dexterity) // 14 + 1 racial

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
