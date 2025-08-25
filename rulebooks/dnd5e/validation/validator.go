// Package validation provides D&D 5e character validation
package validation

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Error represents a validation error (blocking issue)
type Error struct {
	Field   string
	Message string
	Code    rpgerr.Code
}

// Warning represents a validation warning (non-blocking issue)
type Warning struct {
	Field   string
	Message string
	Code    string
}

// Result contains both errors and warnings from validation
type Result struct {
	Errors   []Error
	Warnings []Warning
}

// Common warning codes
const (
	WarningDuplicateSkill              = "duplicate_skill"
	WarningDuplicateLanguage           = "duplicate_language"
	WarningExpertiseWithoutProficiency = "expertise_without_proficiency"
	WarningDuplicateTool               = "duplicate_tool"
	WarningDuplicateInstrument         = "duplicate_instrument"
)

// GetRequiredChoicesForClass returns all the choices a player needs to make for a given class at level 1
func GetRequiredChoicesForClass(classID classes.Class) (*classes.ClassRequirements, error) {
	requirements := classes.GetRequirements(classID)
	if requirements == nil {
		return nil, rpgerr.New(rpgerr.CodeNotFound, fmt.Sprintf("no requirements defined for class: %s", classID))
	}
	return requirements, nil
}

// ValidateClassChoices validates choices for a specific class and returns errors and warnings
func ValidateClassChoices(classID classes.Class, choices []character.Choice) (*Result, error) {
	result := &Result{
		Errors:   []Error{},
		Warnings: []Warning{},
	}
	
	// Get requirements for this class
	requirements := classes.GetRequirements(classID)
	if requirements == nil {
		return nil, rpgerr.New(rpgerr.CodeInternal, fmt.Sprintf("%s requirements not defined", classID))
	}
	
	// Filter for class-sourced choices for error validation
	classChoices := filterChoicesBySource(choices, shared.SourceClass)
	
	// Validate against requirements (errors)
	result.Errors = validateWithRequirements(requirements, classChoices)
	
	// Check for fighting style if Fighter
	if classID == classes.Fighter {
		grouped := groupChoicesByCategory(classChoices)
		if _, ok := grouped[shared.ChoiceFightingStyle]; !ok {
			result.Errors = append(result.Errors, Error{
				Field:   "fighting_style",
				Message: "Fighter requires fighting style selection",
				Code:    rpgerr.CodeInvalidArgument,
			})
		}
	}
	
	// Now check for cross-source warnings using ALL choices
	result.Warnings = append(result.Warnings, detectCrossSourceDuplicates(choices)...)
	
	// Check expertise prerequisites (especially important for Rogue)
	if classID == classes.Rogue {
		result.Warnings = append(result.Warnings, validateExpertisePrerequisites(choices)...)
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

// Helper functions

func filterChoicesBySource(choices []character.Choice, source shared.ChoiceSource) []character.Choice {
	var filtered []character.Choice
	for _, choice := range choices {
		if choice.GetSource() == source {
			filtered = append(filtered, choice)
		}
	}
	return filtered
}

func groupChoicesByCategory(choices []character.Choice) map[shared.ChoiceCategory][]character.Choice {
	grouped := make(map[shared.ChoiceCategory][]character.Choice)
	for _, choice := range choices {
		category := choice.GetCategory()
		grouped[category] = append(grouped[category], choice)
	}
	return grouped
}

// Declarative validation

func validateWithRequirements(requirements *classes.ClassRequirements, choices []character.Choice) []Error {
	if requirements == nil {
		return []Error{{
			Field:   "requirements",
			Message: "No requirements defined for this class",
			Code:    rpgerr.CodeInternal,
		}}
	}

	var errors []Error
	grouped := groupChoicesByCategory(choices)

	// Validate each requirement
	errors = append(errors, validateRequirement(
		requirements.Level1.Skills,
		grouped[shared.ChoiceSkills],
		"skills",
		requirements.ClassName,
	)...)

	errors = append(errors, validateRequirement(
		requirements.Level1.Cantrips,
		grouped[shared.ChoiceCantrips],
		"cantrips",
		requirements.ClassName,
	)...)

	errors = append(errors, validateRequirement(
		requirements.Level1.Spells,
		grouped[shared.ChoiceSpells],
		"spells",
		requirements.ClassName,
	)...)

	errors = append(errors, validateRequirement(
		requirements.Level1.Equipment,
		grouped[shared.ChoiceEquipment],
		"equipment",
		requirements.ClassName,
	)...)

	// Handle instruments separately since they're still under ToolProficiency category
	if requirements.Level1.Instruments != nil {
		errors = append(errors, validateInstrumentRequirement(
			requirements.Level1.Instruments,
			grouped[shared.ChoiceToolProficiency],
			requirements.ClassName,
		)...)
	}

	errors = append(errors, validateRequirement(
		requirements.Level1.Expertise,
		grouped[shared.ChoiceExpertise],
		"expertise",
		requirements.ClassName,
	)...)

	return errors
}

func validateRequirement(req *classes.ChoiceRequirement, choices []character.Choice,
	fieldName, className string) []Error {
	if req == nil || !req.Required {
		return nil
	}

	var errors []Error

	// Check if any choices provided
	if len(choices) == 0 {
		errors = append(errors, Error{
			Field:   fieldName,
			Message: fmt.Sprintf("%s requires %d %s selection(s)", className, req.Count, fieldName),
			Code:    rpgerr.CodeInvalidArgument,
		})
		return errors
	}

	// Validate count for each choice
	for _, choice := range choices {
		count := getChoiceCount(choice)
		if count != req.Count {
			errors = append(errors, Error{
				Field:   fieldName,
				Message: fmt.Sprintf("%s requires exactly %d %s, %d selected",
					className, req.Count, fieldName, count),
				Code:    rpgerr.CodeInvalidArgument,
			})
		}

		// Validate allowed values if specified
		if len(req.AllowedValues) > 0 && !req.AllowAny {
			if invalidValues := getInvalidValues(choice, req.AllowedValues); len(invalidValues) > 0 {
				errors = append(errors, Error{
					Field:   fieldName,
					Message: fmt.Sprintf("%s cannot choose: %v", className, invalidValues),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}
		}
	}

	return errors
}

func validateInstrumentRequirement(req *classes.ChoiceRequirement, choices []character.Choice,
	className string) []Error {
	if req == nil || !req.Required {
		return nil
	}

	var errors []Error

	if len(choices) == 0 {
		errors = append(errors, Error{
			Field:   "instruments",
			Message: fmt.Sprintf("%s requires %d musical instrument selection(s)", className, req.Count),
			Code:    rpgerr.CodeInvalidArgument,
		})
		return errors
	}

	for _, choice := range choices {
		var count int
		switch c := choice.(type) {
		case character.InstrumentProficiencyChoice:
			count = len(c.Instruments)
		case character.ToolProficiencyChoice:
			// For backward compatibility during migration
			count = len(c.Tools)
		}

		if count != req.Count {
			errors = append(errors, Error{
				Field:   "instruments",
				Message: fmt.Sprintf("%s requires exactly %d musical instruments, %d selected",
					className, req.Count, count),
				Code:    rpgerr.CodeInvalidArgument,
			})
		}
	}

	return errors
}

func getChoiceCount(choice character.Choice) int {
	switch c := choice.(type) {
	case character.SkillChoice:
		return len(c.Skills)
	case character.SpellChoice:
		return len(c.Spells)
	case character.CantripChoice:
		return len(c.Cantrips)
	case character.EquipmentChoice:
		// Equipment choices represent selecting ONE equipment pack
		if len(c.Equipment) > 0 {
			return 1
		}
		return 0
	case character.ExpertiseChoice:
		return len(c.Expertise)
	case character.LanguageChoice:
		return len(c.Languages)
	case character.TraitChoice:
		return len(c.Traits)
	case character.ToolProficiencyChoice:
		return len(c.Tools)
	case character.InstrumentProficiencyChoice:
		return len(c.Instruments)
	default:
		return 1 // Single selections like name, race, class
	}
}

func getInvalidValues(choice character.Choice, allowed []string) []string {
	allowedMap := make(map[string]bool)
	for _, v := range allowed {
		allowedMap[v] = true
	}

	var invalid []string

	if c, ok := choice.(character.SkillChoice); ok {
		for _, skill := range c.Skills {
			skillStr := string(skill)
			if !allowedMap[skillStr] {
				invalid = append(invalid, skillStr)
			}
		}
	}
	// Add more types as needed

	return invalid
}

// Warning detection

func detectCrossSourceDuplicates(choices []character.Choice) []Warning {
	var warnings []Warning
	
	// Collect skills from all sources
	skillsBySource := make(map[skills.Skill][]shared.ChoiceSource)
	languagesBySource := make(map[string][]shared.ChoiceSource)
	toolsBySource := make(map[string][]shared.ChoiceSource)
	instrumentsBySource := make(map[string][]shared.ChoiceSource)
	
	for _, choice := range choices {
		source := choice.GetSource()
		
		switch c := choice.(type) {
		case character.SkillChoice:
			for _, skill := range c.Skills {
				skillsBySource[skill] = append(skillsBySource[skill], source)
			}
		case character.LanguageChoice:
			for _, lang := range c.Languages {
				langStr := string(lang)
				languagesBySource[langStr] = append(languagesBySource[langStr], source)
			}
		case character.ToolProficiencyChoice:
			for _, tool := range c.Tools {
				toolsBySource[tool] = append(toolsBySource[tool], source)
			}
		case character.InstrumentProficiencyChoice:
			for _, instrument := range c.Instruments {
				instrumentsBySource[instrument] = append(instrumentsBySource[instrument], source)
			}
		}
	}
	
	// Check for duplicates
	for skill, sources := range skillsBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "skills",
				Message: fmt.Sprintf("Skill '%s' granted by multiple sources: %v", skill, sources),
				Code:    WarningDuplicateSkill,
			})
		}
	}
	
	for lang, sources := range languagesBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "languages",
				Message: fmt.Sprintf("Language '%s' granted by multiple sources: %v", lang, sources),
				Code:    WarningDuplicateLanguage,
			})
		}
	}
	
	for tool, sources := range toolsBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "tools",
				Message: fmt.Sprintf("Tool proficiency '%s' granted by multiple sources: %v", tool, sources),
				Code:    WarningDuplicateTool,
			})
		}
	}
	
	for instrument, sources := range instrumentsBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "instruments",
				Message: fmt.Sprintf("Instrument proficiency '%s' granted by multiple sources: %v", instrument, sources),
				Code:    WarningDuplicateInstrument,
			})
		}
	}
	
	return warnings
}

