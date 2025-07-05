// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ConditionBuilder provides a fluent interface for creating conditions.
type ConditionBuilder struct {
	id            string
	conditionType ConditionType
	target        core.Entity
	source        string
	level         int
	saveDC        int
	duration      events.Duration
	metadata      map[string]interface{}
	errors        []error
}

// NewConditionBuilder creates a new builder for the specified condition type.
func NewConditionBuilder(condType ConditionType) *ConditionBuilder {
	return &ConditionBuilder{
		conditionType: condType,
		id:            fmt.Sprintf("%s_%d", condType, generateID()),
		metadata:      make(map[string]interface{}),
	}
}

// WithID sets a custom ID for the condition.
func (b *ConditionBuilder) WithID(id string) *ConditionBuilder {
	if id == "" {
		b.errors = append(b.errors, fmt.Errorf("condition ID cannot be empty"))
	}
	b.id = id
	return b
}

// WithTarget sets the target entity for the condition.
func (b *ConditionBuilder) WithTarget(target core.Entity) *ConditionBuilder {
	if target == nil {
		b.errors = append(b.errors, fmt.Errorf("condition target cannot be nil"))
	}
	b.target = target
	return b
}

// WithSource sets the source of the condition (spell name, ability, etc.).
func (b *ConditionBuilder) WithSource(source string) *ConditionBuilder {
	if source == "" {
		b.errors = append(b.errors, fmt.Errorf("condition source cannot be empty"))
	}
	b.source = source
	return b
}

// WithLevel sets the level for conditions that support it (e.g., exhaustion levels).
func (b *ConditionBuilder) WithLevel(level int) *ConditionBuilder {
	b.level = level
	return b
}

// WithSaveDC sets the save DC for ending the condition.
func (b *ConditionBuilder) WithSaveDC(dc int) *ConditionBuilder {
	if dc < 0 {
		b.errors = append(b.errors, fmt.Errorf("save DC cannot be negative"))
	}
	b.saveDC = dc
	return b
}

// WithDuration sets how long the condition lasts.
func (b *ConditionBuilder) WithDuration(duration events.Duration) *ConditionBuilder {
	b.duration = duration
	return b
}

// WithRoundsDuration sets the duration in rounds.
func (b *ConditionBuilder) WithRoundsDuration(rounds int) *ConditionBuilder {
	b.duration = &events.RoundsDuration{
		Rounds:       rounds,
		StartRound:   0,
		IncludeStart: false,
	}
	return b
}

// WithMinutesDuration sets the duration in minutes.
func (b *ConditionBuilder) WithMinutesDuration(minutes int) *ConditionBuilder {
	b.duration = &events.MinutesDuration{
		Minutes:   minutes,
		StartTime: time.Now(),
	}
	return b
}

// WithConcentration marks this as requiring concentration.
func (b *ConditionBuilder) WithConcentration() *ConditionBuilder {
	b.metadata["concentration"] = true
	return b
}

// WithMetadata adds custom metadata to the condition.
func (b *ConditionBuilder) WithMetadata(key string, value interface{}) *ConditionBuilder {
	b.metadata[key] = value
	return b
}

// WithRelatedEntity sets a related entity for the condition (e.g., charmer, grappler).
func (b *ConditionBuilder) WithRelatedEntity(key string, entity core.Entity) *ConditionBuilder {
	if entity == nil {
		b.errors = append(b.errors, fmt.Errorf("related entity cannot be nil"))
		return b
	}
	b.metadata[key] = entity
	return b
}

// Build creates the condition, returning an error if validation fails.
func (b *ConditionBuilder) Build() (*EnhancedCondition, error) {
	// Check for errors collected during building
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("condition builder errors: %v", b.errors)
	}

	// Validate required fields
	if b.target == nil {
		return nil, fmt.Errorf("condition target is required")
	}
	if b.source == "" {
		return nil, fmt.Errorf("condition source is required")
	}

	// Games can set defaults for their specific condition types as needed

	// Create the enhanced condition
	config := EnhancedConditionConfig{
		ID:            b.id,
		ConditionType: b.conditionType,
		Target:        b.target,
		Source:        b.source,
		Level:         b.level,
		SaveDC:        b.saveDC,
		Duration:      b.duration,
		Metadata:      b.metadata,
	}

	return NewEnhancedCondition(config)
}

// BuildSimple creates a basic condition without mechanical effects.
// Useful for custom conditions or when effects are handled elsewhere.
func (b *ConditionBuilder) BuildSimple() (*SimpleCondition, error) {
	// Check for errors collected during building
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("condition builder errors: %v", b.errors)
	}

	// Validate required fields
	if b.target == nil {
		return nil, fmt.Errorf("condition target is required")
	}
	if b.source == "" {
		return nil, fmt.Errorf("condition source is required")
	}

	config := SimpleConditionConfig{
		ID:     b.id,
		Type:   string(b.conditionType),
		Target: b.target,
		Source: b.source,
	}

	return NewSimpleCondition(config), nil
}

// Games can create their own builder helper functions for common conditions.
// Example:
//
//   func Poisoned() *ConditionBuilder {
//       return NewConditionBuilder(ConditionType("poisoned"))
//   }

// Simple ID generator (in production, use a better ID generation strategy)
var idCounter int64

func generateID() int {
	return int(atomic.AddInt64(&idCounter, 1))
}
