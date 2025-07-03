// Package events provides an event bus for handling game events.
package events

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Handler processes events.
type Handler interface {
	// Handle processes the event.
	Handle(ctx context.Context, event Event) error

	// Priority determines handler execution order (higher = later).
	Priority() int
}

// EventBus manages event publishing and subscriptions.
//
//go:generate mockgen -destination=mock/mock_eventbus.go -package=mock github.com/KirkDiggler/rpg-toolkit/events EventBus
type EventBus interface {
	// Publish sends an event to all subscribers.
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for specific event types.
	Subscribe(eventType string, handler Handler) (subscriptionID string)

	// SubscribeFunc is a convenience method for function handlers.
	SubscribeFunc(eventType string, priority int, fn HandlerFunc) (subscriptionID string)

	// Unsubscribe removes a subscription.
	Unsubscribe(subscriptionID string) error

	// Clear removes all subscriptions for an event type.
	Clear(eventType string)

	// ClearAll removes all subscriptions.
	ClearAll()
}

// subscription holds handler information.
type subscription struct {
	id        string
	handler   Handler
	eventType string
}

// funcHandler wraps a HandlerFunc to implement Handler.
type funcHandler struct {
	fn       HandlerFunc
	priority int
}

func (h *funcHandler) Handle(ctx context.Context, event Event) error {
	return h.fn(ctx, event)
}

func (h *funcHandler) Priority() int {
	return h.priority
}

// Bus is the default EventBus implementation.
type Bus struct {
	mu            sync.RWMutex
	subscriptions map[string][]*subscription
	idCounter     int
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subscriptions: make(map[string][]*subscription),
	}
}

// Publish sends an event to all subscribers.
func (b *Bus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	subs := b.subscriptions[event.Type()]
	// Create a copy to avoid holding the lock during handler execution
	handlers := make([]*subscription, len(subs))
	copy(handlers, subs)
	b.mu.RUnlock()

	// Sort handlers by priority
	// TODO: Consider maintaining sorted order on subscribe to avoid sorting on every publish
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].handler.Priority() < handlers[j].handler.Priority()
	})

	// Execute handlers
	for _, sub := range handlers {
		if err := sub.handler.Handle(ctx, event); err != nil {
			return fmt.Errorf("handler %s failed: %w", sub.id, err)
		}
	}

	return nil
}

// Subscribe registers a handler for specific event types.
func (b *Bus) Subscribe(eventType string, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.idCounter++
	id := fmt.Sprintf("%s-%d", eventType, b.idCounter)

	sub := &subscription{
		id:        id,
		handler:   handler,
		eventType: eventType,
	}

	b.subscriptions[eventType] = append(b.subscriptions[eventType], sub)
	return id
}

// SubscribeFunc is a convenience method for function handlers.
func (b *Bus) SubscribeFunc(eventType string, priority int, fn HandlerFunc) string {
	return b.Subscribe(eventType, &funcHandler{fn: fn, priority: priority})
}

// Unsubscribe removes a subscription.
func (b *Bus) Unsubscribe(subscriptionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Find and remove the subscription
	for eventType, subs := range b.subscriptions {
		for i, sub := range subs {
			if sub.id == subscriptionID {
				// Remove the subscription
				b.subscriptions[eventType] = append(subs[:i], subs[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("subscription %s not found", subscriptionID)
}

// Clear removes all subscriptions for an event type.
func (b *Bus) Clear(eventType string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscriptions, eventType)
}

// ClearAll removes all subscriptions.
func (b *Bus) ClearAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscriptions = make(map[string][]*subscription)
}
