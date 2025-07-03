// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SimpleConditionConfig holds the configuration for creating a SimpleCondition.
type SimpleConditionConfig struct {
	ID     string
	Type   string
	Target core.Entity
	Source string
	
	// Optional handlers - receive the condition itself for self-reference
	ApplyFunc  func(c *SimpleCondition, bus events.EventBus) error
	RemoveFunc func(c *SimpleCondition, bus events.EventBus) error
}

// SimpleCondition provides a basic implementation of the Condition interface.
// It allows custom behavior to be defined through handler functions.
type SimpleCondition struct {
	id     string
	typ    string
	target core.Entity
	source string
	active bool

	// Handler functions
	applyFunc  func(c *SimpleCondition, bus events.EventBus) error
	removeFunc func(c *SimpleCondition, bus events.EventBus) error

	// Track subscriptions for cleanup
	subscriptions []string
}

// NewSimpleCondition creates a new simple condition from a config.
func NewSimpleCondition(cfg SimpleConditionConfig) *SimpleCondition {
	return &SimpleCondition{
		id:            cfg.ID,
		typ:           cfg.Type,
		source:        cfg.Source,
		target:        cfg.Target,
		active:        false,
		applyFunc:     cfg.ApplyFunc,
		removeFunc:    cfg.RemoveFunc,
		subscriptions: []string{},
	}
}

// GetID implements core.Entity
func (c *SimpleCondition) GetID() string { return c.id }

// GetType implements core.Entity  
func (c *SimpleCondition) GetType() string { return c.typ }

// Target returns the affected entity
func (c *SimpleCondition) Target() core.Entity { return c.target }

// Source returns what created this condition
func (c *SimpleCondition) Source() string { return c.source }

// IsActive returns whether the condition is active
func (c *SimpleCondition) IsActive() bool { return c.active }


// Apply activates the condition
func (c *SimpleCondition) Apply(bus events.EventBus) error {
	if c.active {
		return nil // Already active
	}

	if c.applyFunc != nil {
		if err := c.applyFunc(c, bus); err != nil {
			return err
		}
	}

	c.active = true
	return nil
}

// Subscribe is a helper that subscribes to an event and tracks the subscription
func (c *SimpleCondition) Subscribe(bus events.EventBus, eventType string, priority int, handler events.HandlerFunc) {
	subID := bus.SubscribeFunc(eventType, priority, handler)
	c.AddSubscription(subID)
}

// Remove deactivates the condition
func (c *SimpleCondition) Remove(bus events.EventBus) error {
	if !c.active {
		return nil // Already inactive
	}

	// Unsubscribe all handlers
	for _, subID := range c.subscriptions {
		bus.Unsubscribe(subID)
	}
	c.subscriptions = []string{}

	if c.removeFunc != nil {
		if err := c.removeFunc(c, bus); err != nil {
			return err
		}
	}

	c.active = false
	return nil
}

// AddSubscription tracks an event subscription for cleanup
func (c *SimpleCondition) AddSubscription(id string) {
	c.subscriptions = append(c.subscriptions, id)
}