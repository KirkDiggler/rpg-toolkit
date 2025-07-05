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

// FeatureManager manages features for entities.
type FeatureManager interface {
	// GetFeatures returns all features of a specific type for an entity.
	GetFeatures(entity core.Entity, featureType FeatureType) []Feature

	// GetActiveFeatures returns all currently active features for an entity.
	GetActiveFeatures(entity core.Entity) []Feature

	// AddFeature adds a feature to an entity.
	AddFeature(entity core.Entity, feature Feature) error

	// RemoveFeature removes a feature from an entity.
	RemoveFeature(entity core.Entity, featureKey string) error

	// ActivateFeature activates an activated-type feature.
	ActivateFeature(entity core.Entity, featureKey string) error

	// DeactivateFeature deactivates an activated-type feature.
	DeactivateFeature(entity core.Entity, featureKey string) error

	// ProcessLevelUp grants new features when an entity levels up.
	ProcessLevelUp(entity core.Entity, newLevel int) error

	// GetAvailableFeatures returns features available at a given level.
	GetAvailableFeatures(entity core.Entity, level int) []Feature

	// RegisterFeature registers a feature in the system.
	RegisterFeature(feature Feature) error

	// GetFeature retrieves a registered feature by key.
	GetFeature(key string) (Feature, bool)
}
