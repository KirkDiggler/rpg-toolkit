// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
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

// WithLevel sets the level for exhaustion conditions.
func (b *ConditionBuilder) WithLevel(level int) *ConditionBuilder {
	if b.conditionType != ConditionExhaustion {
		b.errors = append(b.errors, fmt.Errorf("level can only be set for exhaustion conditions"))
		return b
	}
	if level < 1 || level > 6 {
		b.errors = append(b.errors, fmt.Errorf("exhaustion level must be between 1 and 6"))
	}
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

// WithCharmer sets the charmer for charmed conditions.
func (b *ConditionBuilder) WithCharmer(charmer core.Entity) *ConditionBuilder {
	if b.conditionType != ConditionCharmed {
		b.errors = append(b.errors, fmt.Errorf("charmer can only be set for charmed conditions"))
		return b
	}
	if charmer == nil {
		b.errors = append(b.errors, fmt.Errorf("charmer cannot be nil"))
		return b
	}
	b.metadata["charmer"] = charmer
	return b
}

// WithFearSource sets the source of fear for frightened conditions.
func (b *ConditionBuilder) WithFearSource(source core.Entity) *ConditionBuilder {
	if b.conditionType != ConditionFrightened {
		b.errors = append(b.errors, fmt.Errorf("fear source can only be set for frightened conditions"))
		return b
	}
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("fear source cannot be nil"))
		return b
	}
	b.metadata["fear_source"] = source
	return b
}

// WithGrappler sets the grappler for grappled conditions.
func (b *ConditionBuilder) WithGrappler(grappler core.Entity) *ConditionBuilder {
	if b.conditionType != ConditionGrappled {
		b.errors = append(b.errors, fmt.Errorf("grappler can only be set for grappled conditions"))
		return b
	}
	if grappler == nil {
		b.errors = append(b.errors, fmt.Errorf("grappler cannot be nil"))
		return b
	}
	b.metadata["grappler"] = grappler
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

	// Set defaults for exhaustion
	if b.conditionType == ConditionExhaustion && b.level == 0 {
		b.level = 1
	}

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

// Common condition creation helpers

// Blinded creates a builder for the blinded condition.
func Blinded() *ConditionBuilder {
	return NewConditionBuilder(ConditionBlinded)
}

// Charmed creates a builder for the charmed condition.
func Charmed() *ConditionBuilder {
	return NewConditionBuilder(ConditionCharmed)
}

// Deafened creates a builder for the deafened condition.
func Deafened() *ConditionBuilder {
	return NewConditionBuilder(ConditionDeafened)
}

// Exhaustion creates a builder for the exhaustion condition.
func Exhaustion(level int) *ConditionBuilder {
	return NewConditionBuilder(ConditionExhaustion).WithLevel(level)
}

// Frightened creates a builder for the frightened condition.
func Frightened() *ConditionBuilder {
	return NewConditionBuilder(ConditionFrightened)
}

// Grappled creates a builder for the grappled condition.
func Grappled() *ConditionBuilder {
	return NewConditionBuilder(ConditionGrappled)
}

// Incapacitated creates a builder for the incapacitated condition.
func Incapacitated() *ConditionBuilder {
	return NewConditionBuilder(ConditionIncapacitated)
}

// Invisible creates a builder for the invisible condition.
func Invisible() *ConditionBuilder {
	return NewConditionBuilder(ConditionInvisible)
}

// Paralyzed creates a builder for the paralyzed condition.
func Paralyzed() *ConditionBuilder {
	return NewConditionBuilder(ConditionParalyzed)
}

// Petrified creates a builder for the petrified condition.
func Petrified() *ConditionBuilder {
	return NewConditionBuilder(ConditionPetrified)
}

// Poisoned creates a builder for the poisoned condition.
func Poisoned() *ConditionBuilder {
	return NewConditionBuilder(ConditionPoisoned)
}

// Prone creates a builder for the prone condition.
func Prone() *ConditionBuilder {
	return NewConditionBuilder(ConditionProne)
}

// Restrained creates a builder for the restrained condition.
func Restrained() *ConditionBuilder {
	return NewConditionBuilder(ConditionRestrained)
}

// Stunned creates a builder for the stunned condition.
func Stunned() *ConditionBuilder {
	return NewConditionBuilder(ConditionStunned)
}

// Unconscious creates a builder for the unconscious condition.
func Unconscious() *ConditionBuilder {
	return NewConditionBuilder(ConditionUnconscious)
}

// Simple ID generator (in production, use a better ID generation strategy)
var idCounter int

func generateID() int {
	idCounter++
	return idCounter
}
