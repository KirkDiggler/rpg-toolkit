package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Submissions represents player choices submitted for validation (legacy format)
type Submissions map[string][]string

// ChoiceSubmission represents a single choice made by a player with source tracking
type ChoiceSubmission struct {
	Source   Source         `json:"source"`             // Where this choice comes from (typed)
	Field    Field          `json:"field"`              // What field this choice is for (typed)
	ChoiceID string         `json:"choice_id"`          // Unique identifier for this specific choice
	Values   []string       `json:"values"`             // The selected values
	Metadata map[string]any `json:"metadata,omitempty"` // Additional context if needed
}

// TypedSubmissions represents all player choices organized by source and field
type TypedSubmissions struct {
	// Raw submissions for backward compatibility
	Raw map[string][]string `json:"raw,omitempty"`

	// Typed submissions with source tracking
	Choices []ChoiceSubmission `json:"choices"`

	// Quick lookups
	bySource map[Source][]ChoiceSubmission
	byField  map[Field][]ChoiceSubmission
}

// NewTypedSubmissions creates a new TypedSubmissions instance
func NewTypedSubmissions() *TypedSubmissions {
	return &TypedSubmissions{
		Raw:      make(map[string][]string),
		Choices:  []ChoiceSubmission{},
		bySource: make(map[Source][]ChoiceSubmission),
		byField:  make(map[Field][]ChoiceSubmission),
	}
}

// AddChoice adds a typed choice submission
func (ts *TypedSubmissions) AddChoice(choice ChoiceSubmission) {
	ts.Choices = append(ts.Choices, choice)

	// Update lookups
	ts.bySource[choice.Source] = append(ts.bySource[choice.Source], choice)
	ts.byField[choice.Field] = append(ts.byField[choice.Field], choice)

	// Update raw map for backward compatibility
	fieldKey := string(choice.Field)
	if choice.Source != SourceClass {
		// Prefix non-class choices with source for backward compatibility
		fieldKey = string(choice.Source) + "_" + fieldKey
	}
	ts.Raw[fieldKey] = choice.Values
}

// GetBySource returns all choices from a specific source
func (ts *TypedSubmissions) GetBySource(source Source) []ChoiceSubmission {
	return ts.bySource[source]
}

// GetByField returns all choices for a specific field
func (ts *TypedSubmissions) GetByField(field Field) []ChoiceSubmission {
	return ts.byField[field]
}

// GetValues returns the values for a specific field and source
func (ts *TypedSubmissions) GetValues(source Source, field Field) []string {
	choices := ts.byField[field]
	for _, choice := range choices {
		if choice.Source == source {
			return choice.Values
		}
	}
	return nil
}

// GetAllValues returns all values for a field across all sources
func (ts *TypedSubmissions) GetAllValues(field Field) map[Source][]string {
	result := make(map[Source][]string)
	for _, choice := range ts.byField[field] {
		result[choice.Source] = choice.Values
	}
	return result
}

// HasChoice checks if a choice exists for a field and source
func (ts *TypedSubmissions) HasChoice(source Source, field Field) bool {
	choices := ts.byField[field]
	for _, choice := range choices {
		if choice.Source == source {
			return true
		}
	}
	return false
}

// FromLegacySubmissions converts old-style Submissions to TypedSubmissions
func FromLegacySubmissions(legacy Submissions) *TypedSubmissions {
	ts := NewTypedSubmissions()

	for key, values := range legacy {
		// Parse the key to determine source and field
		source, field := parseLegacyKey(key)

		choice := ChoiceSubmission{
			Source:   source,
			Field:    field,
			ChoiceID: key,
			Values:   values,
		}

		ts.AddChoice(choice)
	}

	return ts
}

// ToLegacySubmissions converts TypedSubmissions back to legacy format
func (ts *TypedSubmissions) ToLegacySubmissions() Submissions {
	return ts.Raw
}

// parseLegacyKey parses a legacy submission key into source and field
func parseLegacyKey(key string) (Source, Field) {
	// Common patterns in legacy keys
	switch key {
	case "skills":
		return SourceClass, FieldSkills
	case "race_skills":
		return SourceRace, FieldSkills
	case "background_skills":
		return SourceBackground, FieldSkills
	case "languages":
		return SourceClass, FieldLanguages
	case "race_languages":
		return SourceRace, FieldLanguages
	case "fighting_style":
		return SourceClass, FieldFightingStyle
	case "expertise":
		return SourceClass, FieldExpertise
	case "cantrips":
		return SourceClass, FieldCantrips
	case "spells":
		return SourceClass, FieldSpells
	case "instruments":
		return SourceClass, FieldInstruments
	case "tools":
		return SourceClass, FieldTools
	case "draconic_ancestry":
		return SourceRace, FieldDraconicAncestry
	default:
		// Equipment choices (equipment_0, equipment_1, etc.)
		if len(key) > 10 && key[:10] == "equipment_" {
			return SourceClass, FieldEquipment
		}
		// Default to class source
		return SourceClass, Field(key)
	}
}

