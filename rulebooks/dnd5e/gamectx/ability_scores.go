// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

// AbilityScores holds the six ability scores for a character.
// Purpose: Provides ability score values and modifier calculations for
// features that need to check ability modifiers (e.g., Two-Weapon Fighting).
type AbilityScores struct {
	Strength     int
	Dexterity    int
	Constitution int
	Intelligence int
	Wisdom       int
	Charisma     int
}

// abilityModifier calculates the modifier for an ability score.
// D&D 5e formula: (score - 10) / 2, rounded down (floor division).
// Go's integer division rounds toward zero, so we need explicit floor logic.
func abilityModifier(score int) int {
	diff := score - 10
	if diff >= 0 {
		return diff / 2
	}
	// Floor division for negative numbers: (diff - 1) / 2
	return (diff - 1) / 2
}

// StrengthMod returns the Strength modifier.
func (a *AbilityScores) StrengthMod() int {
	return abilityModifier(a.Strength)
}

// DexterityMod returns the Dexterity modifier.
func (a *AbilityScores) DexterityMod() int {
	return abilityModifier(a.Dexterity)
}

// ConstitutionMod returns the Constitution modifier.
func (a *AbilityScores) ConstitutionMod() int {
	return abilityModifier(a.Constitution)
}

// IntelligenceMod returns the Intelligence modifier.
func (a *AbilityScores) IntelligenceMod() int {
	return abilityModifier(a.Intelligence)
}

// WisdomMod returns the Wisdom modifier.
func (a *AbilityScores) WisdomMod() int {
	return abilityModifier(a.Wisdom)
}

// CharismaMod returns the Charisma modifier.
func (a *AbilityScores) CharismaMod() int {
	return abilityModifier(a.Charisma)
}
