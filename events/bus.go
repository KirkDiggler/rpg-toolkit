// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides a simple, type-safe event bus for game systems.
// Events are defined in their domain packages and the bus is just plumbing.
package events

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Event is the interface for all events.
// Events must return their ref for type-safe routing.
type Event interface {
	EventRef() *core.Ref
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
	mu       sync.RWMutex
	handlers map[string][]handlerEntry
	nextID   int
}

type handlerEntry struct {
	id      string
	ref     *core.Ref // The ref this handler is subscribed to
	handler reflect.Value
	filter  Filter // nil means no filter
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]handlerEntry),
	}
}

// Publish sends an event to all registered handlers.
func (b *Bus) Publish(event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

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

			// Check for error return
			if len(results) > 0 && !results[0].IsNil() {
				if err, ok := results[0].Interface().(error); ok {
					return fmt.Errorf("handler %s failed: %w", entry.id, err)
				}
			}
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

	if handlerType.NumOut() != 1 || handlerType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		return "", fmt.Errorf("handler must return error")
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
