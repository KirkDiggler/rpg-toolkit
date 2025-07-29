// Package class provides D&D 5e class data structures and functionality
package class

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"

// Data contains all the data needed to define a D&D 5e class
type Data struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Core mechanics
	HitDice           int `json:"hit_dice"`             // d6, d8, d10, d12
	HitPointsPerLevel int `json:"hit_points_per_level"` // Average HP per level

	// Proficiencies
	ArmorProficiencies  []string `json:"armor_proficiencies"`
	WeaponProficiencies []string `json:"weapon_proficiencies"`
	ToolProficiencies   []string `json:"tool_proficiencies"`
	SavingThrows        []constants.Ability `json:"saving_throws"` // Two ability scores

	// Skills
	SkillProficiencyCount int                `json:"skill_proficiency_count"`
	SkillOptions          []constants.Skill `json:"skill_options"` // Available skills to choose from

	// Starting equipment
	StartingEquipment []EquipmentData       `json:"starting_equipment"`
	EquipmentChoices  []EquipmentChoiceData `json:"equipment_choices"`

	// Class features by level
	Features map[int][]FeatureData `json:"features"`

	// Spellcasting (if applicable)
	Spellcasting *SpellcastingData `json:"spellcasting,omitempty"`

	// Resources (rage, ki, etc.)
	Resources []ResourceData `json:"resources,omitempty"`

	// Subclasses
	SubclassLevel int            `json:"subclass_level"` // Level when subclass is chosen
	Subclasses    []SubclassData `json:"subclasses"`
}

// FeatureData represents a class feature
type FeatureData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	Description string `json:"description"`

	// Some features grant choices (Fighting Style, Expertise)
	Choice *ChoiceData `json:"choice,omitempty"`
}

// SpellcastingData for spellcasting classes
type SpellcastingData struct {
	Ability         constants.Ability `json:"ability"`                    // Intelligence, Wisdom, Charisma
	PreparedFormula string        `json:"prepared_formula,omitempty"` // e.g., "wisdom_modifier + cleric_level"
	RitualCasting   bool          `json:"ritual_casting"`
	SpellsKnown     map[int]int   `json:"spells_known,omitempty"`   // Level -> number known
	CantripsKnown   map[int]int   `json:"cantrips_known,omitempty"` // Level -> number known
	SpellSlots      map[int][]int `json:"spell_slots"`              // Level -> slots per spell level
}

// ResourceData for class resources
type ResourceData struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	MaxFormula   string      `json:"max_formula"` // e.g., "level", "1 + charisma_modifier"
	UsesPerLevel map[int]int `json:"uses_per_level,omitempty"`
	ResetOn      string      `json:"reset_on"` // "short_rest", "long_rest"
}

// EquipmentData for starting equipment
type EquipmentData struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// EquipmentChoiceData for equipment choices
type EquipmentChoiceData struct {
	ID      string            `json:"id"`
	Choose  int               `json:"choose"`
	Options []EquipmentOption `json:"options"`
}

// EquipmentOption represents one choice option
type EquipmentOption struct {
	ID    string          `json:"id"`
	Items []EquipmentData `json:"items"`
}

// SubclassData represents a class archetype
type SubclassData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Additional features by level
	Features map[int][]FeatureData `json:"features"`

	// Additional spells (for some subclasses)
	AdditionalSpells map[int][]string `json:"additional_spells,omitempty"`
}

// ChoiceData represents a choice the player must make
type ChoiceData struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"` // "skill", "fighting_style", "expertise", etc.
	Choose      int      `json:"choose"`
	From        []string `json:"from"`
	Description string   `json:"description,omitempty"`
}

// LoadFromData creates a Class from ClassData
func LoadFromData(data Data) *Class {
	return &Class{
		data: data,
	}
}

// Class is the domain model with behavior
type Class struct {
	data Data
}

// ID returns the class ID
func (c *Class) ID() string {
	return c.data.ID
}

// Name returns the class name
func (c *Class) Name() string {
	return c.data.Name
}

// HitDice returns the hit die size
func (c *Class) HitDice() int {
	return c.data.HitDice
}

// GetSavingThrowProficiencies returns saving throw proficiencies
func (c *Class) GetSavingThrowProficiencies() []constants.Ability {
	return c.data.SavingThrows
}

// GetSkillOptions returns available skill choices
func (c *Class) GetSkillOptions() (int, []constants.Skill) {
	return c.data.SkillProficiencyCount, c.data.SkillOptions
}

// GetFeaturesAtLevel returns features gained at a specific level
func (c *Class) GetFeaturesAtLevel(level int) []FeatureData {
	return c.data.Features[level]
}

// GetChoicesAtLevel returns all choices required at a level
func (c *Class) GetChoicesAtLevel(level int) []ChoiceData {
	var choices []ChoiceData

	// Starting choices
	if level == 1 {
		// Skills are always a choice at level 1
		skillStrings := make([]string, len(c.data.SkillOptions))
		for i, skill := range c.data.SkillOptions {
			skillStrings[i] = string(skill)
		}
		choices = append(choices, ChoiceData{
			ID:     "class_skills",
			Type:   "skill",
			Choose: c.data.SkillProficiencyCount,
			From:   skillStrings,
		})

		// Equipment choices
		for _, eqChoice := range c.data.EquipmentChoices {
			choices = append(choices, ChoiceData{
				ID:     eqChoice.ID,
				Type:   "equipment",
				Choose: eqChoice.Choose,
				// Note: From would need to be converted from EquipmentOption
			})
		}
	}

	// Feature choices
	for _, feature := range c.data.Features[level] {
		if feature.Choice != nil {
			choices = append(choices, *feature.Choice)
		}
	}

	return choices
}

// IsSpellcaster returns true if this class can cast spells
func (c *Class) IsSpellcaster() bool {
	return c.data.Spellcasting != nil
}

// GetSpellcastingAbility returns the spellcasting ability score
func (c *Class) GetSpellcastingAbility() constants.Ability {
	if c.data.Spellcasting == nil {
		return ""
	}
	return c.data.Spellcasting.Ability
}

// ToData converts back to data form
func (c *Class) ToData() Data {
	return c.data
}