func validateExpertisePrerequisites(choices []character.Choice) []Warning {
	var warnings []Warning
	
	// Collect all skill proficiencies
	proficientSkills := make(map[string]bool)
	for _, choice := range choices {
		if sc, ok := choice.(character.SkillChoice); ok {
			for _, skill := range sc.Skills {
				proficientSkills[string(skill)] = true
			}
		}
	}
	
	// Collect tool proficiencies (for Rogue expertise in thieves' tools)
	proficientTools := make(map[string]bool)
	for _, choice := range choices {
		if tc, ok := choice.(character.ToolProficiencyChoice); ok {
			for _, tool := range tc.Tools {
				proficientTools[tool] = true
			}
		}
	}
	
	// Check expertise choices
	for _, choice := range choices {
		if ec, ok := choice.(character.ExpertiseChoice); ok {
			for _, expertise := range ec.Expertise {
				if !proficientSkills[expertise] && !proficientTools[expertise] {
					warnings = append(warnings, Warning{
						Field:   "expertise",
						Message: fmt.Sprintf("Expertise in '%s' requires proficiency from class, race, or background", expertise),
						Code:    WarningExpertiseWithoutProficiency,
					})
				}
			}
		}
	}
	
	return warnings
}

// Class-specific warnings

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

func validateRogueWarnings(choices []character.Choice) []Warning {
	var warnings []Warning
	
	// Check if Rogue has expertise in thieves' tools without proficiency
	hasThievesToolsProficiency := false
	hasThievesToolsExpertise := false
	
	for _, choice := range choices {
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
	
	// Note: Rogues automatically get thieves' tools
	if hasThievesToolsExpertise && !hasThievesToolsProficiency {
		warnings = append(warnings, Warning{
			Field:   "expertise",
			Message: "Expertise in thieves' tools selected, but proficiency not found (Rogues get this automatically)",
			Code:    WarningExpertiseWithoutProficiency,
		})
	}
	
	return warnings
}

// ValidateClassChoicesCompat provides backward compatibility for ChoiceData
// TODO: Remove this once all callers are updated to use Choice interface
func ValidateClassChoicesCompat(classID classes.Class, choicesData []character.ChoiceData) ([]Error, error) {
	// Convert ChoiceData to Choice interface
	var choices []character.Choice
	for _, cd := range choicesData {
		choice, err := character.ConvertFromChoiceData(cd)
		if err != nil {
			continue // Skip invalid choices
		}
		choices = append(choices, choice)
	}
	
	// Call the new validator
	result, err := ValidateClassChoices(classID, choices)
	if err != nil {
		return nil, err
	}
	
	// For now, only return errors (not warnings) for backward compatibility
	return result.Errors, nil
}
