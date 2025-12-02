// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

// Modifier chain stages define the order of execution for combat and effect modifiers.
// These ensure consistent, predictable ordering across all D&D 5e game mechanics.
//
// Example usage in damage calculation:
//   - StageBase: Roll dice, add ability modifier
//   - StageFeatures: Add rage damage, sneak attack
//   - StageConditions: Apply bless, bane, prone penalties
//   - StageEquipment: Add magic weapon bonuses
//   - StageFinal: Apply resistance/vulnerability, caps
const (
	// StageBase applies base values (dice rolls, proficiency, ability modifiers)
	StageBase chain.Stage = "base"

	// StageFeatures applies class and race feature modifiers (rage, sneak attack, etc.)
	StageFeatures chain.Stage = "features"

	// StageConditions applies condition modifiers (bless, bane, prone, restrained, etc.)
	StageConditions chain.Stage = "conditions"

	// StageEquipment applies equipment bonuses (magic weapons, enchanted armor, etc.)
	StageEquipment chain.Stage = "equipment"

	// StageFinal applies final adjustments (resistance/vulnerability, damage caps, etc.)
	StageFinal chain.Stage = "final"
)

// ModifierStages defines the standard execution order for D&D 5e modifier chains.
// This slice should be used when creating staged chains to ensure consistent ordering.
var ModifierStages = []chain.Stage{
	StageBase,
	StageFeatures,
	StageConditions,
	StageEquipment,
	StageFinal,
}

// DamageSourceType categorizes where damage bonuses come from
type DamageSourceType string

// Damage source type constants
const (
	DamageSourceWeapon          DamageSourceType = "weapon"
	DamageSourceAbility         DamageSourceType = "ability"
	DamageSourceRage            DamageSourceType = "rage"
	DamageSourceSneakAttack     DamageSourceType = "sneak_attack"
	DamageSourceDivineSmite     DamageSourceType = "divine_smite"
	DamageSourceElementalWeapon DamageSourceType = "elemental_weapon"
	DamageSourceBrutalCritical  DamageSourceType = "brutal_critical"
	// Add more as needed
)

// RerollEvent tracks a single die reroll
type RerollEvent struct {
	DieIndex int    // Which die was rerolled (0-based in OriginalDiceRolls)
	Before   int    // Value before reroll
	After    int    // Value after reroll
	Reason   string // Feature that caused reroll (e.g., "great_weapon_fighting")
}

// DamageComponent represents damage from one source
type DamageComponent struct {
	Source            DamageSourceType
	OriginalDiceRolls []int         // As first rolled
	FinalDiceRolls    []int         // After all rerolls
	Rerolls           []RerollEvent // History of rerolls
	FlatBonus         int           // Flat modifier (0 if none)
	DamageType        string        // "slashing", "fire", etc.
	IsCritical        bool          // Was this doubled for crit?
}

// Total returns the total damage for this component
func (dc *DamageComponent) Total() int {
	total := dc.FlatBonus
	for _, roll := range dc.FinalDiceRolls {
		total += roll
	}
	return total
}

// AttackChainEvent represents an attack flowing through the modifier chain
type AttackChainEvent struct {
	AttackerID      string
	TargetID        string
	AttackRoll      int  // The d20 roll
	AttackBonus     int  // Base bonus before modifiers
	TargetAC        int  // Target's armor class
	IsNaturalTwenty bool // Natural 20 always hits
	IsNaturalOne    bool // Natural 1 always misses
}

// DamageChainEvent represents damage flowing through the modifier chain
type DamageChainEvent struct {
	AttackerID   string
	TargetID     string
	Components   []DamageComponent // All damage sources
	DamageType   string            // Type of damage (slashing, piercing, etc.)
	IsCritical   bool              // Double damage dice on crit
	WeaponDamage string            // Weapon damage dice (e.g., "1d8")
	AbilityUsed  abilities.Ability // Which ability was used
}

// AttackChain provides typed chained topic for attack roll modifiers
var AttackChain = events.DefineChainedTopic[AttackChainEvent]("dnd5e.combat.attack.chain")

// DamageChain provides typed chained topic for damage modifiers
var DamageChain = events.DefineChainedTopic[*DamageChainEvent]("dnd5e.combat.damage.chain")
