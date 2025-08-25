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
	fieldCantrips         = "cantrips"
	fieldSkills           = "skills"

	// Choice types for race validation
	choiceTypeSkill    = "skill"
	choiceTypeLanguage = "language"
	choiceTypeCantrip  = "cantrip"
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

// raceChoiceRequirement defines what a race requires for a specific choice type
type raceChoiceRequirement struct {
	raceName      string
	requiredCount int
	field         string
	choiceType    string // "skill", "language", or "cantrip"
	// Custom messages for better error reporting
	missingMsg    string // Message when choice is missing
	wrongCountMsg string // Message format when count is wrong (use %d for actual count)
}

// validateRaceChoicesGeneric handles validation for races with language/skill/cantrip choices
func validateRaceChoicesGeneric(choices []character.ChoiceData, requirements []raceChoiceRequirement) []Error { //nolint:lll
	var errors []Error
	found := make(map[shared.ChoiceCategory]bool)

	for _, choice := range choices {
		// Only validate race-sourced choices
		if choice.Source != shared.SourceRace {
			continue
		}

		switch choice.Category {
		case shared.ChoiceSkills:
			found[choice.Category] = true
			for _, req := range requirements {
				if req.choiceType != choiceTypeSkill {
					continue
				}
				if len(choice.SkillSelection) == 0 {
					errors = append(errors, Error{
						Field:   req.field,
						Message: fmt.Sprintf("%s requires skill selection", req.raceName),
						Code:    rpgerr.CodeInvalidArgument,
					})
					break
				}
				if len(choice.SkillSelection) != req.requiredCount {
					msg := req.wrongCountMsg
					if msg == "" {
						msg = fmt.Sprintf("%s: %d skill(s) required, %%d selected", req.raceName, req.requiredCount)
					}
					errors = append(errors, Error{
						Field:   req.field,
						Message: fmt.Sprintf(msg, len(choice.SkillSelection)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}

		case shared.ChoiceLanguages:
			found[choice.Category] = true
			for _, req := range requirements {
				if req.choiceType != choiceTypeLanguage {
					continue
				}
				if len(choice.LanguageSelection) == 0 {
					errors = append(errors, Error{
						Field:   req.field,
						Message: fmt.Sprintf("%s requires language selection", req.raceName),
						Code:    rpgerr.CodeInvalidArgument,
					})
					break
				}
				if len(choice.LanguageSelection) != req.requiredCount {
					msg := req.wrongCountMsg
					if msg == "" {
						msg = fmt.Sprintf("%s: %d language(s) required, %%d selected", req.raceName, req.requiredCount)
					}
					errors = append(errors, Error{
						Field:   req.field,
						Message: fmt.Sprintf(msg, len(choice.LanguageSelection)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}

		case shared.ChoiceCantrips:
			found[choice.Category] = true
			for _, req := range requirements {
				if req.choiceType != choiceTypeCantrip {
					continue
				}
				if len(choice.CantripSelection) == 0 {
					errors = append(errors, Error{
						Field:   req.field,
						Message: fmt.Sprintf("%s requires cantrip selection", req.raceName),
						Code:    rpgerr.CodeInvalidArgument,
					})
					break
				}
				if len(choice.CantripSelection) != req.requiredCount {
					msg := req.wrongCountMsg
					if msg == "" {
						msg = fmt.Sprintf("%s: %d cantrip(s) required, %%d selected", req.raceName, req.requiredCount)
					}
					errors = append(errors, Error{
						Field:   req.field,
						Message: fmt.Sprintf(msg, len(choice.CantripSelection)),
						Code:    rpgerr.CodeInvalidArgument,
					})
				}
			}
		}
	}

	// Check for missing required choices
	for _, req := range requirements {
		var category shared.ChoiceCategory
		switch req.choiceType {
		case choiceTypeSkill:
			category = shared.ChoiceSkills
		case choiceTypeLanguage:
			category = shared.ChoiceLanguages
		case choiceTypeCantrip:
			category = shared.ChoiceCantrips
		}

		if !found[category] {
			msg := req.missingMsg
			if msg == "" {
				// Default messages if not provided
				switch req.choiceType {
				case choiceTypeSkill:
					msg = fmt.Sprintf("%s requires %d skill proficiency choice(s)", req.raceName, req.requiredCount)
				case choiceTypeLanguage:
					msg = fmt.Sprintf("%s requires %d additional language choice(s)", req.raceName, req.requiredCount)
				case choiceTypeCantrip:
					msg = fmt.Sprintf("%s requires %d cantrip choice(s)", req.raceName, req.requiredCount)
				}
			}
			errors = append(errors, Error{
				Field:   req.field,
				Message: msg,
				Code:    rpgerr.CodeInvalidArgument,
			})
		}
	}

	return errors
}

// validateHalfElfChoices validates Half-Elf specific choices
func validateHalfElfChoices(choices []character.ChoiceData) []Error {
	requirements := []raceChoiceRequirement{
		{
			raceName:      "Half-Elf",
			requiredCount: 2,
			field:         fieldRaceSkills,
			choiceType:    choiceTypeSkill,
			missingMsg:    "Half-Elf requires 2 skill proficiency choices",
			wrongCountMsg: "Half-Elf requires exactly 2 skill proficiencies, %d selected",
		},
		{
			raceName:      "Half-Elf",
			requiredCount: 1,
			field:         fieldLanguages,
			choiceType:    choiceTypeLanguage,
			missingMsg:    "Half-Elf requires 1 additional language choice",
			wrongCountMsg: "Half-Elf requires exactly 1 additional language, %d selected",
		},
	}
	return validateRaceChoicesGeneric(choices, requirements)
}

// validateHighElfChoices validates High Elf subrace specific choices
func validateHighElfChoices(choices []character.ChoiceData) []Error {
	requirements := []raceChoiceRequirement{
		{
			raceName:      "High Elf",
			requiredCount: 1,
			field:         fieldLanguages,
			choiceType:    choiceTypeLanguage,
			missingMsg:    "High Elf requires 1 additional language choice",
			wrongCountMsg: "High Elf requires exactly 1 additional language, %d selected",
		},
		{
			raceName:      "High Elf",
			requiredCount: 1,
			field:         fieldCantrips,
			choiceType:    choiceTypeCantrip,
			missingMsg:    "High Elf requires 1 wizard cantrip choice",
			wrongCountMsg: "High Elf requires exactly 1 wizard cantrip, %d selected",
		},
	}
	return validateRaceChoicesGeneric(choices, requirements)
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
