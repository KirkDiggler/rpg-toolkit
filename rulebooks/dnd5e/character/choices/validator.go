package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// validateClassChoicesInternal validates choices for a specific class
func validateClassChoicesInternal(classID classes.Class, level int, submissions Submissions) *ValidationResult {
	reqs := getClassRequirementsInternal(classID, level)
	if reqs == nil {
		return &ValidationResult{Valid: true}
	}

	result := &ValidationResult{Valid: true}

	// Validate skills
	if reqs.Skills != nil {
		validateSkills(reqs.Skills, submissions["skills"], result)
	}

	// Validate fighting style
	if reqs.FightingStyle != nil {
		validateFightingStyle(reqs.FightingStyle, submissions["fighting_style"], result)
	}

	// Validate equipment choices
	if len(reqs.Equipment) > 0 {
		validateEquipment(reqs.Equipment, submissions, result)
	}

	// Validate cantrips
	if reqs.Cantrips != nil {
		validateSpells(reqs.Cantrips, submissions["cantrips"], result)
	}

	// Validate spells
	if reqs.Spells != nil {
		validateSpells(reqs.Spells, submissions["spells"], result)
	}

	// Validate expertise
	if reqs.Expertise != nil {
		validateExpertise(reqs.Expertise, submissions["expertise"], result)
	}

	// Validate instruments
	if reqs.Instruments != nil {
		validateInstruments(reqs.Instruments, submissions["instruments"], result)
	}

	return result
}

// validateRaceChoicesInternal validates choices for a specific race
func validateRaceChoicesInternal(raceID races.Race, submissions Submissions) *ValidationResult {
	reqs := getRaceRequirementsInternal(raceID)
	if reqs == nil {
		return &ValidationResult{Valid: true}
	}

	result := &ValidationResult{Valid: true}

	// Validate skills (e.g., Half-Elf)
	if reqs.Skills != nil {
		validateSkills(reqs.Skills, submissions["race_skills"], result)
	}

	// Validate languages (e.g., Half-Elf)
	if reqs.Languages != nil {
		validateLanguages(reqs.Languages, submissions["race_languages"], result)
	}

	// Validate draconic ancestry (Dragonborn)
	if reqs.DraconicAncestry != nil {
		validateAncestry(reqs.DraconicAncestry, submissions["draconic_ancestry"], result)
	}

	return result
}

// validateBackgroundChoicesInternal validates choices for a specific background
func validateBackgroundChoicesInternal(backgroundID backgrounds.Background, _ Submissions) *ValidationResult {
	reqs := getBackgroundRequirementsInternal(backgroundID)
	if reqs == nil {
		return &ValidationResult{Valid: true}
	}

	// Most backgrounds have no choices
	// TODO: Implement when we add backgrounds with choices

	return &ValidationResult{Valid: true}
}

// validateAllInternal validates all choices for a character
func validateAllInternal(
	classID classes.Class, raceID races.Race, level int, submissions Submissions,
) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate class choices
	classResult := validateClassChoicesInternal(classID, level, submissions)
	if !classResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, classResult.Errors...)
	}
	result.Warnings = append(result.Warnings, classResult.Warnings...)

	// Validate race choices
	raceResult := validateRaceChoicesInternal(raceID, submissions)
	if !raceResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, raceResult.Errors...)
	}
	result.Warnings = append(result.Warnings, raceResult.Warnings...)

	// Check for cross-source duplicates
	checkCrossSourceDuplicates(classID, raceID, submissions, result)

	return result
}

// Helper validation functions

func validateSkills(req *SkillRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "skills",
			Message: fmt.Sprintf("Must choose exactly %d skills, got %d", req.Count, len(chosen)),
		})
		return
	}

	// If specific options are required, validate against them
	if req.Options != nil {
		validOptions := make(map[string]bool)
		for _, opt := range req.Options {
			validOptions[string(opt)] = true
		}

		for _, skill := range chosen {
			if !validOptions[skill] {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "skills",
					Message: fmt.Sprintf("Invalid skill choice: %s", skill),
				})
			}
		}
	} else {
		// Validate against all valid skills
		for _, skill := range chosen {
			if _, ok := skills.All[skill]; !ok {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "skills",
					Message: fmt.Sprintf("Invalid skill: %s", skill),
				})
			}
		}
	}

	// Check for duplicates within the same choice set
	seen := make(map[string]bool)
	for _, skill := range chosen {
		if seen[skill] {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "skills",
				Message: fmt.Sprintf("Duplicate skill selected: %s", skill),
				Type:    "duplicate_skill",
			})
		}
		seen[skill] = true
	}
}

func validateFightingStyle(req *FightingStyleRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != 1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "fighting_style",
			Message: "Must choose exactly 1 fighting style",
		})
		return
	}

	validStyles := make(map[string]bool)
	for _, style := range req.Options {
		validStyles[string(style)] = true
	}

	if !validStyles[chosen[0]] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "fighting_style",
			Message: fmt.Sprintf("Invalid fighting style: %s", chosen[0]),
		})
	}
}

