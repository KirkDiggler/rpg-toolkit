// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscribeWithCancelledContext verifies Subscribe respects context cancellation
func TestSubscribeWithCancelledContext(t *testing.T) {
	bus := events.NewBus()
	ref, err := core.ParseString("test:cancel:event")
	require.NoError(t, err)

	// Create an already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to subscribe with cancelled context
	_, err = bus.Subscribe(ctx, ref, func(_ context.Context, _ any) error {
		return nil
	})

	// Should fail due to cancelled context
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// TestUnsubscribeWithCancelledContext verifies Unsubscribe respects context cancellation
func TestUnsubscribeWithCancelledContext(t *testing.T) {
	bus := events.NewBus()
	ref, err := core.ParseString("test:cancel:event")
	require.NoError(t, err)

	// First subscribe successfully
	ctx := context.Background()
	id, err := bus.Subscribe(ctx, ref, func(_ context.Context, _ any) error {
		return nil
	})
	require.NoError(t, err)

	// Create a cancelled context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to unsubscribe with cancelled context
	err = bus.Unsubscribe(cancelCtx, id)

	// Should fail due to cancelled context
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")

	// Verify subscription still exists by unsubscribing with valid context
	err = bus.Unsubscribe(ctx, id)
	assert.NoError(t, err)
}

// TestSubscribeWithFilterCancelledContext verifies SubscribeWithFilter respects context
func TestSubscribeWithFilterCancelledContext(t *testing.T) {
	bus := events.NewBus()
	ref, err := core.ParseString("test:cancel:event")
	require.NoError(t, err)

	// Create an already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to subscribe with filter using cancelled context
	_, err = bus.SubscribeWithFilter(ctx, ref,
		func(_ context.Context, _ any) error {
			return nil
		},
		func(_ events.Event) bool {
			return true
		},
	)

	// Should fail due to cancelled context
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}
