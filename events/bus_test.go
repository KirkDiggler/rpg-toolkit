package events

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestBusSubscribeAndPublish(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Track handler calls
	var called bool
	var receivedEvent Event

	// Subscribe to an event
	id := bus.SubscribeFunc(EventBeforeAttackRoll, 100, func(_ context.Context, e Event) error {
		called = true
		receivedEvent = e
		return nil
	})

	if id == "" {
		t.Error("Expected subscription ID")
	}

	// Publish event
	source := &mockEntity{id: "player-1", entityType: "character"}
	event := NewGameEvent(EventBeforeAttackRoll, source, nil)

	err := bus.Publish(ctx, event)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	if !called {
		t.Error("Handler was not called")
	}

	if receivedEvent != event {
		t.Error("Received wrong event")
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Track calls
	var callCount int
	var callOrder []string

	// Subscribe multiple handlers with different priorities
	bus.SubscribeFunc(EventOnDamageRoll, 200, func(_ context.Context, _ Event) error {
		callCount++
		callOrder = append(callOrder, "high-priority")
		return nil
	})

	bus.SubscribeFunc(EventOnDamageRoll, 100, func(_ context.Context, _ Event) error {
		callCount++
		callOrder = append(callOrder, "medium-priority")
		return nil
	})

	bus.SubscribeFunc(EventOnDamageRoll, 50, func(_ context.Context, _ Event) error {
		callCount++
		callOrder = append(callOrder, "low-priority")
		return nil
	})

	// Publish event
	event := NewGameEvent(EventOnDamageRoll, nil, nil)
	err := bus.Publish(ctx, event)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}

	// Verify execution order (lower priority first)
	expectedOrder := []string{"low-priority", "medium-priority", "high-priority"}
	for i, expected := range expectedOrder {
		if i >= len(callOrder) || callOrder[i] != expected {
			t.Errorf("Expected order %v, got %v", expectedOrder, callOrder)
			break
		}
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	var callCount int

	// Subscribe
	id := bus.SubscribeFunc(EventOnTurnStart, 100, func(_ context.Context, _ Event) error {
		callCount++
		return nil
	})

	// Publish - should call handler
	event := NewGameEvent(EventOnTurnStart, nil, nil)
	if err := bus.Publish(ctx, event); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Unsubscribe
	err := bus.Unsubscribe(id)
	if err != nil {
		t.Errorf("Unsubscribe failed: %v", err)
	}

	// Publish again - should not call handler
	if err := bus.Publish(ctx, event); err != nil {
		t.Errorf("Publish after unsubscribe failed: %v", err)
	}

	if callCount != 1 {
		t.Error("Handler called after unsubscribe")
	}

	// Unsubscribe non-existent ID
	err = bus.Unsubscribe("fake-id")
	if err == nil {
		t.Error("Expected error for non-existent subscription")
	}
}

func TestBusClear(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	var attackCalls, damageCalls int

	// Subscribe to different events
	bus.SubscribeFunc(EventBeforeAttackRoll, 100, func(_ context.Context, _ Event) error {
		attackCalls++
		return nil
	})

	bus.SubscribeFunc(EventOnDamageRoll, 100, func(_ context.Context, _ Event) error {
		damageCalls++
		return nil
	})

	// Clear attack subscriptions
	bus.Clear(EventBeforeAttackRoll)

	// Publish both events
	if err := bus.Publish(ctx, NewGameEvent(EventBeforeAttackRoll, nil, nil)); err != nil {
		t.Errorf("Publish attack event failed: %v", err)
	}
	if err := bus.Publish(ctx, NewGameEvent(EventOnDamageRoll, nil, nil)); err != nil {
		t.Errorf("Publish damage event failed: %v", err)
	}

	if attackCalls != 0 {
		t.Error("Attack handler called after clear")
	}

	if damageCalls != 1 {
		t.Error("Damage handler should still work")
	}

	// Clear all
	bus.ClearAll()
	if err := bus.Publish(ctx, NewGameEvent(EventOnDamageRoll, nil, nil)); err != nil {
		t.Errorf("Publish after clear all failed: %v", err)
	}

	if damageCalls != 1 {
		t.Error("Handler called after clear all")
	}
}

func TestBusHandlerError(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Subscribe handler that returns error
	expectedErr := errors.New("handler error")
	bus.SubscribeFunc(EventOnStatusApplied, 100, func(_ context.Context, _ Event) error {
		return expectedErr
	})

	// Publish should return the error
	event := NewGameEvent(EventOnStatusApplied, nil, nil)
	err := bus.Publish(ctx, event)
	if err == nil {
		t.Error("Expected error from handler")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

func TestBusConcurrency(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Test concurrent subscribe/publish
	var wg sync.WaitGroup
	callCount := 0
	var mu sync.Mutex

	// Start multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Subscribe
			bus.SubscribeFunc(EventOnTurnStart, 100, func(_ context.Context, _ Event) error {
				mu.Lock()
				callCount++
				mu.Unlock()
				return nil
			})

			// Publish
			event := NewGameEvent(EventOnTurnStart, nil, nil)
			if err := bus.Publish(ctx, event); err != nil {
				t.Errorf("Publish in goroutine %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Each goroutine publishes once, and all handlers should be called
	// So we expect n * (n+1) / 2 calls (triangular number)
	// But since subscriptions happen concurrently, we can't predict exact count
	// Just verify no panic and some handlers were called
	if callCount == 0 {
		t.Error("No handlers were called")
	}
}

// TestHandler implements Handler interface for testing
type TestHandler struct {
	name     string
	priority int
	calls    []Event
	err      error
}

func (h *TestHandler) Handle(_ context.Context, event Event) error {
	h.calls = append(h.calls, event)
	return h.err
}

func (h *TestHandler) Priority() int {
	return h.priority
}

func TestBusWithHandlerInterface(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Create test handlers
	handler1 := &TestHandler{name: "handler1", priority: 100}
	handler2 := &TestHandler{name: "handler2", priority: 200}

	// Subscribe handlers
	id1 := bus.Subscribe(EventOnSavingThrow, handler1)
	_ = bus.Subscribe(EventOnSavingThrow, handler2)

	// Publish event
	event := NewGameEvent(EventOnSavingThrow, nil, nil)
	err := bus.Publish(ctx, event)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Verify both handlers were called
	if len(handler1.calls) != 1 {
		t.Errorf("Handler1 called %d times, expected 1", len(handler1.calls))
	}

	if len(handler2.calls) != 1 {
		t.Errorf("Handler2 called %d times, expected 1", len(handler2.calls))
	}

	// Unsubscribe one
	if err := bus.Unsubscribe(id1); err != nil {
		t.Errorf("Unsubscribe failed: %v", err)
	}

	// Publish again
	if err := bus.Publish(ctx, event); err != nil {
		t.Errorf("Publish after unsubscribe failed: %v", err)
	}

	// Only handler2 should be called
	if len(handler1.calls) != 1 {
		t.Error("Handler1 called after unsubscribe")
	}

	if len(handler2.calls) != 2 {
		t.Error("Handler2 not called")
	}
}

func TestEventModifiers(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Simulate rage and bless adding modifiers
	bus.SubscribeFunc(EventOnDamageRoll, 100, func(_ context.Context, e Event) error {
		// Rage adds damage bonus
		e.Context().AddModifier(NewIntModifier("rage", ModifierDamageBonus, 2, 100))
		return nil
	})

	bus.SubscribeFunc(EventOnDamageRoll, 200, func(_ context.Context, e Event) error {
		// Bless adds attack bonus
		e.Context().AddModifier(NewIntModifier("bless", ModifierAttackBonus, 4, 50))
		return nil
	})

	// Create and publish event
	event := NewGameEvent(EventOnDamageRoll, nil, nil)
	err := bus.Publish(ctx, event)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Check modifiers were added
	mods := event.Context().Modifiers()
	if len(mods) != 2 {
		t.Errorf("Expected 2 modifiers, got %d", len(mods))
	}

	// Verify modifier details
	foundRage := false
	foundBless := false
	for _, mod := range mods {
		switch mod.Source() {
		case "rage":
			foundRage = true
			if mod.ModifierValue().GetValue() != 2 {
				t.Error("Rage modifier has wrong value")
			}
		case "bless":
			foundBless = true
			if mod.ModifierValue().GetValue() != 4 {
				t.Error("Bless modifier has wrong value")
			}
		}
	}

	if !foundRage || !foundBless {
		t.Error("Missing expected modifiers")
	}
}

func TestBusCancelledEvent(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	// Track handler calls
	var handler1Called, handler2Called, handler3Called bool

	// Subscribe multiple handlers
	bus.SubscribeFunc(EventBeforeAttackRoll, 100, func(_ context.Context, e Event) error {
		handler1Called = true
		// First handler cancels the event
		e.Cancel()
		return nil
	})

	bus.SubscribeFunc(EventBeforeAttackRoll, 200, func(_ context.Context, _ Event) error {
		handler2Called = true
		return nil
	})

	bus.SubscribeFunc(EventBeforeAttackRoll, 300, func(_ context.Context, _ Event) error {
		handler3Called = true
		return nil
	})

	// Create and publish event
	source := &mockEntity{id: "player-1", entityType: "character"}
	event := NewGameEvent(EventBeforeAttackRoll, source, nil)

	err := bus.Publish(ctx, event)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// First handler should be called
	if !handler1Called {
		t.Error("First handler should have been called")
	}

	// Subsequent handlers should NOT be called
	if handler2Called {
		t.Error("Second handler should not have been called after cancellation")
	}

	if handler3Called {
		t.Error("Third handler should not have been called after cancellation")
	}

	// Event should be cancelled
	if !event.IsCancelled() {
		t.Error("Event should be marked as cancelled")
	}
}
