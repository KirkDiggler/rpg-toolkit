package choices

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ValidatorTestSuite tests the validator functionality
type ValidatorTestSuite struct {
	suite.Suite
	validator *Validator
}

func (s *ValidatorTestSuite) SetupTest() {
	s.validator = NewValidator(nil)
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

// Test Fighter class validation
func (s *ValidatorTestSuite) TestValidateClassChoices_Fighter() {
	tests := []struct {
		name              string
		setupSubmissions  func() *TypedSubmissions
		expectCanSave     bool
		expectCanFinalize bool
		expectErrorCount  int
		expectWarnCount   int
	}{
		{
			name: "Valid Fighter Choices",
			setupSubmissions: func() *TypedSubmissions {
				subs := NewTypedSubmissions()
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldSkills,
					ChoiceID: "fighter_skills",
					Values:   []string{"athletics", "intimidation"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldFightingStyle,
					ChoiceID: "fighter_style",
					Values:   []string{"defense"},
				})
				// Add equipment choices
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    Field("equipment_choice_0"),
					ChoiceID: "equipment_0",
					Values:   []string{"chain-mail"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    Field("equipment_choice_1"),
					ChoiceID: "equipment_1",
					Values:   []string{"martial-and-shield"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    Field("equipment_choice_2"),
					ChoiceID: "equipment_2",
					Values:   []string{"light-crossbow"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    Field("equipment_choice_3"),
					ChoiceID: "equipment_3",
					Values:   []string{"dungeoneers-pack"},
				})
				return subs
			},
			expectCanSave:     true,
			expectCanFinalize: true,
			expectErrorCount:  0,
			expectWarnCount:   0,
		},
		{
			name: "Missing Skills",
			setupSubmissions: func() *TypedSubmissions {
				subs := NewTypedSubmissions()
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldFightingStyle,
					ChoiceID: "fighter_style",
					Values:   []string{"defense"},
				})
				return subs
			},
			expectCanSave:     true, // Missing choices are incomplete, not errors
			expectCanFinalize: false,
			expectErrorCount:  0, // No errors, just incomplete
			expectWarnCount:   0,
		},
		{
			name: "Too Many Skills",
			setupSubmissions: func() *TypedSubmissions {
				subs := NewTypedSubmissions()
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldSkills,
					ChoiceID: "fighter_skills",
					Values:   []string{"athletics", "intimidation", "perception"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldFightingStyle,
					ChoiceID: "fighter_style",
					Values:   []string{"defense"},
				})
				return subs
			},
			expectCanSave:     false,
			expectCanFinalize: false,
			expectErrorCount:  1,
			expectWarnCount:   0,
		},
		{
			name: "Duplicate Skills",
			setupSubmissions: func() *TypedSubmissions {
				subs := NewTypedSubmissions()
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldSkills,
					ChoiceID: "fighter_skills",
					Values:   []string{"athletics", "athletics"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceClass,
					Field:    FieldFightingStyle,
					ChoiceID: "fighter_style",
					Values:   []string{"defense"},
				})
				return subs
			},
			expectCanSave:     false,
			expectCanFinalize: false,
			expectErrorCount:  1,
			expectWarnCount:   0,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			subs := tt.setupSubmissions()
			result := s.validator.ValidateClassChoices(classes.Fighter, 1, subs)

			// Debug output
			if !result.CanSave || !result.CanFinalize {
				for _, err := range result.Errors {
					s.T().Logf("Error: %s", err.Message)
				}
				for _, inc := range result.Incomplete {
					s.T().Logf("Incomplete: %s", inc.Message)
				}
			}

			s.Equal(tt.expectCanSave, result.CanSave, "CanSave mismatch")
			s.Equal(tt.expectCanFinalize, result.CanFinalize, "CanFinalize mismatch")
			s.Len(result.Errors, tt.expectErrorCount, "Error count mismatch")
			s.Len(result.Warnings, tt.expectWarnCount, "Warning count mismatch")
		})
	}
}

