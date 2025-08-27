package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Validator provides validation for character choices
type Validator struct {
	context *ValidationContext
}

// NewValidator creates a new validator
func NewValidator(context *ValidationContext) *Validator {
	if context == nil {
		context = NewValidationContext()
	}
	return &Validator{
		context: context,
	}
}

// ValidateClassChoices validates choices for a specific class
func (v *Validator) ValidateClassChoices(
	classID classes.Class,
	level int,
	submissions *TypedSubmissions,
) *ValidationResult {
	result := NewValidationResult()
	reqs := getClassRequirementsInternal(classID, level)
	if reqs == nil {
		return result
	}

	// Validate skills
	if reqs.Skills != nil {
		v.validateSkills(reqs.Skills, submissions.GetValues(SourceClass, FieldSkills), SourceClass, result)
	}

	// Validate fighting style
	if reqs.FightingStyle != nil {
		v.validateFightingStyle(reqs.FightingStyle, submissions.GetValues(SourceClass, FieldFightingStyle), result)
	}

	// Validate equipment choices
	if len(reqs.Equipment) > 0 {
		v.validateEquipment(reqs.Equipment, submissions, result)
	}

	// Validate cantrips
	if reqs.Cantrips != nil {
		v.validateSpells(reqs.Cantrips, submissions.GetValues(SourceClass, FieldCantrips), FieldCantrips, result)
	}

	// Validate spells
	if reqs.Spells != nil {
		v.validateSpells(reqs.Spells, submissions.GetValues(SourceClass, FieldSpells), FieldSpells, result)
	}

	// Validate expertise with proficiency checking
	if reqs.Expertise != nil {
		v.validateExpertise(reqs.Expertise, submissions.GetValues(SourceClass, FieldExpertise), result)
	}

	// Validate instruments
	if reqs.Instruments != nil {
		v.validateInstruments(reqs.Instruments, submissions.GetValues(SourceClass, FieldInstruments), result)
	}

	return result
}

// validateSkills validates skill choices with enhanced error reporting
func (v *Validator) validateSkills(req *SkillRequirement, chosen []string, source Source, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.AddIssue(NewCountError(FieldSkills, req.Count, len(chosen), "skills"))
		return
	}

	// Track seen skills for duplicate detection
	seen := make(map[string]bool)
	validOptions := make(map[string]bool)

	// Build valid options map
	if req.Options != nil {
		for _, opt := range req.Options {
			validOptions[string(opt)] = true
		}
	} else {
		// All skills are valid
		for skill := range skills.All {
			validOptions[skill] = true
		}
	}

	// Validate each chosen skill
	for _, skill := range chosen {
		// Check if skill is valid
		if !validOptions[skill] {
			result.AddIssue(NewInvalidOptionError(FieldSkills, skill, "skill"))
			continue
		}

		// Check for duplicates within this selection
		if seen[skill] {
			result.AddIssue(NewDuplicateError(FieldSkills, skill, source, source))
			continue
		}
		seen[skill] = true

		// Check if this skill is redundant due to automatic grants
		if v.context != nil {
			if hasGrant, grantSource := v.context.HasAutomaticGrant(FieldSkills, skill); hasGrant {
				result.AddIssue(ValidationIssue{
					Code:     CodeRedundantChoice,
					Severity: SeverityWarning,
					Field:    FieldSkills,
					Message: fmt.Sprintf(
						"Skill '%s' is already granted automatically by %s - "+
							"consider choosing a different skill", skill, grantSource),
					Details: map[DetailKey]any{
						DetailValue:   skill,
						DetailSources: []Source{source, grantSource},
					},
				})
			}
		}
	}
}

// validateFightingStyle validates fighting style choices
func (v *Validator) validateFightingStyle(req *FightingStyleRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != 1 {
		result.AddIssue(NewCountError(FieldFightingStyle, 1, len(chosen), "fighting style"))
		return
	}

	// Check if the chosen style is valid for the class
	if req.Options != nil {
		valid := false
		for _, style := range req.Options {
			if string(style) == chosen[0] {
				valid = true
				break
			}
		}
		if !valid {
			result.AddIssue(NewInvalidOptionError(FieldFightingStyle, chosen[0], "fighting style"))
		}
	}
}

