// Package classes provides D&D 5e class grants and definitions
package classes

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Grant represents anything that can be granted at a level.
// Used by classes, races, backgrounds, and subclasses.
// One shape for everything - proficiencies, conditions, features, spells, equipment, languages.
type Grant struct {
	// Level indicates when this grant is given (1 = character creation, 2+ = level up)
	Level int

	// Core mechanics
	HitDice int // The die size (6, 8, 10, 12) - only meaningful at level 1

	// Proficiencies
	SavingThrows        []abilities.Ability
	ArmorProficiencies  []proficiencies.Armor
	WeaponProficiencies []proficiencies.Weapon
	ToolProficiencies   []proficiencies.Tool
	SkillProficiencies  []skills.Skill // e.g., Half-orc gets Intimidation

	// Equipment
	Equipment []EquipmentItem

	// Ref-based grants (conditions, features, spells)
	Conditions []ConditionRef
	Features   []FeatureRef
	Spells     []SpellRef

	// Languages
	Languages []languages.Language
}

// ConditionRef references a condition with optional configuration.
// The config is parsed by the condition itself via its factory.
type ConditionRef struct {
	// Ref is the condition reference in "module:type:value" format
	// e.g., "dnd5e:conditions:unarmored_defense"
	Ref string `json:"ref"`
	// Config is condition-specific configuration parsed by the condition factory
	Config json.RawMessage `json:"config,omitempty"`
}

// FeatureRef references a feature with optional configuration.
// The config is parsed by the feature itself via its factory.
type FeatureRef struct {
	// Ref is the feature reference in "module:type:value" format
	// e.g., "dnd5e:features:rage"
	Ref string `json:"ref"`
	// Config is feature-specific configuration parsed by the feature factory
	Config json.RawMessage `json:"config,omitempty"`
}

// SpellRef references a spell grant.
type SpellRef struct {
	// Ref is the spell reference in "module:type:value" format
	// e.g., "dnd5e:spells:bless"
	Ref string `json:"ref"`
	// SpellLevel is the spell's level (0 = cantrip)
	SpellLevel int `json:"spell_level"`
}

// GetGrants returns all grants for a class.
// This provides a unified way for classes to define what they grant at each level.
// Returns nil for invalid class IDs.
func GetGrants(classID Class) []Grant {
	switch classID {
	case Fighter:
		return getFighterGrants()
	case Barbarian:
		return getBarbarianGrants()
	default:
		// For classes not yet migrated, return grants from AutomaticGrants
		return getGrantsFromAutomatic(classID)
	}
}

// getFighterGrants returns all grants for the Fighter class
func getFighterGrants() []Grant {
	return []Grant{
		{
			Level:   1,
			HitDice: 10,
			SavingThrows: []abilities.Ability{
				abilities.STR,
				abilities.CON,
			},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorHeavy,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
			// Note: Fighting style is a CHOICE, not a grant
			// Note: Second Wind is granted but handled as a feature
			Features: []FeatureRef{
				{
					Ref:    "dnd5e:features:second_wind",
					Config: json.RawMessage(`{"uses": 1}`),
				},
			},
		},
	}
}

// getBarbarianGrants returns all grants for the Barbarian class
func getBarbarianGrants() []Grant {
	return []Grant{
		{
			Level:   1,
			HitDice: 12,
			SavingThrows: []abilities.Ability{
				abilities.STR,
				abilities.CON,
			},
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				// Note: Barbarians don't get heavy armor
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
			// Rage feature with level 1 config
			Features: []FeatureRef{
				{
					Ref:    "dnd5e:features:rage",
					Config: json.RawMessage(`{"uses": 2, "damage_bonus": 2}`),
				},
			},
			// Unarmored Defense condition (always active)
			Conditions: []ConditionRef{
				{
					Ref:    "dnd5e:conditions:unarmored_defense",
					Config: json.RawMessage(`{"variant": "barbarian"}`),
				},
			},
		},
	}
}

// getGrantsFromAutomatic converts legacy AutomaticGrants to the new Grant format.
// This allows backward compatibility while migrating classes one at a time.
func getGrantsFromAutomatic(classID Class) []Grant {
	automatic := GetAutomaticGrants(classID)
	if automatic == nil {
		return nil
	}

	equipment := make([]EquipmentItem, len(automatic.StartingEquipment))
	copy(equipment, automatic.StartingEquipment)

	return []Grant{
		{
			Level:               1,
			HitDice:             automatic.HitDice,
			SavingThrows:        automatic.SavingThrows,
			ArmorProficiencies:  automatic.ArmorProficiencies,
			WeaponProficiencies: automatic.WeaponProficiencies,
			ToolProficiencies:   automatic.ToolProficiencies,
			Equipment:           equipment,
		},
	}
}

// GetGrantsForLevel returns all grants applicable at or before the given level.
// This is useful for determining what a character of a given level should have.
func GetGrantsForLevel(classID Class, level int) []Grant {
	allGrants := GetGrants(classID)
	if allGrants == nil {
		return nil
	}

	result := make([]Grant, 0)
	for _, grant := range allGrants {
		if grant.Level <= level {
			result = append(result, grant)
		}
	}
	return result
}

// MergedGrants holds combined grants from multiple sources for character compilation.
type MergedGrants struct {
	HitDice             int
	SavingThrows        []abilities.Ability
	ArmorProficiencies  []proficiencies.Armor
	WeaponProficiencies []proficiencies.Weapon
	ToolProficiencies   []proficiencies.Tool
	SkillProficiencies  []skills.Skill
	Equipment           []EquipmentItem
	Conditions          []ConditionRef
	Features            []FeatureRef
	Spells              []SpellRef
	Languages           []languages.Language
}

// MergeGrants combines multiple grants into a single merged structure.
// The first grant's HitDice is used (typically level 1 grant).
func MergeGrants(grants []Grant) *MergedGrants {
	if len(grants) == 0 {
		return nil
	}

	result := &MergedGrants{
		HitDice:             grants[0].HitDice,
		SavingThrows:        make([]abilities.Ability, 0),
		ArmorProficiencies:  make([]proficiencies.Armor, 0),
		WeaponProficiencies: make([]proficiencies.Weapon, 0),
		ToolProficiencies:   make([]proficiencies.Tool, 0),
		SkillProficiencies:  make([]skills.Skill, 0),
		Equipment:           make([]EquipmentItem, 0),
		Conditions:          make([]ConditionRef, 0),
		Features:            make([]FeatureRef, 0),
		Spells:              make([]SpellRef, 0),
		Languages:           make([]languages.Language, 0),
	}

	for _, grant := range grants {
		result.SavingThrows = append(result.SavingThrows, grant.SavingThrows...)
		result.ArmorProficiencies = append(result.ArmorProficiencies, grant.ArmorProficiencies...)
		result.WeaponProficiencies = append(result.WeaponProficiencies, grant.WeaponProficiencies...)
		result.ToolProficiencies = append(result.ToolProficiencies, grant.ToolProficiencies...)
		result.SkillProficiencies = append(result.SkillProficiencies, grant.SkillProficiencies...)
		result.Equipment = append(result.Equipment, grant.Equipment...)
		result.Conditions = append(result.Conditions, grant.Conditions...)
		result.Features = append(result.Features, grant.Features...)
		result.Spells = append(result.Spells, grant.Spells...)
		result.Languages = append(result.Languages, grant.Languages...)
	}

	return result
}
