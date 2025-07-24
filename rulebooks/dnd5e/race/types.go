// Package race provides D&D 5e race data structures and functionality
package race

// Data contains all the data needed to define a D&D 5e race
type Data struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Physical characteristics
	Size  string `json:"size"`  // Small, Medium, Large
	Speed int    `json:"speed"` // Base walking speed

	// Ability score improvements
	AbilityScoreIncreases map[string]int `json:"ability_score_increases"`

	// Features and traits
	Traits []TraitData `json:"traits"`

	// Proficiencies
	SkillProficiencies  []string `json:"skill_proficiencies"`
	WeaponProficiencies []string `json:"weapon_proficiencies"`
	ToolProficiencies   []string `json:"tool_proficiencies"`

	// Languages
	Languages      []string    `json:"languages"`
	LanguageChoice *ChoiceData `json:"language_choice,omitempty"`

	// Other choices
	SkillChoice *ChoiceData `json:"skill_choice,omitempty"`
	ToolChoice  *ChoiceData `json:"tool_choice,omitempty"`

	// Subraces
	Subraces []SubraceData `json:"subraces,omitempty"`
}

// SubraceData defines a subrace variant
type SubraceData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Additional ability score improvements
	AbilityScoreIncreases map[string]int `json:"ability_score_increases"`

	// Additional traits
	Traits []TraitData `json:"traits"`

	// Additional proficiencies
	WeaponProficiencies []string `json:"weapon_proficiencies,omitempty"`
	ArmorProficiencies  []string `json:"armor_proficiencies,omitempty"`

	// Spells (for races like Tiefling, Drow)
	Spells []SpellProgressionData `json:"spells,omitempty"`
}

// TraitData represents a racial trait or feature
type TraitData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Some traits grant choices
	Choice *ChoiceData `json:"choice,omitempty"`
}

// ChoiceData represents a choice the player must make
type ChoiceData struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`   // "skill", "language", "tool", etc.
	Choose      int      `json:"choose"` // Number to choose
	From        []string `json:"from"`   // Options to choose from
	Description string   `json:"description,omitempty"`
}

// SpellProgressionData for racial spells
type SpellProgressionData struct {
	Level   int    `json:"level"` // Character level when gained
	SpellID string `json:"spell_id"`
	Uses    string `json:"uses"` // "1/long_rest", "at_will", etc.
}

// LoadFromData creates a Race from RaceData
func LoadFromData(data Data) *Race {
	return &Race{
		data: data,
	}
}

// Race is the domain model with behavior
type Race struct {
	data Data
}

// ID returns the race ID
func (r *Race) ID() string {
	return r.data.ID
}

// Name returns the race name
func (r *Race) Name() string {
	return r.data.Name
}

// Speed returns base walking speed
func (r *Race) Speed() int {
	return r.data.Speed
}

// Size returns the creature size
func (r *Race) Size() string {
	return r.data.Size
}

// GetAbilityScoreIncreases returns ability score improvements
func (r *Race) GetAbilityScoreIncreases() map[string]int {
	return r.data.AbilityScoreIncreases
}

// GetTraits returns racial traits
func (r *Race) GetTraits() []TraitData {
	return r.data.Traits
}

// GetLanguages returns known languages
func (r *Race) GetLanguages() []string {
	return r.data.Languages
}

// GetChoices returns all choices this race requires
func (r *Race) GetChoices() []ChoiceData {
	var choices []ChoiceData

	if r.data.LanguageChoice != nil {
		choices = append(choices, *r.data.LanguageChoice)
	}
	if r.data.SkillChoice != nil {
		choices = append(choices, *r.data.SkillChoice)
	}
	if r.data.ToolChoice != nil {
		choices = append(choices, *r.data.ToolChoice)
	}

	// Add trait choices
	for _, trait := range r.data.Traits {
		if trait.Choice != nil {
			choices = append(choices, *trait.Choice)
		}
	}

	return choices
}

// ToData converts back to data form
func (r *Race) ToData() Data {
	return r.data
}
