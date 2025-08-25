package validation

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ValidateClassChoicesV3 validates choices and returns both errors and warnings
func ValidateClassChoicesV3(classID classes.Class, choices []character.Choice) (*Result, error) {
	result := &Result{
		Errors:   []Error{},
		Warnings: []Warning{},
	}

	// Filter for class-sourced choices for error checking
	classChoices := filterChoicesBySource(choices, shared.SourceClass)

	// Get class-specific errors using existing V2 validation
	errors, err := ValidateClassChoicesV2(classID, classChoices)
	if err != nil {
		return nil, err
	}
	result.Errors = errors

	// Now check for cross-source warnings using ALL choices
	result.Warnings = append(result.Warnings, DetectCrossSourceDuplicates(choices)...)

	// Check expertise prerequisites (especially important for Rogue)
	if classID == classes.Rogue {
		result.Warnings = append(result.Warnings, ValidateExpertisePrerequisites(choices)...)
	}

	// Add class-specific warnings
	switch classID {
	case classes.Fighter:
		result.Warnings = append(result.Warnings, validateFighterWarnings(choices)...)
	case classes.Bard:
		result.Warnings = append(result.Warnings, validateBardWarnings(choices)...)
	case classes.Rogue:
		result.Warnings = append(result.Warnings, validateRogueWarnings(choices)...)
	}

	return result, nil
}

// validateFighterWarnings checks for Fighter-specific warnings
func validateFighterWarnings(choices []character.Choice) []Warning {
	var warnings []Warning

	// Check if Fighter picked Intimidation and race is Half-Orc
	hasClassIntimidation := false
	hasRaceIntimidation := false

	for _, choice := range choices {
		if sc, ok := choice.(character.SkillChoice); ok {
			for _, skill := range sc.Skills {
				if skill == "intimidation" {
					if choice.GetSource() == shared.SourceClass {
						hasClassIntimidation = true
					} else if choice.GetSource() == shared.SourceRace {
						hasRaceIntimidation = true
					}
				}
			}
		}
	}

	if hasClassIntimidation && hasRaceIntimidation {
		msg := "Intimidation skill selected from both Fighter class and Half-Orc race. " +
			"Consider choosing a different Fighter skill."
		warnings = append(warnings, Warning{
			Field:   "skills",
			Message: msg,
			Code:    WarningDuplicateSkill,
		})
	}

	return warnings
}

// validateBardWarnings checks for Bard-specific warnings
func validateBardWarnings(choices []character.Choice) []Warning {
	var warnings []Warning

	// Check if Bard has any overlapping skills from background
	backgroundSkills := make(map[string]bool)
	classSkills := make(map[string]bool)

	for _, choice := range choices {
		if sc, ok := choice.(character.SkillChoice); ok {
			for _, skill := range sc.Skills {
				skillStr := string(skill)
				if choice.GetSource() == shared.SourceBackground {
					backgroundSkills[skillStr] = true
				} else if choice.GetSource() == shared.SourceClass {
					classSkills[skillStr] = true
				}
			}
		}
	}

	// Check for overlaps
	for skill := range classSkills {
		if backgroundSkills[skill] {
			warnings = append(warnings, Warning{
				Field:   "skills",
				Message: fmt.Sprintf("Bard can choose any skill, but '%s' is already provided by background", skill),
				Code:    WarningDuplicateSkill,
			})
		}
	}

	return warnings
}

// validateRogueWarnings checks for Rogue-specific warnings
func validateRogueWarnings(choices []character.Choice) []Warning {
	var warnings []Warning

	// Check if Rogue has expertise in thieves' tools without proficiency
	hasThievesToolsProficiency := false
	hasThievesToolsExpertise := false

	for _, choice := range choices {
		// Rogues automatically get thieves' tools proficiency, but let's check anyway
		if tc, ok := choice.(character.ToolProficiencyChoice); ok {
			for _, tool := range tc.Tools {
				if tool == "thieves-tools" {
					hasThievesToolsProficiency = true
				}
			}
		}

		if ec, ok := choice.(character.ExpertiseChoice); ok {
			for _, expertise := range ec.Expertise {
				if expertise == "thieves-tools" {
					hasThievesToolsExpertise = true
				}
			}
		}
	}

	// Note: Rogues automatically get thieves' tools, so this is more of a data validation
	if hasThievesToolsExpertise && !hasThievesToolsProficiency {
		warnings = append(warnings, Warning{
			Field:   "expertise",
			Message: "Expertise in thieves' tools selected, but proficiency not found (Rogues get this automatically)",
			Code:    WarningExpertiseWithoutProficiency,
		})
	}

	return warnings
}

// ValidateDraftWithWarnings validates an entire character draft including cross-source warnings
func ValidateDraftWithWarnings(draft *character.Draft) (*Result, error) {
	if draft == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "draft cannot be nil")
	}

	result := &Result{
		Errors:   []Error{},
		Warnings: []Warning{},
	}

	// Convert ChoiceData to Choice interface types
	allChoices := []character.Choice{}
	for _, choiceData := range draft.Choices {
		choice, err := character.ConvertFromChoiceData(choiceData)
		if err != nil {
			// Log conversion error but continue
			continue
		}
		allChoices = append(allChoices, choice)
	}

	// Run class validation if class is selected
	if draft.ClassChoice.ClassID != "" {
		classID := draft.ClassChoice.ClassID
		classResult, err := ValidateClassChoicesV3(classID, allChoices)
		if err != nil {
			return nil, err
		}
		result.Errors = append(result.Errors, classResult.Errors...)
		result.Warnings = append(result.Warnings, classResult.Warnings...)
	}

	// Run race validation if implemented
	// TODO: Add race validation when available

	// Check for general cross-source issues
	crossSourceWarnings := DetectCrossSourceDuplicates(allChoices)

	// De-duplicate warnings (in case class validation already added some)
	warningMap := make(map[string]bool)
	for _, w := range result.Warnings {
		key := fmt.Sprintf("%s:%s:%s", w.Field, w.Code, w.Message)
		warningMap[key] = true
	}

	for _, w := range crossSourceWarnings {
		key := fmt.Sprintf("%s:%s:%s", w.Field, w.Code, w.Message)
		if !warningMap[key] {
			result.Warnings = append(result.Warnings, w)
			warningMap[key] = true
		}
	}

	return result, nil
}
