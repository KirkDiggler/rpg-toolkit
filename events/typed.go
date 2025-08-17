// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// EventHandler is a typed event handler function.
type EventHandler[T any] = func(T) error

// ContextEventHandler is a typed event handler function that accepts context.
type ContextEventHandler[T any] = func(context.Context, T) error

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

// apply implements the Option interface
//
//nolint:unused // This is used via the Option interface
func (f filterOption[T]) apply(s *subscription[T]) {
	s.filter = f.filter
}

// Where creates a filter option for subscriptions.
func Where[T any](filter EventFilter[T]) Option[T] {
	return filterOption[T]{filter: filter}
}

// Publish sends an event using its ref for routing.
func Publish[T Event](bus EventBus, event T) error {
	return bus.Publish(event)
}

// Subscribe provides type-safe subscription with ref validation.
// The TypedRef ensures compile-time type safety and runtime validation.
func Subscribe[T Event](
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

	// Simple wrapper - the bus will handle ref matching via pointer comparison
	wrappedHandler := func(e any) error {
		typed, ok := e.(T)
		if !ok {
			return nil // Wrong type, skip
		}
		return handler(typed)
	}

	// Convert typed filter if present
	var busFilter Filter
	if sub.filter != nil {
		busFilter = func(e Event) bool {
			typed, ok := e.(T)
			if !ok {
				return false
			}
			return sub.filter(typed)
		}
	}

	// Subscribe using the ref pointer
	if busFilter != nil {
		return bus.SubscribeWithFilter(ref.Ref, wrappedHandler, busFilter)
	}
	return bus.Subscribe(ref.Ref, wrappedHandler)
}

// SubscribeWithContext provides type-safe subscription with context support.
// The handler receives a context for cancellation and request-scoped values.
func SubscribeWithContext[T Event](
	bus EventBus,
	ref *core.TypedRef[T],
	handler ContextEventHandler[T],
	opts ...Option[T],
) (string, error) {
	// Apply options
	sub := &subscription[T]{}
	for _, opt := range opts {
		opt.apply(sub)
	}

	// Wrapper that accepts context - the bus will provide context.Background() if needed
	wrappedHandler := func(ctx context.Context, e any) error {
		typed, ok := e.(T)
		if !ok {
			return nil // Wrong type, skip
		}
		return handler(ctx, typed)
	}

	// Convert typed filter if present
	var busFilter Filter
	if sub.filter != nil {
		busFilter = func(e Event) bool {
			typed, ok := e.(T)
			if !ok {
				return false
			}
			return sub.filter(typed)
		}
	}

	// Subscribe using the ref pointer
	if busFilter != nil {
		return bus.SubscribeWithFilter(ref.Ref, wrappedHandler, busFilter)
	}
	return bus.Subscribe(ref.Ref, wrappedHandler)
}
