package choices

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
)

// Example_validate_completeCharacter demonstrates validating a complete character build
func Example_validate_completeCharacter() {
	// Create typed submissions for a Fighter/Half-Elf/Soldier build
	submissions := NewTypedSubmissions()

	// Class choices (Fighter)
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSkills,
		ChoiceID: "fighter_skills",
		Values:   []string{"athletics", "intimidation"},
	})

	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldFightingStyle,
		ChoiceID: "fighter_style_1",
		Values:   []string{"defense"},
	})

	// Race choices (Half-Elf)
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceRace,
		Field:    FieldSkills,
		ChoiceID: "half_elf_skills",
		Values:   []string{"perception", "persuasion"},
	})

	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceRace,
		Field:    FieldLanguages,
		ChoiceID: "half_elf_language",
		Values:   []string{"dwarvish"},
	})

	// Background choices (Soldier)
	// Soldier grants Athletics and Intimidation automatically, which will cause warnings

	// Create validation context with proficiencies
	context := &ValidationContext{
		SkillProficiencies: map[string]Source{
			"athletics":    SourceClass,
			"intimidation": SourceClass,
			"perception":   SourceRace,
			"persuasion":   SourceRace,
		},
		CharacterLevel: 1,
		ClassLevel:     1,
	}

	// Validate all choices
	result := Validate(
		classes.Fighter,
		races.HalfElf,
		backgrounds.Soldier,
		1,
		submissions,
		context,
	)

	// Print validation result
	fmt.Printf("Can Save Draft: %v\n", result.CanSave)
	fmt.Printf("Can Finalize: %v\n", result.CanFinalize)
	fmt.Printf("Is Optimal: %v\n", result.IsOptimal)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Incomplete: %d\n", len(result.Incomplete))
	fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Output:
	// Can Save Draft: true
	// Can Finalize: false
	// Is Optimal: false
	// Errors: 0
	// Incomplete: 4
	// Warnings: 0
}

// Example_validate_withIssues demonstrates validation with various issue types
func Example_validate_withIssues() {
	// Create submissions with various issues
	submissions := NewTypedSubmissions()

	// Duplicate skill selection (warning)
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSkills,
		ChoiceID: "rogue_skills",
		Values:   []string{"stealth", "stealth", "perception", "acrobatics"},
	})

	// Expertise without proficiency (error)
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldExpertise,
		ChoiceID: "rogue_expertise_1",
		Values:   []string{"athletics", "thieves-tools"}, // No athletics proficiency
	})

	// Create context with limited proficiencies
	context := &ValidationContext{
		SkillProficiencies: map[string]Source{
			"stealth":    SourceClass,
			"perception": SourceClass,
			"acrobatics": SourceClass,
			"deception":  SourceClass,
		},
		ToolProficiencies: map[string]Source{
			"thieves-tools": SourceClass,
		},
		CharacterLevel: 1,
	}

	// Validate with Rogue class
	result := Validate(
		classes.Rogue,
		races.Human,
		backgrounds.Criminal,
		1,
		submissions,
		context,
	)

	// Show different severity levels
	for _, err := range result.Errors {
		fmt.Printf("ERROR [%s]: %s\n", err.Code, err.Message)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("WARNING [%s]: %s\n", warning.Code, warning.Message)
	}

	// Output:
	// ERROR [duplicate_selection]: Duplicate selection: stealth
	// ERROR [expertise_without_proficiency]: Cannot have expertise in athletics without proficiency
}

func TestValidationResult_JSON(t *testing.T) {
	// Test that validation results serialize properly to JSON
	result := NewValidationResult()

	result.AddIssue(ValidationIssue{
		Code:     CodeTooFewChoices,
		Severity: SeverityIncomplete,
		Field:    FieldSkills,
		Message:  "Must choose exactly 2 skills, got 1",
		Source:   SourceClass,
		Details: CountDetails{
			Expected: 2,
			Actual:   1,
			Options:  []string{"athletics", "acrobatics", "intimidation"},
		}.ToMap(),
	})

	result.AddIssue(ValidationIssue{
		Code:     CodeDuplicateSelection,
		Severity: SeverityWarning,
		Field:    FieldSkills,
		Message:  "Skill 'athletics' granted by multiple sources: [class, background]",
		Details: DuplicateDetails{
			Duplicate: "athletics",
			Sources:   []Source{SourceClass, SourceBackground},
		}.ToMap(),
	})

	// Serialize to JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal validation result: %v", err)
	}

	// Verify it can be unmarshaled
	var unmarshaled ValidationResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal validation result: %v", err)
	}

	// Verify data integrity
	if len(unmarshaled.Incomplete) != 1 {
		t.Errorf("Expected 1 incomplete issue, got %d", len(unmarshaled.Incomplete))
	}
	if len(unmarshaled.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(unmarshaled.Warnings))
	}
}

func TestTypedSubmissions_ComplexScenario(t *testing.T) {
	// Test a complex multiclass scenario
	submissions := NewTypedSubmissions()

	// Level 1 Fighter choices
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceClass,
		Field:    FieldSkills,
		ChoiceID: "fighter_skills",
		Values:   []string{"athletics", "intimidation"},
	})

	// Level 2 - Multiclass to Rogue
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceMulticlass,
		Field:    FieldSkills,
		ChoiceID: "rogue_multiclass_skill",
		Values:   []string{"stealth"},
	})

	// Level 3 - Rogue expertise
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceMulticlass,
		Field:    FieldExpertise,
		ChoiceID: "rogue_expertise",
		Values:   []string{"stealth", "thieves-tools"},
	})

	// Level 4 - Feat choice
	submissions.AddChoice(ChoiceSubmission{
		Source:   SourceFeat,
		Field:    FieldSkills,
		ChoiceID: "skilled_feat",
		Values:   []string{"perception", "investigation", "acrobatics"},
	})

	// Check that we can retrieve by source
	classChoices := submissions.GetBySource(SourceClass)
	if len(classChoices) != 1 {
		t.Errorf("Expected 1 class choice, got %d", len(classChoices))
	}

	multiclassChoices := submissions.GetBySource(SourceMulticlass)
	if len(multiclassChoices) != 2 {
		t.Errorf("Expected 2 multiclass choices, got %d", len(multiclassChoices))
	}

	// Check that we can retrieve all skills across sources
	allSkills := submissions.GetAllValues(FieldSkills)
	if len(allSkills) != 3 {
		t.Errorf("Expected skills from 3 sources, got %d", len(allSkills))
	}

	// Verify specific skill values
	if len(allSkills[SourceClass]) != 2 {
		t.Errorf("Expected 2 class skills, got %d", len(allSkills[SourceClass]))
	}
	if len(allSkills[SourceFeat]) != 3 {
		t.Errorf("Expected 3 feat skills, got %d", len(allSkills[SourceFeat]))
	}
}
