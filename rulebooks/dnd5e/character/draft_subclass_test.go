package character

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDraft_SubclassGrantApplication(t *testing.T) {
	t.Run("Life Domain Cleric Gets Heavy Armor", func(t *testing.T) {
		// Create a draft with Life Domain cleric
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Cleric",
			ClassChoice: ClassChoice{
				ClassID:    classes.Cleric,
				SubclassID: classes.LifeDomain,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Acolyte,
			AbilityScoreChoice: shared.AbilityScores{
				abilities.STR: 14,
				abilities.DEX: 12,
				abilities.CON: 13,
				abilities.INT: 10,
				abilities.WIS: 16,
				abilities.CHA: 11,
			},
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "cleric_skills",
					SkillSelection: []skills.Skill{
						skills.History, // Acolyte gives Insight & Religion, so choose different skills
						skills.Medicine,
					},
				},
			},
		}

		// Create mock data for conversion
		raceData := &race.Data{
			ID:    races.Human,
			Speed: 30,
			Size:  "Medium",
			AbilityScoreIncreases: map[abilities.Ability]int{
				abilities.STR: 1,
				abilities.DEX: 1,
				abilities.CON: 1,
				abilities.INT: 1,
				abilities.WIS: 1,
				abilities.CHA: 1,
			},
		}

		classData := &class.Data{
			ID:                    classes.Cleric,
			HitDice:               8,
			SavingThrows:          []abilities.Ability{abilities.WIS, abilities.CHA},
			SkillProficiencyCount: 2, // Clerics choose 2 skills
			SkillOptions: []skills.Skill{
				skills.History, skills.Insight, skills.Medicine,
				skills.Persuasion, skills.Religion,
			},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple},
		}

		backgroundData := &shared.Background{
			ID:                 backgrounds.Acolyte,
			SkillProficiencies: []skills.Skill{skills.Insight, skills.Religion},
		}

		// Mark draft as complete enough
		draft.Progress.setFlag(ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores)

		// Convert to character
		character, err := draft.ToCharacter(raceData, classData, backgroundData)
		require.NoError(t, err)
		require.NotNil(t, character)

		// Verify Life Domain gets heavy armor proficiency
		charData := character.ToData()
		hasHeavyArmor := false
		for _, armor := range charData.Proficiencies.Armor {
			if armor == proficiencies.ArmorHeavy {
				hasHeavyArmor = true
				break
			}
		}
		assert.True(t, hasHeavyArmor, "Life Domain cleric should have heavy armor proficiency")
	})

	t.Run("War Domain Cleric Gets Martial Weapons", func(t *testing.T) {
		// Create a draft with War Domain cleric
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test War Cleric",
			ClassChoice: ClassChoice{
				ClassID:    classes.Cleric,
				SubclassID: classes.WarDomain,
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Soldier,
			AbilityScoreChoice: shared.AbilityScores{
				abilities.STR: 16,
				abilities.DEX: 10,
				abilities.CON: 14,
				abilities.INT: 10,
				abilities.WIS: 15,
				abilities.CHA: 11,
			},
			Choices: []ChoiceData{
				{
					Category: shared.ChoiceSkills,
					Source:   shared.SourceClass,
					ChoiceID: "cleric_skills",
					SkillSelection: []skills.Skill{
						skills.History, // Soldier gives Athletics & Intimidation
						skills.Medicine,
					},
				},
			},
		}

		// Create mock data
		raceData := &race.Data{
			ID:    races.Human,
			Speed: 30,
			Size:  "Medium",
			AbilityScoreIncreases: map[abilities.Ability]int{
				abilities.STR: 1,
				abilities.DEX: 1,
				abilities.CON: 1,
				abilities.INT: 1,
				abilities.WIS: 1,
				abilities.CHA: 1,
			},
		}

		classData := &class.Data{
			ID:                    classes.Cleric,
			HitDice:               8,
			SavingThrows:          []abilities.Ability{abilities.WIS, abilities.CHA},
			SkillProficiencyCount: 2, // Clerics choose 2 skills
			SkillOptions: []skills.Skill{
				skills.History, skills.Insight, skills.Medicine,
				skills.Persuasion, skills.Religion,
			},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple},
		}

		backgroundData := &shared.Background{
			ID:                 backgrounds.Soldier,
			SkillProficiencies: []skills.Skill{skills.Athletics, skills.Intimidation},
		}

		// Mark draft as complete
		draft.Progress.setFlag(ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores)

		// Convert to character
		character, err := draft.ToCharacter(raceData, classData, backgroundData)
		require.NoError(t, err)
		require.NotNil(t, character)

		// Verify War Domain gets martial weapons AND heavy armor
		charData := character.ToData()
		hasMartialWeapons := false
		hasHeavyArmor := false

		for _, weapon := range charData.Proficiencies.Weapons {
			if weapon == proficiencies.WeaponMartial {
				hasMartialWeapons = true
				break
			}
		}

		for _, armor := range charData.Proficiencies.Armor {
			if armor == proficiencies.ArmorHeavy {
				hasHeavyArmor = true
			}
		}

		assert.True(t, hasMartialWeapons, "War Domain cleric should have martial weapon proficiency")
		assert.True(t, hasHeavyArmor, "War Domain cleric should have heavy armor proficiency")
	})
}

func TestDraft_SubclassValidation(t *testing.T) {
	t.Run("Cleric Without Subclass Fails Validation", func(t *testing.T) {
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Cleric",
			ClassChoice: ClassChoice{
				ClassID: classes.Cleric,
				// No subclass selected!
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Acolyte,
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should have an error about missing subclass
		assert.False(t, result.CanFinalize)
		assert.NotEmpty(t, draft.ValidationErrors)

		foundSubclassError := false
		for _, errMsg := range draft.ValidationErrors {
			if contains(errMsg, "requires a subclass") {
				foundSubclassError = true
				break
			}
		}
		assert.True(t, foundSubclassError, "Should have error about missing subclass")
	})

	t.Run("Sorcerer Without Subclass Fails Validation", func(t *testing.T) {
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Sorcerer",
			ClassChoice: ClassChoice{
				ClassID: classes.Sorcerer,
				// No subclass selected!
			},
			RaceChoice: RaceChoice{
				RaceID: races.Human,
			},
			BackgroundChoice: backgrounds.Sage,
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should have an error about missing subclass
		assert.False(t, result.CanFinalize)
		assert.NotEmpty(t, draft.ValidationErrors)

		foundSubclassError := false
		for _, errMsg := range draft.ValidationErrors {
			if contains(errMsg, "requires a subclass") {
				foundSubclassError = true
				break
			}
		}
		assert.True(t, foundSubclassError, "Should have error about missing subclass for Sorcerer")
	})

	t.Run("Fighter Without Subclass Is Valid", func(t *testing.T) {
		// Fighter doesn't need a subclass at level 1 (gets it at level 3)
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Fighter",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter,
				// No subclass - this is fine for Fighter at level 1
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
		require.NotNil(t, result)

		// Should NOT have an error about missing subclass
		for _, errMsg := range draft.ValidationErrors {
			assert.NotContains(t, errMsg, "requires a subclass",
				"Fighter should not require a subclass at level 1")
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && containsHelper(s[1:], substr)
}

func containsHelper(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[:len(substr)] == substr {
		return true
	}
	return containsHelper(s[1:], substr)
}
