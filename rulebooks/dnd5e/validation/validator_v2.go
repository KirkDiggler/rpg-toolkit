package validation

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ValidateClassChoicesV2 validates choices using the new Choice interface
func ValidateClassChoicesV2(classID classes.Class, choices []character.Choice) ([]Error, error) {
	// Filter for class-sourced choices ONCE
	classChoices := filterChoicesBySource(choices, shared.SourceClass)

	// Route to appropriate validator
	switch classID {
	case classes.Fighter:
		return validateFighterChoicesV2(classChoices)
	case classes.Rogue:
		return validateRogueChoicesV2(classChoices)
	case classes.Wizard:
		return validateWizardChoicesV2(classChoices)
	case classes.Cleric:
		return validateClericChoicesV2(classChoices)
	case classes.Ranger:
		return validateRangerChoicesV2(classChoices)
	case classes.Paladin:
		return validatePaladinChoicesV2(classChoices)
	case classes.Barbarian:
		return validateBarbarianChoicesV2(classChoices)
	case classes.Bard:
		return validateBardChoicesV2(classChoices)
	case classes.Druid:
		return validateDruidChoicesV2(classChoices)
	case classes.Monk:
		return validateMonkChoicesV2(classChoices)
	case classes.Sorcerer:
		return validateSorcererChoicesV2(classChoices)
	case classes.Warlock:
		return validateWarlockChoicesV2(classChoices)
	default:
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, fmt.Sprintf("unsupported class: %s", classID))
	}
}

// filterChoicesBySource filters choices by their source
func filterChoicesBySource(choices []character.Choice, source shared.ChoiceSource) []character.Choice {
	var filtered []character.Choice
	for _, choice := range choices {
		if choice.GetSource() == source {
			filtered = append(filtered, choice)
		}
	}
	return filtered
}

// groupChoicesByCategory groups choices by their category for easier validation
func groupChoicesByCategory(choices []character.Choice) map[shared.ChoiceCategory][]character.Choice {
	grouped := make(map[shared.ChoiceCategory][]character.Choice)
	for _, choice := range choices {
		category := choice.GetCategory()
		grouped[category] = append(grouped[category], choice)
	}
	return grouped
}

// validateBardChoicesV2 validates Bard choices using the new Choice interface
func validateBardChoicesV2(choices []character.Choice) ([]Error, error) {
	// Use declarative validation with requirements!
	requirements := classes.GetRequirements(classes.Bard)
	if requirements == nil {
		return nil, rpgerr.New(rpgerr.CodeInternal, "Bard requirements not defined")
	}

	errors := ValidateWithRequirements(requirements, choices)
	return errors, nil
}

// validateFighterChoicesV2 validates Fighter choices using the new Choice interface
func validateFighterChoicesV2(choices []character.Choice) ([]Error, error) {
	// Use declarative validation
	requirements := classes.GetRequirements(classes.Fighter)
	if requirements == nil {
		return nil, rpgerr.New(rpgerr.CodeInternal, "Fighter requirements not defined")
	}

	errors := ValidateWithRequirements(requirements, choices)

	// Fighting style is handled separately as it's a class feature
	grouped := groupChoicesByCategory(choices)
	if _, ok := grouped[shared.ChoiceFightingStyle]; !ok {
		errors = append(errors, Error{
			Field:   "fighting_style",
			Message: "Fighter requires fighting style selection",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors, nil
}

// validateRogueChoicesV2 validates Rogue choices using the new Choice interface
func validateRogueChoicesV2(choices []character.Choice) ([]Error, error) {
	// Use declarative validation
	requirements := classes.GetRequirements(classes.Rogue)
	if requirements == nil {
		return nil, rpgerr.New(rpgerr.CodeInternal, "Rogue requirements not defined")
	}

	errors := ValidateWithRequirements(requirements, choices)
	return errors, nil
}

// Stub implementations for other classes - to be implemented
func validateWizardChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement wizard validation
	return nil, nil
}

func validateClericChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement cleric validation
	return nil, nil
}

func validateRangerChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement ranger validation
	return nil, nil
}

func validatePaladinChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement paladin validation
	return nil, nil
}

func validateBarbarianChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement barbarian validation
	return nil, nil
}

func validateDruidChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement druid validation
	return nil, nil
}

func validateMonkChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement monk validation
	return nil, nil
}

func validateSorcererChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement sorcerer validation
	return nil, nil
}

func validateWarlockChoicesV2(_ []character.Choice) ([]Error, error) {
	// TODO: Implement warlock validation
	return nil, nil
}
