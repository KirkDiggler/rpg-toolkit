package character

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDraftAPI_SetClass tests the new SetClass method with input types
func TestDraftAPI_SetClass(t *testing.T) {
	tests := []struct {
		name           string
		input          *SetClassInput
		expectError    bool
		expectWarnings int
		expectErrors   int
		checkChoices   func(t *testing.T, d *Draft)
	}{
		{
			name: "Fighter with skill choices",
			input: &SetClassInput{
				ClassID: classes.Fighter,
				Choices: ClassChoices{
					Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
				},
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				// Should have class choice
				assert.Equal(t, classes.Fighter, d.ClassChoice.ClassID)

				// Should have skill choices stored
				hasSkills := false
				for _, choice := range d.Choices {
					if choice.Category == shared.ChoiceSkills && choice.Source == shared.SourceClass {
						hasSkills = true
						assert.Contains(t, choice.SkillSelection, skills.Athletics)
						assert.Contains(t, choice.SkillSelection, skills.Intimidation)
					}
				}
				assert.True(t, hasSkills, "Should have stored skill choices")
			},
		},
		{
			name: "Cleric without subclass gets error in full validation",
			input: &SetClassInput{
				ClassID: classes.Cleric,
				Choices: ClassChoices{
					Skills: []skills.Skill{skills.Medicine, skills.Religion},
				},
			},
			expectError:  false,
			expectErrors: 1, // Missing subclass is an error for Cleric at level 1
			checkChoices: func(t *testing.T, d *Draft) {
				assert.Equal(t, classes.Cleric, d.ClassChoice.ClassID)
				assert.Empty(t, d.ClassChoice.SubclassID)
			},
		},
		{
			name: "Life Domain Cleric with subclass",
			input: &SetClassInput{
				ClassID:    classes.Cleric,
				SubclassID: classes.LifeDomain,
				Choices: ClassChoices{
					Skills: []skills.Skill{skills.Medicine, skills.Religion},
				},
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				assert.Equal(t, classes.Cleric, d.ClassChoice.ClassID)
				assert.Equal(t, classes.LifeDomain, d.ClassChoice.SubclassID)
			},
		},
		{
			name: "Rogue with expertise",
			input: &SetClassInput{
				ClassID: classes.Rogue,
				Choices: ClassChoices{
					Skills:    []skills.Skill{skills.Stealth, skills.Acrobatics, skills.Deception, skills.Perception},
					Expertise: []skills.Skill{skills.Stealth, skills.Deception},
				},
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				// Should have expertise choices stored
				hasExpertise := false
				for _, choice := range d.Choices {
					if choice.Category == shared.ChoiceExpertise && choice.Source == shared.SourceClass {
						hasExpertise = true
						assert.Contains(t, choice.SkillSelection, skills.Stealth)
						assert.Contains(t, choice.SkillSelection, skills.Deception)
					}
				}
				assert.True(t, hasExpertise, "Should have stored expertise choices")
			},
		},
		{
			name:        "nil input returns error",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := &Draft{
				ID:       "test-draft",
				PlayerID: "test-player",
				Name:     "Test Character",
				// Set race so validation can run
				RaceChoice: RaceChoice{
					RaceID: races.Human,
				},
			}

			result, err := draft.SetClass(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check validation results
			if tt.expectWarnings > 0 {
				assert.Len(t, result.Warnings, tt.expectWarnings, "Unexpected number of warnings")
			}
			if tt.expectErrors > 0 {
				assert.Len(t, result.Errors, tt.expectErrors, "Unexpected number of errors")
			}

			// Check choices were stored correctly
			if tt.checkChoices != nil {
				tt.checkChoices(t, draft)
			}

			// Check progress flags
			assert.True(t, draft.Progress.hasFlag(ProgressClass), "Should have class progress flag")
			if tt.input != nil && len(tt.input.Choices.Skills) > 0 {
				assert.True(t, draft.Progress.hasFlag(ProgressSkills), "Should have skills progress flag")
			}
		})
	}
}

