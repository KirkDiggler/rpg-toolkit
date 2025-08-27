package choices

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
)

func TestValidateClassChoices_Fighter(t *testing.T) {
	tests := []struct {
		name        string
		submissions Submissions
		expectValid bool
		expectError string
		expectWarn  string
	}{
		{
			name: "Valid Fighter Choices",
			submissions: Submissions{
				"skills":         []string{"athletics", "intimidation"},
				"fighting_style": []string{"defense"},
				"equipment_0":    []string{"chain-mail"},
				"equipment_1":    []string{"martial-and-shield"},
				"equipment_2":    []string{"light-crossbow"},
				"equipment_3":    []string{"dungeoneers-pack"},
			},
			expectValid: true,
		},
		{
			name: "Missing Skills",
			submissions: Submissions{
				"fighting_style": []string{"defense"},
				"equipment_0":    []string{"chain-mail"},
				"equipment_1":    []string{"martial-and-shield"},
				"equipment_2":    []string{"light-crossbow"},
				"equipment_3":    []string{"dungeoneers-pack"},
			},
			expectValid: false,
			expectError: "Must choose exactly 2 skills",
		},
		{
			name: "Too Many Skills",
			submissions: Submissions{
				"skills":         []string{"athletics", "intimidation", "perception"},
				"fighting_style": []string{"defense"},
				"equipment_0":    []string{"chain-mail"},
				"equipment_1":    []string{"martial-and-shield"},
				"equipment_2":    []string{"light-crossbow"},
				"equipment_3":    []string{"dungeoneers-pack"},
			},
			expectValid: false,
			expectError: "Must choose exactly 2 skills",
		},
		{
			name: "Invalid Skill for Fighter",
			submissions: Submissions{
				"skills":         []string{"athletics", "stealth"}, // stealth not available to Fighter
				"fighting_style": []string{"defense"},
				"equipment_0":    []string{"chain-mail"},
				"equipment_1":    []string{"martial-and-shield"},
				"equipment_2":    []string{"light-crossbow"},
				"equipment_3":    []string{"dungeoneers-pack"},
			},
			expectValid: false,
			expectError: "Invalid skill choice: stealth",
		},
		{
			name: "Missing Fighting Style",
			submissions: Submissions{
				"skills":      []string{"athletics", "intimidation"},
				"equipment_0": []string{"chain-mail"},
				"equipment_1": []string{"martial-and-shield"},
				"equipment_2": []string{"light-crossbow"},
				"equipment_3": []string{"dungeoneers-pack"},
			},
			expectValid: false,
			expectError: "Must choose exactly 1 fighting style",
		},
		{
			name: "Invalid Fighting Style",
			submissions: Submissions{
				"skills":         []string{"athletics", "intimidation"},
				"fighting_style": []string{"sneak-attack"}, // not a valid fighting style
				"equipment_0":    []string{"chain-mail"},
				"equipment_1":    []string{"martial-and-shield"},
				"equipment_2":    []string{"light-crossbow"},
				"equipment_3":    []string{"dungeoneers-pack"},
			},
			expectValid: false,
			expectError: "Invalid fighting style",
		},
		{
			name: "Duplicate Skills Warning",
			submissions: Submissions{
				"skills":         []string{"athletics", "athletics"},
				"fighting_style": []string{"defense"},
				"equipment_0":    []string{"chain-mail"},
				"equipment_1":    []string{"martial-and-shield"},
				"equipment_2":    []string{"light-crossbow"},
				"equipment_3":    []string{"dungeoneers-pack"},
			},
			expectValid: true, // Valid but has warning
			expectWarn:  "Duplicate skill selected: athletics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateClassChoices(classes.Fighter, 1, tt.submissions)

			assert.Equal(t, tt.expectValid, result.Valid)

			if tt.expectError != "" {
				require.NotEmpty(t, result.Errors)
				found := false
				for _, err := range result.Errors {
					if containsString(err.Message, tt.expectError) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error containing '%s', got %v", tt.expectError, result.Errors)
			}

			if tt.expectWarn != "" {
				require.NotEmpty(t, result.Warnings)
				found := false
				for _, warn := range result.Warnings {
					if containsString(warn.Message, tt.expectWarn) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning containing '%s', got %v", tt.expectWarn, result.Warnings)
			}
		})
	}
}

func TestValidateClassChoices_Wizard(t *testing.T) {
	validSubmissions := Submissions{
		"skills":      []string{"arcana", "investigation"},
		"cantrips":    []string{"mage-hand", "light", "ray-of-frost"},
		"spells":      []string{"magic-missile", "shield", "sleep", "identify", "detect-magic", "burning-hands"},
		"equipment_0": []string{"quarterstaff"},
		"equipment_1": []string{"component-pouch"},
		"equipment_2": []string{"scholars-pack"},
	}

	result := ValidateClassChoices(classes.Wizard, 1, validSubmissions)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)

	// Test wrong number of cantrips
	invalidCantrips := copySubmissions(validSubmissions)
	invalidCantrips["cantrips"] = []string{"mage-hand", "light"} // only 2, needs 3

	result = ValidateClassChoices(classes.Wizard, 1, invalidCantrips)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateRaceChoices(t *testing.T) {
	tests := []struct {
		name        string
		raceID      races.Race
		submissions Submissions
		expectValid bool
		expectError string
	}{
		{
			name:   "Valid Half-Elf Choices",
			raceID: races.HalfElf,
			submissions: Submissions{
				"race_skills":    []string{"perception", "persuasion"},
				"race_languages": []string{"elvish"},
			},
			expectValid: true,
		},
		{
			name:   "Half-Elf Missing Skills",
			raceID: races.HalfElf,
			submissions: Submissions{
				"race_languages": []string{"elvish"},
			},
			expectValid: false,
			expectError: "Must choose exactly 2 skills",
		},
		{
			name:   "Valid Dragonborn Ancestry",
			raceID: races.Dragonborn,
			submissions: Submissions{
				"draconic_ancestry": []string{"red"},
			},
			expectValid: true,
		},
		{
			name:   "Dragonborn Invalid Ancestry",
			raceID: races.Dragonborn,
			submissions: Submissions{
				"draconic_ancestry": []string{"purple"}, // not a valid color
			},
			expectValid: false,
			expectError: "Invalid ancestry choice",
		},
		{
			name:        "Human Has No Choices",
			raceID:      races.Human,
			submissions: Submissions{},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRaceChoices(tt.raceID, tt.submissions)

			assert.Equal(t, tt.expectValid, result.Valid)

			if tt.expectError != "" {
				require.NotEmpty(t, result.Errors)
				found := false
				for _, err := range result.Errors {
					if containsString(err.Message, tt.expectError) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error containing '%s'", tt.expectError)
			}
		})
	}
}

func TestValidate_CrossSourceDuplicates(t *testing.T) {
	// Test Half-Orc Fighter with duplicate Intimidation
	// Half-Orc grants Intimidation, Fighter can choose Intimidation
	submissions := Submissions{
		"skills":         []string{"intimidation", "athletics"}, // Fighter chooses intimidation
		"fighting_style": []string{"defense"},
		"equipment_0":    []string{"chain-mail"},
		"equipment_1":    []string{"martial-and-shield"},
		"equipment_2":    []string{"light-crossbow"},
		"equipment_3":    []string{"dungeoneers-pack"},
	}

	result := Validate(classes.Fighter, races.HalfOrc, 1, submissions)
	assert.True(t, result.Valid) // Still valid, just suboptimal

	// Should have warning about duplicate
	found := false
	for _, warn := range result.Warnings {
		if containsString(warn.Message, "intimidation") && containsString(warn.Message, "multiple sources") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should warn about intimidation from multiple sources")
}

func TestValidate_Level4ASI(t *testing.T) {
	submissions := Submissions{
		"level4_choice":  []string{"ability_score_improvement"},
		"ability_scores": []string{"strength", "strength"}, // +2 to Strength
	}

	result := Validate(classes.Fighter, races.Human, 4, submissions)
	assert.True(t, result.Valid)
}

// Helper functions

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > len(needle) &&
				(haystack[:len(needle)] == needle ||
					haystack[len(haystack)-len(needle):] == needle ||
					len(needle) > 0 && len(haystack) > len(needle) &&
						findSubstring(haystack, needle)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func copySubmissions(s Submissions) Submissions {
	result := make(Submissions)
	for k, v := range s {
		newSlice := make([]string, len(v))
		copy(newSlice, v)
		result[k] = newSlice
	}
	return result
}