// Test Wizard class validation
func (s *ValidatorTestSuite) TestValidateClassChoices_Wizard() {
	subs := NewTypedSubmissions()

	// Wizard needs cantrips and spells
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSkills,
		ChoiceID: "wizard_skills",
		Values:   []string{"arcana", "history"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldCantrips,
		ChoiceID: "wizard_cantrips",
		Values:   []string{"fire-bolt", "mage-hand", "prestidigitation"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSpells,
		ChoiceID: "wizard_spells",
		Values:   []string{"burning-hands", "shield", "magic-missile", "detect-magic", "identify", "sleep"},
	})
	// Add equipment choices
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_0"),
		ChoiceID: "equipment_0",
		Values:   []string{"quarterstaff"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_1"),
		ChoiceID: "equipment_1",
		Values:   []string{"component-pouch"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_2"),
		ChoiceID: "equipment_2",
		Values:   []string{"scholars-pack"},
	})

	result := s.validator.ValidateClassChoices(classes.Wizard, 1, subs)

	// Debug output
	if !result.CanSave || !result.CanFinalize {
		for _, err := range result.Errors {
			s.T().Logf("Error: %s", err.Message)
		}
		for _, inc := range result.Incomplete {
			s.T().Logf("Incomplete: %s", inc.Message)
		}
	}

	s.True(result.CanSave)
	s.True(result.CanFinalize)
	s.Empty(result.Errors)
}

// Test Rogue class with expertise
func (s *ValidatorTestSuite) TestValidateClassChoices_RogueExpertise() {
	// Create context with proficiencies
	context := NewValidationContext()
	context.AddProficiency("stealth")
	context.AddProficiency("thieves-tools")
	context.AddProficiency("deception")
	context.AddProficiency("sleight-of-hand")

	validator := NewValidator(context)

	tests := []struct {
		name             string
		expertiseChoices []string
		expectCanSave    bool
		expectErrorMsg   string
	}{
		{
			name:             "Valid Expertise",
			expertiseChoices: []string{"stealth", "thieves-tools"},
			expectCanSave:    true,
		},
		{
			name:             "Expertise Without Proficiency",
			expertiseChoices: []string{"stealth", "athletics"},
			expectCanSave:    false,
			expectErrorMsg:   "Cannot have expertise in athletics without proficiency",
		},
		{
			name:             "Duplicate Expertise",
			expertiseChoices: []string{"stealth", "stealth"},
			expectCanSave:    false,
			expectErrorMsg:   "Duplicate selection",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			subs := NewTypedSubmissions()
			subs.AddChoice(ChoiceSubmission{
				Source:   SourceClass,
				Field:    FieldSkills,
				ChoiceID: "rogue_skills",
				Values:   []string{"stealth", "deception", "sleight-of-hand", "perception"},
			})
			subs.AddChoice(ChoiceSubmission{
				Source:   SourceClass,
				Field:    FieldExpertise,
				ChoiceID: "rogue_expertise",
				Values:   tt.expertiseChoices,
			})

			result := validator.ValidateClassChoices(classes.Rogue, 1, subs)

			s.Equal(tt.expectCanSave, result.CanSave)
			if tt.expectErrorMsg != "" {
				s.NotEmpty(result.Errors)
				found := false
				for _, err := range result.Errors {
					if containsString(err.Message, tt.expectErrorMsg) {
						found = true
						break
					}
				}
				s.True(found, "Expected error message '%s' not found", tt.expectErrorMsg)
			}
		})
	}
}

// Test cross-source validation
func (s *ValidatorTestSuite) TestValidateCrossSourceDuplicates() {
	subs := NewTypedSubmissions()

	// Fighter chooses Athletics
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSkills,
		ChoiceID: "fighter_skills",
		Values:   []string{"athletics", "intimidation"},
	})

	// Background also grants Athletics - should create info message
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceBackground,
		Field:    FieldSkills,
		ChoiceID: "soldier_skills",
		Values:   []string{"athletics", "survival"},
	})

	result := s.validator.ValidateCrossSourceDuplicates(subs)

	// Should have duplicate detection
	s.True(result.CanSave) // Duplicates across sources are allowed
	s.True(result.CanFinalize)
	s.NotEmpty(result.AllIssues)

	// Check for duplicate detection
	foundDuplicate := false
	for _, issue := range result.AllIssues {
		if issue.Code == CodeDuplicateChoice && containsString(issue.Message, "athletics") {
			foundDuplicate = true
			break
		}
	}
	s.True(foundDuplicate, "Should detect duplicate skill across sources")
}

