// Package shared provides common types used across the D&D 5e rulebook
package shared

// AbilityScores holds the six ability scores used throughout D&D 5e
type AbilityScores struct {
	Strength     int
	Dexterity    int
	Constitution int
	Intelligence int
	Wisdom       int
	Charisma     int
}

// ProficiencyLevel represents expertise levels
type ProficiencyLevel int

const (
	// NotProficient indicates no proficiency
	NotProficient ProficiencyLevel = iota
	// Proficient indicates normal proficiency
	Proficient
	// Expertise indicates double proficiency bonus
	Expertise
)

// ResetType defines when a resource resets
type ResetType string

const (
	// ShortRest resets on a short rest
	ShortRest ResetType = "short_rest"
	// LongRest resets on a long rest
	LongRest ResetType = "long_rest"
	// Dawn resets at dawn
	Dawn ResetType = "dawn"
)

// ChoiceCategory represents different types of choices during creation
type ChoiceCategory string

const (
	// ChoiceName represents character name choice
	ChoiceName ChoiceCategory = "name"
	// ChoiceRace represents race selection
	ChoiceRace ChoiceCategory = "race"
	// ChoiceSubrace represents subrace selection
	ChoiceSubrace ChoiceCategory = "subrace"
	// ChoiceClass represents class selection
	ChoiceClass ChoiceCategory = "class"
	// ChoiceBackground represents background selection
	ChoiceBackground ChoiceCategory = "background"
	// ChoiceAbilityScores represents ability score selection
	ChoiceAbilityScores ChoiceCategory = "ability_scores"
	// ChoiceSkills represents skill proficiency selection
	ChoiceSkills ChoiceCategory = "skills"
	// ChoiceLanguages represents language selection
	ChoiceLanguages ChoiceCategory = "languages"
	// ChoiceEquipment represents equipment selection
	ChoiceEquipment ChoiceCategory = "equipment"
	// ChoiceSpells represents spell selection
	ChoiceSpells ChoiceCategory = "spells"
	// ChoiceCantrips represents cantrip selection
	ChoiceCantrips ChoiceCategory = "cantrips"
	// ChoiceFightingStyle represents fighting style selection
	ChoiceFightingStyle ChoiceCategory = "fighting_style"
)

// Proficiencies tracks what the character is proficient with
type Proficiencies struct {
	Armor   []string
	Weapons []string
	Tools   []string
}

// DeathSaves tracks death saving throws
type DeathSaves struct {
	Successes int
	Failures  int
}

// Background represents character background
type Background struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Skill proficiencies (usually 2)
	SkillProficiencies []string `json:"skill_proficiencies"`

	// Languages
	Languages      []string    `json:"languages,omitempty"`
	LanguageChoice *ChoiceData `json:"language_choice,omitempty"`

	// Tool proficiencies
	ToolProficiencies []string    `json:"tool_proficiencies,omitempty"`
	ToolChoice        *ChoiceData `json:"tool_choice,omitempty"`

	// Starting equipment
	Equipment []string `json:"equipment"`

	// Feature
	Feature FeatureData `json:"feature"`
}

// ChoiceData represents any choice in character creation
type ChoiceData struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Choose      int      `json:"choose"`
	From        []string `json:"from"`
	Description string   `json:"description,omitempty"`
}

// FeatureData represents a feature or trait
type FeatureData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
