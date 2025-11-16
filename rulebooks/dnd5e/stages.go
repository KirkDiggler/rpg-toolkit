// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

import "github.com/KirkDiggler/rpg-toolkit/core/chain"

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
