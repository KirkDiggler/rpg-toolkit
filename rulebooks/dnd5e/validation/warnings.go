package validation

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Warning represents a non-blocking validation issue
type Warning struct {
	Field   string
	Message string
	Code    string // e.g., "duplicate_skill", "expertise_without_proficiency"
}

// Common warning codes
const (
	WarningDuplicateSkill              = "duplicate_skill"
	WarningDuplicateLanguage           = "duplicate_language"
	WarningExpertiseWithoutProficiency = "expertise_without_proficiency"
	WarningDuplicateTool               = "duplicate_tool"
	WarningDuplicateInstrument         = "duplicate_instrument"
)

// Result contains both errors and warnings from validation
type Result struct {
	Errors   []Error
	Warnings []Warning
}

// DetectCrossSourceDuplicates checks for duplicate proficiencies across all sources
func DetectCrossSourceDuplicates(choices []character.Choice) []Warning {
	var warnings []Warning

	// Collect skills from all sources
	skillsBySource := make(map[skills.Skill][]shared.ChoiceSource)
	languagesBySource := make(map[string][]shared.ChoiceSource)
	toolsBySource := make(map[string][]shared.ChoiceSource)
	instrumentsBySource := make(map[string][]shared.ChoiceSource)

	for _, choice := range choices {
		source := choice.GetSource()

		switch c := choice.(type) {
		case character.SkillChoice:
			for _, skill := range c.Skills {
				skillsBySource[skill] = append(skillsBySource[skill], source)
			}
		case character.LanguageChoice:
			for _, lang := range c.Languages {
				langStr := string(lang)
				languagesBySource[langStr] = append(languagesBySource[langStr], source)
			}
		case character.ToolProficiencyChoice:
			for _, tool := range c.Tools {
				toolsBySource[tool] = append(toolsBySource[tool], source)
			}
		case character.InstrumentProficiencyChoice:
			for _, instrument := range c.Instruments {
				instrumentsBySource[instrument] = append(instrumentsBySource[instrument], source)
			}
		}
	}

	// Check for duplicates
	for skill, sources := range skillsBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "skills",
				Message: fmt.Sprintf("Skill '%s' granted by multiple sources: %v", skill, sources),
				Code:    WarningDuplicateSkill,
			})
		}
	}

	for lang, sources := range languagesBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "languages",
				Message: fmt.Sprintf("Language '%s' granted by multiple sources: %v", lang, sources),
				Code:    WarningDuplicateLanguage,
			})
		}
	}

	for tool, sources := range toolsBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "tools",
				Message: fmt.Sprintf("Tool proficiency '%s' granted by multiple sources: %v", tool, sources),
				Code:    WarningDuplicateTool,
			})
		}
	}

	for instrument, sources := range instrumentsBySource {
		if len(sources) > 1 {
			warnings = append(warnings, Warning{
				Field:   "instruments",
				Message: fmt.Sprintf("Instrument proficiency '%s' granted by multiple sources: %v", instrument, sources),
				Code:    WarningDuplicateInstrument,
			})
		}
	}

	return warnings
}

// ValidateExpertisePrerequisites checks that expertise choices are for skills the character is proficient in
func ValidateExpertisePrerequisites(choices []character.Choice) []Warning {
	var warnings []Warning

	// First, collect all skill proficiencies
	proficientSkills := make(map[string]bool)
	for _, choice := range choices {
		if sc, ok := choice.(character.SkillChoice); ok {
			for _, skill := range sc.Skills {
				proficientSkills[string(skill)] = true
			}
		}
	}

	// Also collect tool proficiencies (for Rogue expertise in thieves' tools)
	proficientTools := make(map[string]bool)
	for _, choice := range choices {
		if tc, ok := choice.(character.ToolProficiencyChoice); ok {
			for _, tool := range tc.Tools {
				proficientTools[tool] = true
			}
		}
	}

	// Now check expertise choices
	for _, choice := range choices {
		if ec, ok := choice.(character.ExpertiseChoice); ok {
			for _, expertise := range ec.Expertise {
				// Check if it's a skill or tool
				if !proficientSkills[expertise] && !proficientTools[expertise] {
					warnings = append(warnings, Warning{
						Field:   "expertise",
						Message: fmt.Sprintf("Expertise in '%s' requires proficiency from class, race, or background", expertise),
						Code:    WarningExpertiseWithoutProficiency,
					})
				}
			}
		}
	}

	return warnings
}

// ValidateWithWarnings performs full validation including warnings
func ValidateWithWarnings(choices []character.Choice) Result {
	result := Result{
		Errors:   []Error{},
		Warnings: []Warning{},
	}

	// Detect cross-source duplicates
	result.Warnings = append(result.Warnings, DetectCrossSourceDuplicates(choices)...)

	// Validate expertise prerequisites
	result.Warnings = append(result.Warnings, ValidateExpertisePrerequisites(choices)...)

	return result
}
