// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// SimpleCondition provides common functionality for condition implementations.
// It embeds effects.Core for event subscription management and state tracking.
type SimpleCondition struct {
	Core        *effects.Core
	ref         *core.Ref
	name        string
	description string
	target      core.Entity
	source      string
	metadata    map[string]interface{}
	isDirty     bool
}

// IsActive returns whether the condition is currently active
func (c *SimpleCondition) IsActive() bool {
	return c.Core.IsActive()
}

// NewSimpleCondition creates a new simple condition with the given reference
func NewSimpleCondition(ref *core.Ref) (*SimpleCondition, error) {
	if ref == nil {
		return nil, ErrInvalidRef
	}

	// Create the core with a simple ID/Type based on the ref
	effectCore := effects.NewCore(effects.CoreConfig{
		ID:   ref.String(),
		Type: "condition",
	})

	return &SimpleCondition{
		Core:     effectCore,
		ref:      ref,
		metadata: make(map[string]interface{}),
	}, nil
}

// Ref returns the condition's reference
func (c *SimpleCondition) Ref() *core.Ref {
	return c.ref
}

// Name returns the display name of the condition
func (c *SimpleCondition) Name() string {
	if c.name == "" {
		return c.ref.String()
	}
	return c.name
}

// SetName sets the display name of the condition
func (c *SimpleCondition) SetName(name string) {
	c.name = name
	c.isDirty = true
}

// Description returns what this condition does
func (c *SimpleCondition) Description() string {
	return c.description
}

// SetDescription sets the description of the condition
func (c *SimpleCondition) SetDescription(desc string) {
	c.description = desc
	c.isDirty = true
}

// Target returns the entity this condition affects
func (c *SimpleCondition) Target() core.Entity {
	return c.target
}

// Source returns what created this condition
func (c *SimpleCondition) Source() string {
	return c.source
}

// Apply activates the condition's effects
func (c *SimpleCondition) Apply(target core.Entity, bus events.EventBus, opts ...ApplyOption) error {
	if c.Core.IsActive() {
		return ErrAlreadyActive
	}

	if target == nil {
		return ErrNoTarget
	}

	// Apply options
	options := &ApplyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Store the target and source
	c.target = target
	c.source = options.Source

	// Store metadata
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			c.metadata[k] = v
		}
	}

	// Store special options
	if options.SaveDC > 0 {
		c.metadata["save_dc"] = options.SaveDC
	}
	if options.Level > 0 {
		c.metadata["level"] = options.Level
	}
	if options.Duration > 0 {
		c.metadata["duration"] = options.Duration
	}

	// Use the Core's Apply method which manages active state
	if err := c.Core.Apply(bus); err != nil {
		return err
	}

	c.isDirty = true
	return nil
}

// Remove deactivates the condition's effects
func (c *SimpleCondition) Remove(bus events.EventBus) error {
	if !c.Core.IsActive() {
		return ErrNotActive
	}

	// Use the Core's Remove method which handles cleanup
	if err := c.Core.Remove(bus); err != nil {
		return err
	}

	c.isDirty = true
	return nil
}

// GetMetadata returns a metadata value by key
func (c *SimpleCondition) GetMetadata(key string) (interface{}, bool) {
	val, exists := c.metadata[key]
	return val, exists
}

// SetMetadata sets a metadata value
func (c *SimpleCondition) SetMetadata(key string, value interface{}) {
	c.metadata[key] = value
	c.isDirty = true
}

// GetSaveDC returns the save DC for ending this condition
func (c *SimpleCondition) GetSaveDC() int {
	if dc, ok := c.metadata["save_dc"].(int); ok {
		return dc
	}
	return 0
}

// GetLevel returns the level of the condition (e.g., exhaustion)
func (c *SimpleCondition) GetLevel() int {
	if level, ok := c.metadata["level"].(int); ok {
		return level
	}
	return 0
}

// ToJSON serializes the condition to JSON
func (c *SimpleCondition) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"ref":         c.ref,
		"name":        c.name,
		"description": c.description,
		"source":      c.source,
		"is_active":   c.Core.IsActive(),
		"metadata":    c.metadata,
	}

	// Include target ID if we have one
	if c.target != nil {
		data["target_id"] = c.target.GetID()
	}

	return json.Marshal(data)
}

// IsDirty returns true if the condition has unsaved changes
func (c *SimpleCondition) IsDirty() bool {
	return c.isDirty
}

// MarkClean marks the condition as having no unsaved changes
func (c *SimpleCondition) MarkClean() {
	c.isDirty = false
}

// Subscribe is a helper that subscribes to an event and tracks the subscription
func (c *SimpleCondition) Subscribe(bus events.EventBus, eventType string, priority int, handler events.HandlerFunc) {
	c.Core.Subscribe(bus, eventType, priority, handler)
}

// conditionState represents the JSON structure for persistence
type conditionState struct {
	Ref         *core.Ref              `json:"ref"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	IsActive    bool                   `json:"is_active"`
	TargetID    string                 `json:"target_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LoadFromJSON loads condition state from JSON
func (c *SimpleCondition) LoadFromJSON(data json.RawMessage) error {
	var state conditionState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal condition state: %w", err)
	}

	c.ref = state.Ref
	c.name = state.Name
	c.description = state.Description
	c.source = state.Source
	c.metadata = state.Metadata

	// Note: target needs to be resolved by the caller using target_id
	// since we don't have access to the entity repository here

	return nil
}
