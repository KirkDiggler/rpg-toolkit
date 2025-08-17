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
	// Publish sends an event to all subscribers
	Publish(event Event) error

	// PublishWithContext sends an event with a context for cancellation and values
	PublishWithContext(ctx context.Context, event Event) error

	// Subscribe registers a handler for events with the given ref
	// Handler can be either func(T) error or func(context.Context, T) error where T is the event type
	Subscribe(ref *core.Ref, handler any) (string, error)

	// SubscribeWithFilter registers a handler with a filter
	SubscribeWithFilter(ref *core.Ref, handler any, filter Filter) (string, error)

	// SubscribeFunc registers a handler function with priority (for compatibility)
	SubscribeFunc(eventType string, priority int, handler func(context.Context, Event) error) (string, error)

	// Unsubscribe removes a subscription by ID
	Unsubscribe(id string) error

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
	id             string
	ref            *core.Ref // The ref this handler is subscribed to
	handler        reflect.Value
	filter         Filter // nil means no filter
	acceptsContext bool   // true if handler takes context as first parameter
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

// Publish sends an event to all registered handlers using context.Background().
func (b *Bus) Publish(event Event) error {
	return b.PublishWithContext(context.Background(), event)
}

// PublishWithContext sends an event to all registered handlers with the given context.
func (b *Bus) PublishWithContext(ctx context.Context, event Event) error {
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

	// Phase 1: Collect handlers and call them (with read lock)
	var deferred []*DeferredAction
	var immediateError error

	b.mu.RLock()
	// Find handlers by comparing ref pointers
	for _, entries := range b.handlers {
		for _, entry := range entries {
			// Check if this handler wants this event (pointer comparison!)
			if entry.ref != event.EventRef() {
				continue
			}

			// Check filter
			if entry.filter != nil && !entry.filter(event) {
				continue
			}

			// Call handler with or without context
			var results []reflect.Value
			if entry.acceptsContext {
				// Handler expects context as first parameter
				results = entry.handler.Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(event),
				})
			} else {
				// Legacy handler without context
				results = entry.handler.Call([]reflect.Value{
					reflect.ValueOf(event),
				})
			}

			// Check what the handler returned
			if len(results) > 0 && !results[0].IsNil() {
				// Check if it's a DeferredAction or an error
				switch v := results[0].Interface().(type) {
				case *DeferredAction:
					// Handler returned deferred actions
					deferred = append(deferred, v)
				case error:
					// Handler returned an error (backwards compatibility)
					immediateError = fmt.Errorf("handler %s failed: %w", entry.id, v)
				}
			}
		}
		if immediateError != nil {
			break
		}
	}
	b.mu.RUnlock()

	// Return immediate errors
	if immediateError != nil {
		return immediateError
	}

	// Phase 2: Process deferred actions (no lock held)
	for _, action := range deferred {
		// Process unsubscribes
		for _, id := range action.Unsubscribes {
			if err := b.Unsubscribe(id); err != nil {
				// Ignore error - subscription might already be gone
				continue
			}
		}

		// Process publishes
		for _, evt := range action.Publishes {
			if err := b.Publish(evt); err != nil {
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
func (b *Bus) Subscribe(ref *core.Ref, handler any) (string, error) {
	return b.SubscribeWithFilter(ref, handler, nil)
}

// SubscribeWithFilter registers a handler with a filter.
func (b *Bus) SubscribeWithFilter(ref *core.Ref, handler any, filter Filter) (string, error) {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	// Validate handler signature
	if handlerType.Kind() != reflect.Func {
		return "", fmt.Errorf("handler must be a function")
	}

	// Check if handler accepts context as first parameter
	acceptsContext := false
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()

	if handlerType.NumIn() == 2 {
		// Check if first parameter is context.Context
		if handlerType.In(0) == contextType {
			acceptsContext = true
		} else {
			return "", fmt.Errorf("handler with 2 parameters must have context.Context as first parameter")
		}
	} else if handlerType.NumIn() != 1 {
		return "", fmt.Errorf("handler must take either 1 parameter (event) or 2 parameters (context, event)")
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

	// Generate subscription ID
	b.nextID++
	id := fmt.Sprintf("sub-%d", b.nextID)

	// Add handler - use ref string just for grouping in map
	refStr := ref.String()
	b.handlers[refStr] = append(b.handlers[refStr], handlerEntry{
		id:             id,
		ref:            ref,
		handler:        handlerValue,
		filter:         filter,
		acceptsContext: acceptsContext,
	})

	return id, nil
}

// Unsubscribe removes a subscription by ID.
func (b *Bus) Unsubscribe(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

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

// SubscribeFunc registers a handler function with priority (for compatibility).
// This method exists for backward compatibility with code that expects it.
// The priority parameter is currently ignored as the bus processes handlers in registration order.
func (b *Bus) SubscribeFunc(eventType string, _ int, handler func(context.Context, Event) error) (string, error) {
	// Parse the event type string to get a ref
	ref, err := core.ParseString(eventType)
	if err != nil {
		// If parsing fails, create a simple ref
		ref = &core.Ref{
			Module: "legacy",
			Type:   "event",
			Value:  eventType,
		}
	}

	// Wrap the handler to match our expected signature
	wrappedHandler := func(ctx context.Context, e any) error {
		event, ok := e.(Event)
		if !ok {
			return nil // Skip if not an Event
		}
		return handler(ctx, event)
	}

	return b.Subscribe(ref, wrappedHandler)
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
