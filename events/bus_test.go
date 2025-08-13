// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Test event
type TestEvent struct {
	Target string
	Value  int
}

func (e TestEvent) Type() string {
	return "test.event"
}

func TestBusPublishSubscribe(t *testing.T) {
	bus := events.NewBus()
	
	received := false
	id, err := bus.Subscribe("test.event", func(e TestEvent) error {
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
	bus.SubscribeWithFilter("test.event", 
		func(e TestEvent) error {
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
	
	// Player2 only wants events targeting them
	bus.SubscribeWithFilter("test.event",
		func(e TestEvent) error {
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
	
	// Publish events
	bus.Publish(TestEvent{Target: "player1", Value: 1})
	bus.Publish(TestEvent{Target: "player2", Value: 2})
	bus.Publish(TestEvent{Target: "player1", Value: 3})
	bus.Publish(TestEvent{Target: "npc", Value: 4})     // Neither gets this
	
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
		bus.Subscribe("test.event", func(e TestEvent) error {
			count++
			return nil
		})
	}
	
	// One event should trigger all 3
	bus.Publish(TestEvent{Target: "all", Value: 1})
	
	if count != 3 {
		t.Errorf("expected 3 handlers called, got %d", count)
	}
}