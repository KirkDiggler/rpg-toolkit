// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func TestCore(t *testing.T) {
	// Create event bus
	bus := events.NewBus()

	// Create a core effect with custom apply logic
	effectCore := effects.NewCore(effects.CoreConfig{
		ID:     "test-effect",
		Type:   "test.effect",
		Source: &core.Source{Category: core.SourceManual, Name: "test"},
		ApplyFunc: func(_ events.EventBus) error {
			// This would normally set up subscriptions
			return nil
		},
	})

	// Test initial state
	if effectCore.GetID() != "test-effect" {
		t.Errorf("Expected ID 'test-effect', got %s", effectCore.GetID())
	}
	if effectCore.GetType() != "test.effect" {
		t.Errorf("Expected type 'test.effect', got %s", effectCore.GetType())
	}
	if effectCore.Source() == nil || effectCore.Source().Name != "test" {
		t.Errorf("Expected source name 'test', got %v", effectCore.Source())
	}
	if effectCore.IsActive() {
		t.Error("Expected effect to be inactive initially")
	}

	// Apply the effect
	if err := effectCore.Apply(bus); err != nil {
		t.Fatalf("Failed to apply effect: %v", err)
	}
	if !effectCore.IsActive() {
		t.Error("Expected effect to be active after apply")
	}

	// Apply again should be idempotent
	if err := effectCore.Apply(bus); err != nil {
		t.Fatalf("Failed to apply effect twice: %v", err)
	}

	// Remove the effect
	if err := effectCore.Remove(bus); err != nil {
		t.Fatalf("Failed to remove effect: %v", err)
	}
	if effectCore.IsActive() {
		t.Error("Expected effect to be inactive after remove")
	}
}

func TestCoreSubscriptionTracking(t *testing.T) {
	bus := events.NewBus()
	handlerCallCount := 0

	// Create core that subscribes to events
	effectCore := effects.NewCore(effects.CoreConfig{
		ID:   "subscriber",
		Type: "test.subscriber",
	})

	// Apply should allow subscriptions
	if err := effectCore.Apply(bus); err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}

	// Subscribe to an event using the core's method
	effectCore.Subscribe(bus, "test.event", 100, func(_ context.Context, _ events.Event) error {
		handlerCallCount++
		return nil
	})

	// Verify subscription count
	if effectCore.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", effectCore.SubscriptionCount())
	}

	// Publish event - handler should be called
	testEvent := events.NewGameEvent("test.event", nil, nil)
	if err := bus.Publish(context.Background(), testEvent); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	if handlerCallCount != 1 {
		t.Errorf("Expected handler to be called once, got %d", handlerCallCount)
	}

	// Remove should unsubscribe
	if err := effectCore.Remove(bus); err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}

	// Publish again - handler should not be called
	if err := bus.Publish(context.Background(), testEvent); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	if handlerCallCount != 1 {
		t.Errorf("Expected handler to still be 1, got %d", handlerCallCount)
	}
}

func TestCoreLifecycleHandlers(t *testing.T) {
	bus := events.NewBus()
	applyCount := 0
	removeCount := 0

	effectCore := effects.NewCore(effects.CoreConfig{
		ID:     "lifecycle-test",
		Type:   "test.lifecycle",
		Source: &core.Source{Category: core.SourceManual, Name: "test"},
		ApplyFunc: func(_ events.EventBus) error {
			applyCount++
			return nil
		},
		RemoveFunc: func(_ events.EventBus) error {
			removeCount++
			return nil
		},
	})

	// Apply should call ApplyFunc
	if err := effectCore.Apply(bus); err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}
	if applyCount != 1 {
		t.Errorf("Expected ApplyFunc to be called once, got %d", applyCount)
	}

	// Apply again should not call ApplyFunc (already active)
	if err := effectCore.Apply(bus); err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}
	if applyCount != 1 {
		t.Errorf("Expected ApplyFunc to still be 1, got %d", applyCount)
	}

	// Remove should call RemoveFunc
	if err := effectCore.Remove(bus); err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}
	if removeCount != 1 {
		t.Errorf("Expected RemoveFunc to be called once, got %d", removeCount)
	}

	// Remove again should not call RemoveFunc (already inactive)
	if err := effectCore.Remove(bus); err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}
	if removeCount != 1 {
		t.Errorf("Expected RemoveFunc to still be 1, got %d", removeCount)
	}
}
