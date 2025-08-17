// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/core/events"
)

// D&D 5e specific modifier sources
const (
	ModifierSourceRage       events.ModifierSource = "rage"
	ModifierSourceBless      events.ModifierSource = "bless"
	ModifierSourceHaste      events.ModifierSource = "haste"
	ModifierSourceExhaustion events.ModifierSource = "exhaustion"
)

// D&D 5e specific modifier types
const (
	ModifierTypeAdditive       events.ModifierType = "additive"
	ModifierTypeMultiplicative events.ModifierType = "multiplicative"
	ModifierTypeResistance     events.ModifierType = "resistance"
	ModifierTypeAdvantage      events.ModifierType = "advantage"
	ModifierTypeDisadvantage   events.ModifierType = "disadvantage"
)

// D&D 5e specific modifier targets
const (
	ModifierTargetDamage       events.ModifierTarget = "damage"
	ModifierTargetAttackRoll   events.ModifierTarget = "attack_roll"
	ModifierTargetArmorClass   events.ModifierTarget = "armor_class"
	ModifierTargetSavingThrow  events.ModifierTarget = "saving_throw"
	ModifierTargetAbilityCheck events.ModifierTarget = "ability_check"
)
