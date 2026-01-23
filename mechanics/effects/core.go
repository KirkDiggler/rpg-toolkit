// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package effects provides shared infrastructure for game mechanics that
// modify behavior through the event system. This includes conditions,
// proficiencies, features, equipment effects, and other game modifiers.
package effects

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Compile-time check that Core implements core.Entity
var _ core.Entity = (*Core)(nil)

// Core provides base functionality for effects including subscription
// management, lifecycle methods, and state tracking. Domain types should
// embed this type to gain standard effect behavior.
//
// Core implements core.Entity interface.
type Core struct {
	id     string
	typ    string
	source *core.Source
	active bool

	// Subscription tracking for cleanup
	tracker *SubscriptionTracker

	// Optional lifecycle handlers
	applyFunc  func(bus events.EventBus) error
	removeFunc func(bus events.EventBus) error
}

// CoreConfig holds configuration for creating a Core effect.
type CoreConfig struct {
	ID     string
	Type   string
	Source *core.Source

	// Optional lifecycle handlers
	ApplyFunc  func(bus events.EventBus) error
	RemoveFunc func(bus events.EventBus) error
}

// NewCore creates a new effect core from configuration.
func NewCore(cfg CoreConfig) *Core {
	return &Core{
		id:         cfg.ID,
		typ:        cfg.Type,
		source:     cfg.Source,
		active:     false,
		tracker:    NewSubscriptionTracker(),
		applyFunc:  cfg.ApplyFunc,
		removeFunc: cfg.RemoveFunc,
	}
}

// GetID implements core.Entity
func (c *Core) GetID() string { return c.id }

// GetType implements core.Entity
func (c *Core) GetType() core.EntityType { return core.EntityType(c.typ) }

// Source returns what created or granted this effect.
func (c *Core) Source() *core.Source { return c.source }

// IsActive returns whether the effect is currently active.
func (c *Core) IsActive() bool { return c.active }

// Apply activates the effect and runs any custom apply logic.
func (c *Core) Apply(bus events.EventBus) error {
	if c.active {
		return nil // Already active
	}

	if c.applyFunc != nil {
		if err := c.applyFunc(bus); err != nil {
			return err
		}
	}

	c.active = true
	return nil
}

// Remove deactivates the effect, unsubscribes all handlers, and runs any custom remove logic.
func (c *Core) Remove(bus events.EventBus) error {
	if !c.active {
		return nil // Already inactive
	}

	// Unsubscribe all tracked event handlers
	if err := c.tracker.UnsubscribeAll(bus); err != nil {
		return err
	}

	if c.removeFunc != nil {
		if err := c.removeFunc(bus); err != nil {
			return err
		}
	}

	c.active = false
	return nil
}

// Subscribe is a convenience method that subscribes to an event and tracks the subscription.
func (c *Core) Subscribe(bus events.EventBus, eventType string, priority int, handler events.HandlerFunc) {
	c.tracker.Subscribe(bus, eventType, priority, handler)
}

// SubscriptionCount returns the number of active subscriptions.
func (c *Core) SubscriptionCount() int {
	return c.tracker.Count()
}
