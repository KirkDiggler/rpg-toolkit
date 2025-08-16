// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

// ModifierSource identifies what added a modifier
type ModifierSource string

const (
	ModifierSourceRage        ModifierSource = "rage"
	ModifierSourceBless       ModifierSource = "bless"
	ModifierSourceHaste       ModifierSource = "haste"
	ModifierSourceExhaustion  ModifierSource = "exhaustion"
)

// ModifierType describes how a modifier is applied
type ModifierType string

const (
	ModifierTypeAdditive      ModifierType = "additive"
	ModifierTypeMultiplicative ModifierType = "multiplicative"
	ModifierTypeResistance    ModifierType = "resistance"
	ModifierTypeAdvantage     ModifierType = "advantage"
	ModifierTypeDisadvantage  ModifierType = "disadvantage"
)

// ModifierTarget identifies what is being modified
type ModifierTarget string

const (
	ModifierTargetDamage       ModifierTarget = "damage"
	ModifierTargetAttackRoll   ModifierTarget = "attack_roll"
	ModifierTargetArmorClass   ModifierTarget = "armor_class"
	ModifierTargetSavingThrow  ModifierTarget = "saving_throw"
	ModifierTargetAbilityCheck ModifierTarget = "ability_check"
)