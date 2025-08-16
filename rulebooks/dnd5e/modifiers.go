// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/core/events"
)

// D&D 5e specific modifier sources
const (
	ModifierSourceRage        events.ModifierSource = "rage"
	ModifierSourceBless       events.ModifierSource = "bless"
	ModifierSourceHaste       events.ModifierSource = "haste"
	ModifierSourceExhaustion  events.ModifierSource = "exhaustion"
)

// D&D 5e specific modifier types
const (
	ModifierTypeAdditive       events.ModifierType = "additive"
	ModifierTypeMultiplicative events.ModifierType = "multiplicative"
	ModifierTypeResistance     events.ModifierType = "resistance"
	ModifierTypeAdvantage      events.ModifierType = "advantage"
	ModifierTypeDisadvantage   events.ModifierType = "disadvantage"
)

// ModifierTarget identifies what is being modified
// Note: This doesn't exist in core/events yet, so we define our own
type ModifierTarget string

const (
	ModifierTargetDamage       ModifierTarget = "damage"
	ModifierTargetAttackRoll   ModifierTarget = "attack_roll"
	ModifierTargetArmorClass   ModifierTarget = "armor_class"
	ModifierTargetSavingThrow  ModifierTarget = "saving_throw"
	ModifierTargetAbilityCheck ModifierTarget = "ability_check"
)