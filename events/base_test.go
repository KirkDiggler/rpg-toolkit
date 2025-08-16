// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

type BaseEventTestSuite struct {
	suite.Suite
}

func TestBaseEventSuite(t *testing.T) {
	suite.Run(t, new(BaseEventTestSuite))
}

func (s *BaseEventTestSuite) TestBaseEventImplementsInterface() {
	ref := core.MustNewRef(core.RefInput{Module: "test", Type: "event", Value: "test"})
	baseEvent := events.NewBaseEvent(ref)

	// Should implement Event interface
	var event events.Event = baseEvent
	s.NotNil(event)

	s.Equal(ref, event.EventRef())
	s.NotNil(event.Context())
}

func (s *BaseEventTestSuite) TestBaseEventContext() {
	ref := core.MustNewRef(core.RefInput{Module: "test", Type: "event", Value: "context"})
	baseEvent := events.NewBaseEvent(ref)

	// Context should be initialized
	ctx := baseEvent.Context()
	s.NotNil(ctx)

	// Should be able to use the context
	key := events.NewTypedKey[string]("test")
	events.Set(ctx, key, "value")

	val, ok := events.Get(ctx, key)
	s.True(ok)
	s.Equal("value", val)
}

func (s *BaseEventTestSuite) TestBaseEventWithContext() {
	ref := core.MustNewRef(core.RefInput{Module: "test", Type: "event", Value: "custom"})
	customCtx := events.NewEventContext()

	// Set some data in custom context
	key := events.NewTypedKey[int]("custom")
	events.Set(customCtx, key, 42)

	// Create base event with custom context
	baseEvent := events.NewBaseEvent(ref).WithContext(customCtx)

	// Should use the custom context
	val, ok := events.Get(baseEvent.Context(), key)
	s.True(ok)
	s.Equal(42, val)
}

func (s *BaseEventTestSuite) TestBaseEventModifiers() {
	ref := core.MustNewRef(core.RefInput{Module: "test", Type: "event", Value: "modifiers"})
	baseEvent := events.NewBaseEvent(ref)

	// Add modifiers through context
	ctx := baseEvent.Context()
	ctx.AddModifier(events.NewSimpleModifier(events.TestModifierSourceTest, events.TestModifierTypeAdditive, events.TestModifierTargetDamage, 10, 5))
	ctx.AddModifier(events.NewSimpleModifier(events.TestModifierSourceTest2, events.TestModifierTypeMultiplicative, events.TestModifierTargetDamage, 20, 2.0))

	// Verify modifiers
	mods := ctx.GetModifiers()
	s.Len(mods, 2)
	s.Equal(events.TestModifierSourceTest, mods[0].Source())
	s.Equal(events.TestModifierSourceTest2, mods[1].Source())
}

// Example of how a domain event would use BaseEvent
type DomainDamageEvent struct {
	*events.BaseEvent
	Damage int
	Type   string
}

func NewDomainDamageEvent(damage int, damageType string) *DomainDamageEvent {
	ref := core.MustNewRef(core.RefInput{Module: "test", Type: "damage", Value: "event"})
	return &DomainDamageEvent{
		BaseEvent: events.NewBaseEvent(ref),
		Damage:    damage,
		Type:      damageType,
	}
}

func (s *BaseEventTestSuite) TestDomainEventEmbedding() {
	damageEvent := NewDomainDamageEvent(10, "fire")

	// Should implement Event interface
	var event events.Event = damageEvent
	s.NotNil(event)

	// Can access embedded methods
	s.NotNil(event.EventRef())
	s.NotNil(event.Context())

	// Can access domain fields
	s.Equal(10, damageEvent.Damage)
	s.Equal("fire", damageEvent.Type)

	// Can use context
	ctx := event.Context()
	events.Set(ctx, events.KeyDamage, damageEvent.Damage)
	events.Set(ctx, events.KeyDamageType, damageEvent.Type)

	damage, ok := events.Get(ctx, events.KeyDamage)
	s.True(ok)
	s.Equal(10, damage)

	dmgType, ok := events.Get(ctx, events.KeyDamageType)
	s.True(ok)
	s.Equal("fire", dmgType)
}
