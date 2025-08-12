// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// SimpleFeature provides common feature infrastructure.
// It embeds effects.Core for event subscription management and
// implements the boilerplate methods of the Feature interface.
type SimpleFeature struct {
	*effects.Core // Event subscriptions, activation tracking
	
	ref         *core.Ref
	name        string
	description string
	needsTarget bool
	isActive    bool
	dirty       bool
	
	// Hooks for customization
	onActivate func(owner core.Entity, ctx *ActivateContext) error
	onApply    func(bus events.EventBus) error
	onRemove   func(bus events.EventBus) error
}

// SimpleFeatureConfig configures a SimpleFeature.
type SimpleFeatureConfig struct {
	Ref         *core.Ref
	Name        string
	Description string
	NeedsTarget bool
	OnActivate  func(owner core.Entity, ctx *ActivateContext) error
	OnApply     func(bus events.EventBus) error
	OnRemove    func(bus events.EventBus) error
}

// NewSimpleFeature creates a new SimpleFeature with the given configuration.
func NewSimpleFeature(config SimpleFeatureConfig) *SimpleFeature {
	if config.Ref == nil {
		panic("feature ref cannot be nil")
	}
	
	// Create effects.Core for event management
	effectsCore := effects.NewCore(effects.CoreConfig{
		ID:   config.Ref.String(),
		Type: "feature",
	})
	
	return &SimpleFeature{
		Core:        effectsCore,
		ref:         config.Ref,
		name:        config.Name,
		description: config.Description,
		needsTarget: config.NeedsTarget,
		onActivate:  config.OnActivate,
		onApply:     config.OnApply,
		onRemove:    config.OnRemove,
	}
}

// Ref returns the feature's unique identifier.
func (f *SimpleFeature) Ref() *core.Ref {
	return f.ref
}

// Name returns the display name.
func (f *SimpleFeature) Name() string {
	return f.name
}

// Description returns the human-readable description.
func (f *SimpleFeature) Description() string {
	return f.description
}

// NeedsTarget returns whether this feature requires a target.
func (f *SimpleFeature) NeedsTarget() bool {
	return f.needsTarget
}

// Activate activates the feature with optional parameters.
func (f *SimpleFeature) Activate(owner core.Entity, opts ...ActivateOption) error {
	if f.onActivate != nil {
		ctx := parseOptions(opts...)
		
		// Check if target is required but not provided
		if f.needsTarget && ctx.Target == nil {
			return fmt.Errorf("feature %s requires a target", f.ref)
		}
		
		err := f.onActivate(owner, ctx)
		if err != nil {
			return err
		}
		
		f.isActive = true
		f.dirty = true
	}
	return nil
}

// IsActive returns whether the feature is currently active.
func (f *SimpleFeature) IsActive() bool {
	return f.isActive
}

// Apply subscribes the feature to relevant events.
func (f *SimpleFeature) Apply(bus events.EventBus) error {
	// First apply the effects.Core subscriptions
	if err := f.Core.Apply(bus); err != nil {
		return err
	}
	
	// Then apply any custom subscriptions
	if f.onApply != nil {
		return f.onApply(bus)
	}
	
	return nil
}

// Remove unsubscribes the feature from events.
func (f *SimpleFeature) Remove(bus events.EventBus) error {
	// First remove custom subscriptions
	if f.onRemove != nil {
		if err := f.onRemove(bus); err != nil {
			return err
		}
	}
	
	// Then remove effects.Core subscriptions
	return f.Core.Remove(bus)
}

// ToJSON serializes the feature state.
func (f *SimpleFeature) ToJSON() json.RawMessage {
	data := map[string]interface{}{
		"ref":       f.ref.String(),
		"is_active": f.isActive,
	}
	
	bytes, _ := json.Marshal(data)
	return bytes
}

// IsDirty returns whether the feature has unsaved changes.
func (f *SimpleFeature) IsDirty() bool {
	return f.dirty
}

// MarkClean marks the feature as saved.
func (f *SimpleFeature) MarkClean() {
	f.dirty = false
}

// SetActive sets the active state (used when loading from JSON).
func (f *SimpleFeature) SetActive(active bool) {
	f.isActive = active
	f.dirty = false // Just loaded, not dirty
}