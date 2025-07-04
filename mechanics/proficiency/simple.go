// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SimpleProficiencyConfig holds the configuration for creating a SimpleProficiency.
type SimpleProficiencyConfig struct {
	ID      string
	Type    string // e.g., "proficiency.weapon", "proficiency.skill"
	Owner   core.Entity
	Subject string // What they're proficient with
	Source  string // What granted this proficiency

	// Optional handlers - receive the proficiency itself for self-reference
	ApplyFunc  func(p *SimpleProficiency, bus events.EventBus) error
	RemoveFunc func(p *SimpleProficiency, bus events.EventBus) error
}

// SimpleProficiency provides a basic implementation of the Proficiency interface.
// It allows custom behavior to be defined through handler functions.
type SimpleProficiency struct {
	id      string
	typ     string
	owner   core.Entity
	subject string
	source  string
	active  bool

	// Handler functions
	applyFunc  func(p *SimpleProficiency, bus events.EventBus) error
	removeFunc func(p *SimpleProficiency, bus events.EventBus) error

	// Track subscriptions for cleanup
	subscriptions []string
}

// NewSimpleProficiency creates a new simple proficiency from a config.
func NewSimpleProficiency(cfg SimpleProficiencyConfig) *SimpleProficiency {
	return &SimpleProficiency{
		id:            cfg.ID,
		typ:           cfg.Type,
		owner:         cfg.Owner,
		subject:       cfg.Subject,
		source:        cfg.Source,
		active:        false,
		applyFunc:     cfg.ApplyFunc,
		removeFunc:    cfg.RemoveFunc,
		subscriptions: []string{},
	}
}

// GetID implements core.Entity
func (p *SimpleProficiency) GetID() string { return p.id }

// GetType implements core.Entity
func (p *SimpleProficiency) GetType() string { return p.typ }

// Owner returns the entity that has this proficiency
func (p *SimpleProficiency) Owner() core.Entity { return p.owner }

// Subject returns what the entity is proficient with
func (p *SimpleProficiency) Subject() string { return p.subject }

// Source returns what granted this proficiency
func (p *SimpleProficiency) Source() string { return p.source }

// IsActive returns whether the proficiency is active
func (p *SimpleProficiency) IsActive() bool { return p.active }

// Apply activates the proficiency
func (p *SimpleProficiency) Apply(bus events.EventBus) error {
	if p.active {
		return nil // Already active
	}

	if p.applyFunc != nil {
		if err := p.applyFunc(p, bus); err != nil {
			return err
		}
	}

	p.active = true
	return nil
}

// Subscribe is a helper that subscribes to an event and tracks the subscription
func (p *SimpleProficiency) Subscribe(bus events.EventBus, eventType string, priority int, handler events.HandlerFunc) {
	subID := bus.SubscribeFunc(eventType, priority, handler)
	p.AddSubscription(subID)
}

// Remove deactivates the proficiency
func (p *SimpleProficiency) Remove(bus events.EventBus) error {
	if !p.active {
		return nil // Already inactive
	}

	// Unsubscribe all handlers
	for _, subID := range p.subscriptions {
		if err := bus.Unsubscribe(subID); err != nil {
			return err
		}
	}
	p.subscriptions = []string{}

	if p.removeFunc != nil {
		if err := p.removeFunc(p, bus); err != nil {
			return err
		}
	}

	p.active = false
	return nil
}

// AddSubscription tracks an event subscription for cleanup
func (p *SimpleProficiency) AddSubscription(id string) {
	p.subscriptions = append(p.subscriptions, id)
}
