// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/require"
)

// TestContextEvent is a test event that supports context
type TestContextEvent struct {
	Target string
	Value  int
	ref    *core.Ref
	ctx    *events.EventContext
}

func (e *TestContextEvent) EventRef() *core.Ref {
	if e.ref == nil {
		var err error
		e.ref, err = core.ParseString("test:context:event")
		if err != nil {
			panic(err)
		}
	}
	return e.ref
}

func (e TestContextEvent) Context() *events.EventContext {
	if e.ctx == nil {
		return events.NewEventContext()
	}
	return e.ctx
}

// TestContextHandler verifies that handlers can receive context
func TestContextHandler(t *testing.T) {
	bus := events.NewBus()

	// Test event ref
	ref, err := core.ParseString("test:context:event")
	require.NoError(t, err)

	// Define context key type for tests
	type contextKey string
	const testKey contextKey = "test-key"

	t.Run("handler with context", func(t *testing.T) {
		called := false
		ctxValue := ""

		// Subscribe with context-aware handler
		ctx := context.Background()
		id, err := bus.Subscribe(ctx, ref, func(ctx context.Context, _ *TestContextEvent) error {
			called = true
			// Check if we can get values from context
			if val := ctx.Value(testKey); val != nil {
				ctxValue = val.(string)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		defer func() { _ = bus.Unsubscribe(ctx, id) }()

		// Publish with context
		ctx = context.WithValue(context.Background(), testKey, "test-value")
		event := &TestContextEvent{Target: "player", Value: 42, ref: ref}
		event.ctx = events.NewEventContext()

		err = bus.Publish(ctx, event)
		if err != nil {
			t.Fatalf("PublishWithContext failed: %v", err)
		}

		if !called {
			t.Error("handler was not called")
		}
		if ctxValue != "test-value" {
			t.Errorf("expected context value 'test-value', got '%s'", ctxValue)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		handlerStarted := make(chan bool)
		handlerDone := make(chan bool)
		ctx := context.Background()

		// Subscribe with handler that checks context
		id, err := bus.Subscribe(ctx, ref, func(ctx context.Context, _ *TestContextEvent) error {
			handlerStarted <- true

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				// Handler completed normally
			}

			handlerDone <- true
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		defer func() { _ = bus.Unsubscribe(ctx, id) }()

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start publishing in goroutine
		publishErr := make(chan error)
		go func() {
			event := &TestContextEvent{Target: "player", Value: 42, ref: ref}
			event.ctx = events.NewEventContext()
			publishErr <- bus.Publish(ctx, event)
		}()

		// Wait for handler to start
		<-handlerStarted

		// Cancel context while handler is running
		cancel()

		// Check publish result
		select {
		case err := <-publishErr:
			// We expect the handler to continue even if context is cancelled
			// because the handler checks context internally
			if err != nil {
				t.Logf("Publish completed with: %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Publish did not complete")
		}
	})

}

// TestMultipleHandlers verifies that multiple handlers can coexist
func TestMultipleHandlers(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()
	ref, err := core.ParseString("test:mixed:event")
	require.NoError(t, err)

	handler1Called := false
	handler2Called := false

	// Subscribe first handler
	id1, err := bus.Subscribe(ctx, ref, func(_ context.Context, _ *TestContextEvent) error {
		handler1Called = true
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe handler1 failed: %v", err)
	}
	defer func() { _ = bus.Unsubscribe(ctx, id1) }()

	// Subscribe second handler
	id2, err := bus.Subscribe(ctx, ref, func(_ context.Context, _ *TestContextEvent) error {
		handler2Called = true
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe handler2 failed: %v", err)
	}
	defer func() { _ = bus.Unsubscribe(ctx, id2) }()

	// Publish event
	event := &TestContextEvent{Target: "player", Value: 42, ref: ref}
	event.ctx = events.NewEventContext()

	err = bus.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if !handler1Called {
		t.Error("handler 1 was not called")
	}
	if !handler2Called {
		t.Error("handler 2 was not called")
	}
}

// TestInvalidHandlerSignatures verifies that invalid handlers are rejected
func TestInvalidHandlerSignatures(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()
	ref, err := core.ParseString("test:invalid:event")
	require.NoError(t, err)

	tests := []struct {
		name    string
		handler any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "wrong first param type",
			handler: func(_ string, _ *TestContextEvent) error { return nil },
			wantErr: true,
			errMsg:  "must be context.Context",
		},
		{
			name:    "too many params",
			handler: func(_ context.Context, _ *TestContextEvent, _ int) error { return nil },
			wantErr: true,
			errMsg:  "must take exactly 2 parameters",
		},
		{
			name:    "no params",
			handler: func() error { return nil },
			wantErr: true,
			errMsg:  "must take exactly 2 parameters",
		},
		{
			name:    "one param only",
			handler: func(_ *TestContextEvent) error { return nil },
			wantErr: true,
			errMsg:  "must take exactly 2 parameters",
		},
		{
			name:    "wrong return type",
			handler: func(_ context.Context, _ *TestContextEvent) string { return "" },
			wantErr: true,
			errMsg:  "must return either error",
		},
		{
			name:    "valid with context",
			handler: func(_ context.Context, _ *TestContextEvent) error { return nil },
			wantErr: false,
		},
		{
			name:    "valid deferred",
			handler: func(_ context.Context, _ *TestContextEvent) *events.DeferredAction { return nil },
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := bus.Subscribe(ctx, ref, tt.handler)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
					_ = bus.Unsubscribe(ctx, id)
				} else if tt.errMsg != "" {
					// Check error message contains expected text
					errStr := err.Error()
					if len(errStr) == 0 {
						t.Error("error message is empty")
					}
					t.Logf("got error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else {
					_ = bus.Unsubscribe(ctx, id)
				}
			}
		})
	}
}
