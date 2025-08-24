package validation

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
	var errors []Error
	grouped := groupChoicesByCategory(choices)

	// Check skills - Bard requires 3 skills (any)
	if skillChoices, ok := grouped[shared.ChoiceSkills]; !ok || len(skillChoices) == 0 {
		errors = append(errors, Error{
			Field:   fieldSkills,
			Message: "Bard requires 3 skill selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		// Type assert to SkillChoice
		for _, choice := range skillChoices {
			if sc, ok := choice.(character.SkillChoice); ok {
				if len(sc.Skills) != 3 {
					errors = append(errors, Error{
						Field:   fieldSkills,
						Message: fmt.Sprintf("Bard requires exactly 3 skills, %d selected", len(sc.Skills)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check cantrips - Bard requires 2
	if cantripChoices, ok := grouped[shared.ChoiceCantrips]; !ok || len(cantripChoices) == 0 {
		errors = append(errors, Error{
			Field:   fieldCantrips,
			Message: "Bard requires 2 cantrip selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		for _, choice := range cantripChoices {
			if cc, ok := choice.(character.CantripChoice); ok {
				if len(cc.Cantrips) != 2 {
					errors = append(errors, Error{
						Field:   fieldCantrips,
						Message: fmt.Sprintf("Bard requires exactly 2 cantrips, %d selected", len(cc.Cantrips)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check spells - Bard requires 4 level 1 spells
	if spellChoices, ok := grouped[shared.ChoiceSpells]; !ok || len(spellChoices) == 0 {
		errors = append(errors, Error{
			Field:   fieldSpells,
			Message: "Bard requires 4 spell selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		for _, choice := range spellChoices {
			if sc, ok := choice.(character.SpellChoice); ok {
				if len(sc.Spells) != 4 {
					errors = append(errors, Error{
						Field:   fieldSpells,
						Message: fmt.Sprintf("Bard requires exactly 4 spells, %d selected", len(sc.Spells)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check equipment
	if _, ok := grouped[shared.ChoiceEquipment]; !ok {
		errors = append(errors, Error{
			Field:   "equipment",
			Message: "Bard requires equipment selection",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	// Check musical instruments - Bard requires 3
	// For now, we're still using ToolProficiency category
	// TODO: Add separate InstrumentProficiency category
	if toolChoices, ok := grouped[shared.ChoiceToolProficiency]; !ok || len(toolChoices) == 0 {
		errors = append(errors, Error{
			Field:   "instruments",
			Message: "Bard requires 3 musical instrument selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		for _, choice := range toolChoices {
			// Check for both types since we might have either during migration
			var count int
			switch c := choice.(type) {
			case character.ToolProficiencyChoice:
				count = len(c.Tools)
			case character.InstrumentProficiencyChoice:
				count = len(c.Instruments)
			}
			if count != 3 {
				errors = append(errors, Error{
					Field:   "instruments",
					Message: fmt.Sprintf("Bard requires exactly 3 musical instruments, %d selected", count),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}
		}
	}

	// If no choices at all, provide helpful message
	if len(choices) == 0 {
		missingCategories := []string{"skills", "cantrips", "spells", "equipment", "instruments"}
		errors = append(errors, Error{
			Field:   fieldClassChoices,
			Message: fmt.Sprintf("Bard requires choices for: %v", missingCategories),
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors, nil
}

// validateFighterChoicesV2 validates Fighter choices using the new Choice interface
func validateFighterChoicesV2(choices []character.Choice) ([]Error, error) {
	var errors []Error
	grouped := groupChoicesByCategory(choices)

	// Fighter can choose 2 skills from their list
	fighterSkills := map[skills.Skill]bool{
		skills.Acrobatics:     true,
		skills.AnimalHandling: true,
		skills.Athletics:      true,
		skills.History:        true,
		skills.Insight:        true,
		skills.Intimidation:   true,
		skills.Perception:     true,
		skills.Survival:       true,
	}

	if skillChoices, ok := grouped[shared.ChoiceSkills]; !ok || len(skillChoices) == 0 {
		errors = append(errors, Error{
			Field:   fieldSkills,
			Message: "Fighter requires 2 skill selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		for _, choice := range skillChoices {
			if sc, ok := choice.(character.SkillChoice); ok {
				if len(sc.Skills) != 2 {
					errors = append(errors, Error{
						Field:   fieldSkills,
						Message: fmt.Sprintf("Fighter requires exactly 2 skills, %d selected", len(sc.Skills)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
				// Validate skills are from Fighter list
				for _, skill := range sc.Skills {
					if !fighterSkills[skill] {
						errors = append(errors, Error{
							Field:   fieldSkills,
							Message: fmt.Sprintf("Fighter cannot choose %s skill", skill),
							Code:    rpgerr.CodeInvalidArgument,
						})
					}
				}
			}
		}
	}

	// Check equipment
	if _, ok := grouped[shared.ChoiceEquipment]; !ok {
		errors = append(errors, Error{
			Field:   "equipment",
			Message: "Fighter requires equipment selection",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	// Check fighting style
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
	var errors []Error
	grouped := groupChoicesByCategory(choices)

	// Rogue can choose 4 skills from their list
	rogueSkills := map[skills.Skill]bool{
		skills.Acrobatics:     true,
		skills.Athletics:      true,
		skills.Deception:      true,
		skills.Insight:        true,
		skills.Intimidation:   true,
		skills.Investigation:  true,
		skills.Perception:     true,
		skills.Performance:    true,
		skills.Persuasion:     true,
		skills.SleightOfHand:  true,
		skills.Stealth:        true,
	}

	if skillChoices, ok := grouped[shared.ChoiceSkills]; !ok || len(skillChoices) == 0 {
		errors = append(errors, Error{
			Field:   fieldSkills,
			Message: "Rogue requires 4 skill selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		for _, choice := range skillChoices {
			if sc, ok := choice.(character.SkillChoice); ok {
				if len(sc.Skills) != 4 {
					errors = append(errors, Error{
						Field:   fieldSkills,
						Message: fmt.Sprintf("Rogue requires exactly 4 skills, %d selected", len(sc.Skills)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
				// Validate skills are from Rogue list
				for _, skill := range sc.Skills {
					if !rogueSkills[skill] {
						errors = append(errors, Error{
							Field:   fieldSkills,
							Message: fmt.Sprintf("Rogue cannot choose %s skill", skill),
							Code:    rpgerr.CodeInvalidArgument,
						})
					}
				}
			}
		}
	}

	// Check expertise - Rogue gets 2 at level 1
	if expertiseChoices, ok := grouped[shared.ChoiceExpertise]; !ok || len(expertiseChoices) == 0 {
		errors = append(errors, Error{
			Field:   fieldExpertise,
			Message: "Rogue requires 2 expertise selections",
			Code:    rpgerr.CodeInvalidArgument,
		})
	} else {
		for _, choice := range expertiseChoices {
			if ec, ok := choice.(character.ExpertiseChoice); ok {
				if len(ec.Expertise) != 2 {
					errors = append(errors, Error{
						Field:   fieldExpertise,
						Message: fmt.Sprintf("Rogue requires exactly 2 expertise, %d selected", len(ec.Expertise)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check equipment
	if _, ok := grouped[shared.ChoiceEquipment]; !ok {
		errors = append(errors, Error{
			Field:   "equipment",
			Message: "Rogue requires equipment selection",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors, nil
}

// Stub implementations for other classes - to be implemented
func validateWizardChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement wizard validation
	return nil, nil
}

func validateClericChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement cleric validation
	return nil, nil
}

func validateRangerChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement ranger validation
	return nil, nil
}

func validatePaladinChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement paladin validation
	return nil, nil
}

func validateBarbarianChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement barbarian validation
	return nil, nil
}

func validateDruidChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement druid validation
	return nil, nil
}

func validateMonkChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement monk validation
	return nil, nil
}

func validateSorcererChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement sorcerer validation
	return nil, nil
}

func validateWarlockChoicesV2(choices []character.Choice) ([]Error, error) {
	// TODO: Implement warlock validation
	return nil, nil
}