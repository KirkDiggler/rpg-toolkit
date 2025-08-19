// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// TypedTopic provides type-safe publish/subscribe for events of type T.
// It wraps the event bus to ensure compile-time type safety.
// Note: T must implement the Event interface.
type TypedTopic[T Event] interface {
	// Subscribe registers a handler for events of type T.
	// The handler can modify the event for chain-style processing.
	// Returns a subscription ID that can be used to unsubscribe.
	Subscribe(handler func(context.Context, T) (T, error)) (string, error)

	// Unsubscribe removes a handler using its subscription ID.
	// Returns an error if the ID is not found.
	Unsubscribe(id string) error

	// Publish sends an event to all subscribers.
	// Note: Current bus doesn't support returning modified events,
	// so modifications happen through the event's chain if present.
	Publish(ctx context.Context, event T) error
}

// GetTopic returns a typed topic for the specified event type.
// This provides type-safe access to the event bus for a specific topic.
func GetTopic[T Event](bus EventBus, topic Topic) TypedTopic[T] {
	return &typedTopic[T]{
		bus:   bus,
		topic: string(topic),
	}
}

// typedTopic is the implementation of TypedTopic[T]
type typedTopic[T Event] struct {
	bus   EventBus
	topic string
}

// Subscribe implements TypedTopic[T]
func (t *typedTopic[T]) Subscribe(handler func(context.Context, T) (T, error)) (string, error) {
	ctx := context.Background()
	
	// Create ref for this topic
	ref, err := core.NewRef(core.RefInput{
		Module: "topic",
		Type:   "event",
		Value:  t.topic,
	})
	if err != nil {
		return "", err
	}
	
	// Create TypedRef for type-safe subscription
	typedRef := &core.TypedRef[T]{Ref: ref}
	
	// Wrap handler to match EventHandler[T] signature
	wrappedHandler := func(ctx context.Context, event T) error {
		// Call the handler that can modify the event
		modified, err := handler(ctx, event)
		if err != nil {
			return err
		}
		
		// If event has a chain, modifications happen there
		// Otherwise, we can't modify in-place with current bus
		_ = modified
		return nil
	}
	
	// Use the existing typed Subscribe function
	return Subscribe(ctx, t.bus, typedRef, wrappedHandler)
}

// Unsubscribe implements TypedTopic[T]
func (t *typedTopic[T]) Unsubscribe(id string) error {
	ctx := context.Background()
	return Unsubscribe(ctx, t.bus, id)
}

// Publish implements TypedTopic[T]
func (t *typedTopic[T]) Publish(ctx context.Context, event T) error {
	return Publish(ctx, t.bus, event)
}