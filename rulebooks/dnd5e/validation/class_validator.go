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

// validSorcererSkills defines the skills available for sorcerer class selection
var validSorcererSkills = map[skills.Skill]bool{
	skills.Arcana:       true,
	skills.Deception:    true,
	skills.Insight:      true,
	skills.Intimidation: true,
	skills.Persuasion:   true,
	skills.Religion:     true,
}

// validWarlockSkills defines the skills available for warlock class selection
var validWarlockSkills = map[skills.Skill]bool{
	skills.Arcana:        true,
	skills.Deception:     true,
	skills.History:       true,
	skills.Intimidation:  true,
	skills.Investigation: true,
	skills.Nature:        true,
	skills.Religion:      true,
}

// spellcasterValidationConfig holds configuration for validating spellcaster classes
type spellcasterValidationConfig struct {
	className         string
	validSkills       map[skills.Skill]bool
	requiredSkills    int
	requiredCantrips  int
	requiredSpells    int
	spellsDescription string // e.g., "spells for spellbook" or "spells known"
	equipmentChoices  map[string]string
}

// getSkillsList returns a formatted string of valid skills from the provided map
func getSkillsList(validSkills map[skills.Skill]bool) string {
	skillNames := make([]string, 0, len(validSkills))
	titleCaser := cases.Title(language.English)
	for skill := range validSkills {
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

// validateSpellcasterChoices provides common validation logic for spellcaster classes
func validateSpellcasterChoices(config spellcasterValidationConfig, choices []character.ChoiceData) []Error {
	var errors []Error

	// Track what we've found
	hasSkills := false
	skillCount := 0
	hasCantrips := false
	cantripCount := 0
	hasSpells := false
	spellCount := 0
	foundEquipment := make(map[string]bool)

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
			if skillCount < config.requiredSkills {
				errors = append(errors, Error{
					Field: "skills",
					Message: fmt.Sprintf("%s requires %d skill proficiencies, only %d selected",
						config.className, config.requiredSkills, skillCount),
					Code: rpgerr.CodeInvalidArgument,
				})
			}
			// Validate that skills are from class list
			for _, skill := range choice.SkillSelection {
				if !config.validSkills[skill] {
					errors = append(errors, Error{
						Field: "skills",
						Message: fmt.Sprintf("Invalid %s skill: %s. Must choose from %s",
							strings.ToLower(config.className), string(skill), getSkillsList(config.validSkills)),
						Code: rpgerr.CodeInvalidArgument,
					})
				}
			}

		case shared.ChoiceCantrips:
			hasCantrips = true
			if choice.CantripSelection != nil {
				cantripCount = len(choice.CantripSelection)
			}
			if cantripCount < config.requiredCantrips {
				errors = append(errors, Error{
					Field: "cantrips",
					Message: fmt.Sprintf("%s requires %d cantrips at level 1, only %d selected",
						config.className, config.requiredCantrips, cantripCount),
					Code: rpgerr.CodeInvalidArgument,
				})
			}

		case shared.ChoiceSpells:
			hasSpells = true
			if choice.SpellSelection != nil {
				spellCount = len(choice.SpellSelection)
			}
			if spellCount < config.requiredSpells {
				errors = append(errors, Error{
					Field:   "spells",
					Message: fmt.Sprintf("%s %s, only %d selected", config.className, config.spellsDescription, spellCount),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}

		case shared.ChoiceEquipment:
			foundEquipment[choice.ChoiceID] = true
			if len(choice.EquipmentSelection) == 0 {
				if desc, ok := config.equipmentChoices[choice.ChoiceID]; ok {
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
		missing = append(missing, fmt.Sprintf("skill proficiencies (choose %d from %s list)",
			config.requiredSkills, strings.ToLower(config.className)))
	}

	if !hasCantrips {
		missing = append(missing, fmt.Sprintf("cantrips (choose %d)", config.requiredCantrips))
	}

	if !hasSpells {
		missing = append(missing, config.spellsDescription)
	}

	for choiceID, description := range config.equipmentChoices {
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

// ValidateClassChoices validates that all required choices for a class are satisfied
func ValidateClassChoices(classID classes.Class, choices []character.ChoiceData) ([]Error, error) {
	var errors []Error

	switch classID {
	case classes.Fighter:
		errors = validateFighterChoices(choices)
	case classes.Wizard:
		errors = validateWizardChoices(choices)
	case classes.Sorcerer:
		errors = validateSorcererChoices(choices)
	case classes.Warlock:
		errors = validateWarlockChoices(choices)
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
	config := spellcasterValidationConfig{
		className:         "Wizard",
		validSkills:       validWizardSkills,
		requiredSkills:    2,
		requiredCantrips:  3,
		requiredSpells:    6,
		spellsDescription: "spells for spellbook (choose 6)",
		equipmentChoices: map[string]string{
			"wizard-equipment-primary-weapon": "weapon choice (quarterstaff or dagger)",
			"wizard-equipment-focus":          "arcane focus or component pouch",
			"wizard-equipment-pack":           "equipment pack",
		},
	}
	return validateSpellcasterChoices(config, choices)
}

// validateSorcererChoices validates Sorcerer-specific requirements
func validateSorcererChoices(choices []character.ChoiceData) []Error {
	config := spellcasterValidationConfig{
		className:         "Sorcerer",
		validSkills:       validSorcererSkills,
		requiredSkills:    2,
		requiredCantrips:  4,
		requiredSpells:    2,
		spellsDescription: "spells known (choose 2)",
		equipmentChoices: map[string]string{
			"sorcerer-equipment-primary-weapon": "weapon choice (light crossbow or simple weapon)",
			"sorcerer-equipment-focus":          "arcane focus or component pouch",
			"sorcerer-equipment-pack":           "equipment pack",
		},
	}
	return validateSpellcasterChoices(config, choices)
}

// validateWarlockChoices validates Warlock-specific requirements
func validateWarlockChoices(choices []character.ChoiceData) []Error {
	config := spellcasterValidationConfig{
		className:         "Warlock",
		validSkills:       validWarlockSkills,
		requiredSkills:    2,
		requiredCantrips:  2,
		requiredSpells:    2,
		spellsDescription: "spells known (choose 2)",
		equipmentChoices: map[string]string{
			"warlock-equipment-primary-weapon": "weapon choice (light crossbow or simple weapon)",
			"warlock-equipment-focus":          "arcane focus or component pouch",
			"warlock-equipment-pack":           "equipment pack",
		},
	}
	return validateSpellcasterChoices(config, choices)
}
