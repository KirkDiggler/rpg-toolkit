// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"
	"fmt"
	"sync"
)

// EventBus provides a simple pub/sub mechanism for typed topics
type EventBus interface {
	// Subscribe registers a handler for a specific topic
	Subscribe(ctx context.Context, topic Topic, handler any) (string, error)

	// Unsubscribe removes a subscription by ID
	Unsubscribe(ctx context.Context, id string) error

	// Publish sends an event to all subscribers of its topic
	Publish(ctx context.Context, topic Topic, event any) error
}

// NewEventBus creates a new event bus instance
func NewEventBus() EventBus {
	return &simpleEventBus{
		subscribers: make(map[Topic][]subscription),
		idToTopic:   make(map[string]Topic),
	}
}

type subscription struct {
	id      string
	handler any
}

type simpleEventBus struct {
	mu          sync.RWMutex
	subscribers map[Topic][]subscription
	idToTopic   map[string]Topic
	nextID      int
}

func (b *simpleEventBus) Subscribe(ctx context.Context, topic Topic, handler any) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := fmt.Sprintf("%s-%d", topic, b.nextID)

	sub := subscription{
		id:      id,
		handler: handler,
	}

	b.subscribers[topic] = append(b.subscribers[topic], sub)
	b.idToTopic[id] = topic

	return id, nil
}

func (b *simpleEventBus) Unsubscribe(ctx context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic, exists := b.idToTopic[id]
	if !exists {
		return nil // Already unsubscribed
	}

	subs := b.subscribers[topic]
	for i, sub := range subs {
		if sub.id == id {
			b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			delete(b.idToTopic, id)
			break
		}
	}

	return nil
}

func (b *simpleEventBus) Publish(ctx context.Context, topic Topic, event any) error {
	b.mu.RLock()
	subs := b.subscribers[topic]
	handlers := make([]any, len(subs))
	for i, sub := range subs {
		handlers[i] = sub.handler
	}
	b.mu.RUnlock()

	// Call handlers outside lock to avoid deadlock
	// The handlers are wrapped functions that know how to handle the event
	for _, handler := range handlers {
		if fn, ok := handler.(func(any) error); ok {
			if err := fn(event); err != nil {
				return err
			}
		}
	}

	return nil
}