// TestDraftAPI_SetRace tests the new SetRace method with input types
func TestDraftAPI_SetRace(t *testing.T) {
	tests := []struct {
		name         string
		input        *SetRaceInput
		expectError  bool
		expectErrors int
		checkChoices func(t *testing.T, d *Draft)
	}{
		{
			name: "Human with no special choices",
			input: &SetRaceInput{
				RaceID: races.Human,
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				assert.Equal(t, races.Human, d.RaceChoice.RaceID)
				assert.Empty(t, d.RaceChoice.SubraceID)
			},
		},
		{
			name: "Variant Human with language and ability choices",
			input: &SetRaceInput{
				RaceID: races.Human,
				Choices: RaceChoices{
					Languages: []languages.Language{languages.Elvish, languages.Dwarvish},
					AbilityIncrease: map[abilities.Ability]int{
						abilities.STR: 1,
						abilities.CON: 1,
					},
				},
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				// Should have language choices
				hasLangs := false
				for _, choice := range d.Choices {
					if choice.Category == shared.ChoiceLanguages && choice.Source == shared.SourceRace {
						hasLangs = true
						assert.Contains(t, choice.LanguageSelection, languages.Elvish)
						assert.Contains(t, choice.LanguageSelection, languages.Dwarvish)
					}
				}
				assert.True(t, hasLangs, "Should have stored language choices")

				// Should have ability score choices
				hasAbility := false
				for _, choice := range d.Choices {
					if choice.Category == shared.ChoiceAbilityScores && choice.Source == shared.SourceRace {
						hasAbility = true
						assert.NotNil(t, choice.AbilityScoreSelection)
						scores := *choice.AbilityScoreSelection
						assert.Equal(t, 1, scores[abilities.STR])
						assert.Equal(t, 1, scores[abilities.CON])
					}
				}
				assert.True(t, hasAbility, "Should have stored ability score choices")
			},
		},
		{
			name: "Half-Elf with skill choice triggers validation error",
			input: &SetRaceInput{
				RaceID: races.HalfElf,
				Choices: RaceChoices{
					SkillProficiency: &[]skills.Skill{skills.Persuasion}[0],
					Languages:        []languages.Language{languages.Orc},
				},
			},
			expectError:  false,
			expectErrors: 1, // Half-elf gets 2 skills but we only provide 1
			checkChoices: func(t *testing.T, d *Draft) {
				// Should have skill choice
				hasSkill := false
				for _, choice := range d.Choices {
					if choice.Category == shared.ChoiceSkills && choice.Source == shared.SourceRace {
						hasSkill = true
						assert.Contains(t, choice.SkillSelection, skills.Persuasion)
					}
				}
				assert.True(t, hasSkill, "Should have stored skill choice")
			},
		},
		{
			name: "High Elf with subrace",
			input: &SetRaceInput{
				RaceID:    races.Elf,
				SubraceID: races.HighElf,
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				assert.Equal(t, races.Elf, d.RaceChoice.RaceID)
				assert.Equal(t, races.HighElf, d.RaceChoice.SubraceID)
			},
		},
		{
			name: "Elf without subrace gets error",
			input: &SetRaceInput{
				RaceID: races.Elf,
			},
			expectError:  false,
			expectErrors: 1, // Should error about missing subrace
		},
		{
			name:        "nil input returns error",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := &Draft{
				ID:       "test-draft",
				PlayerID: "test-player",
				Name:     "Test Character",
				// Set class so validation can run
				ClassChoice: ClassChoice{
					ClassID: classes.Fighter,
				},
			}

			result, err := draft.SetRace(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check validation results
			assert.Len(t, result.Errors, tt.expectErrors, "Unexpected number of errors")

			// Check choices were stored correctly
			if tt.checkChoices != nil {
				tt.checkChoices(t, draft)
			}

			// Check progress flags
			assert.True(t, draft.Progress.hasFlag(ProgressRace), "Should have race progress flag")
			if tt.input != nil && len(tt.input.Choices.Languages) > 0 {
				assert.True(t, draft.Progress.hasFlag(ProgressLanguages), "Should have languages progress flag")
			}
		})
	}
}

