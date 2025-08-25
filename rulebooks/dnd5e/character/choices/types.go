// Package choices provides the complete character choice system for D&D 5e.
package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Requirements represents what choices need to be made
type Requirements struct {
	// Skills that need to be chosen
	Skills *SkillRequirement `json:"skills,omitempty"`

	// Spells and cantrips
	Cantrips *SpellRequirement `json:"cantrips,omitempty"`
	Spells   *SpellRequirement `json:"spells,omitempty"`

	// Equipment choices
	Equipment []*EquipmentRequirement `json:"equipment,omitempty"`

	// Proficiency choices
	Languages   *LanguageRequirement   `json:"languages,omitempty"`
	Tools       *ToolRequirement       `json:"tools,omitempty"`
	Instruments *InstrumentRequirement `json:"instruments,omitempty"`

	// Class-specific choices
	FightingStyle *FightingStyleRequirement `json:"fighting_style,omitempty"`
	Expertise     *ExpertiseRequirement     `json:"expertise,omitempty"`

	// Racial choices
	DraconicAncestry *AncestryRequirement `json:"draconic_ancestry,omitempty"`

	// Level-up choices
	AbilityScoreImprovement *ASIRequirement  `json:"ability_score_improvement,omitempty"`
	Feat                    *FeatRequirement `json:"feat,omitempty"`
}

// SkillRequirement defines skill choice requirements
type SkillRequirement struct {
	Count   int            `json:"count"`
	Options []skills.Skill `json:"options,omitempty"` // nil means any skill
	Label   string         `json:"label"`             // e.g., "Choose 2 skills"
}

// SpellRequirement defines spell/cantrip choice requirements
type SpellRequirement struct {
	Count int    `json:"count"`
	Level int    `json:"level"` // 0 for cantrips, 1+ for spells
	Label string `json:"label"` // e.g., "Choose 2 cantrips from the Wizard spell list"
}

// EquipmentRequirement defines equipment choice requirements
type EquipmentRequirement struct {
	Choose  int               `json:"choose"` // How many options to pick (usually 1)
	Options []EquipmentOption `json:"options"`
	Label   string            `json:"label"` // e.g., "(a) chain mail or (b) leather armor"
}

// EquipmentOption represents one equipment choice option
type EquipmentOption struct {
	ID    string     `json:"id"`    // Unique identifier for this option
	Items []ItemSpec `json:"items"` // What you get if you choose this
	Label string     `json:"label"` // e.g., "Chain mail"
}

// ItemSpec represents an item in an equipment option
type ItemSpec struct {
	Type     string `json:"type"`     // "weapon", "armor", "pack", etc.
	ID       string `json:"id"`       // The specific item or category
	Quantity int    `json:"quantity"` // How many (default 1)
}

// LanguageRequirement defines language choice requirements
type LanguageRequirement struct {
	Count   int                  `json:"count"`
	Options []languages.Language `json:"options,omitempty"` // nil means any language
	Label   string               `json:"label"`
}

// ToolRequirement defines tool proficiency choice requirements
type ToolRequirement struct {
	Count   int      `json:"count"`
	Options []string `json:"options"`
	Label   string   `json:"label"`
}

// InstrumentRequirement defines musical instrument choice requirements
type InstrumentRequirement struct {
	Count   int      `json:"count"`
	Options []string `json:"options,omitempty"` // nil means any instrument
	Label   string   `json:"label"`
}

// FightingStyleRequirement defines fighting style choice requirements
type FightingStyleRequirement struct {
	Options []FightingStyle `json:"options"`
	Label   string          `json:"label"`
}

// ExpertiseRequirement defines expertise choice requirements
type ExpertiseRequirement struct {
	Count int    `json:"count"`
	Label string `json:"label"` // e.g., "Choose 2 skills or thieves' tools for expertise"
}

// AncestryRequirement defines draconic ancestry choice requirements
type AncestryRequirement struct {
	Options []AncestryID `json:"options"`
	Label   string       `json:"label"`
}

// ASIRequirement defines ability score improvement requirements
type ASIRequirement struct {
	Points int    `json:"points"` // Usually 2
	Label  string `json:"label"`
}

// FeatRequirement defines feat choice requirements
type FeatRequirement struct {
	Label string `json:"label"`
}

// ValidationResult represents the result of validating choices
type ValidationResult struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError   `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
}

// ValidationError represents a validation error that blocks character creation
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationWarning represents a validation warning (informational, doesn't block)
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"` // e.g., "duplicate_skill", "missing_prerequisite"
}

// Submissions represents player choices submitted for validation
type Submissions map[string][]string

// API Functions - These are the main entry points

// GetClassRequirements returns the requirements for a specific class at a given level
func GetClassRequirements(classID classes.Class, level int) *Requirements {
	// Implementation in requirements.go
	return getClassRequirementsInternal(classID, level)
}

// GetRaceRequirements returns the requirements for a specific race
func GetRaceRequirements(raceID races.Race) *Requirements {
	// Implementation in requirements.go
	return getRaceRequirementsInternal(raceID)
}

// GetBackgroundRequirements returns the requirements for a specific background
func GetBackgroundRequirements(backgroundID backgrounds.Background) *Requirements {
	// Most backgrounds have no choices, just grants
	// Implementation in requirements.go
	return getBackgroundRequirementsInternal(backgroundID)
}

// GetRequirements returns combined requirements for a character at a specific level
func GetRequirements(classID classes.Class, raceID races.Race, level int) *Requirements {
	// This combines class and race requirements for level-up scenarios
	classReqs := GetClassRequirements(classID, level)
	raceReqs := GetRaceRequirements(raceID)

	// Merge requirements (implementation will be in requirements.go)
	return mergeRequirements(classReqs, raceReqs)
}

// ValidateClassChoices validates choices for a specific class
func ValidateClassChoices(classID classes.Class, level int, submissions Submissions) *ValidationResult {
	// Implementation in validator.go
	return validateClassChoicesInternal(classID, level, submissions)
}

// ValidateRaceChoices validates choices for a specific race
func ValidateRaceChoices(raceID races.Race, submissions Submissions) *ValidationResult {
	// Implementation in validator.go
	return validateRaceChoicesInternal(raceID, submissions)
}

// ValidateBackgroundChoices validates choices for a specific background
func ValidateBackgroundChoices(backgroundID backgrounds.Background, submissions Submissions) *ValidationResult {
	// Implementation in validator.go
	return validateBackgroundChoicesInternal(backgroundID, submissions)
}

// Validate validates all choices for a character (used for level-up and cross-source validation)
func Validate(classID classes.Class, raceID races.Race, level int, submissions Submissions) *ValidationResult {
	// Implementation in validator.go
	return validateAllInternal(classID, raceID, level, submissions)
}
