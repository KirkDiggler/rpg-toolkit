// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

type ModifierTestSuite struct {
	suite.Suite
}

func TestModifierSuite(t *testing.T) {
	suite.Run(t, new(ModifierTestSuite))
}

func (s *ModifierTestSuite) TestSimpleModifier() {
	// Use a test-specific source to verify any string works
	mod := events.NewSimpleModifier(events.TestModifierSourceTestSource, events.TestModifierTypeAdditive, events.TestModifierTargetDamage, 10, 5)

	s.Equal(events.TestModifierSourceTestSource, mod.Source())
	s.Equal(events.TestModifierTypeAdditive, mod.Type())
	s.Equal(events.TestModifierTargetDamage, mod.Target())
	s.Equal(10, mod.Priority())
	s.Equal(5, mod.Value())
}

func (s *ModifierTestSuite) TestSimpleModifierExamples() {
	// Rage damage bonus
	rage := events.NewSimpleModifier(events.TestModifierSourceRage, events.TestModifierTypeAdditive, events.TestModifierTargetDamage, 20, 2)
	s.Equal(events.TestModifierSourceRage, rage.Source())
	s.Equal(events.TestModifierTypeAdditive, rage.Type())
	s.Equal(events.TestModifierTargetDamage, rage.Target())
	s.Equal(20, rage.Priority())
	s.Equal(2, rage.Value())

	// Rage resistance
	resistance := events.NewSimpleModifier(events.TestModifierSourceRage, events.TestModifierTypeMultiplicative, events.TestModifierTargetDamage, 100, 0.5)
	s.Equal(events.TestModifierSourceRage, resistance.Source())
	s.Equal(events.TestModifierTypeMultiplicative, resistance.Type())
	s.Equal(events.TestModifierTargetDamage, resistance.Target())
	s.Equal(100, resistance.Priority()) // Applied late
	s.Equal(0.5, resistance.Value())

	// Shield spell AC bonus
	shield := events.NewSimpleModifier(events.TestModifierSourceShield, events.TestModifierTypeAdditive, events.TestModifierTargetAC, 50, 5)
	s.Equal(events.TestModifierSourceShield, shield.Source())
	s.Equal(events.TestModifierTypeAdditive, shield.Type())
	s.Equal(events.TestModifierTargetAC, shield.Target())
	s.Equal(5, shield.Value())

	// Bless attack bonus (dice)
	bless := events.NewSimpleModifier(events.TestModifierSourceBless, events.TestModifierTypeDice, events.TestModifierTargetAttackRoll, 10, "1d4")
	s.Equal(events.TestModifierSourceBless, bless.Source())
	s.Equal(events.TestModifierTypeDice, bless.Type())
	s.Equal(events.TestModifierTargetAttackRoll, bless.Target())
	s.Equal("1d4", bless.Value())
}

func (s *ModifierTestSuite) TestModifierWithDifferentValueTypes() {
	// String value (dice expression)
	stringMod := events.NewSimpleModifier(events.TestModifierSourceTest, events.TestModifierTypeDice, events.TestModifierTargetDamage, 10, "2d6+3")
	s.Equal("2d6+3", stringMod.Value())

	// Bool value (flag)
	boolMod := events.NewSimpleModifier(events.TestModifierSourceTest, events.TestModifierTypeFlag, events.TestModifierTargetAdvantage, 5, true)
	s.Equal(true, boolMod.Value())

	// Float value (multiplier)
	floatMod := events.NewSimpleModifier(events.TestModifierSourceTest, events.TestModifierTypeMultiplicative, events.TestModifierTargetDamage, 20, 1.5)
	s.Equal(1.5, floatMod.Value())

	// Struct value (custom)
	type CustomData struct {
		Min int
		Max int
	}
	customMod := events.NewSimpleModifier(events.TestModifierSourceTest, events.TestModifierTypeCustom, events.TestModifierTargetRoll, 15, CustomData{Min: 1, Max: 10})
	val := customMod.Value().(CustomData)
	s.Equal(1, val.Min)
	s.Equal(10, val.Max)
}

func (s *ModifierTestSuite) TestModifierInterface() {
	// Test that SimpleModifier implements the interface

	mod := events.NewSimpleModifier(events.TestModifierSourceTest, events.TestModifierTypeType, events.TestModifierTargetTarget, 10, "value")
	s.NotNil(mod)
	s.Equal(events.TestModifierSourceTest, mod.Source())
	s.Equal(events.TestModifierTypeType, mod.Type())
	s.Equal(events.TestModifierTargetTarget, mod.Target())
	s.Equal(10, mod.Priority())
	s.Equal("value", mod.Value())
}
