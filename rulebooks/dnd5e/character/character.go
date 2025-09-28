// Package character provides D&D 5e character creation and management functionality
package character

import (
	"encoding/json"
	"maps"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Character represents a playable D&D 5e character
// This is the domain model used during gameplay
type Character struct {
	// Identity
	id       string
	playerID string
	name     string

	// Core attributes
	level            int
	proficiencyBonus int

	// Race and class
	raceID     races.Race
	subraceID  races.Subrace
	classID    classes.Class
	subclassID classes.Subclass

	// Ability scores (includes racial modifiers)
	abilityScores shared.AbilityScores

	// Combat stats
	hitPoints    int
	maxHitPoints int
	armorClass   int
	hitDice      int // Size of hit die (d6, d8, d10, d12)

	// Proficiencies and skills
	skills       map[skills.Skill]shared.ProficiencyLevel
	savingThrows map[abilities.Ability]shared.ProficiencyLevel
	languages    []languages.Language

	// Equipment and resources
	inventory      []InventoryItem
	spellSlots     map[int]SpellSlotData
	classResources map[shared.ClassResourceType]ResourceData

	// Features (rage, second wind, etc)
	features []features.Feature
}

// GetID returns the character's unique identifier
func (c *Character) GetID() string {
	return c.id
}

// GetName returns the character's name
func (c *Character) GetName() string {
	return c.name
}

// GetLevel returns the character's level
func (c *Character) GetLevel() int {
	return c.level
}

// GetAbilityScore returns the character's ability score (including racial modifiers)
func (c *Character) GetAbilityScore(ability abilities.Ability) int {
	return c.abilityScores[ability]
}

// GetAbilityModifier returns the modifier for an ability score
func (c *Character) GetAbilityModifier(ability abilities.Ability) int {
	return c.abilityScores.Modifier(ability)
}

// GetProficiencyBonus returns the character's proficiency bonus
func (c *Character) GetProficiencyBonus() int {
	return c.proficiencyBonus
}

// GetSkillModifier returns the total modifier for a skill check
func (c *Character) GetSkillModifier(skill skills.Skill) int {
	ability := skills.Ability(skill)
	modifier := c.GetAbilityModifier(ability)

	if level, hasProficiency := c.skills[skill]; hasProficiency {
		switch level {
		case shared.Proficient:
			modifier += c.proficiencyBonus
		case shared.Expert:
			modifier += c.proficiencyBonus * 2
		}
	}

	return modifier
}

// GetSavingThrowModifier returns the total modifier for a saving throw
func (c *Character) GetSavingThrowModifier(ability abilities.Ability) int {
	modifier := c.GetAbilityModifier(ability)

	if level, hasProficiency := c.savingThrows[ability]; hasProficiency && level == shared.Proficient {
		modifier += c.proficiencyBonus
	}

	return modifier
}

// GetFeatures returns all character features
func (c *Character) GetFeatures() []features.Feature {
	return c.features
}

// GetFeature returns a specific feature by ID
func (c *Character) GetFeature(id string) features.Feature {
	for _, f := range c.features {
		if f.GetID() == id {
			return f
		}
	}
	return nil
}

// ToData converts the character to its persistent data form
func (c *Character) ToData() *Data {
	data := &Data{
		ID:               c.id,
		PlayerID:         c.playerID,
		Name:             c.name,
		Level:            c.level,
		ProficiencyBonus: c.proficiencyBonus,
		RaceID:           c.raceID,
		SubraceID:        c.subraceID,
		ClassID:          c.classID,
		SubclassID:       c.subclassID,
		AbilityScores:    c.abilityScores,
		HitPoints:        c.hitPoints,
		MaxHitPoints:     c.maxHitPoints,
		ArmorClass:       c.armorClass,
		Skills:           maps.Clone(c.skills),
		SavingThrows:     maps.Clone(c.savingThrows),
		UpdatedAt:        time.Now(),
	}

	// Convert inventory to data
	data.Inventory = make([]InventoryItemData, 0, len(c.inventory))
	for _, item := range c.inventory {
		data.Inventory = append(data.Inventory, item.ToData())
	}

	// Convert languages to strings
	// TODO: Convert typed language constants to strings

	// Copy spell slots map directly since SpellSlotData is already the data type
	data.SpellSlots = maps.Clone(c.spellSlots)

	// Copy class resources map directly since ResourceData is already the data type
	data.ClassResources = maps.Clone(c.classResources)

	// Convert features to persisted JSON
	data.Features = make([]json.RawMessage, 0, len(c.features))
	for _, feature := range c.features {
		// Use the feature's ToJSON method to get the serialized form
		jsonData, err := feature.ToJSON()
		if err != nil {
			// Skip features that can't be serialized
			// TODO: Consider how to handle serialization errors
			continue
		}
		// The feature's ToJSON already includes the fully qualified ref
		data.Features = append(data.Features, jsonData)
	}

	return data
}
