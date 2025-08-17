// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test ref for deferred operations
var testDeferredEventRef = func() *core.Ref {
	r, _ := core.ParseString("test:event:deferred")
	return r
}()

// Test event for deferred operations
type TestDeferredEvent struct {
	ctx *events.EventContext
	ID  string
}

func (e *TestDeferredEvent) EventRef() *core.Ref {
	return testDeferredEventRef
}

func (e *TestDeferredEvent) Context() *events.EventContext {
	return e.ctx
}

func NewTestDeferredEvent(id string) *TestDeferredEvent {
	return &TestDeferredEvent{
		ctx: events.NewEventContext(),
		ID:  id,
	}
}

func TestDeferredOperations_BackwardsCompatibility(t *testing.T) {
	bus := events.NewBus()

	// Old-style handler that returns error
	called := false
	oldHandler := func(e any) error {
		called = true
		return nil
	}

	sub, err := bus.Subscribe(testDeferredEventRef, oldHandler)
	require.NoError(t, err)
	require.NotEmpty(t, sub)

	// Publish event
	err = bus.Publish(NewTestDeferredEvent("test"))
	require.NoError(t, err)
	assert.True(t, called)
}

func TestDeferredOperations_Unsubscribe(t *testing.T) {
	bus := events.NewBus()

	var subID string

	// Handler that unsubscribes itself
	handler := func(e any) *events.DeferredAction {
		event := e.(*TestDeferredEvent)
		if event.ID == "trigger-unsub" {
			// Return deferred unsubscribe
			return events.NewDeferredAction().Unsubscribe(subID)
		}
		return nil
	}

	// Track if handler is called after unsubscribe
	var callCount int32
	trackingHandler := func(e any) *events.DeferredAction {
		atomic.AddInt32(&callCount, 1)
		return handler(e)
	}

	subID, err := bus.Subscribe(testDeferredEventRef, trackingHandler)
	require.NoError(t, err)

	// First event - handler should be called
	err = bus.Publish(NewTestDeferredEvent("first"))
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Second event triggers unsubscribe
	err = bus.Publish(NewTestDeferredEvent("trigger-unsub"))
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))

	// Third event - handler should NOT be called (unsubscribed)
	err = bus.Publish(NewTestDeferredEvent("third"))
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "handler should not be called after unsubscribe")
}

func TestDeferredOperations_CascadingEvents(t *testing.T) {
	bus := events.NewBus()

	var sequence []string
	mu := sync.Mutex{}

	// Handler A triggers event B only for event A
	handlerA := func(e any) *events.DeferredAction {
		event := e.(*TestDeferredEvent)
		if event.ID == "A" {
			mu.Lock()
			sequence = append(sequence, "A")
			mu.Unlock()

			// Trigger B event
			return events.NewDeferredAction().Publish(NewTestDeferredEvent("B"))
		}
		return nil
	}

	// Handler B triggers event C only for event B
	handlerB := func(e any) *events.DeferredAction {
		event := e.(*TestDeferredEvent)
		if event.ID == "B" {
			mu.Lock()
			sequence = append(sequence, "B")
			mu.Unlock()

			// Trigger C event
			return events.NewDeferredAction().Publish(NewTestDeferredEvent("C"))
		}
		return nil
	}

	// Handler C is final, only processes event C
	handlerC := func(e any) *events.DeferredAction {
		event := e.(*TestDeferredEvent)
		if event.ID == "C" {
			mu.Lock()
			sequence = append(sequence, "C")
			mu.Unlock()
		}
		return nil
	}

	// Subscribe all handlers
	_, err := bus.Subscribe(testDeferredEventRef, handlerA)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDeferredEventRef, handlerB)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDeferredEventRef, handlerC)
	require.NoError(t, err)

	// Start cascade with event A
	err = bus.Publish(NewTestDeferredEvent("A"))
	require.NoError(t, err)

	// Check sequence
	assert.Equal(t, []string{"A", "B", "C"}, sequence)
}

func TestDeferredOperations_ErrorHandling(t *testing.T) {
	bus := events.NewBus()

	testErr := errors.New("test error")

	// Handler that returns deferred error
	handler := func(e any) *events.DeferredAction {
		return events.NewDeferredAction().WithError(testErr)
	}

	_, err := bus.Subscribe(testDeferredEventRef, handler)
	require.NoError(t, err)

	// Publish should return the deferred error
	err = bus.Publish(NewTestDeferredEvent("test"))
	assert.Equal(t, testErr, err)
}

func TestDeferredOperations_MultipleDeferred(t *testing.T) {
	bus := events.NewBus()

	var eventList []string
	mu := sync.Mutex{}

	// Handler 1 publishes event X (only for initial)
	handler1 := func(e any) *events.DeferredAction {
		event := e.(*TestDeferredEvent)
		if event.ID == "initial" {
			return events.NewDeferredAction().Publish(NewTestDeferredEvent("X"))
		}
		return nil
	}

	// Handler 2 publishes event Y (only for initial)
	handler2 := func(e any) *events.DeferredAction {
		event := e.(*TestDeferredEvent)
		if event.ID == "initial" {
			return events.NewDeferredAction().Publish(NewTestDeferredEvent("Y"))
		}
		return nil
	}

	// Track all events
	tracker := func(e any) error {
		event := e.(*TestDeferredEvent)
		mu.Lock()
		eventList = append(eventList, event.ID)
		mu.Unlock()
		return nil
	}

	// Subscribe handlers
	_, err := bus.Subscribe(testDeferredEventRef, handler1)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDeferredEventRef, handler2)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDeferredEventRef, tracker)
	require.NoError(t, err)

	// Publish initial event
	err = bus.Publish(NewTestDeferredEvent("initial"))
	require.NoError(t, err)

	// Should have: initial, X, Y (order of X and Y may vary)
	assert.Contains(t, eventList, "initial")
	assert.Contains(t, eventList, "X")
	assert.Contains(t, eventList, "Y")
}

func TestDeferredOperations_NoDeadlock(t *testing.T) {
	// This test verifies the original deadlock scenario is fixed
	bus := events.NewBus()

	var subID string

	// Handler that tries to unsubscribe during event processing
	// This would deadlock in the old implementation
	handler := func(e any) *events.DeferredAction {
		// With deferred operations, this is safe
		return events.NewDeferredAction().
			Unsubscribe(subID).
			Publish(NewTestDeferredEvent("removal-complete"))
	}

	subID, err := bus.Subscribe(testDeferredEventRef, handler)
	require.NoError(t, err)

	// Track removal event
	removalReceived := false
	removalHandler := func(e any) error {
		event := e.(*TestDeferredEvent)
		if event.ID == "removal-complete" {
			removalReceived = true
		}
		return nil
	}

	_, err = bus.Subscribe(testDeferredEventRef, removalHandler)
	require.NoError(t, err)

	// This should not deadlock
	done := make(chan bool)
	go func() {
		err = bus.Publish(NewTestDeferredEvent("trigger"))
		done <- true
	}()

	// Wait with timeout
	select {
	case <-done:
		// Success - no deadlock
		require.NoError(t, err)
		assert.True(t, removalReceived)
	case <-time.After(1 * time.Second):
		t.Fatal("Deadlock detected - operation timed out")
	}
}

