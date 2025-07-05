// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Manager is the default implementation of FeatureManager.
type Manager struct {
	mu                 sync.RWMutex
	registeredFeatures map[string]Feature
	entityFeatures     map[string]map[string]Feature // entity ID -> feature key -> feature
	activeFeatures     map[string]map[string]bool    // entity ID -> feature key -> is active
}

// NewManager creates a new feature manager.
func NewManager() *Manager {
	return &Manager{
		registeredFeatures: make(map[string]Feature),
		entityFeatures:     make(map[string]map[string]Feature),
		activeFeatures:     make(map[string]map[string]bool),
	}
}

// RegisterFeature registers a feature in the system.
func (m *Manager) RegisterFeature(feature Feature) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := feature.Key()
	if _, exists := m.registeredFeatures[key]; exists {
		return fmt.Errorf("feature %s already registered", key)
	}

	m.registeredFeatures[key] = feature
	return nil
}

// GetFeature retrieves a registered feature by key.
func (m *Manager) GetFeature(key string) (Feature, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feature, exists := m.registeredFeatures[key]
	return feature, exists
}

// GetFeatures returns all features of a specific type for an entity.
func (m *Manager) GetFeatures(entity core.Entity, featureType FeatureType) []Feature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entityID := entity.GetID()
	features, exists := m.entityFeatures[entityID]
	if !exists {
		return []Feature{}
	}

	var result []Feature
	for _, feature := range features {
		if feature.Type() == featureType {
			result = append(result, feature)
		}
	}
	return result
}

// GetActiveFeatures returns all currently active features for an entity.
func (m *Manager) GetActiveFeatures(entity core.Entity) []Feature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entityID := entity.GetID()
	features, exists := m.entityFeatures[entityID]
	if !exists {
		return []Feature{}
	}

	activeMap, hasActive := m.activeFeatures[entityID]
	if !hasActive {
		activeMap = make(map[string]bool)
	}

	var result []Feature
	for key, feature := range features {
		// Passive features are always active
		if feature.GetTiming() == TimingPassive {
			result = append(result, feature)
		} else if activeMap[key] {
			result = append(result, feature)
		}
	}
	return result
}

// AddFeature adds a feature to an entity.
func (m *Manager) AddFeature(entity core.Entity, feature Feature) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check prerequisites
	if feature.HasPrerequisites() && !feature.MeetsPrerequisites(entity) {
		return fmt.Errorf("entity does not meet prerequisites for feature %s", feature.Key())
	}

	entityID := entity.GetID()
	if _, exists := m.entityFeatures[entityID]; !exists {
		m.entityFeatures[entityID] = make(map[string]Feature)
	}

	featureKey := feature.Key()
	if _, exists := m.entityFeatures[entityID][featureKey]; exists {
		return fmt.Errorf("entity already has feature %s", featureKey)
	}

	m.entityFeatures[entityID][featureKey] = feature

	// Passive features are automatically active
	if feature.GetTiming() == TimingPassive {
		if _, exists := m.activeFeatures[entityID]; !exists {
			m.activeFeatures[entityID] = make(map[string]bool)
		}
		m.activeFeatures[entityID][featureKey] = true
	}

	return nil
}

// RemoveFeature removes a feature from an entity.
func (m *Manager) RemoveFeature(entity core.Entity, featureKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entityID := entity.GetID()
	features, exists := m.entityFeatures[entityID]
	if !exists {
		return fmt.Errorf("entity has no features")
	}

	if _, exists := features[featureKey]; !exists {
		return fmt.Errorf("entity does not have feature %s", featureKey)
	}

	delete(m.entityFeatures[entityID], featureKey)

	// Also remove from active features
	if activeMap, exists := m.activeFeatures[entityID]; exists {
		delete(activeMap, featureKey)
	}

	return nil
}

// ActivateFeature activates an activated-type feature.
func (m *Manager) ActivateFeature(entity core.Entity, featureKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entityID := entity.GetID()
	features, exists := m.entityFeatures[entityID]
	if !exists {
		return fmt.Errorf("entity has no features")
	}

	feature, exists := features[featureKey]
	if !exists {
		return fmt.Errorf("entity does not have feature %s", featureKey)
	}

	if feature.GetTiming() != TimingActivated {
		return fmt.Errorf("feature %s is not an activated feature", featureKey)
	}

	if _, exists := m.activeFeatures[entityID]; !exists {
		m.activeFeatures[entityID] = make(map[string]bool)
	}

	m.activeFeatures[entityID][featureKey] = true
	return nil
}

// DeactivateFeature deactivates an activated-type feature.
func (m *Manager) DeactivateFeature(entity core.Entity, featureKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entityID := entity.GetID()
	features, exists := m.entityFeatures[entityID]
	if !exists {
		return fmt.Errorf("entity has no features")
	}

	feature, exists := features[featureKey]
	if !exists {
		return fmt.Errorf("entity does not have feature %s", featureKey)
	}

	if feature.GetTiming() != TimingActivated {
		return fmt.Errorf("feature %s is not an activated feature", featureKey)
	}

	if activeMap, exists := m.activeFeatures[entityID]; exists {
		delete(activeMap, featureKey)
	}

	return nil
}

// ProcessLevelUp grants new features when an entity levels up.
func (m *Manager) ProcessLevelUp(_ core.Entity, _ int) error {
	// This would check available features for the new level
	// and automatically grant them based on class/race
	// For now, this is a placeholder
	return nil
}

// GetAvailableFeatures returns features available at a given level.
func (m *Manager) GetAvailableFeatures(entity core.Entity, level int) []Feature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var available []Feature
	for _, feature := range m.registeredFeatures {
		if feature.Level() <= level && feature.MeetsPrerequisites(entity) {
			// Check if entity already has this feature
			entityID := entity.GetID()
			if entityFeatures, exists := m.entityFeatures[entityID]; exists {
				if _, hasFeature := entityFeatures[feature.Key()]; hasFeature {
					continue
				}
			}
			available = append(available, feature)
		}
	}
	return available
}
