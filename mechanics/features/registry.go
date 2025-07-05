// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"fmt"
	"strings"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Registry is the default implementation of FeatureRegistry.
type Registry struct {
	mu       sync.RWMutex
	features map[string]Feature
}

// NewRegistry creates a new feature registry.
func NewRegistry() *Registry {
	return &Registry{
		features: make(map[string]Feature),
	}
}

// RegisterFeature registers a feature definition.
func (r *Registry) RegisterFeature(feature Feature) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := feature.Key()
	if _, exists := r.features[key]; exists {
		return fmt.Errorf("feature %s already registered", key)
	}

	r.features[key] = feature
	return nil
}

// GetFeature retrieves a registered feature by key.
func (r *Registry) GetFeature(key string) (Feature, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	feature, exists := r.features[key]
	return feature, exists
}

// GetAllFeatures returns all registered features.
func (r *Registry) GetAllFeatures() []Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Feature, 0, len(r.features))
	for _, feature := range r.features {
		result = append(result, feature)
	}
	return result
}

// GetFeaturesByType returns all features of a specific type.
func (r *Registry) GetFeaturesByType(featureType FeatureType) []Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Feature
	for _, feature := range r.features {
		if feature.Type() == featureType {
			result = append(result, feature)
		}
	}
	return result
}

// GetAvailableFeatures returns features available for an entity at a given level.
func (r *Registry) GetAvailableFeatures(entity core.Entity, level int) []Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []Feature
	for _, feature := range r.features {
		if feature.Level() <= level && feature.MeetsPrerequisites(entity) {
			available = append(available, feature)
		}
	}
	return available
}

// GetFeaturesForClass returns features for a specific class at a level.
func (r *Registry) GetFeaturesForClass(class string, level int) []Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Feature
	classPrereq := "class:" + strings.ToLower(class)

	for _, feature := range r.features {
		// Check if this feature is for the specified class
		if feature.Type() == FeatureClass {
			hasClassPrereq := false
			for _, prereq := range feature.GetPrerequisites() {
				if strings.ToLower(prereq) == classPrereq {
					hasClassPrereq = true
					break
				}
			}

			if hasClassPrereq && feature.Level() <= level {
				result = append(result, feature)
			}
		}
	}
	return result
}

// GetFeaturesForRace returns features for a specific race.
func (r *Registry) GetFeaturesForRace(race string) []Feature {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Feature
	racePrereq := "race:" + strings.ToLower(race)

	for _, feature := range r.features {
		// Check if this feature is for the specified race
		if feature.Type() == FeatureRacial {
			hasRacePrereq := false
			for _, prereq := range feature.GetPrerequisites() {
				if strings.ToLower(prereq) == racePrereq {
					hasRacePrereq = true
					break
				}
			}

			if hasRacePrereq {
				result = append(result, feature)
			}
		}
	}
	return result
}
