package choices

import "fmt"

// Source represents the origin of a choice submission
type Source string

// Source constants - where choices come from
const (
	SourceClass      Source = "class"
	SourceRace       Source = "race"
	SourceSubrace    Source = "subrace"
	SourceBackground Source = "background"
	SourceFeat       Source = "feat"
	SourceMulticlass Source = "multiclass"
	SourceLevelUp    Source = "level_up"
	SourceManual     Source = "manual" // DM granted or house rules
)

// Field represents a validation field identifier
type Field string

// Field constants - what's being validated
const (
	FieldSkills              Field = "skills"
	FieldRaceSkills          Field = "race_skills"
	FieldBackgroundSkills    Field = "background_skills"
	FieldLanguages           Field = "languages"
	FieldRaceLanguages       Field = "race_languages"
	FieldTools               Field = "tools"
	FieldInstruments         Field = "instruments"
	FieldFightingStyle       Field = "fighting_style"
	FieldExpertise           Field = "expertise"
	FieldCantrips            Field = "cantrips"
	FieldSpells              Field = "spells"
	FieldEquipment           Field = "equipment"
	FieldDraconicAncestry    Field = "draconic_ancestry"
	FieldAbilityScores       Field = "ability_scores"
	FieldFeat                Field = "feat"
	FieldArmorProficiencies  Field = "armor_proficiencies"
	FieldWeaponProficiencies Field = "weapon_proficiencies"
)

// ValidationCode represents a specific validation issue type
type ValidationCode string

// ValidationCode constants - specific validation issues
const (
	// Count violations
	CodeTooFewChoices      ValidationCode = "too_few_choices"
	CodeTooManyChoices     ValidationCode = "too_many_choices"
	CodeExactCountRequired ValidationCode = "exact_count_required"

	// Invalid selections
	CodeInvalidOption      ValidationCode = "invalid_option"
	CodeOptionNotAvailable ValidationCode = "option_not_available"
	CodePrerequisiteNotMet ValidationCode = "prerequisite_not_met"

	// Duplicates and conflicts
	CodeDuplicateChoice      ValidationCode = "duplicate_choice"
	CodeDuplicateSelection   ValidationCode = "duplicate_selection"
	CodeDuplicateProficiency ValidationCode = "duplicate_proficiency"
	CodeConflictingChoice    ValidationCode = "conflicting_choice"
	CodeRedundantChoice      ValidationCode = "redundant_choice"

	// Missing requirements
	CodeMissingRequired        ValidationCode = "missing_required"
	CodeRequiredChoiceMissing  ValidationCode = "required_choice_missing"
	CodeDependentChoiceMissing ValidationCode = "dependent_choice_missing"

	// Expertise specific
	CodeExpertiseWithoutProficiency ValidationCode = "expertise_without_proficiency"
	CodeExpertiseAlreadyApplied     ValidationCode = "expertise_already_applied"

	// Ability scores
	CodeAbilityScoreTooHigh         ValidationCode = "ability_score_too_high"
	CodeAbilityScoreTooLow          ValidationCode = "ability_score_too_low"
	CodeAbilityScorePointsRemaining ValidationCode = "ability_score_points_remaining"
	CodeAbilityScoreInvalidRacial   ValidationCode = "ability_score_invalid_racial"

	// Equipment
	CodeIncompatibleEquipment  ValidationCode = "incompatible_equipment"
	CodeMissingEquipmentChoice ValidationCode = "missing_equipment_choice"

	// Spells
	CodeSpellNotInList          ValidationCode = "spell_not_in_list"
	CodeSpellLevelTooHigh       ValidationCode = "spell_level_too_high"
	CodeSpellPrerequisiteNotMet ValidationCode = "spell_prerequisite_not_met"
	CodeCantripLimitExceeded    ValidationCode = "cantrip_limit_exceeded"

	// Cross-source validation
	CodeCrossSourceDuplicate ValidationCode = "cross_source_duplicate"
	CodeMaximumReached       ValidationCode = "maximum_reached"
)

// Severity represents the severity level of a validation issue
type Severity string

// Severity constants - how serious is the issue
const (
	SeverityError      Severity = "error"      // Blocks saving draft (invalid choice)
	SeverityIncomplete Severity = "incomplete" // Can save draft, cannot finalize (missing required)
	SeverityWarning    Severity = "warning"    // Can finalize but suboptimal (wasted choice)
)

// DetailKey represents keys for structured validation details
type DetailKey string

