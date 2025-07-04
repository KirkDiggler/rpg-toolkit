// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

func TestSubscriptionTracker(t *testing.T) {
	bus := events.NewBus()
	tracker := effects.NewSubscriptionTracker()

	// Initial state
	if tracker.Count() != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", tracker.Count())
	}

	// Track some subscriptions manually
	sub1 := bus.SubscribeFunc("event1", 100, func(_ context.Context, _ events.Event) error {
		return nil
	})
	tracker.Track(sub1)

	sub2 := bus.SubscribeFunc("event2", 200, func(_ context.Context, _ events.Event) error {
		return nil
	})
	tracker.Track(sub2)

	if tracker.Count() != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", tracker.Count())
	}

	// Use Subscribe convenience method
	handlerCalled := false
	tracker.Subscribe(bus, "event3", 300, func(_ context.Context, _ events.Event) error {
		handlerCalled = true
		return nil
	})

	if tracker.Count() != 3 {
		t.Errorf("Expected 3 subscriptions, got %d", tracker.Count())
	}

	// Verify subscription works
	testEvent := events.NewGameEvent("event3", nil, nil)
	if err := bus.Publish(context.Background(), testEvent); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}

	// Unsubscribe all
	if err := tracker.UnsubscribeAll(bus); err != nil {
		t.Fatalf("Failed to unsubscribe all: %v", err)
	}

	if tracker.Count() != 0 {
		t.Errorf("Expected 0 subscriptions after unsubscribe, got %d", tracker.Count())
	}

	// Handler should not be called after unsubscribe
	handlerCalled = false
	if err := bus.Publish(context.Background(), testEvent); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}
	if handlerCalled {
		t.Error("Expected handler not to be called after unsubscribe")
	}
}

func TestSubscriptionTrackerClear(t *testing.T) {
	tracker := effects.NewSubscriptionTracker()

	// Add some tracked IDs
	tracker.Track("sub1")
	tracker.Track("sub2")
	tracker.Track("sub3")

	if tracker.Count() != 3 {
		t.Errorf("Expected 3 subscriptions, got %d", tracker.Count())
	}

	// Clear without unsubscribing
	tracker.Clear()

	if tracker.Count() != 0 {
		t.Errorf("Expected 0 subscriptions after clear, got %d", tracker.Count())
	}
}
