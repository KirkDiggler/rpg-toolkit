// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
)

// TypedTopic provides type-safe publish/subscribe for events of type T.
// It wraps the event bus to ensure compile-time type safety.
// Use this for pure notifications. For events needing chain processing, use ChainedTopic.
type TypedTopic[T any] interface {
	// Subscribe registers a handler for events of type T.
	// This is for pure notifications - the handler processes but doesn't transform the event.
	// Returns a subscription ID that can be used to unsubscribe.
	Subscribe(ctx context.Context, handler func(context.Context, T) error) (string, error)

	// Unsubscribe removes a handler using its subscription ID.
	// Returns an error if the ID is not found.
	Unsubscribe(ctx context.Context, id string) error

	// Publish sends an event to all subscribers.
	// Note: Current bus doesn't support returning modified events,
	// so modifications happen through the event's chain if present.
	Publish(ctx context.Context, event T) error
}

// typedTopic is the implementation of TypedTopic[T]
type typedTopic[T any] struct {
	bus   EventBus
	topic Topic
}

// Subscribe implements TypedTopic[T]
func (t *typedTopic[T]) Subscribe(ctx context.Context, handler func(context.Context, T) error) (string, error) {
	// Wrap handler to match bus signature
	wrappedHandler := func(event any) error {
		typedEvent, ok := event.(T)
		if !ok {
			return nil // Ignore events of wrong type
		}
		return handler(ctx, typedEvent)
	}

	return t.bus.Subscribe(t.topic, wrappedHandler)
}

// Unsubscribe implements TypedTopic[T]
func (t *typedTopic[T]) Unsubscribe(ctx context.Context, id string) error {
	return t.bus.Unsubscribe(id)
}

// Publish implements TypedTopic[T]
func (t *typedTopic[T]) Publish(ctx context.Context, event T) error {
	return t.bus.Publish(t.topic, event)
}
