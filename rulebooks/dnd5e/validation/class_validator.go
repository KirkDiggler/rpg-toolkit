// Package validation provides validation logic for D&D 5e character creation choices
package validation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Error represents a validation issue
type Error struct {
	Field   string
	Message string
	Code    rpgerr.Code
}

// validWizardSkills defines the skills available for wizard class selection
var validWizardSkills = map[skills.Skill]bool{
	skills.Arcana:        true,
	skills.History:       true,
	skills.Insight:       true,
	skills.Investigation: true,
	skills.Medicine:      true,
	skills.Religion:      true,
}

// getWizardSkillsList returns a formatted string of valid wizard skills
func getWizardSkillsList() string {
	skillNames := make([]string, 0, len(validWizardSkills))
	titleCaser := cases.Title(language.English)
	for skill := range validWizardSkills {
		// Capitalize using proper Unicode-aware casing
		name := string(skill)
		if len(name) > 0 {
			skillNames = append(skillNames, titleCaser.String(name))
		}
	}
	// Sort for consistent output
	sort.Strings(skillNames)
	return strings.Join(skillNames, ", ")
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
func validateWizardChoices(choices []character.ChoiceData) []Error {
	var errors []Error

	// Track what we've found
	hasSkills := false
	skillCount := 0
	hasCantrips := false
	cantripCount := 0
	hasSpells := false
	spellCount := 0
	foundEquipment := make(map[string]bool)

	// Wizard required equipment choices
	requiredEquipment := map[string]string{
		"wizard-equipment-primary-weapon": "weapon choice (quarterstaff or dagger)",
		"wizard-equipment-focus":          "arcane focus or component pouch",
		"wizard-equipment-pack":           "equipment pack",
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
					Message: fmt.Sprintf("Wizard requires 2 skill proficiencies, only %d selected", skillCount),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}
			// Validate that skills are from wizard list
			for _, skill := range choice.SkillSelection {
				if !validWizardSkills[skill] {
					errors = append(errors, Error{
						Field:   "skills",
						Message: fmt.Sprintf("Invalid wizard skill: %s. Must choose from %s", string(skill), getWizardSkillsList()),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}

		case shared.ChoiceCantrips:
			hasCantrips = true
			if choice.CantripSelection != nil {
				cantripCount = len(choice.CantripSelection)
			}
			if cantripCount < 3 {
				errors = append(errors, Error{
					Field:   "cantrips",
					Message: fmt.Sprintf("Wizard requires 3 cantrips at level 1, only %d selected", cantripCount),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}

		case shared.ChoiceSpells:
			hasSpells = true
			if choice.SpellSelection != nil {
				spellCount = len(choice.SpellSelection)
			}
			if spellCount < 6 {
				errors = append(errors, Error{
					Field:   "spells",
					Message: fmt.Sprintf("Wizard spellbook requires 6 spells at level 1, only %d selected", spellCount),
					Code:    rpgerr.CodeInvalidArgument,
				})
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
		missing = append(missing, "skill proficiencies (choose 2 from wizard list)")
	}

	if !hasCantrips {
		missing = append(missing, "cantrips (choose 3)")
	}

	if !hasSpells {
		missing = append(missing, "spells for spellbook (choose 6)")
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
