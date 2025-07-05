// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// BasicFeature is a standard implementation of the Feature interface.
type BasicFeature struct {
	key            string
	name           string
	description    string
	featureType    FeatureType
	level          int
	source         string
	timing         FeatureTiming
	modifiers      []events.Modifier
	proficiencies  []string
	resources      []resources.Resource
	eventListeners []EventListener
	prerequisites  []string
	isActive       bool
}

// NewBasicFeature creates a new basic feature.
func NewBasicFeature(key, name string) *BasicFeature {
	return &BasicFeature{
		key:           key,
		name:          name,
		timing:        TimingPassive,
		modifiers:     []events.Modifier{},
		proficiencies: []string{},
		resources:     []resources.Resource{},
		prerequisites: []string{},
	}
}

// Key returns the unique identifier for the feature.
func (f *BasicFeature) Key() string {
	return f.key
}

// Name returns the display name of the feature.
func (f *BasicFeature) Name() string {
	return f.name
}

// Description returns a human-readable description of the feature.
func (f *BasicFeature) Description() string {
	return f.description
}

// Type returns the category of the feature.
func (f *BasicFeature) Type() FeatureType {
	return f.featureType
}

// Level returns the minimum level required for this feature.
func (f *BasicFeature) Level() int {
	return f.level
}

// Source returns where this feature comes from.
func (f *BasicFeature) Source() string {
	return f.source
}

// IsPassive returns true if the feature is always active.
func (f *BasicFeature) IsPassive() bool {
	return f.timing == TimingPassive
}

// GetTiming returns when the feature takes effect.
func (f *BasicFeature) GetTiming() FeatureTiming {
	return f.timing
}

// GetModifiers returns any modifiers this feature provides.
func (f *BasicFeature) GetModifiers() []events.Modifier {
	return f.modifiers
}

// GetProficiencies returns any proficiencies this feature grants.
func (f *BasicFeature) GetProficiencies() []string {
	return f.proficiencies
}

// GetResources returns any resources this feature provides or consumes.
func (f *BasicFeature) GetResources() []resources.Resource {
	return f.resources
}

// GetEventListeners returns event listeners for this feature.
func (f *BasicFeature) GetEventListeners() []EventListener {
	return f.eventListeners
}

// CanTrigger checks if this feature can trigger on the given event.
func (f *BasicFeature) CanTrigger(event events.Event) bool {
	if f.timing != TimingTriggered {
		return false
	}

	eventType := event.Type()
	for _, listener := range f.eventListeners {
		for _, listenType := range listener.EventTypes() {
			if listenType == eventType {
				return true
			}
		}
	}
	return false
}

// TriggerFeature executes the feature's effect.
func (f *BasicFeature) TriggerFeature(entity core.Entity, event events.Event) error {
	if !f.CanTrigger(event) {
		return nil
	}

	for _, listener := range f.eventListeners {
		for _, listenType := range listener.EventTypes() {
			if listenType == event.Type() {
				if err := listener.HandleEvent(f, entity, event); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// HasPrerequisites returns true if this feature has prerequisites.
func (f *BasicFeature) HasPrerequisites() bool {
	return len(f.prerequisites) > 0
}

// MeetsPrerequisites checks if the entity meets all prerequisites.
func (f *BasicFeature) MeetsPrerequisites(_ core.Entity) bool {
	// This is a simplified implementation
	// Real implementation would need to parse and check prerequisites
	return true
}

// GetPrerequisites returns the list of prerequisites.
func (f *BasicFeature) GetPrerequisites() []string {
	return f.prerequisites
}

// IsActive checks if the feature is currently active for the entity.
func (f *BasicFeature) IsActive(_ core.Entity) bool {
	if f.timing == TimingPassive {
		return true
	}
	return f.isActive
}

// Builder methods for fluent API

// WithDescription sets the feature description.
func (f *BasicFeature) WithDescription(description string) *BasicFeature {
	f.description = description
	return f
}

// WithType sets the feature type.
func (f *BasicFeature) WithType(featureType FeatureType) *BasicFeature {
	f.featureType = featureType
	return f
}

// WithLevel sets the minimum level.
func (f *BasicFeature) WithLevel(level int) *BasicFeature {
	f.level = level
	return f
}

// WithSource sets the feature source.
func (f *BasicFeature) WithSource(source string) *BasicFeature {
	f.source = source
	return f
}

// WithTiming sets the feature timing.
func (f *BasicFeature) WithTiming(timing FeatureTiming) *BasicFeature {
	f.timing = timing
	return f
}

// WithModifiers adds modifiers to the feature.
func (f *BasicFeature) WithModifiers(modifiers ...events.Modifier) *BasicFeature {
	f.modifiers = append(f.modifiers, modifiers...)
	return f
}

// WithProficiencies adds proficiencies to the feature.
func (f *BasicFeature) WithProficiencies(proficiencies ...string) *BasicFeature {
	f.proficiencies = append(f.proficiencies, proficiencies...)
	return f
}

// WithResources adds resources to the feature.
func (f *BasicFeature) WithResources(resources ...resources.Resource) *BasicFeature {
	f.resources = append(f.resources, resources...)
	return f
}

// WithEventListeners adds event listeners to the feature.
func (f *BasicFeature) WithEventListeners(listeners ...EventListener) *BasicFeature {
	f.eventListeners = append(f.eventListeners, listeners...)
	return f
}

// WithPrerequisites adds prerequisites to the feature.
func (f *BasicFeature) WithPrerequisites(prerequisites ...string) *BasicFeature {
	f.prerequisites = append(f.prerequisites, prerequisites...)
	return f
}