// DetailKey constants - for structured error/warning details
const (
	DetailKeyExpected     DetailKey = "expected"
	DetailKeyActual       DetailKey = "actual"
	DetailKeyOptions      DetailKey = "options"
	DetailKeySource       DetailKey = "source"
	DetailKeySources      DetailKey = "sources"
	DetailKeyMissing      DetailKey = "missing"
	DetailKeyDuplicate    DetailKey = "duplicate"
	DetailKeyConflict     DetailKey = "conflict"
	DetailKeyPrerequisite DetailKey = "prerequisite"
	DetailKeyMaximum      DetailKey = "maximum"
	DetailKeyMinimum      DetailKey = "minimum"
	DetailKeyRemaining    DetailKey = "remaining"
	DetailKeyInvalidValue DetailKey = "invalid_value"
	DetailKeyValidRange   DetailKey = "valid_range"
	DetailKeySuggestion   DetailKey = "suggestion"
	DetailSkill           DetailKey = "skill"
	DetailRequired        DetailKey = "required"
	DetailValue           DetailKey = "value"
	DetailSources         DetailKey = "sources"
)

// ChoiceType represents the type of choice being made
type ChoiceType string

// ChoiceType constants
const (
	ChoiceTypeSkill         ChoiceType = "skill"
	ChoiceTypeLanguage      ChoiceType = "language"
	ChoiceTypeTool          ChoiceType = "tool"
	ChoiceTypeInstrument    ChoiceType = "instrument"
	ChoiceTypeFightingStyle ChoiceType = "fighting_style"
	ChoiceTypeExpertise     ChoiceType = "expertise"
	ChoiceTypeCantrip       ChoiceType = "cantrip"
	ChoiceTypeSpell         ChoiceType = "spell"
	ChoiceTypeEquipment     ChoiceType = "equipment"
	ChoiceTypeAncestry      ChoiceType = "ancestry"
	ChoiceTypeAbilityScore  ChoiceType = "ability_score"
	ChoiceTypeFeat          ChoiceType = "feat"
	ChoiceTypeProficiency   ChoiceType = "proficiency"
)

// Message templates for consistent error messages
const (
	MessageTooFewChoices          = "Must choose at least %d %s, got %d"
	MessageTooManyChoices         = "Must choose at most %d %s, got %d"
	MessageExactCount             = "Must choose exactly %d %s, got %d"
	MessageInvalidOption          = "Invalid %s choice: %s"
	MessageNotAvailable           = "%s '%s' is not available for %s"
	MessageDuplicate              = "Duplicate %s selected: %s"
	MessageCrossSource            = "%s '%s' granted by multiple sources: %v"
	MessagePrerequisite           = "%s requires %s"
	MessageExpertiseNoProficiency = "Cannot have expertise in %s without proficiency"
	MessageRequiredMissing        = "Required choice '%s' is missing"
	MessagePointsRemaining        = "%d ability score points remaining"
)

// Helper functions for creating validation issues

// NewCountError creates a validation issue for incorrect count
func NewCountError(field Field, expected, actual int, itemType string) ValidationIssue {
	return ValidationIssue{
		Code:     CodeExactCountRequired,
		Severity: SeverityError,
		Field:    field,
		Message:  fmt.Sprintf(MessageExactCount, expected, itemType, actual),
		Details: map[DetailKey]any{
			DetailKeyExpected: expected,
			DetailKeyActual:   actual,
		},
	}
}

// NewInvalidOptionError creates a validation issue for invalid option
func NewInvalidOptionError(field Field, value string, itemType string) ValidationIssue {
	return ValidationIssue{
		Code:     CodeInvalidOption,
		Severity: SeverityError,
		Field:    field,
		Message:  fmt.Sprintf(MessageInvalidOption, itemType, value),
		Details: map[DetailKey]any{
			DetailKeyInvalidValue: value,
		},
	}
}

// NewDuplicateError creates a validation issue for duplicate selection
func NewDuplicateError(field Field, value string, source1, source2 Source) ValidationIssue {
	if source1 == source2 {
		// Same source duplicate
		return ValidationIssue{
			Code:     CodeDuplicateSelection,
			Severity: SeverityError,
			Field:    field,
			Message:  fmt.Sprintf("Duplicate selection: %s", value),
			Details: map[DetailKey]any{
				DetailKeyDuplicate: value,
				DetailKeySource:    source1,
			},
		}
	}
	// Cross-source duplicate - this is informational in D&D 5e
	// When you get the same proficiency from multiple sources, you pick another
	return ValidationIssue{
		Code:     CodeDuplicateChoice,
		Severity: SeverityWarning,
		Field:    field,
		Message:  fmt.Sprintf("'%s' selected in multiple sources (pick another skill instead)", value),
		Details: map[DetailKey]any{
			DetailValue:   value,
			DetailSources: []Source{source1, source2},
		},
	}
}