// ValidationContext provides context for validation
type ValidationContext struct {
	// Character proficiencies for expertise validation
	SkillProficiencies    map[string]Source // skill -> source of proficiency
	ToolProficiencies     map[string]Source // tool -> source of proficiency
	WeaponProficiencies   map[string]Source // weapon -> source of proficiency
	ArmorProficiencies    map[string]Source // armor -> source of proficiency
	LanguageProficiencies map[string]Source // language -> source of proficiency

	// Automatic grants from race/class/background
	// These are NOT player choices, but inherent features that inform validation
	AutomaticGrants struct {
		Skills    map[skills.Skill]Source       // skill -> source that grants it automatically
		Languages map[languages.Language]Source // language -> source that grants it automatically
		Tools     map[string]Source             // tool -> source that grants it automatically
	}

	// Existing expertise
	ExistingExpertise map[string]bool // skill/tool -> has expertise

	// Class and race data for validation
	AllowedSkills         []string
	AllowedLanguages      []string
	AllowedTools          []string
	AllowedFightingStyles []string

	// Level for level-gated choices
	CharacterLevel int
	ClassLevel     int
}

// NewValidationContext creates a new validation context with default values
func NewValidationContext() *ValidationContext {
	return &ValidationContext{
		SkillProficiencies:    make(map[string]Source),
		ToolProficiencies:     make(map[string]Source),
		WeaponProficiencies:   make(map[string]Source),
		ArmorProficiencies:    make(map[string]Source),
		LanguageProficiencies: make(map[string]Source),
		AutomaticGrants: struct {
			Skills    map[skills.Skill]Source
			Languages map[languages.Language]Source
			Tools     map[string]Source
		}{
			Skills:    make(map[skills.Skill]Source),
			Languages: make(map[languages.Language]Source),
			Tools:     make(map[string]Source),
		},
		ExistingExpertise: make(map[string]bool),
	}
}

// HasProficiency checks if the character has proficiency in a skill or tool
func (vc *ValidationContext) HasProficiency(name string) bool {
	if vc == nil {
		return false
	}
	_, hasSkill := vc.SkillProficiencies[name]
	_, hasTool := vc.ToolProficiencies[name]
	return hasSkill || hasTool
}

// AddProficiency adds a proficiency to the context for validation
func (vc *ValidationContext) AddProficiency(name string) {
	if vc == nil {
		return
	}
	// Default to class source if not specified
	vc.SkillProficiencies[name] = SourceClass
}

// AddAutomaticSkillGrant adds an automatic skill grant to the context
func (vc *ValidationContext) AddAutomaticSkillGrant(skill skills.Skill, source Source) {
	if vc == nil {
		return
	}
	vc.AutomaticGrants.Skills[skill] = source
}

// AddAutomaticLanguageGrant adds an automatic language grant to the context
func (vc *ValidationContext) AddAutomaticLanguageGrant(lang languages.Language, source Source) {
	if vc == nil {
		return
	}
	vc.AutomaticGrants.Languages[lang] = source
}

// AddAutomaticToolGrant adds an automatic tool grant to the context (tool is still string until we have typed tools)
func (vc *ValidationContext) AddAutomaticToolGrant(tool string, source Source) {
	if vc == nil {
		return
	}
	vc.AutomaticGrants.Tools[tool] = source
}

// HasAutomaticSkillGrant checks if a skill is automatically granted
func (vc *ValidationContext) HasAutomaticSkillGrant(skill skills.Skill) (bool, Source) {
	if vc == nil {
		return false, ""
	}
	source, exists := vc.AutomaticGrants.Skills[skill]
	return exists, source
}

// HasAutomaticLanguageGrant checks if a language is automatically granted
func (vc *ValidationContext) HasAutomaticLanguageGrant(lang languages.Language) (bool, Source) {
	if vc == nil {
		return false, ""
	}
	source, exists := vc.AutomaticGrants.Languages[lang]
	return exists, source
}

// HasAutomaticToolGrant checks if a tool is automatically granted
func (vc *ValidationContext) HasAutomaticToolGrant(tool string) (bool, Source) {
	if vc == nil {
		return false, ""
	}
	source, exists := vc.AutomaticGrants.Tools[tool]
	return exists, source
}