// Test Half-Elf race validation
func (s *ValidatorTestSuite) TestValidateRaceChoices_HalfElf() {
	tests := []struct {
		name              string
		setupSubmissions  func() *TypedSubmissions
		expectCanSave     bool
		expectCanFinalize bool
	}{
		{
			name: "Valid Half-Elf Choices",
			setupSubmissions: func() *TypedSubmissions {
				subs := NewTypedSubmissions()
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceRace,
					Field:    FieldSkills,
					ChoiceID: "half_elf_skills",
					Values:   []string{"perception", "persuasion"},
				})
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceRace,
					Field:    FieldLanguages,
					ChoiceID: "half_elf_language",
					Values:   []string{"dwarvish"},
				})
				return subs
			},
			expectCanSave:     true,
			expectCanFinalize: true,
		},
		{
			name: "Missing Language Choice",
			setupSubmissions: func() *TypedSubmissions {
				subs := NewTypedSubmissions()
				subs.AddChoice(ChoiceSubmission{
					Source:   SourceRace,
					Field:    FieldSkills,
					ChoiceID: "half_elf_skills",
					Values:   []string{"perception", "persuasion"},
				})
				return subs
			},
			expectCanSave:     true, // Missing choices are incomplete, not errors
			expectCanFinalize: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			subs := tt.setupSubmissions()
			result := s.validator.ValidateRaceChoices(races.HalfElf, subs)

			s.Equal(tt.expectCanSave, result.CanSave)
			s.Equal(tt.expectCanFinalize, result.CanFinalize)
		})
	}
}

// Test complete character validation
func (s *ValidatorTestSuite) TestValidateAll_CompleteCharacter() {
	// Create a complete Fighter/Half-Elf/Soldier build
	subs := NewTypedSubmissions()

	// Class choices (Fighter)
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSkills,
		ChoiceID: "fighter_skills",
		Values:   []string{"athletics", "intimidation"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldFightingStyle,
		ChoiceID: "fighter_style",
		Values:   []string{"defense"},
	})
	// Add equipment choices for Fighter
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_0"),
		ChoiceID: "equipment_0",
		Values:   []string{"chain-mail"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_1"),
		ChoiceID: "equipment_1",
		Values:   []string{"martial-and-shield"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_2"),
		ChoiceID: "equipment_2",
		Values:   []string{"light-crossbow"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    Field("equipment_choice_3"),
		ChoiceID: "equipment_3",
		Values:   []string{"dungeoneers-pack"},
	})

	// Race choices (Half-Elf)
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceRace,
		Field:    FieldSkills,
		ChoiceID: "half_elf_skills",
		Values:   []string{"perception", "persuasion"},
	})
	subs.AddChoice(ChoiceSubmission{
		Source:   SourceRace,
		Field:    FieldLanguages,
		ChoiceID: "half_elf_language",
		Values:   []string{"dwarvish"},
	})

	result := s.validator.ValidateAll(
		classes.Fighter,
		races.HalfElf,
		backgrounds.Soldier,
		1,
		subs,
	)

	s.True(result.CanSave)
	s.True(result.CanFinalize)
	// Note: IsOptimal will be false if there are any warnings about duplicate skills
}

// Test severity levels
func (s *ValidatorTestSuite) TestValidationSeverityLevels() {
	subs := NewTypedSubmissions()

	// Missing required choice (incomplete)
	result := s.validator.ValidateClassChoices(classes.Fighter, 1, subs)

	// Should have incomplete issues
	s.False(result.CanFinalize, "Missing choices should prevent finalization")
	s.NotEmpty(result.Incomplete, "Should have incomplete issues for missing required choices")

	// Check severity categorization
	for _, inc := range result.Incomplete {
		s.Equal(SeverityIncomplete, inc.Severity)
	}
}

// Helper function
func containsString(text, substr string) bool {
	return len(text) > 0 && len(substr) > 0 && assert.Contains(nil, text, substr)
}
