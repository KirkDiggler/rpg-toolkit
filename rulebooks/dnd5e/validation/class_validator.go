// Package validation provides validation logic for D&D 5e character creation choices
package validation

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Error represents a validation issue
type Error struct {
	Field   string
	Message string
	Code    rpgerr.Code
}

// ValidateClassChoices validates that all required choices for a class are satisfied
func ValidateClassChoices(classID classes.Class, choices []character.ChoiceData) ([]Error, error) {
	var errors []Error

	switch classID {
	case classes.Fighter:
		errors = validateFighterChoices(choices)
	case classes.Wizard:
		errors = validateWizardChoices(choices)
	// TODO: Add other classes
	default:
		// Unknown class, no validation yet
		return nil, nil
	}

	return errors, nil
}

// validateFighterChoices validates Fighter-specific requirements
func validateFighterChoices(choices []character.ChoiceData) []Error {
	var errors []Error

	// Track what we've found
	hasSkills := false
	skillCount := 0
	hasFightingStyle := false
	foundEquipment := make(map[string]bool)

	// Fighter required equipment choices
	requiredEquipment := map[string]string{
		"fighter-armor-choice":        "starting armor",
		"fighter-primary-weapon":      "primary martial weapon",
		"fighter-secondary-equipment": "shield or second weapon",
		"fighter-ranged-choice":       "ranged weapon",
		"fighter-pack-choice":         "equipment pack",
	}

	// Check each choice
	for _, choice := range choices {
		// Only validate class choices
		if choice.Source != shared.SourceClass {
			continue
		}

		switch choice.Category {
		case shared.ChoiceSkills:
			hasSkills = true
			skillCount = len(choice.SkillSelection)
			if skillCount < 2 {
				errors = append(errors, Error{
					Field:   "skills",
					Message: fmt.Sprintf("Fighter requires 2 skill proficiencies, only %d selected", skillCount),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}

		case shared.ChoiceFightingStyle:
			hasFightingStyle = true
			if choice.FightingStyleSelection == nil || *choice.FightingStyleSelection == "" {
				errors = append(errors, Error{
					Field:   "fighting_style",
					Message: "Fighter requires a fighting style selection",
					Code:    rpgerr.CodeInvalidArgument,
				})
			} else {
				// Validate it's a valid fighting style
				validStyles := map[string]bool{
					"archery":               true,
					"defense":               true,
					"dueling":               true,
					"great-weapon-fighting": true,
					"protection":            true,
					"two-weapon-fighting":   true,
				}
				if !validStyles[*choice.FightingStyleSelection] {
					errors = append(errors, Error{
						Field:   "fighting_style",
						Message: fmt.Sprintf("Invalid fighting style: %s", *choice.FightingStyleSelection),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}

		case shared.ChoiceEquipment:
			foundEquipment[choice.ChoiceID] = true
			if len(choice.EquipmentSelection) == 0 {
				if desc, ok := requiredEquipment[choice.ChoiceID]; ok {
					errors = append(errors, Error{
						Field:   choice.ChoiceID,
						Message: fmt.Sprintf("No selection made for %s", desc),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check for missing required choices
	var missing []string

	if !hasSkills {
		missing = append(missing, "skill proficiencies (choose 2)")
	}

	if !hasFightingStyle {
		missing = append(missing, "fighting style")
	}

	for choiceID, description := range requiredEquipment {
		if !foundEquipment[choiceID] {
			missing = append(missing, description)
		}
	}

	if len(missing) > 0 {
		errors = append(errors, Error{
			Field:   "class_choices",
			Message: fmt.Sprintf("Missing required choices: %s", strings.Join(missing, ", ")),
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors
}

// validateWizardChoices validates Wizard-specific requirements
func validateWizardChoices(_ []character.ChoiceData) []Error {
	var errors []Error

	// TODO: Implement wizard validation
	// Wizards need:
	// - Skills (2 from their list)
	// - Cantrips (3 at level 1)
	// - Spells (6 at level 1)
	// - Equipment choices

	return errors
}