// validateExpertise validates expertise choices with proficiency checking
func (v *Validator) validateExpertise(req *ExpertiseRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.AddIssue(NewCountError(FieldExpertise, req.Count, len(chosen), "expertise skills"))
		return
	}

	// Check each expertise choice
	seen := make(map[string]bool)
	for _, skill := range chosen {
		// Check for duplicates
		if seen[skill] {
			result.AddIssue(NewDuplicateError(FieldExpertise, skill, SourceClass, SourceClass))
			continue
		}
		seen[skill] = true

		// Check if character has proficiency in this skill
		if v.context != nil && !v.context.HasProficiency(skill) {
			result.AddIssue(ValidationIssue{
				Code:     CodeExpertiseWithoutProficiency,
				Severity: SeverityError,
				Field:    FieldExpertise,
				Message:  fmt.Sprintf("Cannot have expertise in %s without proficiency", skill),
				Details: map[DetailKey]any{
					DetailSkill:    skill,
					DetailRequired: "proficiency",
				},
			})
		}
	}
}

// validateEquipment validates equipment choices
func (v *Validator) validateEquipment(
	reqs []*EquipmentRequirement,
	submissions *TypedSubmissions,
	result *ValidationResult,
) {
	// For each equipment choice requirement
	for i, req := range reqs {
		choiceKey := fmt.Sprintf("equipment_choice_%d", i)

		// Get the values for this specific equipment choice
		var chosen []string
		for _, choice := range submissions.GetByField(Field(choiceKey)) {
			if choice.Source == SourceClass {
				chosen = choice.Values
				break
			}
		}

		// Check if a choice was made
		if len(chosen) == 0 {
			result.AddIssue(ValidationIssue{
				Code:     CodeMissingRequired,
				Severity: SeverityIncomplete,
				Field:    Field(choiceKey),
				Message:  fmt.Sprintf("Equipment choice %d is required", i+1),
			})
			continue
		}

		// Validate the choice is from available options
		if req.Options != nil {
			valid := false
			for _, opt := range req.Options {
				if len(chosen) == 1 && chosen[0] == opt.ID {
					valid = true
					break
				}
			}
			if !valid {
				result.AddIssue(NewInvalidOptionError(Field(choiceKey), chosen[0], "equipment"))
			}
		}
	}
}

