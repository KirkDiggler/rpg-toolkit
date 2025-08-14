// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Package-level ref for test events
var testEventRef = func() *core.Ref {
	r, _ := core.ParseString("test:event:basic")
	return r
}()

// Test event
type TestEvent struct {
	Target string
	Value  int
	ctx    *events.EventContext
}

func (e TestEvent) EventRef() *core.Ref {
	return testEventRef
}

func (e TestEvent) Context() *events.EventContext {
	if e.ctx == nil {
		return events.NewEventContext()
	}
	return e.ctx
}

func TestBusPublishSubscribe(t *testing.T) {
	bus := events.NewBus()

	received := false
	id, err := bus.Subscribe(testEventRef, func(e TestEvent) error {
		received = true
		if e.Value != 42 {
			t.Errorf("expected Value=42, got %d", e.Value)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish event
	err = bus.Publish(TestEvent{Target: "player", Value: 42})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if !received {
		t.Error("handler was not called")
	}

	// Unsubscribe
	err = bus.Unsubscribe(id)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
}

func TestBusFilter(t *testing.T) {
	bus := events.NewBus()

	player1Count := 0
	player2Count := 0

	// Player1 only wants events targeting them
	_, err := bus.SubscribeWithFilter(testEventRef,
		func(_ TestEvent) error {
			player1Count++
			return nil
		},
		func(e events.Event) bool {
			if te, ok := e.(TestEvent); ok {
				return te.Target == "player1"
			}
			return false
		},
	)
	if err != nil {
		t.Fatalf("SubscribeWithFilter for player1 failed: %v", err)
	}

	// Player2 only wants events targeting them
	_, err = bus.SubscribeWithFilter(testEventRef,
		func(_ TestEvent) error {
			player2Count++
			return nil
		},
		func(e events.Event) bool {
			if te, ok := e.(TestEvent); ok {
				return te.Target == "player2"
			}
			return false
		},
	)
	if err != nil {
		t.Fatalf("SubscribeWithFilter failed: %v", err)
	}

	// Publish events
	if err := bus.Publish(TestEvent{Target: "player1", Value: 1}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := bus.Publish(TestEvent{Target: "player2", Value: 2}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := bus.Publish(TestEvent{Target: "player1", Value: 3}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := bus.Publish(TestEvent{Target: "npc", Value: 4}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if player1Count != 2 {
		t.Errorf("player1 should have received 2 events, got %d", player1Count)
	}

	if player2Count != 1 {
		t.Errorf("player2 should have received 1 event, got %d", player2Count)
	}
}

func TestBusMultipleHandlers(t *testing.T) {
	bus := events.NewBus()

	count := 0

	// Register 3 handlers
	for i := 0; i < 3; i++ {
		_, err := bus.Subscribe(testEventRef, func(_ TestEvent) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	// One event should trigger all 3
	if err := bus.Publish(TestEvent{Target: "all", Value: 1}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 handlers called, got %d", count)
	}
}