// TestDraftAPI_SetBackground tests the new SetBackground method
func TestDraftAPI_SetBackground(t *testing.T) {
	tests := []struct {
		name         string
		input        *SetBackgroundInput
		expectError  bool
		checkChoices func(t *testing.T, d *Draft)
	}{
		{
			name: "Soldier background",
			input: &SetBackgroundInput{
				BackgroundID: backgrounds.Soldier,
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				assert.Equal(t, backgrounds.Soldier, d.BackgroundChoice)
			},
		},
		{
			name: "Background with language choices",
			input: &SetBackgroundInput{
				BackgroundID: backgrounds.Sage,
				Choices: BackgroundChoices{
					Languages: []languages.Language{languages.Draconic, languages.Primordial},
				},
			},
			expectError: false,
			checkChoices: func(t *testing.T, d *Draft) {
				assert.Equal(t, backgrounds.Sage, d.BackgroundChoice)

				// Should have language choices
				hasLangs := false
				for _, choice := range d.Choices {
					if choice.Category == shared.ChoiceLanguages && choice.Source == shared.SourceBackground {
						hasLangs = true
						assert.Contains(t, choice.LanguageSelection, languages.Draconic)
						assert.Contains(t, choice.LanguageSelection, languages.Primordial)
					}
				}
				assert.True(t, hasLangs, "Should have stored language choices")
			},
		},
		{
			name:        "nil input returns error",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := &Draft{
				ID:       "test-draft",
				PlayerID: "test-player",
				Name:     "Test Character",
				// Set class and race for validation
				ClassChoice: ClassChoice{
					ClassID: classes.Fighter,
				},
				RaceChoice: RaceChoice{
					RaceID: races.Human,
				},
			}

			result, err := draft.SetBackground(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check choices were stored correctly
			if tt.checkChoices != nil {
				tt.checkChoices(t, draft)
			}

			// Check progress flags
			assert.True(t, draft.Progress.hasFlag(ProgressBackground), "Should have background progress flag")
		})
	}
}

// TestDraftAPI_SetAbilityScores tests the new SetAbilityScores method
func TestDraftAPI_SetAbilityScores(t *testing.T) {
	tests := []struct {
		name           string
		input          *SetAbilityScoresInput
		expectError    bool
		expectWarnings int
		expectErrors   int
	}{
		{
			name: "Standard array",
			input: &SetAbilityScoresInput{
				Scores: AbilityScores{
					abilities.STR: 15,
					abilities.DEX: 14,
					abilities.CON: 13,
					abilities.INT: 12,
					abilities.WIS: 10,
					abilities.CHA: 8,
				},
				Method: "standard",
			},
			expectError: false,
		},
		{
			name: "Invalid standard array gets warning",
			input: &SetAbilityScoresInput{
				Scores: AbilityScores{
					abilities.STR: 16, // Not in standard array
					abilities.DEX: 14,
					abilities.CON: 13,
					abilities.INT: 12,
					abilities.WIS: 10,
					abilities.CHA: 8,
				},
				Method: "standard",
			},
			expectError:    false,
			expectWarnings: 1,
		},
		{
			name: "Point buy valid",
			input: &SetAbilityScoresInput{
				Scores: AbilityScores{
					abilities.STR: 15,
					abilities.DEX: 15,
					abilities.CON: 15,
					abilities.INT: 8,
					abilities.WIS: 8,
					abilities.CHA: 8,
				},
				Method: "point-buy",
			},
			expectError: false,
		},
		{
			name: "Point buy exceeds cost gets warning",
			input: &SetAbilityScoresInput{
				Scores: AbilityScores{
					abilities.STR: 15,
					abilities.DEX: 15,
					abilities.CON: 15,
					abilities.INT: 15,
					abilities.WIS: 15,
					abilities.CHA: 15,
				},
				Method: "point-buy",
			},
			expectError:    false,
			expectWarnings: 1,
		},
		{
			name: "Missing ability score",
			input: &SetAbilityScoresInput{
				Scores: AbilityScores{
					abilities.STR: 15,
					abilities.DEX: 14,
					abilities.CON: 13,
					abilities.INT: 12,
					abilities.WIS: 10,
					// Missing CHA
				},
			},
			expectError:  false,
			expectErrors: 1,
		},
		{
			name: "Score out of range",
			input: &SetAbilityScoresInput{
				Scores: AbilityScores{
					abilities.STR: 25, // Too high
					abilities.DEX: 14,
					abilities.CON: 13,
					abilities.INT: 12,
					abilities.WIS: 10,
					abilities.CHA: 8,
				},
			},
			expectError:  false,
			expectErrors: 1,
		},
		{
			name:        "nil input returns error",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := &Draft{
				ID:       "test-draft",
				PlayerID: "test-player",
				Name:     "Test Character",
				// Set class and race for validation
				ClassChoice: ClassChoice{
					ClassID: classes.Fighter,
				},
				RaceChoice: RaceChoice{
					RaceID: races.Human,
				},
			}

			result, err := draft.SetAbilityScores(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check validation results
			if tt.expectWarnings > 0 {
				assert.Len(t, result.Warnings, tt.expectWarnings, "Unexpected number of warnings")
			}
			if tt.expectErrors > 0 {
				assert.Len(t, result.Errors, tt.expectErrors, "Unexpected number of errors")
			}

			// Check progress flags
			assert.True(t, draft.Progress.hasFlag(ProgressAbilityScores), "Should have ability scores progress flag")

			// Check scores were stored
			if tt.input != nil && len(tt.input.Scores) > 0 {
				for ability, score := range tt.input.Scores {
					assert.Equal(t, score, draft.AbilityScoreChoice[ability])
				}
			}
		})
	}
}