// validateSpells validates spell choices
func (v *Validator) validateSpells(req *SpellRequirement, chosen []string, field Field, result *ValidationResult) {
	expectedCount := req.Count
	if len(chosen) != expectedCount {
		itemType := "spells"
		if field == FieldCantrips {
			itemType = "cantrips"
		}
		result.AddIssue(NewCountError(field, expectedCount, len(chosen), itemType))
		return
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, spell := range chosen {
		if seen[spell] {
			result.AddIssue(NewDuplicateError(field, spell, SourceClass, SourceClass))
			continue
		}
		seen[spell] = true

		// TODO: Validate spell is valid for class and level when spell data is available
		// For now, any non-empty string is considered valid
		if spell == "" {
			result.AddIssue(NewInvalidOptionError(field, spell, string(field)))
		}
	}
}

// validateInstruments validates instrument choices
func (v *Validator) validateInstruments(req *InstrumentRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != req.Count {
		result.AddIssue(NewCountError(FieldInstruments, req.Count, len(chosen), "instruments"))
		return
	}

	// Check for duplicates and validate options
	seen := make(map[string]bool)
	for _, instrument := range chosen {
		if seen[instrument] {
			result.AddIssue(NewDuplicateError(FieldInstruments, instrument, SourceClass, SourceClass))
			continue
		}
		seen[instrument] = true

		// TODO: Validate instrument is valid when instrument data is available
		if instrument == "" {
			result.AddIssue(NewInvalidOptionError(FieldInstruments, instrument, "instrument"))
		}
	}
}

// ValidateCrossSourceDuplicates checks for duplicates across different sources
func (v *Validator) ValidateCrossSourceDuplicates(submissions *TypedSubmissions) *ValidationResult {
	result := NewValidationResult()

	// Check each field type that can have duplicates
	v.checkFieldDuplicates(FieldSkills, "skill", submissions, result)
	v.checkFieldDuplicates(FieldLanguages, "language", submissions, result)

	return result
}

// checkFieldDuplicates checks for duplicates of a specific field across sources
func (v *Validator) checkFieldDuplicates(
	field Field,
	_ string,
	submissions *TypedSubmissions,
	result *ValidationResult,
) {
	seen := make(map[string]Source)

	for _, choice := range submissions.GetByField(field) {
		for _, value := range choice.Values {
			if firstSource, exists := seen[value]; exists {
				if firstSource != choice.Source {
					result.AddIssue(NewDuplicateError(field, value, firstSource, choice.Source))
				}
			} else {
				seen[value] = choice.Source
			}
		}
	}
}

// ValidateProficiencyDuplicates checks for proficiency duplicates with special handling
func (v *Validator) ValidateProficiencyDuplicates(submissions *TypedSubmissions) *ValidationResult {
	result := NewValidationResult()

	// Collect all proficiencies by source, deduplicating within source
	// to avoid redundant warnings for duplicates already caught as errors
	skillProfs := make(map[Source][]string)
	for _, choice := range submissions.GetByField(FieldSkills) {
		seen := make(map[string]bool)
		dedupedValues := []string{}
		for _, val := range choice.Values {
			if !seen[val] {
				dedupedValues = append(dedupedValues, val)
				seen[val] = true
			}
		}
		skillProfs[choice.Source] = dedupedValues
	}

	v.checkProficiencyDuplicates(skillProfs, "skill", FieldSkills, result)

	return result
}

// checkProficiencyDuplicates checks for duplicates with information about redundancy
func (v *Validator) checkProficiencyDuplicates(
	proficiencies map[Source][]string,
	profType string,
	field Field,
	result *ValidationResult,
) {
	seen := make(map[string][]Source)

	// Track all occurrences
	for source, profs := range proficiencies {
		for _, prof := range profs {
			seen[prof] = append(seen[prof], source)
		}
	}

	// Report duplicates
	for prof, sources := range seen {
		if len(sources) > 1 {
			// This is a warning - player is wasting a choice
			result.AddIssue(ValidationIssue{
				Code:     CodeRedundantChoice,
				Severity: SeverityWarning,
				Field:    field,
				Message: fmt.Sprintf("%s '%s' chosen from %v but already granted by %v - pick another instead",
					profType, prof, sources[0], sources[1]),
				Details: map[DetailKey]any{
					DetailValue:   prof,
					DetailSources: sources,
				},
			})
		}
	}
}

// ValidateRaceChoices validates choices for a specific race
func (v *Validator) ValidateRaceChoices(raceID races.Race, submissions *TypedSubmissions) *ValidationResult {
	result := NewValidationResult()
	reqs := getRaceRequirementsInternal(raceID)

	// Even if there are no formal requirements, check for redundant choices
	// This handles cases where players make choices that aren't required (like choosing a language already granted)

	// Check skills - either validate requirements or just check for redundancy
	raceSkills := submissions.GetValues(SourceRace, FieldSkills)
	if reqs != nil && reqs.Skills != nil {
		v.validateSkills(reqs.Skills, raceSkills, SourceRace, result)
	} else if len(raceSkills) > 0 {
		// No skill requirements, but player made skill choices - check for redundancy
		v.checkRedundantChoices(FieldSkills, raceSkills, SourceRace, result)
	}

	// Check languages - either validate requirements or just check for redundancy
	raceLanguages := submissions.GetValues(SourceRace, FieldLanguages)
	if reqs != nil && reqs.Languages != nil {
		v.validateLanguages(reqs.Languages, raceLanguages, SourceRace, result)
	} else if len(raceLanguages) > 0 {
		// No language requirements, but player made language choices - check for redundancy
		v.checkRedundantChoices(FieldLanguages, raceLanguages, SourceRace, result)
	}

	// Validate draconic ancestry (Dragonborn)
	if reqs != nil && reqs.DraconicAncestry != nil {
		v.validateAncestry(reqs.DraconicAncestry, submissions.GetValues(SourceRace, FieldDraconicAncestry), result)
	}

	return result
}

// checkRedundantChoices checks if choices are redundant due to automatic grants
func (v *Validator) checkRedundantChoices(field Field, chosen []string, source Source, result *ValidationResult) {
	if v.context == nil {
		return
	}

	var itemType string

	switch field {
	case FieldSkills:
		itemType = "skill"
	case FieldLanguages:
		itemType = "language"
	default:
		itemType = "unknown"
	}

	title := cases.Title(language.English)
	for _, value := range chosen {
		if hasGrant, grantSource := v.context.HasAutomaticGrant(field, value); hasGrant {
			result.AddIssue(ValidationIssue{
				Code:     CodeRedundantChoice,
				Severity: SeverityWarning,
				Field:    field,
				Message: fmt.Sprintf("%s '%s' is already granted automatically by %s - consider choosing a different %s",
					title.String(itemType), value, grantSource, itemType),
				Details: map[DetailKey]any{
					DetailValue:   value,
					DetailSources: []Source{source, grantSource},
				},
			})
		}
	}
}

// validateLanguages validates language choices
func (v *Validator) validateLanguages(
	req *LanguageRequirement,
	chosen []string,
	source Source,
	result *ValidationResult,
) {
	if len(chosen) != req.Count {
		result.AddIssue(NewCountError(FieldLanguages, req.Count, len(chosen), "languages"))
		return
	}

	// Track seen languages for duplicate detection
	seen := make(map[string]bool)
	validOptions := make(map[string]bool)

	// Build valid options map
	if req.Options != nil {
		for _, opt := range req.Options {
			validOptions[string(opt)] = true
		}
	} else {
		// All languages are valid if no options specified
		// TODO: Get all valid languages from language package
		validOptions["common"] = true
		validOptions["dwarvish"] = true
		validOptions["elvish"] = true
		validOptions["giant"] = true
		validOptions["gnomish"] = true
		validOptions["goblin"] = true
		validOptions["halfling"] = true
		validOptions["orc"] = true
	}

	// Validate each chosen language
	for _, lang := range chosen {
		// Check if language is valid
		if !validOptions[lang] {
			result.AddIssue(NewInvalidOptionError(FieldLanguages, lang, "language"))
			continue
		}

		// Check for duplicates within this selection
		if seen[lang] {
			result.AddIssue(NewDuplicateError(FieldLanguages, lang, source, source))
			continue
		}
		seen[lang] = true

		// Check if this language is redundant due to automatic grants
		if v.context != nil {
			if hasGrant, grantSource := v.context.HasAutomaticGrant(FieldLanguages, lang); hasGrant {
				result.AddIssue(ValidationIssue{
					Code:     CodeRedundantChoice,
					Severity: SeverityWarning,
					Field:    FieldLanguages,
					Message: fmt.Sprintf(
						"Language '%s' is already granted automatically by %s - "+
							"consider choosing a different language", lang, grantSource),
					Details: map[DetailKey]any{
						DetailValue:   lang,
						DetailSources: []Source{source, grantSource},
					},
				})
			}
		}
	}
}

// validateAncestry validates draconic ancestry choices
func (v *Validator) validateAncestry(_ *AncestryRequirement, chosen []string, result *ValidationResult) {
	if len(chosen) != 1 {
		result.AddIssue(NewCountError(FieldDraconicAncestry, 1, len(chosen), "draconic ancestry"))
		return
	}

	// Validate the chosen ancestry
	// TODO: Add proper ancestry validation when data is available
	if chosen[0] == "" {
		result.AddIssue(NewInvalidOptionError(FieldDraconicAncestry, chosen[0], "draconic ancestry"))
	}
}

// ValidateAll validates all choices for a character
func (v *Validator) ValidateAll(
	classID classes.Class,
	raceID races.Race,
	_ backgrounds.Background,
	level int,
	submissions *TypedSubmissions,
) *ValidationResult {
	result := NewValidationResult()

	// Validate class choices
	classResult := v.ValidateClassChoices(classID, level, submissions)
	v.mergeResults(result, classResult)

	// Validate race choices
	raceResult := v.ValidateRaceChoices(raceID, submissions)
	v.mergeResults(result, raceResult)

	// Validate cross-source duplicates
	dupResult := v.ValidateCrossSourceDuplicates(submissions)
	v.mergeResults(result, dupResult)

	// Check proficiency redundancy (informational)
	profResult := v.ValidateProficiencyDuplicates(submissions)
	v.mergeResults(result, profResult)

	return result
}

// mergeResults merges source validation results into target
func (v *Validator) mergeResults(target, source *ValidationResult) {
	target.AllIssues = append(target.AllIssues, source.AllIssues...)
	target.Errors = append(target.Errors, source.Errors...)
	target.Incomplete = append(target.Incomplete, source.Incomplete...)
	target.Warnings = append(target.Warnings, source.Warnings...)

	// Update status flags
	if !source.CanSave {
		target.CanSave = false
	}
	if !source.CanFinalize {
		target.CanFinalize = false
	}
	if !source.IsOptimal {
		target.IsOptimal = false
	}
}