func validateEquipment(reqs []*EquipmentRequirement, submissions Submissions, result *ValidationResult) {
	for i, req := range reqs {
		fieldName := fmt.Sprintf("equipment_%d", i)
		chosen := submissions[fieldName]

		if len(chosen) != req.Choose {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("%s: must choose %d option(s)", req.Label, req.Choose),
			})
			continue
		}

		// Validate that chosen IDs are valid options
		validOptions := make(map[string]bool)
		for _, opt := range req.Options {
			validOptions[opt.ID] = true
		}

		for _, choice := range chosen {
			if !validOptions[choice] {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("Invalid equipment choice: %s", choice),
				})
			}
		}
	}
}

func validateSpells(req *SpellRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.Valid = false
		spellType := "spells"
		if req.Level == 0 {
			spellType = "cantrips"
		}
		result.Errors = append(result.Errors, ValidationError{
			Field:   spellType,
			Message: fmt.Sprintf("Must choose exactly %d %s, got %d", req.Count, spellType, len(chosen)),
		})
	}

	// TODO: Validate against actual spell lists when we have them
}

func validateExpertise(req *ExpertiseRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "expertise",
			Message: fmt.Sprintf("Must choose exactly %d skills/tools for expertise, got %d", req.Count, len(chosen)),
		})
	}

	// TODO: Validate that chosen skills/tools are ones the character has proficiency in
}

func validateInstruments(req *InstrumentRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "instruments",
			Message: fmt.Sprintf("Must choose exactly %d instruments, got %d", req.Count, len(chosen)),
		})
	}

	// If specific options are required, validate against them
	if req.Options != nil {
		validOptions := make(map[string]bool)
		for _, opt := range req.Options {
			validOptions[opt] = true
		}

		for _, instrument := range chosen {
			if !validOptions[instrument] {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "instruments",
					Message: fmt.Sprintf("Invalid instrument choice: %s", instrument),
				})
			}
		}
	}
	// TODO: Validate against all valid instruments when we have that list
}

func validateLanguages(req *LanguageRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "languages",
			Message: fmt.Sprintf("Must choose exactly %d languages, got %d", req.Count, len(chosen)),
		})
		return
	}

	// If specific options are required, validate against them
	if req.Options != nil {
		validOptions := make(map[string]bool)
		for _, opt := range req.Options {
			validOptions[string(opt)] = true
		}

		for _, lang := range chosen {
			if !validOptions[lang] {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "languages",
					Message: fmt.Sprintf("Invalid language choice: %s", lang),
				})
			}
		}
	} else {
		// Validate against all valid languages
		for _, lang := range chosen {
			if _, ok := languages.All[lang]; !ok {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "languages",
					Message: fmt.Sprintf("Invalid language: %s", lang),
				})
			}
		}
	}
}

func validateAncestry(req *AncestryRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != 1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "draconic_ancestry",
			Message: "Must choose exactly 1 draconic ancestry",
		})
		return
	}

	validOptions := make(map[string]bool)
	for _, opt := range req.Options {
		validOptions[string(opt)] = true
	}

	if !validOptions[chosen[0]] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "draconic_ancestry",
			Message: fmt.Sprintf("Invalid ancestry choice: %s", chosen[0]),
		})
	}
}

// checkCrossSourceDuplicates checks for duplicate choices across race, class, and background
func checkCrossSourceDuplicates(_ classes.Class, raceID races.Race, submissions Submissions, result *ValidationResult) {
	// Track all skill sources
	allSkills := make(map[string][]string) // skill -> sources

	// Collect class skills
	if classSkills, ok := submissions["skills"]; ok {
		for _, skill := range classSkills {
			allSkills[skill] = append(allSkills[skill], "class")
		}
	}

	// Collect race skills
	if raceSkills, ok := submissions["race_skills"]; ok {
		for _, skill := range raceSkills {
			allSkills[skill] = append(allSkills[skill], "race")
		}
	}

	// Collect background skills (when implemented)
	if bgSkills, ok := submissions["background_skills"]; ok {
		for _, skill := range bgSkills {
			allSkills[skill] = append(allSkills[skill], "background")
		}
	}

	// Add granted skills (not choices, but automatic grants)
	// This would need to be expanded based on actual race/background grants
	if raceID == races.HalfOrc {
		allSkills["intimidation"] = append(allSkills["intimidation"], "race (granted)")
	}

	// Check for duplicates
	for skill, sources := range allSkills {
		if len(sources) > 1 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "skills",
				Message: fmt.Sprintf("Skill '%s' granted by multiple sources: %v", skill, sources),
				Type:    "duplicate_skill",
			})
		}
	}
}
