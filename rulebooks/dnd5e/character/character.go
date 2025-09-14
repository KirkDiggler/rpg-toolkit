// Package character provides D&D 5e character creation and management functionality
package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
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
	spellSlots     map[int]SpellSlot
	classResources map[shared.ClassResourceType]Resource
}

// SpellSlot tracks spell slot usage
type SpellSlot struct {
	Max  int
	Used int
}

// Resource represents a class resource like Ki points or Superiority Dice
type Resource struct {
	Name    string
	Max     int
	Current int
	Resets  shared.ResetType
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
