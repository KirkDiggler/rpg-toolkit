// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

// DeferredAction represents operations to perform after all handlers have completed.
// This prevents deadlocks when handlers need to interact with the bus.
type DeferredAction struct {
	// Subscriptions to remove after handlers complete
	Unsubscribes []string
	
	// Events to publish after handlers complete
	Publishes []Event
	
	// Error to return (if any)
	Error error
}

// NewDeferredAction creates a new deferred action.
func NewDeferredAction() *DeferredAction {
	return &DeferredAction{}
}

// Unsubscribe adds a subscription ID to unsubscribe after handlers complete.
func (d *DeferredAction) Unsubscribe(ids ...string) *DeferredAction {
	d.Unsubscribes = append(d.Unsubscribes, ids...)
	return d
}

// Publish adds an event to publish after handlers complete.
func (d *DeferredAction) Publish(events ...Event) *DeferredAction {
	d.Publishes = append(d.Publishes, events...)
	return d
}

// WithError sets an error to return.
func (d *DeferredAction) WithError(err error) *DeferredAction {
	d.Error = err
	return d
}

// HandlerFunc is the original handler signature for backwards compatibility.
type HandlerFunc func(event any) error

// DeferredHandlerFunc is the new handler signature that can return deferred actions.
type DeferredHandlerFunc func(event any) *DeferredAction