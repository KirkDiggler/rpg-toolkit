// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
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
	// Embedded Core for common functionality
	core *effects.Core

	target core.Entity

	// Handler functions
	applyFunc  func(c *SimpleCondition, bus events.EventBus) error
	removeFunc func(c *SimpleCondition, bus events.EventBus) error
}

// NewSimpleCondition creates a new simple condition from a config.
func NewSimpleCondition(cfg SimpleConditionConfig) *SimpleCondition {
	c := &SimpleCondition{
		target:     cfg.Target,
		applyFunc:  cfg.ApplyFunc,
		removeFunc: cfg.RemoveFunc,
	}

	// Create Core with adapted handlers
	var coreApplyFunc func(bus events.EventBus) error
	if cfg.ApplyFunc != nil {
		coreApplyFunc = func(bus events.EventBus) error {
			return c.applyFunc(c, bus)
		}
	}

	var coreRemoveFunc func(bus events.EventBus) error
	if cfg.RemoveFunc != nil {
		coreRemoveFunc = func(bus events.EventBus) error {
			return c.removeFunc(c, bus)
		}
	}

	c.core = effects.NewCore(effects.CoreConfig{
		ID:         cfg.ID,
		Type:       cfg.Type,
		Source:     cfg.Source,
		ApplyFunc:  coreApplyFunc,
		RemoveFunc: coreRemoveFunc,
	})

	return c
}

// GetID implements core.Entity
func (c *SimpleCondition) GetID() string { return c.core.GetID() }

// GetType implements core.Entity
func (c *SimpleCondition) GetType() string { return c.core.GetType() }

// Target returns the affected entity
func (c *SimpleCondition) Target() core.Entity { return c.target }

// Source returns what created this condition
func (c *SimpleCondition) Source() string { return c.core.Source() }

// IsActive returns whether the condition is active
func (c *SimpleCondition) IsActive() bool { return c.core.IsActive() }

// Apply activates the condition
func (c *SimpleCondition) Apply(bus events.EventBus) error {
	// Delegate to Core which handles activation state and calls our wrapped handler
	return c.core.Apply(bus)
}

// Subscribe is a helper that subscribes to an event and tracks the subscription
func (c *SimpleCondition) Subscribe(bus events.EventBus, eventType string, priority int, handler events.HandlerFunc) {
	// Delegate to Core which handles subscription tracking
	c.core.Subscribe(bus, eventType, priority, handler)
}

// Remove deactivates the condition
func (c *SimpleCondition) Remove(bus events.EventBus) error {
	// Delegate to Core which handles deactivation and cleanup
	return c.core.Remove(bus)
}

// AddSubscription tracks an event subscription for cleanup
// Deprecated: This is now handled internally by Core
func (c *SimpleCondition) AddSubscription(_ string) {
	// No-op for backward compatibility
	// Core handles subscription tracking internally
}
