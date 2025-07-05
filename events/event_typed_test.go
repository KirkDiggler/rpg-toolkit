// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestEventType_String(t *testing.T) {
	tests := []struct {
		name      string
		eventType events.EventType
		expected  string
	}{
		{"attack roll", events.EventTypeBeforeAttackRoll, events.EventBeforeAttackRoll},
		{"damage", events.EventTypeOnDamageRoll, events.EventOnDamageRoll},
		{"save", events.EventTypeAfterSavingThrow, events.EventAfterSavingThrow},
		{"custom", events.EventTypeCustom, "custom_event"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

func TestParseEventType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected events.EventType
	}{
		{"valid attack", events.EventBeforeAttackRoll, events.EventTypeBeforeAttackRoll},
		{"valid damage", events.EventOnDamageRoll, events.EventTypeOnDamageRoll},
		{"invalid", "invalid_event", events.EventTypeCustom},
		{"empty", "", events.EventTypeCustom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, events.ParseEventType(tt.input))
		})
	}
}

func TestNewTypedGameEvent(t *testing.T) {
	source := &mockEntity{id: "attacker", typ: "character"}
	target := &mockEntity{id: "defender", typ: "monster"}

	event := events.NewTypedGameEvent(events.EventTypeBeforeAttackRoll, source, target)

	assert.Equal(t, events.EventBeforeAttackRoll, event.Type())
	assert.Equal(t, events.EventTypeBeforeAttackRoll, event.TypedType())
	assert.Equal(t, source, event.Source())
	assert.Equal(t, target, event.Target())
	assert.False(t, event.IsCancelled())
}

func TestGameEvent_TypedType(t *testing.T) {
	// Test with string-based constructor
	event1 := events.NewGameEvent(events.EventBeforeAttackRoll, nil, nil)
	assert.Equal(t, events.EventTypeBeforeAttackRoll, event1.TypedType())

	// Test with typed constructor
	event2 := events.NewTypedGameEvent(events.EventTypeOnDamageRoll, nil, nil)
	assert.Equal(t, events.EventTypeOnDamageRoll, event2.TypedType())

	// Test with custom event
	event3 := events.NewGameEvent("custom_event_type", nil, nil)
	assert.Equal(t, events.EventTypeCustom, event3.TypedType())
}

func TestTypedContextAccessors(t *testing.T) {
	ctx := events.NewEventContext()

	// Test GetInt
	ctx.Set("damage", 10)
	val, ok := ctx.GetInt("damage")
	assert.True(t, ok)
	assert.Equal(t, 10, val)

	// Test GetString
	ctx.Set("weapon", "longsword")
	str, ok := ctx.GetString("weapon")
	assert.True(t, ok)
	assert.Equal(t, "longsword", str)

	// Test GetBool
	ctx.Set("critical", true)
	b, ok := ctx.GetBool("critical")
	assert.True(t, ok)
	assert.True(t, b)

	// Test GetFloat64
	ctx.Set("multiplier", 1.5)
	f, ok := ctx.GetFloat64("multiplier")
	assert.True(t, ok)
	assert.Equal(t, 1.5, f)

	// Test GetEntity
	entity := &mockEntity{id: "player", typ: "character"}
	ctx.Set("attacker", entity)
	e, ok := ctx.GetEntity("attacker")
	assert.True(t, ok)
	assert.Equal(t, entity, e)

	// Test GetDuration
	duration := &events.RoundsDuration{Rounds: 3}
	ctx.Set("duration", duration)
	d, ok := ctx.GetDuration("duration")
	assert.True(t, ok)
	assert.Equal(t, duration, d)

	// Test missing key
	_, ok = ctx.GetInt("missing")
	assert.False(t, ok)

	// Test wrong type
	ctx.Set("not_int", "string")
	_, ok = ctx.GetInt("not_int")
	assert.False(t, ok)
}

