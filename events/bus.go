// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides a simple, type-safe event bus for game systems.
// Events are defined in their domain packages and the bus is just plumbing.
package events

import (
	"fmt"
	"log"
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

	// Subscribe registers a handler for events with the given ref
	// Handler should be func(T) error where T is the event type
	Subscribe(ref *core.Ref, handler any) (string, error)

	// SubscribeWithFilter registers a handler with a filter
	SubscribeWithFilter(ref *core.Ref, handler any, filter Filter) (string, error)

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
	id      string
	ref     *core.Ref // The ref this handler is subscribed to
	handler reflect.Value
	filter  Filter // nil means no filter
}

// Default limits for event cascading protection
const (
	DefaultMaxDepth  = 10 // Maximum recursion depth
	WarnDepthPercent = 70 // Warn at 70% of max depth (7 levels)
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

// Publish sends an event to all registered handlers.
func (b *Bus) Publish(event Event) error {
	// Check recursion depth
	depth := atomic.AddInt32(&b.publishDepth, 1)
	defer atomic.AddInt32(&b.publishDepth, -1)

	// Check if we've hit max depth
	if depth > b.maxDepth {
		return fmt.Errorf("event cascade depth exceeded: current=%d, max=%d, event=%s",
			depth, b.maxDepth, event.EventRef())
	}

	// Warn if we're getting close to the limit
	warnThreshold := (b.maxDepth * WarnDepthPercent) / 100
	if depth > warnThreshold && depth <= warnThreshold+1 {
		// Only warn once when crossing threshold
		log.Printf("WARNING: Event cascade depth %d approaching limit %d for event %s",
			depth, b.maxDepth, event.EventRef())
	}

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

			// Call handler
			eventValue := reflect.ValueOf(event)
			results := entry.handler.Call([]reflect.Value{eventValue})

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
					break
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
				// Log but don't fail - subscription might already be gone
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

	if handlerType.NumIn() != 1 {
		return "", fmt.Errorf("handler must take exactly one parameter")
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
		id:      id,
		ref:     ref,
		handler: handlerValue,
		filter:  filter,
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
