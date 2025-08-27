package choices

import "fmt"

// ValidationIssue represents any validation problem with typed details
type ValidationIssue struct {
	Code     ValidationCode    `json:"code"`
	Severity Severity          `json:"severity"`
	Field    Field             `json:"field"`
	Message  string            `json:"message"`
	Details  map[DetailKey]any `json:"details,omitempty"`
	Source   Source            `json:"source,omitempty"`
}

// IsBlocking returns true if this issue prevents saving the draft
func (v *ValidationIssue) IsBlocking() bool {
	return v.Severity == SeverityError
}

// IsIncomplete returns true if this issue prevents finalizing the character
func (v *ValidationIssue) IsIncomplete() bool {
	return v.Severity == SeverityIncomplete
}

// ValidationResult represents the validation result with severity levels
type ValidationResult struct {
	// Overall status flags
	CanSave     bool `json:"can_save"`     // Can the draft be saved?
	CanFinalize bool `json:"can_finalize"` // Can the character be finalized?
	IsOptimal   bool `json:"is_optimal"`   // Are there any warnings?

	// Categorized issues
	Errors     []ValidationIssue `json:"errors,omitempty"`     // Blocking errors (invalid choices)
	Incomplete []ValidationIssue `json:"incomplete,omitempty"` // Missing required choices
	Warnings   []ValidationIssue `json:"warnings,omitempty"`   // Suboptimal but allowed (wasted choices)

	// Quick access to all issues
	AllIssues []ValidationIssue `json:"all_issues,omitempty"`
}

// AddIssue adds a validation issue and updates status flags
func (r *ValidationResult) AddIssue(issue ValidationIssue) {
	r.AllIssues = append(r.AllIssues, issue)

	switch issue.Severity {
	case SeverityError:
		r.Errors = append(r.Errors, issue)
		r.CanSave = false
		r.CanFinalize = false
		r.IsOptimal = false
	case SeverityIncomplete:
		r.Incomplete = append(r.Incomplete, issue)
		r.CanFinalize = false
		r.IsOptimal = false
	case SeverityWarning:
		r.Warnings = append(r.Warnings, issue)
		r.IsOptimal = false
	}
}

// NewValidationResult creates a new validation result with default values
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		CanSave:     true,
		CanFinalize: true,
		IsOptimal:   true,
		AllIssues:   []ValidationIssue{},
	}
}

// Specific detail structures for common validation scenarios

// CountDetails provides details for count-related validation issues
type CountDetails struct {
	Expected int      `json:"expected"`
	Actual   int      `json:"actual"`
	Options  []string `json:"options,omitempty"`
}

// ToMap converts CountDetails to a generic map for ValidationIssue.Details
func (d CountDetails) ToMap() map[DetailKey]any {
	m := map[DetailKey]any{
		DetailKeyExpected: d.Expected,
		DetailKeyActual:   d.Actual,
	}
	if len(d.Options) > 0 {
		m[DetailKeyOptions] = d.Options
	}
	return m
}

// DuplicateDetails provides details for duplicate selection issues
type DuplicateDetails struct {
	Duplicate string   `json:"duplicate"`
	Sources   []Source `json:"sources"`
}

// ToMap converts DuplicateDetails to a generic map
func (d DuplicateDetails) ToMap() map[DetailKey]any {
	sourcesStr := make([]string, len(d.Sources))
	for i, s := range d.Sources {
		sourcesStr[i] = string(s)
	}
	return map[DetailKey]any{
		DetailKeyDuplicate: d.Duplicate,
		DetailKeySources:   sourcesStr,
	}
}

// PrerequisiteDetails provides details for prerequisite failures
type PrerequisiteDetails struct {
	Missing      string `json:"missing"`
	Prerequisite string `json:"prerequisite"`
	Suggestion   string `json:"suggestion,omitempty"`
}

// ToMap converts PrerequisiteDetails to a generic map
func (d PrerequisiteDetails) ToMap() map[DetailKey]any {
	m := map[DetailKey]any{
		DetailKeyMissing:      d.Missing,
		DetailKeyPrerequisite: d.Prerequisite,
	}
	if d.Suggestion != "" {
		m[DetailKeySuggestion] = d.Suggestion
	}
	return m
}

// RangeDetails provides details for value range violations
type RangeDetails struct {
	InvalidValue any    `json:"invalid_value"`
	Minimum      any    `json:"minimum,omitempty"`
	Maximum      any    `json:"maximum,omitempty"`
	ValidRange   string `json:"valid_range,omitempty"`
}

// ToMap converts RangeDetails to a generic map
func (d RangeDetails) ToMap() map[DetailKey]any {
	m := map[DetailKey]any{
		DetailKeyInvalidValue: d.InvalidValue,
	}
	if d.Minimum != nil {
		m[DetailKeyMinimum] = d.Minimum
	}
	if d.Maximum != nil {
		m[DetailKeyMaximum] = d.Maximum
	}
	if d.ValidRange != "" {
		m[DetailKeyValidRange] = d.ValidRange
	}
	return m
}

// NewDuplicateWarning creates a warning for duplicate selections
func NewDuplicateWarning(field Field, duplicate string, sources []Source, choiceType string) ValidationIssue {
	return ValidationIssue{
		Code:     CodeDuplicateSelection,
		Severity: SeverityWarning,
		Field:    field,
		Message:  fmt.Sprintf(MessageCrossSource, choiceType, duplicate, sources),
		Details: DuplicateDetails{
			Duplicate: duplicate,
			Sources:   sources,
		}.ToMap(),
	}
}

// NewExpertiseWithoutProficiencyError creates an incomplete error for expertise without proficiency
// This allows saving the draft but prevents finalization
func NewExpertiseWithoutProficiencyError(skill string) ValidationIssue {
	return ValidationIssue{
		Code:     CodeExpertiseWithoutProficiency,
		Severity: SeverityIncomplete,
		Field:    FieldExpertise,
		Message:  fmt.Sprintf(MessageExpertiseNoProficiency, skill),
		Details: PrerequisiteDetails{
			Missing:      skill + " proficiency",
			Prerequisite: "proficiency in " + skill,
			Suggestion:   "Select a skill you are proficient in",
		}.ToMap(),
	}
}

// NewRequiredChoiceMissing creates an incomplete error for missing required choices
func NewRequiredChoiceMissing(field Field, choiceName string) ValidationIssue {
	return ValidationIssue{
		Code:     CodeRequiredChoiceMissing,
		Severity: SeverityIncomplete,
		Field:    field,
		Message:  fmt.Sprintf(MessageRequiredMissing, choiceName),
		Details: map[DetailKey]any{
			DetailKeyMissing: choiceName,
		},
	}
}
