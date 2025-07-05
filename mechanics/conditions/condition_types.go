// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

// ConditionType represents a specific type of condition.
// Games define their own condition types (e.g., "blinded", "poisoned", "burning").
type ConditionType string

// EffectType represents a type of mechanical effect.
type EffectType string

// Mechanical effect types that games can use
const (
	EffectAdvantage      EffectType = "advantage"
	EffectDisadvantage   EffectType = "disadvantage"
	EffectAutoFail       EffectType = "auto_fail"
	EffectAutoSucceed    EffectType = "auto_succeed"
	EffectImmunity       EffectType = "immunity"
	EffectSpeedReduction EffectType = "speed_reduction"
	EffectSpeedZero      EffectType = "speed_zero"
	EffectIncapacitated  EffectType = "incapacitated"
	EffectNoReactions    EffectType = "no_reactions"
	EffectVulnerability  EffectType = "vulnerability"
	EffectResistance     EffectType = "resistance"
	EffectCantSpeak      EffectType = "cant_speak"
	EffectCantHear       EffectType = "cant_hear"
	EffectCantSee        EffectType = "cant_see"
	EffectDropItems      EffectType = "drop_items"
	EffectMaxHPReduction EffectType = "max_hp_reduction"
)

// EffectTarget represents what the effect applies to.
type EffectTarget string

// Effect targets that games can use
const (
	TargetAttackRolls    EffectTarget = "attack_rolls"
	TargetAttacksAgainst EffectTarget = "attacks_against"
	TargetSavingThrows   EffectTarget = "saving_throws"
	TargetAbilityChecks  EffectTarget = "ability_checks"
	TargetAllSaves       EffectTarget = "all_saves"
	TargetAllChecks      EffectTarget = "all_checks"
	TargetDexSaves       EffectTarget = "dex_saves"
	TargetStrSaves       EffectTarget = "str_saves"
	TargetSight          EffectTarget = "sight"
	TargetHearing        EffectTarget = "hearing"
	TargetMovement       EffectTarget = "movement"
	TargetActions        EffectTarget = "actions"
	TargetReactions      EffectTarget = "reactions"
	TargetSpeech         EffectTarget = "speech"
	TargetDamage         EffectTarget = "damage"
)

// ConditionEffect represents a mechanical effect of a condition.
type ConditionEffect struct {
	Type   EffectType   // What kind of effect this is
	Target EffectTarget // What the effect applies to
	Value  interface{}  // Optional value (e.g., reduction amount, damage type)
}

// ConditionDefinition defines a condition type and its mechanical effects.
type ConditionDefinition struct {
	Type        ConditionType     // Unique identifier for this condition
	Name        string            // Display name
	Description string            // Human-readable description
	Effects     []ConditionEffect // Mechanical effects this condition applies
	Immunities  []ConditionType   // Conditions this prevents
	Includes    []ConditionType   // Other conditions this automatically includes
	Suppresses  []ConditionType   // Weaker conditions this overrides
}

// conditionDefinitions holds registered condition definitions.
// Games register their condition types using RegisterConditionDefinition.
var conditionDefinitions = make(map[ConditionType]*ConditionDefinition)

// RegisterConditionDefinition registers a condition definition for use by the system.
// This allows games to define their own condition types and effects.
func RegisterConditionDefinition(def *ConditionDefinition) {
	conditionDefinitions[def.Type] = def
}

// GetConditionDefinition returns the definition for a condition type.
func GetConditionDefinition(condType ConditionType) (*ConditionDefinition, bool) {
	def, exists := conditionDefinitions[condType]
	return def, exists
}

// GetAllConditionDefinitions returns all registered condition definitions.
func GetAllConditionDefinitions() map[ConditionType]*ConditionDefinition {
	result := make(map[ConditionType]*ConditionDefinition)
	for k, v := range conditionDefinitions {
		result[k] = v
	}
	return result
}
