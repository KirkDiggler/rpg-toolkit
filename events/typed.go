// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"reflect"
	
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// RefEvent is an event that knows its ref.
// Events should return a package-level ref variable.
type RefEvent interface {
	EventRef() *core.Ref
}

// EventHandler is a typed event handler function.
type EventHandler[T any] = func(T) error

// EventFilter is a typed event filter function.
type EventFilter[T any] = func(T) bool

// Option configures a subscription.
type Option[T any] interface {
	apply(*subscription[T])
}

type subscription[T any] struct {
	filter EventFilter[T]
}

type filterOption[T any] struct {
	filter EventFilter[T]
}

func (f filterOption[T]) apply(s *subscription[T]) {
	s.filter = f.filter
}

// Where creates a filter option for subscriptions.
func Where[T any](filter EventFilter[T]) Option[T] {
	return filterOption[T]{filter: filter}
}

// Publish sends an event that knows its own ref.
// The event's EventRef() method provides the routing key.
func Publish[T RefEvent](bus EventBus, event T) error {
	// Register this event type for duplicate detection
	refString := event.EventRef().String()
	eventType := reflect.TypeOf(event)
	
	if err := RegisterEventType(refString, eventType); err != nil {
		// Log but don't fail - duplicates are mainly a debugging concern
		// In production this would be logged
	}
	
	// Wrap to implement Event interface
	return bus.Publish(&refEventWrapper[T]{event})
}

type refEventWrapper[T RefEvent] struct {
	event T
}

func (w *refEventWrapper[T]) Type() string {
	return w.event.EventRef().String()
}

// Subscribe provides type-safe subscription with ref validation.
// The TypedRef ensures compile-time type safety and runtime validation.
func Subscribe[T RefEvent](
	bus EventBus,
	ref *core.TypedRef[T],
	handler EventHandler[T],
	opts ...Option[T],
) (string, error) {
	// Apply options
	sub := &subscription[T]{}
	for _, opt := range opts {
		opt.apply(sub)
	}
	
	// Wrap handler with validation
	wrappedHandler := func(e any) error {
		// Handle both wrapped and unwrapped events
		var typed T
		switch v := e.(type) {
		case T:
			typed = v
		case *refEventWrapper[T]:
			typed = v.event
		default:
			return nil // Wrong type, skip
		}
		
		// Validate refs match (pointer comparison!)
		// This ensures the event and subscription use the same ref object
		if typed.EventRef() != ref.Ref {
			// This indicates a bug in the package definition
			return &ErrRefMismatch{
				EventType:   reflect.TypeOf(typed).String(),
				EventRef:    typed.EventRef().String(),
				ExpectedRef: ref.String(),
			}
		}
		
		return handler(typed)
	}
	
	// Convert typed filter if present
	var busFilter Filter
	if sub.filter != nil {
		busFilter = func(e Event) bool {
			// Extract the actual event from wrapper if needed
			var typed T
			switch v := e.(type) {
			case T:
				typed = v
			case *refEventWrapper[T]:
				typed = v.event
			default:
				return false
			}
			return sub.filter(typed)
		}
	}
	
	// Subscribe using the ref's string as routing key
	if busFilter != nil {
		return bus.SubscribeWithFilter(ref.String(), wrappedHandler, busFilter)
	}
	return bus.Subscribe(ref.String(), wrappedHandler)
}