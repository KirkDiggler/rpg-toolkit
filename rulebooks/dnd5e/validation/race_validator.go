// Package validation provides D&D 5e character validation functionality
package validation

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	fieldLanguages        = "languages"
	fieldTraits           = "traits"
	fieldRaceSkills       = "race_skills"
	fieldDraconicAncestry = "draconic_ancestry"
)

// ValidateRaceChoices validates that all required racial choices are satisfied
func ValidateRaceChoices(raceID races.Race, subraceID races.Subrace, choices []character.ChoiceData) ([]Error, error) {
	var errors []Error

	// First validate the race/subrace combination
	if raceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "race ID is required")
	}

	// Check if subrace is required
	if requiresSubrace(raceID) && subraceID == "" {
		errors = append(errors, Error{
			Field:   "subrace",
			Message: fmt.Sprintf("%s requires a subrace selection", raceID),
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	// Validate subrace matches parent race if provided
	if subraceID != "" && !isValidSubrace(raceID, subraceID) {
		errors = append(errors, Error{
			Field:   "subrace",
			Message: fmt.Sprintf("%s is not a valid subrace for %s", subraceID, raceID),
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	// Now validate race-specific choices
	switch raceID {
	case races.HalfElf:
		errors = append(errors, validateHalfElfChoices(choices)...)
	case races.Human:
		// Variant Human has skill and feat choices, but we're focusing on standard Human
		// Standard Human has no choices
	case races.Dragonborn:
		errors = append(errors, validateDragonbornChoices(choices)...)
	case races.Elf:
		if subraceID == races.HighElf {
			errors = append(errors, validateHighElfChoices(choices)...)
		}
		// Other elf subraces don't have additional choices
	case races.Dwarf:
		// Dwarves have no racial choices
	case races.Halfling:
		// Halflings have no racial choices
	case races.Gnome:
		// Gnomes have no racial choices
	case races.HalfOrc:
		// Half-Orcs have no racial choices
	case races.Tiefling:
		// Tieflings have no racial choices
	}

	return errors, nil
}

// requiresSubrace returns true if the race requires a subrace selection
func requiresSubrace(raceID races.Race) bool {
	switch raceID {
	case races.Elf, races.Dwarf, races.Halfling, races.Gnome:
		return true
	default:
		return false
	}
}

// isValidSubrace checks if a subrace is valid for the given parent race
func isValidSubrace(raceID races.Race, subraceID races.Subrace) bool {
	switch raceID {
	case races.Elf:
		return subraceID == races.HighElf || subraceID == races.WoodElf || subraceID == races.DarkElf
	case races.Dwarf:
		return subraceID == races.MountainDwarf || subraceID == races.HillDwarf
	case races.Halfling:
		return subraceID == races.LightfootHalfling || subraceID == races.StoutHalfling
	case races.Gnome:
		return subraceID == races.ForestGnome || subraceID == races.RockGnome
	default:
		return false
	}
}

// validateHalfElfChoices validates Half-Elf specific choices
func validateHalfElfChoices(choices []character.ChoiceData) []Error {
	var errors []Error
	foundSkillChoice := false
	foundLanguageChoice := false

	for _, choice := range choices {
		// Only validate race-sourced choices
		if choice.Source != shared.SourceRace {
			continue
		}

		switch choice.Category {
		case shared.ChoiceSkills:
			foundSkillChoice = true
			if len(choice.SkillSelection) == 0 {
				errors = append(errors, Error{
					Field:   fieldRaceSkills,
					Message: "Half-Elf requires skill selection",
					Code:    rpgerr.CodeInvalidArgument,
				})
				continue
			}

			// Half-Elf can choose ANY 2 skills
			if len(choice.SkillSelection) != 2 {
				errors = append(errors, Error{
					Field:   fieldRaceSkills,
					Message: fmt.Sprintf("Half-Elf requires exactly 2 skill proficiencies, %d selected", len(choice.SkillSelection)),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}
			// Half-Elf can choose from ANY skills, so no validation needed for specific skills

		case shared.ChoiceLanguages:
			foundLanguageChoice = true
			if len(choice.LanguageSelection) == 0 {
				errors = append(errors, Error{
					Field:   fieldLanguages,
					Message: "Half-Elf requires language selection",
					Code:    rpgerr.CodeInvalidArgument,
				})
				continue
			}

			// Half-Elf chooses 1 additional language
			if len(choice.LanguageSelection) != 1 {
				errors = append(errors, Error{
					Field: fieldLanguages,
					Message: fmt.Sprintf("Half-Elf requires exactly 1 additional language, %d selected",
						len(choice.LanguageSelection)),
					Code: rpgerr.CodeInvalidArgument,
				})
			}
		}
	}

	// Check for required choices
	if !foundSkillChoice {
		errors = append(errors, Error{
			Field:   fieldRaceSkills,
			Message: "Half-Elf requires 2 skill proficiency choices",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	if !foundLanguageChoice {
		errors = append(errors, Error{
			Field:   fieldLanguages,
			Message: "Half-Elf requires 1 additional language choice",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors
}

// validateHighElfChoices validates High Elf subrace specific choices
func validateHighElfChoices(choices []character.ChoiceData) []Error {
	var errors []Error
	foundLanguageChoice := false
	foundCantripChoice := false

	for _, choice := range choices {
		// Only validate race-sourced choices
		if choice.Source != shared.SourceRace {
			continue
		}

		switch choice.Category {
		case shared.ChoiceLanguages:
			foundLanguageChoice = true
			if len(choice.LanguageSelection) == 0 {
				errors = append(errors, Error{
					Field:   fieldLanguages,
					Message: "High Elf requires language selection",
					Code:    rpgerr.CodeInvalidArgument,
				})
				continue
			}

			// High Elf chooses 1 additional language
			if len(choice.LanguageSelection) != 1 {
				errors = append(errors, Error{
					Field: fieldLanguages,
					Message: fmt.Sprintf("High Elf requires exactly 1 additional language, %d selected",
						len(choice.LanguageSelection)),
					Code: rpgerr.CodeInvalidArgument,
				})
			}

		case shared.ChoiceCantrips:
			foundCantripChoice = true
			if len(choice.CantripSelection) == 0 {
				errors = append(errors, Error{
					Field:   fieldCantrips,
					Message: "High Elf requires cantrip selection",
					Code:    rpgerr.CodeInvalidArgument,
				})
				continue
			}

			// High Elf gets 1 wizard cantrip
			if len(choice.CantripSelection) != 1 {
				errors = append(errors, Error{
					Field:   fieldCantrips,
					Message: fmt.Sprintf("High Elf requires exactly 1 wizard cantrip, %d selected", len(choice.CantripSelection)),
					Code:    rpgerr.CodeInvalidArgument,
				})
			}
		}
	}

	// Check for required choices
	if !foundLanguageChoice {
		errors = append(errors, Error{
			Field:   fieldLanguages,
			Message: "High Elf requires 1 additional language choice",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	if !foundCantripChoice {
		errors = append(errors, Error{
			Field:   fieldCantrips,
			Message: "High Elf requires 1 wizard cantrip choice",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors
}

// validateDragonbornChoices validates Dragonborn specific choices
func validateDragonbornChoices(choices []character.ChoiceData) []Error {
	var errors []Error
	foundAncestryChoice := false

	// Valid draconic ancestry options
	validAncestries := map[string]bool{
		"black":  true,
		"blue":   true,
		"brass":  true,
		"bronze": true,
		"copper": true,
		"gold":   true,
		"green":  true,
		"red":    true,
		"silver": true,
		"white":  true,
	}

	for _, choice := range choices {
		// Only validate race-sourced choices
		if choice.Source != shared.SourceRace {
			continue
		}

		if choice.Category == shared.ChoiceTraits {
			foundAncestryChoice = true
			if len(choice.TraitSelection) == 0 {
				errors = append(errors, Error{
					Field:   fieldDraconicAncestry,
					Message: "Dragonborn requires draconic ancestry selection",
					Code:    rpgerr.CodeInvalidArgument,
				})
				continue
			}

			// Dragonborn chooses 1 draconic ancestry
			if len(choice.TraitSelection) != 1 {
				errors = append(errors, Error{
					Field:   fieldDraconicAncestry,
					Message: fmt.Sprintf("Dragonborn requires exactly 1 draconic ancestry, %d selected", len(choice.TraitSelection)),
					Code:    rpgerr.CodeInvalidArgument,
				})
			} else {
				// Validate the selected ancestry
				selected := choice.TraitSelection[0]
				// Extract color from trait ID like "draconic-ancestry-red"
				parts := strings.Split(selected, "-")
				if len(parts) >= 3 && parts[0] == "draconic" && parts[1] == "ancestry" {
					color := parts[2]
					if !validAncestries[color] {
						errors = append(errors, Error{
							Field:   fieldDraconicAncestry,
							Message: fmt.Sprintf("Invalid draconic ancestry: %s", color),
							Code:    rpgerr.CodeInvalidArgument,
						})
					}
				} else {
					errors = append(errors, Error{
						Field:   fieldDraconicAncestry,
						Message: fmt.Sprintf("Invalid draconic ancestry format: %s", selected),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check for required choice
	if !foundAncestryChoice {
		errors = append(errors, Error{
			Field:   fieldDraconicAncestry,
			Message: "Dragonborn requires draconic ancestry choice",
			Code:    rpgerr.CodeInvalidArgument,
		})
	}

	return errors
}
