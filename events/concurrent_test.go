// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentSubscription verifies that handlers can subscribe during event processing
func TestConcurrentSubscription(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()
	ref, err := core.ParseString("test:concurrent:event")
	require.NoError(t, err)

	var newHandlerCalled bool
	var mu sync.Mutex

	// First handler that subscribes a new handler during event processing
	firstHandler := func(ctx context.Context, _ any) error {
		// Try to subscribe while processing - should not deadlock
		// because we release the read lock before calling handlers
		_, err := bus.Subscribe(ctx, ref, func(_ context.Context, _ any) error {
			mu.Lock()
			newHandlerCalled = true
			mu.Unlock()
			return nil
		})
		require.NoError(t, err)
		return nil
	}

	// Subscribe the first handler
	id, err := bus.Subscribe(ctx, ref, firstHandler)
	require.NoError(t, err)
	defer bus.Unsubscribe(ctx, id)

	// Create test event
	event := &TestEvent{
		Target: "test",
		Value:  1,
		ctx:    events.NewEventContext(),
	}

	// Override EventRef to return our ref
	oldRef := testEventRef
	testEventRef = ref
	defer func() { testEventRef = oldRef }()

	// Publish first event - should subscribe new handler without deadlock
	done := make(chan bool)
	go func() {
		err = bus.Publish(ctx, event)
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Deadlock detected - subscription during event processing blocked")
	}

	// Publish second event - new handler should be called
	err = bus.Publish(ctx, event)
	require.NoError(t, err)

	mu.Lock()
	assert.True(t, newHandlerCalled, "newly subscribed handler should have been called")
	mu.Unlock()
}

// TestConcurrentUnsubscribe verifies that handlers can unsubscribe during event processing
func TestConcurrentUnsubscribe(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()
	ref, err := core.ParseString("test:concurrent:unsub")
	require.NoError(t, err)

	var handler2Called int
	var mu sync.Mutex

	var handler2ID string

	// First handler that unsubscribes the second handler
	firstHandler := func(ctx context.Context, _ any) error {
		// Unsubscribe handler2 during event processing
		// Should not deadlock because we copy handlers before execution
		err := bus.Unsubscribe(ctx, handler2ID)
		// Might fail if handler2 already unsubscribed itself
		_ = err
		return nil
	}

	// Second handler that counts calls
	secondHandler := func(_ context.Context, _ any) error {
		mu.Lock()
		handler2Called++
		mu.Unlock()
		return nil
	}

	// Subscribe both handlers
	id1, err := bus.Subscribe(ctx, ref, firstHandler)
	require.NoError(t, err)
	defer bus.Unsubscribe(ctx, id1)

	handler2ID, err = bus.Subscribe(ctx, ref, secondHandler)
	require.NoError(t, err)
	// Don't defer unsubscribe - will be done by handler1

	// Create test event
	event := &TestEvent{
		Target: "test",
		Value:  1,
		ctx:    events.NewEventContext(),
	}

	// Override EventRef to return our ref
	oldRef := testEventRef
	testEventRef = ref
	defer func() { testEventRef = oldRef }()

	// First publish - both handlers called, handler2 gets unsubscribed
	err = bus.Publish(ctx, event)
	require.NoError(t, err)

	mu.Lock()
	firstCallCount := handler2Called
	mu.Unlock()
	assert.Equal(t, 1, firstCallCount, "handler2 should be called once")

	// Second publish - only handler1 called
	err = bus.Publish(ctx, event)
	require.NoError(t, err)

	mu.Lock()
	secondCallCount := handler2Called
	mu.Unlock()
	assert.Equal(t, 1, secondCallCount, "handler2 should not be called after unsubscribe")
}

// TestConcurrentPublish verifies that multiple goroutines can publish simultaneously
func TestConcurrentPublish(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()
	ref, err := core.ParseString("test:concurrent:publish")
	require.NoError(t, err)

	var callCount int32
	var mu sync.Mutex

	// Handler that counts calls
	handler := func(_ context.Context, e any) error {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	// Subscribe handler
	id, err := bus.Subscribe(ctx, ref, handler)
	require.NoError(t, err)
	defer bus.Unsubscribe(ctx, id)

	// Create test event
	event := &TestEvent{
		Target: "test",
		Value:  1,
		ctx:    events.NewEventContext(),
	}

	// Override EventRef to return our ref
	oldRef := testEventRef
	testEventRef = ref
	defer func() { testEventRef = oldRef }()

	// Publish from multiple goroutines
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := bus.Publish(ctx, event)
			assert.NoError(t, err)
		}()
	}

	// Wait for all publishes to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - all publishes completed
	case <-time.After(5 * time.Second):
		t.Fatal("Concurrent publish operations timed out")
	}

	// Verify all events were processed
	assert.Equal(t, int32(numGoroutines), callCount, "all events should have been processed")
}
