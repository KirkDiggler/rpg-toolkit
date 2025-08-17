// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Test event following the recommended pattern
var testDamageRef = func() *core.Ref {
	r, err := core.ParseString("test:event:damage")
	if err != nil {
		panic(err)
	}
	return r
}()

type TypedDamageEvent struct {
	Target string
	Amount int
}

func (e *TypedDamageEvent) EventRef() *core.Ref {
	return testDamageRef // Returns THE ref
}

// Context implements the Event interface
func (e *TypedDamageEvent) Context() *events.EventContext {
	return events.NewEventContext() // For tests, just return a new context
}

var TypedDamageEventRef = &core.TypedRef[*TypedDamageEvent]{
	Ref: testDamageRef, // Uses THE ref
}

func TestTypedPublishSubscribe(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()

	received := false

	// Subscribe with TypedRef and context
	id, err := events.Subscribe(ctx, bus, TypedDamageEventRef,
		func(_ context.Context, e *TypedDamageEvent) error {
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

	// Publish with context - event knows its ref
	err = events.Publish(ctx, bus, &TypedDamageEvent{
		Target: "player",
		Amount: 10,
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if !received {
		t.Error("handler not called")
	}

	if err := bus.Unsubscribe(ctx, id); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
}

func TestTypedFilter(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()

	highDamageCount := 0

	// Subscribe with filter and context
	_, err := events.Subscribe(ctx, bus, TypedDamageEventRef,
		func(_ context.Context, _ *TypedDamageEvent) error {
			highDamageCount++
			return nil
		},
		events.Where(func(e *TypedDamageEvent) bool {
			return e.Amount > 5
		}),
	)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish events with context
	if err := events.Publish(ctx, bus, &TypedDamageEvent{Target: "a", Amount: 3}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := events.Publish(ctx, bus, &TypedDamageEvent{Target: "b", Amount: 10}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := events.Publish(ctx, bus, &TypedDamageEvent{Target: "c", Amount: 7}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if highDamageCount != 2 {
		t.Errorf("expected highDamageCount=2, got %d", highDamageCount)
	}
}
