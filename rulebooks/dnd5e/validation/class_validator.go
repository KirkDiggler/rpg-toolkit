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

const (
	fieldSkills    = "skills"
	fieldCantrips  = "cantrips"
	fieldExpertise = "expertise"
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

// validClericSkills defines the skills available for cleric class selection
var validClericSkills = map[skills.Skill]bool{
	skills.History:    true,
	skills.Insight:    true,
	skills.Medicine:   true,
	skills.Persuasion: true,
	skills.Religion:   true,
}

// validDruidSkills defines the skills available for druid class selection
var validDruidSkills = map[skills.Skill]bool{
	skills.Arcana:         true,
	skills.AnimalHandling: true,
	skills.Insight:        true,
	skills.Medicine:       true,
	skills.Nature:         true,
	skills.Perception:     true,
	skills.Religion:       true,
	skills.Survival:       true,
}

// validRogueSkills defines the skills available for rogue class selection
var validRogueSkills = map[skills.Skill]bool{
	skills.Acrobatics:    true,
	skills.Athletics:     true,
	skills.Deception:     true,
	skills.Insight:       true,
	skills.Intimidation:  true,
	skills.Investigation: true,
	skills.Perception:    true,
	skills.Performance:   true,
	skills.Persuasion:    true,
	skills.SleightOfHand: true,
	skills.Stealth:       true,
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

// preparedCasterValidationConfig holds configuration for validating prepared spellcaster classes
type preparedCasterValidationConfig struct {
	className        string
	validSkills      map[skills.Skill]bool
	requiredSkills   int
	requiredCantrips int
	equipmentChoices map[string]string
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

// validateSkillChoice validates skill selection for a class
func validateSkillChoice(choice character.ChoiceData, className string,
	validSkills map[skills.Skill]bool, requiredSkills int) []Error {
	var errors []Error

	skillCount := len(choice.SkillSelection)
	if skillCount < requiredSkills {
		errors = append(errors, Error{
			Field: fieldSkills,
			Message: fmt.Sprintf("%s requires %d skill proficiencies, only %d selected",
				className, requiredSkills, skillCount),
			Code: rpgerr.CodeInvalidArgument,
		})
	}

	// Validate that skills are from class list
	for _, skill := range choice.SkillSelection {
		if !validSkills[skill] {
			errors = append(errors, Error{
				Field: fieldSkills,
				Message: fmt.Sprintf("Invalid %s skill: %s. Must choose from %s",
					strings.ToLower(className), string(skill), getSkillsList(validSkills)),
				Code: rpgerr.CodeInvalidArgument,
			})
		}
	}

	return errors
}

// validateCantripChoice validates cantrip selection for a class
func validateCantripChoice(choice character.ChoiceData, className string, requiredCantrips int) []Error {
	var errors []Error

	cantripCount := 0
	if choice.CantripSelection != nil {
		cantripCount = len(choice.CantripSelection)
	}

	if cantripCount < requiredCantrips {
		errors = append(errors, Error{
			Field: fieldCantrips,
			Message: fmt.Sprintf("%s requires %d cantrips at level 1, only %d selected",
				className, requiredCantrips, cantripCount),
			Code: rpgerr.CodeInvalidArgument,
		})
	}

	return errors
}

// validateEquipmentChoice validates equipment selection
func validateEquipmentChoice(choice character.ChoiceData, equipmentChoices map[string]string) []Error {
	var errors []Error

	if len(choice.EquipmentSelection) == 0 {
		if desc, ok := equipmentChoices[choice.ChoiceID]; ok {
			errors = append(errors, Error{
				Field:   choice.ChoiceID,
				Message: fmt.Sprintf("No selection made for %s", desc),
				Code:    rpgerr.CodeInvalidArgument,
			})
		}
	}

	return errors
}

// validateSpellcasterChoices provides common validation logic for spellcaster classes
func validateSpellcasterChoices(config spellcasterValidationConfig, choices []character.ChoiceData) []Error {
	var errors []Error

	// Track what we've found
	hasSkills := false
	hasCantrips := false
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
			errors = append(errors, validateSkillChoice(choice, config.className, config.validSkills, config.requiredSkills)...)

		case shared.ChoiceCantrips:
			hasCantrips = true
			errors = append(errors, validateCantripChoice(choice, config.className, config.requiredCantrips)...)

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
			errors = append(errors, validateEquipmentChoice(choice, config.equipmentChoices)...)
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

// validatePreparedCasterChoices provides validation logic for prepared spellcaster classes
func validatePreparedCasterChoices(config preparedCasterValidationConfig, choices []character.ChoiceData) []Error {
	var errors []Error

	// Track what we've found
	hasSkills := false
	hasCantrips := false
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
			errors = append(errors, validateSkillChoice(choice, config.className, config.validSkills, config.requiredSkills)...)

		case shared.ChoiceCantrips:
			hasCantrips = true
			errors = append(errors, validateCantripChoice(choice, config.className, config.requiredCantrips)...)

		case shared.ChoiceEquipment:
			foundEquipment[choice.ChoiceID] = true
			errors = append(errors, validateEquipmentChoice(choice, config.equipmentChoices)...)
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
	case classes.Cleric:
		errors = validateClericChoices(choices)
	case classes.Druid:
		errors = validateDruidChoices(choices)
	case classes.Rogue:
		errors = validateRogueChoices(choices)
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

// validateClericChoices validates Cleric-specific requirements
func validateClericChoices(choices []character.ChoiceData) []Error {
	return validatePreparedCasterChoices(preparedCasterValidationConfig{
		className:        "Cleric",
		validSkills:      validClericSkills,
		requiredSkills:   2,
		requiredCantrips: 3,
		equipmentChoices: map[string]string{
			"cleric-equipment-primary-weapon": "weapon choice (mace or warhammer)",
			"cleric-equipment-armor":          "armor choice (scale mail, leather, or chain mail)",
			"cleric-equipment-ranged":         "ranged weapon choice (light crossbow or simple weapon)",
			"cleric-equipment-pack":           "pack choice (priest's pack or explorer's pack)",
			"cleric-equipment-holy-symbol":    "holy symbol",
		},
	}, choices)
}

// validateDruidChoices validates Druid-specific requirements
func validateDruidChoices(choices []character.ChoiceData) []Error {
	return validatePreparedCasterChoices(preparedCasterValidationConfig{
		className:        "Druid",
		validSkills:      validDruidSkills,
		requiredSkills:   2,
		requiredCantrips: 2,
		equipmentChoices: map[string]string{
			"druid-equipment-shield-weapon": "shield or simple weapon choice",
			"druid-equipment-melee":         "scimitar or simple melee weapon choice",
			"druid-equipment-focus":         "druidic focus",
		},
	}, choices)
}

// validateRogueChoices validates Rogue-specific requirements
func validateRogueChoices(choices []character.ChoiceData) []Error {
	var errors []Error

	// Track what we've found
	hasSkills := false
	hasExpertise := false
	expertiseCount := 0
	foundEquipment := make(map[string]bool)

	// Rogue required equipment choices
	requiredEquipment := map[string]string{
		"rogue-equipment-primary-weapon": "primary weapon (rapier or shortsword)",
		"rogue-equipment-secondary":      "secondary weapon (shortbow or shortsword)",
		"rogue-equipment-pack":           "equipment pack",
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
			errors = append(errors, validateSkillChoice(choice, "Rogue", validRogueSkills, 4)...)

		case shared.ChoiceExpertise:
			hasExpertise = true
			// Rogue can choose 2 skills OR 1 skill + thieves' tools
			if choice.ExpertiseSelection != nil {
				expertiseCount = len(choice.ExpertiseSelection)
			}
			if expertiseCount < 2 {
				errors = append(errors, Error{
					Field: fieldExpertise,
					Message: fmt.Sprintf("Rogue requires 2 expertise choices at level 1, only %d selected",
						expertiseCount),
					Code: rpgerr.CodeInvalidArgument,
				})
			}

		case shared.ChoiceEquipment:
			foundEquipment[choice.ChoiceID] = true
			errors = append(errors, validateEquipmentChoice(choice, requiredEquipment)...)
		}
	}

	// Check for missing required choices
	var missing []string

	if !hasSkills {
		missing = append(missing, "skill proficiencies (choose 4 from rogue list)")
	}

	if !hasExpertise {
		missing = append(missing, "expertise (choose 2 skills or 1 skill + thieves' tools)")
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
