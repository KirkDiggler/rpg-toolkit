// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import "github.com/KirkDiggler/rpg-toolkit/events"

// SubscriptionTracker manages event subscriptions for automatic cleanup.
// It tracks subscription IDs and provides bulk unsubscribe functionality.
type SubscriptionTracker struct {
	subscriptions []string
}

// NewSubscriptionTracker creates a new subscription tracker.
func NewSubscriptionTracker() *SubscriptionTracker {
	return &SubscriptionTracker{
		subscriptions: make([]string, 0),
	}
}

// Track adds a subscription ID to be tracked for cleanup.
func (st *SubscriptionTracker) Track(subscriptionID string) {
	st.subscriptions = append(st.subscriptions, subscriptionID)
}

// Subscribe is a convenience method that subscribes to an event and tracks the subscription.
func (st *SubscriptionTracker) Subscribe(
	bus events.EventBus,
	eventType string,
	priority int,
	handler events.HandlerFunc,
) {
	subID := bus.SubscribeFunc(eventType, priority, handler)
	st.Track(subID)
}

// UnsubscribeAll removes all tracked subscriptions from the event bus.
func (st *SubscriptionTracker) UnsubscribeAll(bus events.EventBus) error {
	for _, subID := range st.subscriptions {
		if err := bus.Unsubscribe(subID); err != nil {
			return err
		}
	}
	st.subscriptions = st.subscriptions[:0] // Clear but keep capacity
	return nil
}

// Count returns the number of tracked subscriptions.
func (st *SubscriptionTracker) Count() int {
	return len(st.subscriptions)
}

// Clear removes all tracked subscription IDs without unsubscribing.
// Use this only if subscriptions have been cleaned up elsewhere.
func (st *SubscriptionTracker) Clear() {
	st.subscriptions = st.subscriptions[:0]
}
