// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Condition represents a temporary or permanent status effect on an entity.
type Condition interface {
	// ID returns the unique identifier for this condition instance.
	ID() string

	// Type returns the condition type (e.g., "poisoned", "blessed").
	Type() string

	// Source returns what caused this condition (e.g., "poison_dart", "bless_spell").
	Source() string

	// SourceEntity returns the entity that applied this condition, if any.
	SourceEntity() core.Entity

	// Duration returns how long this condition lasts.
	Duration() Duration

	// AppliedAt returns when the condition was applied.
	AppliedAt() time.Time

	// IsExpired checks if the condition should be removed based on the given event.
	IsExpired(event events.Event) bool

	// OnApply is called when the condition is first applied.
	OnApply(bus events.EventBus, target core.Entity) error

	// OnRemove is called when the condition is removed.
	OnRemove(bus events.EventBus, target core.Entity) error

	// ModifyEvent allows the condition to modify events (add modifiers, change values).
	ModifyEvent(event events.Event)
}

// Duration defines how long a condition lasts.
type Duration interface {
	// IsExpired returns true if the duration has expired based on the event.
	IsExpired(event events.Event) bool

	// Description returns a human-readable description of the duration.
	Description() string
}

// BaseCondition provides a standard implementation of Condition.
type BaseCondition struct {
	id            string
	conditionType string
	source        string
	sourceEntity  core.Entity
	duration      Duration
	appliedAt     time.Time
}

// NewCondition creates a new condition with the given parameters.
func NewCondition(id, conditionType, source string, sourceEntity core.Entity, duration Duration) *BaseCondition {
	return &BaseCondition{
		id:            id,
		conditionType: conditionType,
		source:        source,
		sourceEntity:  sourceEntity,
		duration:      duration,
		appliedAt:     time.Now(),
	}
}

func (c *BaseCondition) ID() string                { return c.id }
func (c *BaseCondition) Type() string              { return c.conditionType }
func (c *BaseCondition) Source() string            { return c.source }
func (c *BaseCondition) SourceEntity() core.Entity { return c.sourceEntity }
func (c *BaseCondition) Duration() Duration        { return c.duration }
func (c *BaseCondition) AppliedAt() time.Time      { return c.appliedAt }

func (c *BaseCondition) IsExpired(event events.Event) bool {
	return c.duration.IsExpired(event)
}

func (c *BaseCondition) OnApply(bus events.EventBus, target core.Entity) error {
	// Base implementation does nothing
	return nil
}

func (c *BaseCondition) OnRemove(bus events.EventBus, target core.Entity) error {
	// Base implementation does nothing
	return nil
}

func (c *BaseCondition) ModifyEvent(event events.Event) {
	// Base implementation does nothing
}
