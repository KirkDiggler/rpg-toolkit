// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

type EventContextTestSuite struct {
	suite.Suite
}

func TestEventContextSuite(t *testing.T) {
	suite.Run(t, new(EventContextTestSuite))
}

func (s *EventContextTestSuite) TestTypedKeySetAndGet() {
	ctx := events.NewEventContext()

	// Define typed keys
	intKey := events.NewTypedKey[int]("testInt")
	stringKey := events.NewTypedKey[string]("testString")
	boolKey := events.NewTypedKey[bool]("testBool")

	// Set values
	events.Set(ctx, intKey, 42)
	events.Set(ctx, stringKey, "hello")
	events.Set(ctx, boolKey, true)

	// Get values
	intVal, ok := events.Get(ctx, intKey)
	s.Require().True(ok)
	s.Equal(42, intVal)

	strVal, ok := events.Get(ctx, stringKey)
	s.Require().True(ok)
	s.Equal("hello", strVal)

	boolVal, ok := events.Get(ctx, boolKey)
	s.Require().True(ok)
	s.Equal(true, boolVal)
}

func (s *EventContextTestSuite) TestTypedKeyMissingValue() {
	ctx := events.NewEventContext()

	key := events.NewTypedKey[int]("missing")

	val, ok := events.Get(ctx, key)
	s.False(ok)
	s.Equal(0, val) // Zero value for int
}

func (s *EventContextTestSuite) TestTypedKeyTypeSafety() {
	ctx := events.NewEventContext()

	// Set a string value
	stringKey := events.NewTypedKey[string]("myKey")
	events.Set(ctx, stringKey, "value")

	// Try to get it with a different type key (same name, different type)
	intKey := events.NewTypedKey[int]("myKey")
	val, ok := events.Get(ctx, intKey)

	// Should fail because types don't match
	s.False(ok)
	s.Equal(0, val)
}

func (s *EventContextTestSuite) TestHasKey() {
	ctx := events.NewEventContext()
	key := events.NewTypedKey[string]("test")

	s.False(events.HasKey(ctx, key))

	events.Set(ctx, key, "value")

	s.True(events.HasKey(ctx, key))
}

func (s *EventContextTestSuite) TestDeleteKey() {
	ctx := events.NewEventContext()
	key := events.NewTypedKey[int]("test")

	events.Set(ctx, key, 42)
	s.True(events.HasKey(ctx, key))

	events.Delete(ctx, key)
	s.False(events.HasKey(ctx, key))

	val, ok := events.Get(ctx, key)
	s.False(ok)
	s.Equal(0, val)
}

func (s *EventContextTestSuite) TestModifiers() {
	ctx := events.NewEventContext()

	// Add modifiers
	mod1 := events.NewSimpleModifier(
		events.TestModifierSourceRage,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		10, 2)
	mod2 := events.NewSimpleModifier(
		events.TestModifierSourceResistance,
		events.TestModifierTypeMultiplicative,
		events.TestModifierTargetDamage,
		20, 0.5)
	mod3 := events.NewSimpleModifier(
		events.TestModifierSourceBlessed,
		events.TestModifierTypeFlag,
		events.TestModifierTargetAdvantage,
		5, true)

	ctx.AddModifier(mod1)
	ctx.AddModifier(mod2)
	ctx.AddModifier(mod3)

	// Get modifiers
	mods := ctx.GetModifiers()
	s.Len(mods, 3)

	// Verify they're in order added
	s.Equal(events.TestModifierSourceRage, mods[0].Source())
	s.Equal(events.TestModifierSourceResistance, mods[1].Source())
	s.Equal(events.TestModifierSourceBlessed, mods[2].Source())
}

func (s *EventContextTestSuite) TestModifiersSorted() {
	ctx := events.NewEventContext()

	// Add modifiers with different priorities
	mod1 := events.NewSimpleModifier(
		events.TestModifierSourceTest,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		30, 5)
	mod2 := events.NewSimpleModifier(
		events.TestModifierSourceTest2,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		10, 3)
	mod3 := events.NewSimpleModifier(
		events.TestModifierSourceRage,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		20, 4)

	ctx.AddModifier(mod1)
	ctx.AddModifier(mod2)
	ctx.AddModifier(mod3)

	mods := ctx.GetModifiers()
	s.Len(mods, 3)

	// They should be in the order added (sorting happens at resolution)
	s.Equal(30, mods[0].Priority())
	s.Equal(10, mods[1].Priority())
	s.Equal(20, mods[2].Priority())
}

func (s *EventContextTestSuite) TestClearModifiers() {
	ctx := events.NewEventContext()

	ctx.AddModifier(events.NewSimpleModifier(
		events.TestModifierSourceTest,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		10, 5))
	s.Len(ctx.GetModifiers(), 1)

	ctx.ClearModifiers()
	s.Len(ctx.GetModifiers(), 0)
}

func (s *EventContextTestSuite) TestCommonKeys() {
	ctx := events.NewEventContext()

	// Test the minimal common keys
	events.Set(ctx, events.KeyDamage, 10)
	events.Set(ctx, events.KeyDamageType, "fire")

	// Entity keys are defined but we don't need to test them here
	// They're for rulebook use with actual entities

	damage, ok := events.Get(ctx, events.KeyDamage)
	s.True(ok)
	s.Equal(10, damage)

	dmgType, ok := events.Get(ctx, events.KeyDamageType)
	s.True(ok)
	s.Equal("fire", dmgType)
}

func (s *EventContextTestSuite) TestComplexTypes() {
	type DamageInfo struct {
		Amount int
		Type   string
		Source string
	}

	ctx := events.NewEventContext()
	key := events.NewTypedKey[DamageInfo]("damageInfo")

	info := DamageInfo{
		Amount: 12,
		Type:   "slashing",
		Source: "longsword",
	}

	events.Set(ctx, key, info)

	retrieved, ok := events.Get(ctx, key)
	s.True(ok)
	s.Equal(info, retrieved)
	s.Equal(12, retrieved.Amount)
	s.Equal("slashing", retrieved.Type)
	s.Equal("longsword", retrieved.Source)
}

func (s *EventContextTestSuite) TestConcurrentAccess() {
	ctx := events.NewEventContext()
	key := events.NewTypedKey[int]("counter")

	// Run concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(val int) {
			events.Set(ctx, key, val)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have some value (last write wins)
	val, ok := events.Get(ctx, key)
	s.True(ok)
	s.GreaterOrEqual(val, 0)
	s.Less(val, 10)
}

func (s *EventContextTestSuite) TestGetModifiersReturnsCopy() {
	ctx := events.NewEventContext()

	mod := events.NewSimpleModifier(
		events.TestModifierSourceTest,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		10, 5)
	ctx.AddModifier(mod)

	// Get modifiers and modify the returned slice
	getMods := ctx.GetModifiers()
	s.Len(getMods, 1)

	// Try to modify the returned slice
	_ = append(getMods, events.NewSimpleModifier(
		events.TestModifierSourceTest2,
		events.TestModifierTypeAdditive,
		events.TestModifierTargetDamage,
		0, 100))

	// Original should be unchanged
	originalMods := ctx.GetModifiers()
	s.Len(originalMods, 1)
	s.Equal(events.TestModifierSourceTest, originalMods[0].Source())
}
