// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func TestCore(t *testing.T) {
	// Create event bus
	bus := events.NewBus()

	// Create a core effect with custom apply logic
	core := effects.NewCore(effects.CoreConfig{
		ID:     "test-effect",
		Type:   "test.effect",
		Source: "test",
		ApplyFunc: func(_ events.EventBus) error {
			// This would normally set up subscriptions
			return nil
		},
	})

	// Test initial state
	if core.GetID() != "test-effect" {
		t.Errorf("Expected ID 'test-effect', got %s", core.GetID())
	}
	if core.GetType() != "test.effect" {
		t.Errorf("Expected type 'test.effect', got %s", core.GetType())
	}
	if core.Source() != "test" {
		t.Errorf("Expected source 'test', got %s", core.Source())
	}
	if core.IsActive() {
		t.Error("Expected effect to be inactive initially")
	}

	// Apply the effect
	if err := core.Apply(bus); err != nil {
		t.Fatalf("Failed to apply effect: %v", err)
	}
	if !core.IsActive() {
		t.Error("Expected effect to be active after apply")
	}

	// Apply again should be idempotent
	if err := core.Apply(bus); err != nil {
		t.Fatalf("Failed to apply effect twice: %v", err)
	}

	// Remove the effect
	if err := core.Remove(bus); err != nil {
		t.Fatalf("Failed to remove effect: %v", err)
	}
	if core.IsActive() {
		t.Error("Expected effect to be inactive after remove")
	}
}

func TestCoreSubscriptionTracking(t *testing.T) {
	bus := events.NewBus()
	handlerCallCount := 0

	// Create core that subscribes to events
	core := effects.NewCore(effects.CoreConfig{
		ID:   "subscriber",
		Type: "test.subscriber",
	})

	// Apply should allow subscriptions
	if err := core.Apply(bus); err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}

	// Subscribe to an event using the core's method
	core.Subscribe(bus, "test.event", 100, func(_ context.Context, _ events.Event) error {
		handlerCallCount++
		return nil
	})

	// Verify subscription count
	if core.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", core.SubscriptionCount())
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
	if err := core.Remove(bus); err != nil {
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

	core := effects.NewCore(effects.CoreConfig{
		ID:     "lifecycle-test",
		Type:   "test.lifecycle",
		Source: "test",
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
	if err := core.Apply(bus); err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}
	if applyCount != 1 {
		t.Errorf("Expected ApplyFunc to be called once, got %d", applyCount)
	}

	// Apply again should not call ApplyFunc (already active)
	if err := core.Apply(bus); err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}
	if applyCount != 1 {
		t.Errorf("Expected ApplyFunc to still be 1, got %d", applyCount)
	}

	// Remove should call RemoveFunc
	if err := core.Remove(bus); err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}
	if removeCount != 1 {
		t.Errorf("Expected RemoveFunc to be called once, got %d", removeCount)
	}

	// Remove again should not call RemoveFunc (already inactive)
	if err := core.Remove(bus); err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}
	if removeCount != 1 {
		t.Errorf("Expected RemoveFunc to still be 1, got %d", removeCount)
	}
}
