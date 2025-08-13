// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"
	
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Test event following the recommended pattern
var testDamageRef = func() *core.Ref {
	r, _ := core.ParseString("test:event:damage")
	return r
}()

type TestDamageEvent struct {
	Target string
	Amount int
}

func (e *TestDamageEvent) EventRef() *core.Ref {
	return testDamageRef  // Returns THE ref
}

var TestDamageEventRef = &core.TypedRef[*TestDamageEvent]{
	Ref: testDamageRef,  // Uses THE ref
}

func TestTypedPublishSubscribe(t *testing.T) {
	bus := events.NewBus()
	
	received := false
	
	// Subscribe with TypedRef
	id, err := events.Subscribe(bus, TestDamageEventRef, 
		func(e *TestDamageEvent) error {
			received = true
			if e.Amount != 10 {
				t.Errorf("expected Amount=10, got %d", e.Amount)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	
	// Publish - event knows its ref
	err = events.Publish(bus, &TestDamageEvent{
		Target: "player",
		Amount: 10,
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	
	if !received {
		t.Error("handler not called")
	}
	
	bus.Unsubscribe(id)
}

func TestTypedFilter(t *testing.T) {
	bus := events.NewBus()
	
	highDamageCount := 0
	
	// Subscribe with filter
	events.Subscribe(bus, TestDamageEventRef,
		func(e *TestDamageEvent) error {
			highDamageCount++
			return nil
		},
		events.Where(func(e *TestDamageEvent) bool {
			return e.Amount > 5
		}),
	)
	
	// Publish events
	events.Publish(bus, &TestDamageEvent{Target: "a", Amount: 3})  // Filtered
	events.Publish(bus, &TestDamageEvent{Target: "b", Amount: 10}) // Passes
	events.Publish(bus, &TestDamageEvent{Target: "c", Amount: 7})  // Passes
	
	if highDamageCount != 2 {
		t.Errorf("expected highDamageCount=2, got %d", highDamageCount)
	}
}