// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "github.com/KirkDiggler/rpg-toolkit/core/events"

// Test constants for modifier types, sources, and targets.
// These are used in tests to validate the modifier system.
const (
	// Test modifier sources
	TestModifierSourceRage       events.ModifierSource = "rage"
	TestModifierSourceBless      events.ModifierSource = "bless"
	TestModifierSourceShield     events.ModifierSource = "shield"
	TestModifierSourceTest       events.ModifierSource = "test"
	TestModifierSourceTestSource events.ModifierSource = "TestSource"
	TestModifierSourceTest2      events.ModifierSource = "test2"
	TestModifierSourceResistance events.ModifierSource = "resistance"
	TestModifierSourceBlessed    events.ModifierSource = "blessed"

	// Test modifier types
	TestModifierTypeAdditive       events.ModifierType = "additive"
	TestModifierTypeMultiplicative events.ModifierType = "multiplicative"
	TestModifierTypeDice           events.ModifierType = "dice"
	TestModifierTypeFlag           events.ModifierType = "flag"
	TestModifierTypeCustom         events.ModifierType = "custom"
	TestModifierTypeType           events.ModifierType = "type"

	// Test modifier targets
	TestModifierTargetDamage     events.ModifierTarget = "damage"
	TestModifierTargetAC         events.ModifierTarget = "ac"
	TestModifierTargetAttackRoll events.ModifierTarget = "attack_roll"
	TestModifierTargetAdvantage  events.ModifierTarget = "advantage"
	TestModifierTargetRoll       events.ModifierTarget = "roll"
	TestModifierTargetTarget     events.ModifierTarget = "target"
)