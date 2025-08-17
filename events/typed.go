// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// EventHandler is a typed event handler function that accepts context.
type EventHandler[T any] = func(context.Context, T) error

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

// Publish sends an event using its ref for routing with context.
func Publish[T Event](ctx context.Context, bus EventBus, event T) error {
	return bus.Publish(ctx, event)
}

// Subscribe provides type-safe subscription with ref validation.
// The TypedRef ensures compile-time type safety and runtime validation.
func Subscribe[T Event](
	ctx context.Context,
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

	// Wrapper that provides context to the handler
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

	// Subscribe using the ref from TypedRef
	// The ref value will be used for matching (not pointer)
	if busFilter != nil {
		return bus.SubscribeWithFilter(ctx, ref.Ref, wrappedHandler, busFilter)
	}
	return bus.Subscribe(ctx, ref.Ref, wrappedHandler)
}

// Unsubscribe removes a subscription by ID.
func Unsubscribe(ctx context.Context, bus EventBus, id string) error {
	return bus.Unsubscribe(ctx, id)
}

// PublishWithTypedRef sends an event and verifies it matches the TypedRef.
// This provides an extra type-safety check at publish time.
func PublishWithTypedRef[T Event](
	ctx context.Context,
	bus EventBus,
	ref *core.TypedRef[T],
	event T,
) error {
	// Verify the event's ref matches the TypedRef
	if event.EventRef().String() != ref.Ref.String() {
		return fmt.Errorf("event ref mismatch: expected %s, got %s",
			ref.Ref, event.EventRef())
	}
	return bus.Publish(ctx, event)
}
