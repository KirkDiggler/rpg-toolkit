package validation

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ValidateWithRequirements validates choices against declarative requirements
func ValidateWithRequirements(requirements *classes.ClassRequirements, choices []character.Choice) []Error {
	if requirements == nil {
		return []Error{{
			Field:   "requirements",
			Message: "No requirements defined for this class",
			Code:    rpgerr.CodeInternal,
		}}
	}

	var errors []Error

	// Group choices by category for easier validation
	grouped := groupChoicesByCategory(choices)

	// Validate each requirement - this is now data-driven!
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

// validateRequirement validates a single requirement against provided choices
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

	// Validate count for each choice (there should usually be just one)
	for _, choice := range choices {
		count := getChoiceCount(choice)
		if count != req.Count {
			errors = append(errors, Error{
				Field: fieldName,
				Message: fmt.Sprintf("%s requires exactly %d %s, %d selected",
					className, req.Count, fieldName, count),
				Code: rpgerr.CodeInvalidArgument,
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

// validateInstrumentRequirement handles the special case of musical instruments
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
			// For backward compatibility
			count = len(c.Tools)
		}

		if count != req.Count {
			errors = append(errors, Error{
				Field: "instruments",
				Message: fmt.Sprintf("%s requires exactly %d musical instruments, %d selected",
					className, req.Count, count),
				Code: rpgerr.CodeInvalidArgument,
			})
		}
	}

	return errors
}

// getChoiceCount returns the number of items in a choice
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
		// The Equipment array contains the items IN that pack
		// So if Equipment has items, count it as 1 choice made
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

// getInvalidValues returns values that aren't in the allowed list
func getInvalidValues(choice character.Choice, allowed []string) []string {
	allowedMap := make(map[string]bool)
	for _, v := range allowed {
		allowedMap[v] = true
	}

	var invalid []string

	if c, ok := choice.(character.SkillChoice); ok {
		for _, skill := range c.Skills {
			// Convert skill to string for comparison
			skillStr := string(skill)
			if !allowedMap[skillStr] {
				invalid = append(invalid, skillStr)
			}
		}
	}
	// Add more types as needed

	return invalid
}
