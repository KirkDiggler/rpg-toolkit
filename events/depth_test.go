// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test ref for depth testing
var testDepthEventRef = func() *core.Ref {
	r, _ := core.ParseString("test:event:depth")
	return r
}()

// Test event for depth testing
type TestDepthEvent struct {
	ctx   *events.EventContext
	Level int
}

func (e *TestDepthEvent) EventRef() *core.Ref {
	return testDepthEventRef
}

func (e *TestDepthEvent) Context() *events.EventContext {
	return e.ctx
}

func NewTestDepthEvent(level int) *TestDepthEvent {
	return &TestDepthEvent{
		ctx:   events.NewEventContext(),
		Level: level,
	}
}

func TestDepthProtection_MaxDepthExceeded(t *testing.T) {
	bus := events.NewBusWithMaxDepth(5)

	// Handler that always triggers another event
	handler := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		// Each handler triggers the next level
		return events.NewDeferredAction().Publish(NewTestDepthEvent(event.Level + 1))
	}

	_, err := bus.Subscribe(testDepthEventRef, handler)
	require.NoError(t, err)

	// Start cascade - should fail when depth exceeds 5
	err = bus.Publish(NewTestDepthEvent(1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event cascade depth exceeded")
	assert.Contains(t, err.Error(), "max=5")
}

func TestDepthProtection_ExactLimit(t *testing.T) {
	bus := events.NewBusWithMaxDepth(3)

	var reached []int

	// Handler that triggers next level only if below limit
	handler := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		reached = append(reached, event.Level)

		if event.Level < 3 {
			return events.NewDeferredAction().Publish(NewTestDepthEvent(event.Level + 1))
		}
		return nil
	}

	_, err := bus.Subscribe(testDepthEventRef, handler)
	require.NoError(t, err)

	// Should succeed - stays within limit
	err = bus.Publish(NewTestDepthEvent(1))
	require.NoError(t, err)

	// Should have processed levels 1, 2, 3
	assert.Equal(t, []int{1, 2, 3}, reached)
}

func TestDepthProtection_DefaultLimit(t *testing.T) {
	bus := events.NewBus() // Uses default limit (10)

	// Verify default is 10
	assert.Equal(t, int32(10), bus.GetMaxDepth())

	cascadeCount := 0

	// Handler that counts cascades
	handler := func(_ any) *events.DeferredAction {
		cascadeCount++
		if cascadeCount < 15 { // Try to go beyond default
			return events.NewDeferredAction().Publish(NewTestDepthEvent(cascadeCount))
		}
		return nil
	}

	_, err := bus.Subscribe(testDepthEventRef, handler)
	require.NoError(t, err)

	// Should fail at depth 11
	err = bus.Publish(NewTestDepthEvent(0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth exceeded")

	// Should have stopped at 10
	assert.Equal(t, 10, cascadeCount)
}

func TestDepthProtection_MultipleHandlers(t *testing.T) {
	bus := events.NewBusWithMaxDepth(4)

	// Handler A triggers B
	handlerA := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		if event.Level == 1 {
			return events.NewDeferredAction().Publish(NewTestDepthEvent(2))
		}
		return nil
	}

	// Handler B triggers C
	handlerB := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		if event.Level == 2 {
			return events.NewDeferredAction().Publish(NewTestDepthEvent(3))
		}
		return nil
	}

	// Handler C triggers D
	handlerC := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		if event.Level == 3 {
			return events.NewDeferredAction().Publish(NewTestDepthEvent(4))
		}
		return nil
	}

	// Handler D tries to go further
	handlerD := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		if event.Level == 4 {
			// This should fail - exceeds depth
			return events.NewDeferredAction().Publish(NewTestDepthEvent(5))
		}
		return nil
	}

	// Subscribe all handlers
	_, err := bus.Subscribe(testDepthEventRef, handlerA)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDepthEventRef, handlerB)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDepthEventRef, handlerC)
	require.NoError(t, err)
	_, err = bus.Subscribe(testDepthEventRef, handlerD)
	require.NoError(t, err)

	// Start cascade
	err = bus.Publish(NewTestDepthEvent(1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth exceeded")
}

func TestDepthProtection_ResetsBetweenCalls(t *testing.T) {
	bus := events.NewBusWithMaxDepth(2)

	handler := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		if event.Level == 1 {
			// One level of cascade
			return events.NewDeferredAction().Publish(NewTestDepthEvent(2))
		}
		return nil
	}

	_, err := bus.Subscribe(testDepthEventRef, handler)
	require.NoError(t, err)

	// First call - should work
	err = bus.Publish(NewTestDepthEvent(1))
	require.NoError(t, err)
	assert.Equal(t, int32(0), bus.GetDepth()) // Depth reset to 0

	// Second call - should also work (depth was reset)
	err = bus.Publish(NewTestDepthEvent(1))
	require.NoError(t, err)
	assert.Equal(t, int32(0), bus.GetDepth()) // Depth reset to 0 again
}

func TestDepthProtection_GetDepthDuringExecution(t *testing.T) {
	bus := events.NewBusWithMaxDepth(5)

	depths := []int32{}

	handler := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		// Capture current depth during execution
		depths = append(depths, bus.GetDepth())

		if event.Level < 3 {
			return events.NewDeferredAction().Publish(NewTestDepthEvent(event.Level + 1))
		}
		return nil
	}

	_, err := bus.Subscribe(testDepthEventRef, handler)
	require.NoError(t, err)

	err = bus.Publish(NewTestDepthEvent(1))
	require.NoError(t, err)

	// Should see increasing depths: 1, 2, 3
	assert.Equal(t, []int32{1, 2, 3}, depths)

	// After completion, depth should be 0
	assert.Equal(t, int32(0), bus.GetDepth())
}

func TestDepthProtection_InvalidMaxDepth(t *testing.T) {
	// Zero or negative max depth should use default
	bus1 := events.NewBusWithMaxDepth(0)
	assert.Equal(t, int32(10), bus1.GetMaxDepth())

	bus2 := events.NewBusWithMaxDepth(-5)
	assert.Equal(t, int32(10), bus2.GetMaxDepth())
}

func TestDepthProtection_ErrorPropagation(t *testing.T) {
	bus := events.NewBusWithMaxDepth(3)

	// Handler that triggers cascade
	handler := func(e any) *events.DeferredAction {
		event := e.(*TestDepthEvent)
		// Always cascade (will hit limit)
		return events.NewDeferredAction().Publish(NewTestDepthEvent(event.Level + 1))
	}

	_, err := bus.Subscribe(testDepthEventRef, handler)
	require.NoError(t, err)

	// The error should bubble up through the cascade
	err = bus.Publish(NewTestDepthEvent(1))
	require.Error(t, err)

	// Error message should be informative
	assert.True(t, strings.Contains(err.Error(), "depth exceeded") ||
		strings.Contains(err.Error(), "cascade"))
	assert.True(t, strings.Contains(err.Error(), "3") ||
		strings.Contains(err.Error(), "4")) // Should mention the limit or current depth
}