// TestDraftAPI_DuplicateSkillWarning tests that duplicate skill grants produce warnings
// For example, Half-Orc gets Intimidation, Soldier background also grants Intimidation
func TestDraftAPI_DuplicateSkillWarning(t *testing.T) {
	draft := &Draft{
		ID:       "test-draft",
		PlayerID: "test-player",
		Name:     "Gruk",
		Progress: DraftProgress{},
	}
	// Set the name progress flag
	draft.Progress.setFlag(ProgressName)

	// Step 1: Set race to Half-Orc (grants Intimidation automatically)
	raceResult, err := draft.SetRace(&SetRaceInput{
		RaceID: races.HalfOrc,
	})
	require.NoError(t, err)
	require.NotNil(t, raceResult)

	// Step 2: Set class to Fighter and choose Intimidation (which Half-Orc already grants)
	classResult, err := draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Intimidation, skills.Athletics},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, classResult)

	// Should get a warning about Intimidation being redundant
	assert.True(t, len(classResult.Warnings) > 0, "Should have warnings about duplicate skill")

	// Find the warning about Intimidation
	foundWarning := false
	for _, warning := range classResult.Warnings {
		if warning.Field == "skills" && warning.Code == "redundant_choice" {
			foundWarning = true
			assert.Contains(t, warning.Message, "intimidation")
			assert.Contains(t, warning.Message, "already granted")
		}
	}
	assert.True(t, foundWarning, "Should have warning about Intimidation being redundant")

	// Step 3: Set background to Soldier (also grants Intimidation AND Athletics)
	bgResult, err := draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	require.NoError(t, err)
	require.NotNil(t, bgResult)

	// Now both skills should show as redundant because Soldier grants both
	assert.True(t, len(bgResult.Warnings) >= 2, "Should have warnings about both skills being redundant")

	// Step 4: Set ability scores to complete the draft
	scoreResult, err := draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 13,
			abilities.CON: 14,
			abilities.INT: 8,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, scoreResult)

	// Verify that despite the warnings, skills are NOT double-counted
	// When we compile to a character, Intimidation should only appear once
	compiledSkills := make(map[skills.Skill]bool)

	// Check what skills are actually stored in choices
	for _, choice := range draft.Choices {
		if choice.Category == shared.ChoiceSkills {
			for _, skill := range choice.SkillSelection {
				// Each skill should only be recorded once per source
				key := skill
				if compiledSkills[key] && choice.Source == shared.SourceClass {
					t.Errorf("Skill %s is recorded multiple times from class source", skill)
				}
				compiledSkills[key] = true
			}
		}
	}

	// The draft should still be valid despite the redundancy warnings
	assert.True(t, draft.IsComplete(), "Draft should be complete")
}

