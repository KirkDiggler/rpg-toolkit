// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides a simple, type-safe event bus for game systems.
// Events are defined in their domain packages and the bus is just plumbing.
package events

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Event is the interface for all events.
// Events must return their ref for type-safe routing and provide a context for modifications.
type Event interface {
	EventRef() *core.Ref
	Context() *EventContext
}

// Filter determines if a handler should receive an event.
// Return true to receive the event, false to skip it.
type Filter func(event Event) bool

// EventBus handles event publishing and subscriptions.
type EventBus interface {
	// Publish sends an event with a context to all subscribers
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for events with the given ref
	// Handler must be func(context.Context, T) error where T is the event type
	Subscribe(ctx context.Context, ref *core.Ref, handler any) (string, error)

	// SubscribeWithFilter registers a handler with a filter
	SubscribeWithFilter(ctx context.Context, ref *core.Ref, handler any, filter Filter) (string, error)

	// Unsubscribe removes a subscription by ID
	Unsubscribe(ctx context.Context, id string) error

	// Clear removes all subscriptions (useful for tests)
	Clear()
}

// Bus is the simple, synchronous event bus implementation.
type Bus struct {
	mu           sync.RWMutex
	handlers     map[string][]handlerEntry
	nextID       int
	publishDepth int32 // Current recursion depth (atomic)
	maxDepth     int32 // Maximum allowed depth
}

type handlerEntry struct {
	id      string
	ref     *core.Ref // The ref this handler is subscribed to
	handler reflect.Value
	filter  Filter // nil means no filter
}

// Default limits for event cascading protection
const (
	DefaultMaxDepth = 10 // Maximum recursion depth
)

// NewBus creates a new event bus with default settings.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]handlerEntry),
		maxDepth: DefaultMaxDepth,
	}
}

// NewBusWithMaxDepth creates a new event bus with custom max depth.
func NewBusWithMaxDepth(maxDepth int32) *Bus {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return &Bus{
		handlers: make(map[string][]handlerEntry),
		maxDepth: maxDepth,
	}
}

// Publish sends an event to all registered handlers with the given context.
func (b *Bus) Publish(ctx context.Context, event Event) error {
	// Check recursion depth
	depth := atomic.AddInt32(&b.publishDepth, 1)
	defer atomic.AddInt32(&b.publishDepth, -1)

	// Check if we've hit max depth
	if depth > b.maxDepth {
		return fmt.Errorf("event cascade depth exceeded: current=%d, max=%d, event=%s",
			depth, b.maxDepth, event.EventRef())
	}

	// Note: Consumers can monitor depth via GetDepth() if they want warnings
	// We don't log here to avoid forcing logging behavior on library users

	// Phase 1: Collect handlers that match this event's ref
	var handlersToCall []handlerEntry

	b.mu.RLock()
	// Get handlers registered for this event's ref
	// Use ref value comparison, not pointer
	refStr := event.EventRef().String()
	if entries, ok := b.handlers[refStr]; ok {
		// Copy handlers to call (avoid holding lock during execution)
		handlersToCall = make([]handlerEntry, 0, len(entries))
		for _, entry := range entries {
			// Check filter
			if entry.filter != nil && !entry.filter(event) {
				continue
			}
			// Add to handlers to call
			handlersToCall = append(handlersToCall, entry)
		}
	}
	b.mu.RUnlock()

	// Phase 2: Call handlers without holding lock
	var deferred []*DeferredAction
	var immediateError error

	for _, entry := range handlersToCall {
		// Call handler with context
		results := entry.handler.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(event),
		})

		// Check what the handler returned
		if len(results) > 0 && !results[0].IsNil() {
			// Check if it's a DeferredAction or an error
			switch v := results[0].Interface().(type) {
			case *DeferredAction:
				// Handler returned deferred actions
				deferred = append(deferred, v)
			case error:
				// Handler returned an error
				immediateError = fmt.Errorf("handler %s failed: %w", entry.id, v)
			}
		}
	}

	// Return immediate errors
	if immediateError != nil {
		return immediateError
	}

	// Phase 3: Process deferred actions (no lock held)
	for _, action := range deferred {
		// Process unsubscribes
		for _, id := range action.Unsubscribes {
			if err := b.Unsubscribe(ctx, id); err != nil {
				// Ignore error - subscription might already be gone
				continue
			}
		}

		// Process publishes
		for _, evt := range action.Publishes {
			if err := b.Publish(ctx, evt); err != nil {
				return err
			}
		}

		// Check for deferred errors
		if action.Error != nil {
			return action.Error
		}
	}

	return nil
}

// Subscribe registers a handler for events with the given ref.
func (b *Bus) Subscribe(ctx context.Context, ref *core.Ref, handler any) (string, error) {
	// Check if context is already cancelled before proceeding
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("subscribe cancelled: %w", err)
	}
	return b.SubscribeWithFilter(ctx, ref, handler, nil)
}

// SubscribeWithFilter registers a handler with a filter.
func (b *Bus) SubscribeWithFilter(ctx context.Context, ref *core.Ref, handler any, filter Filter) (string, error) {
	// Check if context is already cancelled before proceeding
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("subscribe cancelled: %w", err)
	}

	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	// Validate handler signature
	if handlerType.Kind() != reflect.Func {
		return "", fmt.Errorf("handler must be a function")
	}

	// Handler must take exactly 2 parameters: context and event
	if handlerType.NumIn() != 2 {
		return "", fmt.Errorf("handler must take exactly 2 parameters (context.Context, event)")
	}

	// First parameter must be context.Context
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if handlerType.In(0) != contextType {
		return "", fmt.Errorf("handler first parameter must be context.Context")
	}

	// Handler must return either error or *DeferredAction
	if handlerType.NumOut() != 1 {
		return "", fmt.Errorf("handler must return exactly one value")
	}

	returnType := handlerType.Out(0)
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	deferredType := reflect.TypeOf((*DeferredAction)(nil))

	if returnType != errorType && returnType != deferredType {
		return "", fmt.Errorf("handler must return either error or *DeferredAction")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check again after acquiring lock in case we waited
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("subscribe cancelled while waiting for lock: %w", err)
	}

	// Generate subscription ID
	b.nextID++
	id := fmt.Sprintf("sub-%d", b.nextID)

	// Add handler - group by ref string value
	refStr := ref.String()
	b.handlers[refStr] = append(b.handlers[refStr], handlerEntry{
		id:      id,
		ref:     ref, // Store ref for reference, but not used for matching
		handler: handlerValue,
		filter:  filter,
	})

	return id, nil
}

// Unsubscribe removes a subscription by ID.
func (b *Bus) Unsubscribe(ctx context.Context, id string) error {
	// Check if context is already cancelled before proceeding
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("unsubscribe cancelled: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check again after acquiring lock in case we waited
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("unsubscribe cancelled while waiting for lock: %w", err)
	}

	// Find and remove the handler
	for eventType, handlers := range b.handlers {
		for i, entry := range handlers {
			if entry.id == id {
				// Remove this handler
				b.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("subscription %s not found", id)
}

// Clear removes all subscriptions.
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers = make(map[string][]handlerEntry)
}

// GetDepth returns the current event publishing depth (for monitoring).
func (b *Bus) GetDepth() int32 {
	return atomic.LoadInt32(&b.publishDepth)
}

// GetMaxDepth returns the maximum allowed depth.
func (b *Bus) GetMaxDepth() int32 {
	return b.maxDepth
}
