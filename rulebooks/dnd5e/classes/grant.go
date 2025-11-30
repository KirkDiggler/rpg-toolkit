// Package classes provides D&D 5e class mechanics
package classes

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Grant represents what a character receives at a given level.
// For intrinsic class properties (hit dice, saving throws), see Data.
type Grant struct {
	Level int // When this grant is given (1 = character creation)

	// Proficiencies
	ArmorProficiencies  []proficiencies.Armor
	WeaponProficiencies []proficiencies.Weapon
	ToolProficiencies   []proficiencies.Tool

	// Equipment
	Equipment []EquipmentItem

	// Ref-based grants
	Conditions []ConditionRef
	Features   []FeatureRef
	Spells     []SpellRef

	// Languages
	Languages []languages.Language
}

// ConditionRef represents a reference to a condition that should be created
type ConditionRef struct {
	Ref    string          `json:"ref"`              // "dnd5e:conditions:unarmored_defense"
	Config json.RawMessage `json:"config,omitempty"` // Type-specific configuration
}

// FeatureRef represents a reference to a feature that should be created
type FeatureRef struct {
	Ref    string          `json:"ref"`              // "dnd5e:features:rage"
	Config json.RawMessage `json:"config,omitempty"` // Type-specific configuration
}

// SpellRef represents a reference to a spell grant
type SpellRef struct {
	Ref        string `json:"ref"`         // "dnd5e:spells:fireball"
	SpellLevel int    `json:"spell_level"` // Spell level
}

// classGrants maps class IDs to their grants at each level
var classGrants = map[Class][]Grant{
	Barbarian: {
		{
			Level: 1,
			ArmorProficiencies: []proficiencies.Armor{
				proficiencies.ArmorLight,
				proficiencies.ArmorMedium,
				proficiencies.ArmorShields,
			},
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponMartial,
			},
			Features: []FeatureRef{
				{Ref: "dnd5e:features:rage"},
			},
			Conditions: []ConditionRef{
				{
					Ref:    "dnd5e:conditions:unarmored_defense",
					Config: json.RawMessage(`{"type":"barbarian"}`),
				},
			},
		},
	},

	Fighter: {
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
			Features: []FeatureRef{
				{Ref: "dnd5e:features:second_wind"},
			},
		},
	},

	Monk: {
		{
			Level: 1,
			WeaponProficiencies: []proficiencies.Weapon{
				proficiencies.WeaponSimple,
				proficiencies.WeaponShortsword,
			},
			Equipment: []EquipmentItem{
				{ID: shared.EquipmentID("dart"), Quantity: 10},
			},
			Conditions: []ConditionRef{
				{
					Ref:    "dnd5e:conditions:unarmored_defense",
					Config: json.RawMessage(`{"type":"monk"}`),
				},
			},
		},
	},
}

// GetGrants returns all grants for a class.
// Returns nil for classes not yet implemented in Phase 1 (Barbarian, Fighter, Monk).
func GetGrants(classID Class) []Grant {
	grants, ok := classGrants[classID]
	if !ok {
		return nil
	}
	return grants
}

// GetGrantsForLevel returns grants that apply at the specified level.
// Returns nil for classes not yet implemented in Phase 1.
func GetGrantsForLevel(classID Class, level int) []Grant {
	allGrants := GetGrants(classID)
	if allGrants == nil {
		return nil
	}

	result := make([]Grant, 0)
	for _, grant := range allGrants {
		if grant.Level == level {
			result = append(result, grant)
		}
	}

	return result
}
