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

	// Subclass choice (required at specific levels)
	Subclass *SubclassRequirement `json:"subclass,omitempty"`
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

// SubclassRequirement defines subclass choice requirements
type SubclassRequirement struct {
	Options []classes.Subclass `json:"options"` // Available subclasses
	Label   string             `json:"label"`   // e.g., "Choose your Martial Archetype"
}

// API Functions - These are the main entry points

// GetClassRequirements returns the requirements for a specific class at level 1
func GetClassRequirements(classID classes.Class) *Requirements {
	// Implementation in requirements.go
	return getClassRequirementsInternal(classID)
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

// GetRequirements returns combined requirements for character creation
func GetRequirements(classID classes.Class, raceID races.Race) *Requirements {
	// This combines class and race requirements for character creation
	classReqs := GetClassRequirements(classID)
	raceReqs := GetRaceRequirements(raceID)

	// Merge requirements (implementation will be in requirements.go)
	return mergeRequirements(classReqs, raceReqs)
}

// Validate provides validation with severity levels and typed submissions
func Validate(
	classID classes.Class,
	raceID races.Race,
	backgroundID backgrounds.Background,
	level int,
	submissions *TypedSubmissions,
	context *ValidationContext,
) *ValidationResult {
	validator := NewValidator(context)
	return validator.ValidateAll(classID, raceID, backgroundID, level, submissions)
}

// ValidateWithSubclass provides validation with subclass-specific requirements
func ValidateWithSubclass(
	classID classes.Class,
	subclassID classes.Subclass,
	raceID races.Race,
	backgroundID backgrounds.Background,
	level int,
	submissions *TypedSubmissions,
	context *ValidationContext,
) *ValidationResult {
	validator := NewValidator(context)
	return validator.ValidateAllWithSubclass(classID, subclassID, raceID, backgroundID, level, submissions)
}

// ValidateClassChoices validates choices for a specific class
func ValidateClassChoices(
	classID classes.Class,
	level int,
	submissions *TypedSubmissions,
	context *ValidationContext,
) *ValidationResult {
	validator := NewValidator(context)
	return validator.ValidateClassChoices(classID, level, submissions)
}

// ValidateClassChoicesWithSubclass validates choices for a specific class and subclass
func ValidateClassChoicesWithSubclass(
	classID classes.Class,
	subclassID classes.Subclass,
	level int,
	submissions *TypedSubmissions,
	context *ValidationContext,
) *ValidationResult {
	validator := NewValidator(context)
	return validator.ValidateClassChoicesWithSubclass(classID, subclassID, level, submissions)
}

// ValidateRaceChoices validates choices for a specific race
func ValidateRaceChoices(
	raceID races.Race,
	submissions *TypedSubmissions,
	context *ValidationContext,
) *ValidationResult {
	validator := NewValidator(context)
	return validator.ValidateRaceChoices(raceID, submissions)
}
