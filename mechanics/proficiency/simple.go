// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// SimpleProficiencyConfig holds the configuration for creating a SimpleProficiency.
type SimpleProficiencyConfig struct {
	ID      string
	Type    string // e.g., "proficiency.weapon", "proficiency.skill"
	Owner   core.Entity
	Subject *core.Ref    // What they're proficient with
	Source  *core.Source // What granted this proficiency

	// Optional handlers - receive the proficiency itself for self-reference
	ApplyFunc  func(p *SimpleProficiency, bus events.EventBus) error
	RemoveFunc func(p *SimpleProficiency, bus events.EventBus) error
}

// SimpleProficiency provides a basic implementation of the Proficiency interface.
// It allows custom behavior to be defined through handler functions.
type SimpleProficiency struct {
	// Embedded Core for common functionality
	core *effects.Core

	owner   core.Entity
	subject *core.Ref
	source  *core.Source

	// Handler functions
	applyFunc  func(p *SimpleProficiency, bus events.EventBus) error
	removeFunc func(p *SimpleProficiency, bus events.EventBus) error
}

// NewSimpleProficiency creates a new simple proficiency from a config.
func NewSimpleProficiency(cfg SimpleProficiencyConfig) *SimpleProficiency {
	p := &SimpleProficiency{
		owner:      cfg.Owner,
		subject:    cfg.Subject,
		source:     cfg.Source,
		applyFunc:  cfg.ApplyFunc,
		removeFunc: cfg.RemoveFunc,
	}

	// Create Core with adapted handlers
	var coreApplyFunc func(bus events.EventBus) error
	if cfg.ApplyFunc != nil {
		coreApplyFunc = func(bus events.EventBus) error {
			return p.applyFunc(p, bus)
		}
	}

	var coreRemoveFunc func(bus events.EventBus) error
	if cfg.RemoveFunc != nil {
		coreRemoveFunc = func(bus events.EventBus) error {
			return p.removeFunc(p, bus)
		}
	}

	// Convert core.Source to string for effects.Core compatibility
	var sourceString string
	if cfg.Source != nil {
		sourceString = cfg.Source.String()
	}

	p.core = effects.NewCore(effects.CoreConfig{
		ID:         cfg.ID,
		Type:       cfg.Type,
		Source:     sourceString,
		ApplyFunc:  coreApplyFunc,
		RemoveFunc: coreRemoveFunc,
	})

	return p
}

// GetID implements core.Entity
func (p *SimpleProficiency) GetID() string { return p.core.GetID() }

// GetType implements core.Entity
func (p *SimpleProficiency) GetType() string { return p.core.GetType() }

// Owner returns the entity that has this proficiency
func (p *SimpleProficiency) Owner() core.Entity { return p.owner }

// Subject returns what the entity is proficient with
func (p *SimpleProficiency) Subject() *core.Ref { return p.subject }

// Source returns what granted this proficiency
func (p *SimpleProficiency) Source() *core.Source {
	return p.source
}

// IsActive returns whether the proficiency is active
func (p *SimpleProficiency) IsActive() bool { return p.core.IsActive() }

// Apply activates the proficiency
func (p *SimpleProficiency) Apply(bus events.EventBus) error {
	// Delegate to Core which handles activation state and calls our wrapped handler
	return p.core.Apply(bus)
}

// Subscribe is a helper that subscribes to an event and tracks the subscription
func (p *SimpleProficiency) Subscribe(bus events.EventBus, eventType string, priority int, handler events.HandlerFunc) {
	// Delegate to Core which handles subscription tracking
	p.core.Subscribe(bus, eventType, priority, handler)
}

// Remove deactivates the proficiency
func (p *SimpleProficiency) Remove(bus events.EventBus) error {
	// Delegate to Core which handles deactivation and cleanup
	return p.core.Remove(bus)
}

// AddSubscription tracks an event subscription for cleanup
// Deprecated: This is now handled internally by Core
func (p *SimpleProficiency) AddSubscription(_ string) {
	// No-op for backward compatibility
	// Core handles subscription tracking internally
}
