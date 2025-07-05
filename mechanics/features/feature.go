// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package features provides a system for implementing character abilities, racial traits, and feats.
package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// FeatureType categorizes features by their source.
type FeatureType string

const (
	// FeatureRacial represents features from race (e.g., Darkvision, Lucky).
	FeatureRacial FeatureType = "racial"

	// FeatureClass represents features from class (e.g., Rage, Sneak Attack).
	FeatureClass FeatureType = "class"

	// FeatureSubclass represents features from subclass (e.g., Berserker Frenzy).
	FeatureSubclass FeatureType = "subclass"

	// FeatureFeat represents features from feats (e.g., Great Weapon Master).
	FeatureFeat FeatureType = "feat"

	// FeatureItem represents features from items (e.g., magical item abilities).
	FeatureItem FeatureType = "item"
)

// FeatureTiming indicates when a feature takes effect.
type FeatureTiming string

const (
	// TimingPassive indicates a feature that is always active.
	TimingPassive FeatureTiming = "passive"

	// TimingTriggered indicates a feature that reacts to events.
	TimingTriggered FeatureTiming = "triggered"

	// TimingActivated indicates a feature that must be activated by the player.
	TimingActivated FeatureTiming = "activated"
)

// PrerequisiteChecker is a function that checks if an entity meets a prerequisite.
// Games can provide their own implementations to handle game-specific prerequisites.
type PrerequisiteChecker func(entity core.Entity, prerequisite string) bool

// Feature represents a character ability, racial trait, or feat.
type Feature interface {
	// Key returns the unique identifier for the feature.
	Key() string

	// Name returns the display name of the feature.
	Name() string

	// Description returns a human-readable description of the feature.
	Description() string

	// Type returns the category of the feature.
	Type() FeatureType

	// Level returns the minimum level required for this feature.
	Level() int

	// Source returns where this feature comes from (e.g., "Barbarian", "Half-Orc").
	Source() string

	// IsPassive returns true if the feature is always active.
	IsPassive() bool

	// GetTiming returns when the feature takes effect.
	GetTiming() FeatureTiming

	// GetModifiers returns any modifiers this feature provides.
	GetModifiers() []events.Modifier

	// GetProficiencies returns any proficiencies this feature grants.
	GetProficiencies() []string

	// GetResources returns any resources this feature provides or consumes.
	GetResources() []resources.Resource

	// GetEventListeners returns event listeners for this feature.
	GetEventListeners() []EventListener

	// CanTrigger checks if this feature can trigger on the given event.
	CanTrigger(event events.Event) bool

	// TriggerFeature executes the feature's effect.
	TriggerFeature(entity core.Entity, event events.Event) error

	// HasPrerequisites returns true if this feature has prerequisites.
	HasPrerequisites() bool

	// MeetsPrerequisites checks if the entity meets all prerequisites.
	MeetsPrerequisites(entity core.Entity) bool

	// GetPrerequisites returns the list of prerequisites.
	GetPrerequisites() []string

	// IsActive checks if the feature is currently active for the entity.
	IsActive(entity core.Entity) bool

	// Activate activates the feature for an entity.
	Activate(entity core.Entity, bus events.EventBus) error

	// Deactivate deactivates the feature for an entity.
	Deactivate(entity core.Entity, bus events.EventBus) error
}

// EventListener handles events for features.
type EventListener interface {
	// EventTypes returns the event types this listener cares about.
	EventTypes() []string

	// Priority returns the priority for event handling (higher = later).
	Priority() int

	// HandleEvent processes the event for the feature.
	HandleEvent(feature Feature, entity core.Entity, event events.Event) error
}

// FeatureHolder represents an entity that can have features.
type FeatureHolder interface {
	// AddFeature adds a feature to the entity.
	AddFeature(feature Feature) error

	// RemoveFeature removes a feature by key.
	RemoveFeature(key string) error

	// GetFeature retrieves a feature by key.
	GetFeature(key string) (Feature, bool)

	// GetFeatures returns all features.
	GetFeatures() []Feature

	// GetActiveFeatures returns all currently active features.
	GetActiveFeatures() []Feature

	// ActivateFeature activates a feature by key.
	ActivateFeature(key string, bus events.EventBus) error

	// DeactivateFeature deactivates a feature by key.
	DeactivateFeature(key string, bus events.EventBus) error
}

// FeatureRegistry manages feature definitions and availability.
type FeatureRegistry interface {
	// RegisterFeature registers a feature definition.
	RegisterFeature(feature Feature) error

	// GetFeature retrieves a registered feature by key.
	GetFeature(key string) (Feature, bool)

	// GetAllFeatures returns all registered features.
	GetAllFeatures() []Feature

	// GetFeaturesByType returns all features of a specific type.
	GetFeaturesByType(featureType FeatureType) []Feature

	// GetAvailableFeatures returns features available for an entity at a given level.
	GetAvailableFeatures(entity core.Entity, level int) []Feature

	// GetFeaturesForClass returns features for a specific class at a level.
	GetFeaturesForClass(class string, level int) []Feature

	// GetFeaturesForRace returns features for a specific race.
	GetFeaturesForRace(race string) []Feature
}