func TestModifierWithCondition(t *testing.T) {
	// Create a modifier that only applies to attacks with advantage
	mod := events.NewModifierWithConfig(events.ModifierConfig{
		Source:   "bless",
		Type:     events.ModifierAttackBonus,
		Value:    &mockModifierValue{value: 4},
		Priority: 100,
		Condition: func(e events.Event) bool {
			adv, _ := e.Context().GetBool(events.ContextKeyAdvantage)
			return adv
		},
	})

	// Test with advantage
	event1 := events.NewGameEvent(events.EventBeforeAttackRoll, nil, nil)
	event1.Context().Set(events.ContextKeyAdvantage, true)
	assert.True(t, mod.Condition(event1))

	// Test without advantage
	event2 := events.NewGameEvent(events.EventBeforeAttackRoll, nil, nil)
	event2.Context().Set(events.ContextKeyAdvantage, false)
	assert.False(t, mod.Condition(event2))
}

func TestModifierWithDuration(t *testing.T) {
	duration := &events.RoundsDuration{
		Rounds:     10,
		StartRound: 1,
	}

	sourceDetails := &events.ModifierSource{
		Type:        "spell",
		Name:        "Bless",
		Description: "You bless up to three creatures of your choice within range",
		Entity:      &mockEntity{id: "cleric", typ: "character"},
	}

	mod := events.NewModifierWithConfig(events.ModifierConfig{
		Source:        "bless",
		Type:          events.ModifierAttackBonus,
		Value:         &mockModifierValue{value: 4},
		Priority:      100,
		Duration:      duration,
		SourceDetails: sourceDetails,
	})

	assert.Equal(t, duration, mod.Duration())
	assert.Equal(t, sourceDetails, mod.SourceDetails())
	assert.Equal(t, "spell", mod.SourceDetails().Type)
	assert.Equal(t, "Bless", mod.SourceDetails().Name)
}

func TestEventBuilderPattern(t *testing.T) {
	source := &mockEntity{id: "caster", typ: "character"}
	target := &mockEntity{id: "target", typ: "monster"}

	modifier := events.NewModifier("test", events.ModifierDamageBonus, &mockModifierValue{value: 5}, 50)

	event := events.NewGameEvent(events.EventBeforeDamageRoll, nil, nil).
		WithSource(source).
		WithTarget(target).
		WithContext(events.ContextKeyDamageType, "fire").
		WithContext(events.ContextKeySpellLevel, 3).
		WithModifier(modifier)

	// Verify all values were set
	assert.Equal(t, source, event.Source())
	assert.Equal(t, target, event.Target())

	damageType, ok := event.Context().GetString(events.ContextKeyDamageType)
	assert.True(t, ok)
	assert.Equal(t, "fire", damageType)

	spellLevel, ok := event.Context().GetInt(events.ContextKeySpellLevel)
	assert.True(t, ok)
	assert.Equal(t, 3, spellLevel)

	modifiers := event.Context().Modifiers()
	assert.Len(t, modifiers, 1)
	assert.Equal(t, "test", modifiers[0].Source())
}

func TestEventCancellation(t *testing.T) {
	bus := events.NewBus()

	// Handler that cancels the event
	var handler1Called bool
	bus.SubscribeFunc(events.EventBeforeAttackRoll, 10, events.HandlerFunc(func(_ context.Context, e events.Event) error {
		handler1Called = true
		e.Cancel()
		return nil
	}))

	// Handler that should not be called
	var handler2Called bool
	bus.SubscribeFunc(events.EventBeforeAttackRoll, 20, events.HandlerFunc(func(_ context.Context, _ events.Event) error {
		handler2Called = true
		return nil
	}))

	event := events.NewGameEvent(events.EventBeforeAttackRoll, nil, nil)
	err := bus.Publish(context.Background(), event)

	require.NoError(t, err)
	assert.True(t, handler1Called)
	assert.False(t, handler2Called)
	assert.True(t, event.IsCancelled())
}

// Mock implementations for testing

type mockEntity struct {
	id  string
	typ string
}

func (e *mockEntity) GetID() string   { return e.id }
func (e *mockEntity) GetType() string { return e.typ }

type mockModifierValue struct {
	value int
}

func (m *mockModifierValue) GetValue() int          { return m.value }
func (m *mockModifierValue) GetDescription() string { return "+test" }
