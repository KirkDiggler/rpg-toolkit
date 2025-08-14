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
	mod := events.NewSimpleModifier("TestSource", "additive", "damage", 10, 5)

	s.Equal("TestSource", mod.Source())
	s.Equal("additive", mod.Type())
	s.Equal("damage", mod.Target())
	s.Equal(10, mod.Priority())
	s.Equal(5, mod.Value())
}

func (s *ModifierTestSuite) TestSimpleModifierExamples() {
	// Rage damage bonus
	rage := events.NewSimpleModifier("rage", "additive", "damage", 20, 2)
	s.Equal("rage", rage.Source())
	s.Equal("additive", rage.Type())
	s.Equal("damage", rage.Target())
	s.Equal(20, rage.Priority())
	s.Equal(2, rage.Value())

	// Rage resistance
	resistance := events.NewSimpleModifier("rage", "multiplicative", "damage", 100, 0.5)
	s.Equal("rage", resistance.Source())
	s.Equal("multiplicative", resistance.Type())
	s.Equal("damage", resistance.Target())
	s.Equal(100, resistance.Priority()) // Applied late
	s.Equal(0.5, resistance.Value())

	// Shield spell AC bonus
	shield := events.NewSimpleModifier("shield", "additive", "ac", 50, 5)
	s.Equal("shield", shield.Source())
	s.Equal("additive", shield.Type())
	s.Equal("ac", shield.Target())
	s.Equal(5, shield.Value())

	// Bless attack bonus (dice)
	bless := events.NewSimpleModifier("bless", "dice", "attack_roll", 10, "1d4")
	s.Equal("bless", bless.Source())
	s.Equal("dice", bless.Type())
	s.Equal("attack_roll", bless.Target())
	s.Equal("1d4", bless.Value())
}

func (s *ModifierTestSuite) TestModifierWithDifferentValueTypes() {
	// String value (dice expression)
	stringMod := events.NewSimpleModifier("test", "dice", "damage", 10, "2d6+3")
	s.Equal("2d6+3", stringMod.Value())

	// Bool value (flag)
	boolMod := events.NewSimpleModifier("test", "flag", "advantage", 5, true)
	s.Equal(true, boolMod.Value())

	// Float value (multiplier)
	floatMod := events.NewSimpleModifier("test", "multiplicative", "damage", 20, 1.5)
	s.Equal(1.5, floatMod.Value())

	// Struct value (custom)
	type CustomData struct {
		Min int
		Max int
	}
	customMod := events.NewSimpleModifier("test", "custom", "roll", 15, CustomData{Min: 1, Max: 10})
	val := customMod.Value().(CustomData)
	s.Equal(1, val.Min)
	s.Equal(10, val.Max)
}

func (s *ModifierTestSuite) TestModifierInterface() {
	// Test that SimpleModifier implements the interface

	mod := events.NewSimpleModifier("test", "type", "target", 10, "value")
	s.NotNil(mod)
	s.Equal("test", mod.Source())
	s.Equal("type", mod.Type())
	s.Equal("target", mod.Target())
	s.Equal(10, mod.Priority())
	s.Equal("value", mod.Value())
}
