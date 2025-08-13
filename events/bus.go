// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides a simple, type-safe event bus for game systems.
// Events are defined in their domain packages and the bus is just plumbing.
package events

import (
	"fmt"
	"reflect"
	"sync"
)

// Event is the minimal interface for all events.
// Events are defined in their domain packages.
type Event interface {
	Type() string // Returns the event type identifier
}

// Filter determines if a handler should receive an event.
// Return true to receive the event, false to skip it.
type Filter func(event Event) bool

// EventBus handles event publishing and subscriptions.
type EventBus interface {
	// Publish sends an event to all subscribers
	Publish(event Event) error
	
	// Subscribe registers a handler for events of the given type
	// Handler should be func(T) error where T is the event type
	Subscribe(eventType string, handler any) (string, error)
	
	// SubscribeWithFilter registers a handler with a filter
	SubscribeWithFilter(eventType string, handler any, filter Filter) (string, error)
	
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
	
	eventType := event.Type()
	handlers := b.handlers[eventType]
	
	// Call each handler that passes its filter
	eventValue := reflect.ValueOf(event)
	for _, entry := range handlers {
		// Check filter
		if entry.filter != nil && !entry.filter(event) {
			continue // Skip this handler
		}
		
		// Call handler
		results := entry.handler.Call([]reflect.Value{eventValue})
		
		// Check for error return
		if len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return fmt.Errorf("handler %s failed: %w", entry.id, err)
			}
		}
	}
	
	return nil
}

// Subscribe registers a handler for events of the given type.
func (b *Bus) Subscribe(eventType string, handler any) (string, error) {
	return b.SubscribeWithFilter(eventType, handler, nil)
}

// SubscribeWithFilter registers a handler with a filter.
func (b *Bus) SubscribeWithFilter(eventType string, handler any, filter Filter) (string, error) {
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
	
	// Add handler
	b.handlers[eventType] = append(b.handlers[eventType], handlerEntry{
		id:      id,
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