// TestDraftAPI_Workflow tests the full rpg-api workflow
func TestDraftAPI_Workflow(t *testing.T) {
	// Simulate the rpg-api workflow:
	// 1. Create draft
	// 2. Set race with choices
	// 3. Set class with choices
	// 4. Set background with choices
	// 5. Set ability scores
	// 6. Validate everything is ready

	draft := &Draft{
		ID:       "test-draft",
		PlayerID: "test-player",
		Name:     "Thorin Oakenshield",
		Progress: DraftProgress{},
	}
	// Set the name progress flag since we have a name
	draft.Progress.setFlag(ProgressName)

	// Step 1: Set race (Mountain Dwarf)
	raceResult, err := draft.SetRace(&SetRaceInput{
		RaceID:    races.Dwarf,
		SubraceID: races.MountainDwarf,
	})
	require.NoError(t, err)
	require.NotNil(t, raceResult)
	assert.True(t, raceResult.CanFinalize) // Not complete yet but no errors

	// Step 2: Set class (Fighter)
	// Note: Soldier background grants Athletics and Intimidation, so we choose different skills
	classResult, err := draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Acrobatics, skills.History},
			// TODO: Add fighting style and equipment when constants available
		},
	})
	require.NoError(t, err)
	require.NotNil(t, classResult)

	// Step 3: Set background (Soldier)
	bgResult, err := draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	require.NoError(t, err)
	require.NotNil(t, bgResult)

	// Step 4: Set ability scores (standard array)
	scoreResult, err := draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 13,
			abilities.CON: 14,
			abilities.INT: 8,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Method: "standard",
	})
	require.NoError(t, err)
	require.NotNil(t, scoreResult)

	// Debug: Check what validation issues exist
	if !scoreResult.CanFinalize {
		t.Logf("Cannot finalize. Errors: %v, Warnings: %v, Incomplete: %v",
			scoreResult.Errors, scoreResult.Warnings, scoreResult.Incomplete)
	}

	// The draft is NOT finalizable because Fighter requires fighting style and equipment choices
	// This is correct validation behavior
	assert.False(t, scoreResult.CanFinalize, "Should not be finalizable without fighting style and equipment")

	// Check we have the expected incomplete items
	assert.True(t, len(scoreResult.Incomplete) > 0, "Should have incomplete items")

	// Step 5: Verify draft has all basic required fields (but not all choices)
	assert.True(t, draft.IsComplete(), "Should have all basic fields complete")

	// Step 6: Verify all choices were stored
	assert.Equal(t, races.Dwarf, draft.RaceChoice.RaceID)
	assert.Equal(t, races.MountainDwarf, draft.RaceChoice.SubraceID)
	assert.Equal(t, classes.Fighter, draft.ClassChoice.ClassID)
	assert.Equal(t, backgrounds.Soldier, draft.BackgroundChoice)
	assert.Equal(t, 15, draft.AbilityScoreChoice[abilities.STR])

	// Step 7: Verify choices array has all the data
	hasClassSkills := false
	for _, choice := range draft.Choices {
		if choice.Category == shared.ChoiceSkills && choice.Source == shared.SourceClass {
			hasClassSkills = true
			assert.Contains(t, choice.SkillSelection, skills.Acrobatics)
			assert.Contains(t, choice.SkillSelection, skills.History)
		}
	}
	assert.True(t, hasClassSkills, "Should have class skill choices")

	// Step 8: Convert to persistent data (what rpg-api would save)
	data := draft.ToData()
	assert.Equal(t, draft.ID, data.ID)
	assert.Equal(t, draft.PlayerID, data.PlayerID)
	assert.Equal(t, draft.Name, data.Name)
	assert.Equal(t, draft.RaceChoice, data.RaceChoice)
	assert.Equal(t, draft.ClassChoice, data.ClassChoice)
	assert.Equal(t, draft.BackgroundChoice, data.BackgroundChoice)
	assert.Equal(t, draft.AbilityScoreChoice, data.AbilityScoreChoice)
	assert.Equal(t, draft.Choices, data.Choices)

	// Step 9: Load from persistent data (what rpg-api would do on read)
	loadedDraft, err := LoadDraftFromData(data)
	require.NoError(t, err)
	require.NotNil(t, loadedDraft)

	// Verify everything survived the round trip
	assert.Equal(t, draft.ID, loadedDraft.ID)
	assert.Equal(t, draft.Name, loadedDraft.Name)
	assert.Equal(t, draft.RaceChoice, loadedDraft.RaceChoice)
	assert.Equal(t, draft.ClassChoice, loadedDraft.ClassChoice)
	assert.Equal(t, draft.BackgroundChoice, loadedDraft.BackgroundChoice)
	assert.Equal(t, draft.AbilityScoreChoice, loadedDraft.AbilityScoreChoice)
	assert.Equal(t, draft.Choices, loadedDraft.Choices)
}
