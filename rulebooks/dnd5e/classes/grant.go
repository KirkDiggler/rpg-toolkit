// Package classes provides D&D 5e class grants and definitions
package classes

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Grant represents what a character receives at a given level.
// Used by classes, races, backgrounds, and subclasses.
// One shape for everything - proficiencies, conditions, features, spells, equipment, languages.
//
// For intrinsic class properties (hit dice, saving throws), see classes.Data.
// Grant = "what you get" (features, conditions, proficiencies granted at levels)
// Data = "what you are" (intrinsic properties like hit dice, saving throws)
type Grant struct {
	// Level indicates when this grant is given (1 = character creation, 2+ = level up)
	Level int

	// Proficiencies
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
	// Ref is the typed condition reference (e.g., dnd5e:conditions:unarmored_defense)
	Ref *core.Ref `json:"ref"`
	// Config is condition-specific configuration parsed by the condition factory
	Config json.RawMessage `json:"config,omitempty"`
}

// FeatureRef references a feature with optional configuration.
// The config is parsed by the feature itself via its factory.
type FeatureRef struct {
	// Ref is the typed feature reference (e.g., dnd5e:features:rage)
	Ref *core.Ref `json:"ref"`
	// Config is feature-specific configuration parsed by the feature factory
	Config json.RawMessage `json:"config,omitempty"`
}

// SpellRef references a spell grant.
type SpellRef struct {
	// Ref is the typed spell reference (e.g., dnd5e:spells:bless)
	Ref *core.Ref `json:"ref"`
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
	case Monk:
		return getMonkGrants()
	default:
		// Unmigrated classes return nil - add them explicitly above
		return nil
	}
}

// getFighterGrants returns all grants for the Fighter class.
// Note: HitDice and SavingThrows are intrinsic class properties in classes.Data,
// not level-based grants.
func getFighterGrants() []Grant {
	return []Grant{
		{
			Level: 1,
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
					Ref:    &core.Ref{Module: "dnd5e", Type: "features", ID: "second_wind"},
					Config: json.RawMessage(`{"uses": 1}`),
				},
			},
		},
	}
}

// getBarbarianGrants returns all grants for the Barbarian class.
// Note: HitDice and SavingThrows are intrinsic class properties in classes.Data,
// not level-based grants.
func getBarbarianGrants() []Grant {
	return []Grant{
		{
			Level: 1,
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
					Ref:    &core.Ref{Module: "dnd5e", Type: "features", ID: "rage"},
					Config: json.RawMessage(`{"uses": 2, "damage_bonus": 2}`),
				},
			},
			// Unarmored Defense condition (always active)
			Conditions: []ConditionRef{
				{
					Ref:    &core.Ref{Module: "dnd5e", Type: "conditions", ID: "unarmored_defense"},
					Config: json.RawMessage(`{"variant": "barbarian"}`),
				},
			},
		},
	}
}

// getMonkGrants returns all grants for the Monk class.
// Note: HitDice and SavingThrows are intrinsic class properties in classes.Data,
// not level-based grants.
func getMonkGrants() []Grant {
	return []Grant{
		{
			Level: 1,
			// Monks have NO armor proficiencies (they don't wear armor)
			ArmorProficiencies: []proficiencies.Armor{},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponShortsword,
			},
			// Note: Tool proficiency (artisan's tool OR musical instrument) is a CHOICE, not a grant
			// Note: Martial Arts feature does not exist yet in the codebase
			// Unarmored Defense condition (monk variant - uses WIS instead of CON)
			Conditions: []ConditionRef{
				{
					Ref:    &core.Ref{Module: "dnd5e", Type: "conditions", ID: "unarmored_defense"},
					Config: json.RawMessage(`{"variant": "monk"}`),
				},
			},
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
// Note: HitDice and SavingThrows are not included here - they are intrinsic class
// properties available via classes.GetData(classID). This struct only contains
// "what you get" (things granted at levels), not "what you are" (intrinsic properties).
type MergedGrants struct {
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
// Note: HitDice and SavingThrows are not merged - use classes.GetData(classID)
// to get these intrinsic class properties.
func MergeGrants(grants []Grant) *MergedGrants {
	if len(grants) == 0 {
		return nil
	}

	result := &MergedGrants{
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
