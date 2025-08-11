// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SimpleFeatureHolder is a basic implementation of FeatureHolder.
// This is provided as a reference implementation. Games should implement
// their own FeatureHolder as part of their entity types.
type SimpleFeatureHolder struct {
	mu       sync.RWMutex
	features map[string]Feature // Still keyed by string for simpler lookups
	entity   core.Entity
}

// NewSimpleFeatureHolder creates a new feature holder.
func NewSimpleFeatureHolder(entity core.Entity) *SimpleFeatureHolder {
	return &SimpleFeatureHolder{
		features: make(map[string]Feature),
		entity:   entity,
	}
}

// AddFeature adds a feature to the entity.
func (h *SimpleFeatureHolder) AddFeature(feature Feature) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := feature.Key().String()
	if _, exists := h.features[key]; exists {
		return fmt.Errorf("feature %s already exists", key)
	}

	h.features[key] = feature
	return nil
}

// RemoveFeature removes a feature by key.
func (h *SimpleFeatureHolder) RemoveFeature(key string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.features[key]; !exists {
		return fmt.Errorf("feature %s not found", key)
	}

	delete(h.features, key)
	return nil
}

// GetFeature retrieves a feature by key.
func (h *SimpleFeatureHolder) GetFeature(key string) (Feature, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	feature, exists := h.features[key]
	return feature, exists
}

// GetFeatures returns all features.
func (h *SimpleFeatureHolder) GetFeatures() []Feature {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]Feature, 0, len(h.features))
	for _, feature := range h.features {
		result = append(result, feature)
	}
	return result
}

// GetActiveFeatures returns all currently active features.
func (h *SimpleFeatureHolder) GetActiveFeatures() []Feature {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var active []Feature
	for _, feature := range h.features {
		if feature.IsActive(h.entity) {
			active = append(active, feature)
		}
	}
	return active
}

// ActivateFeature activates a feature by key.
func (h *SimpleFeatureHolder) ActivateFeature(key string, bus events.EventBus) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	feature, exists := h.features[key]
	if !exists {
		return fmt.Errorf("feature %s not found", key)
	}

	return feature.Activate(h.entity, bus)
}

// DeactivateFeature deactivates a feature by key.
func (h *SimpleFeatureHolder) DeactivateFeature(key string, bus events.EventBus) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	feature, exists := h.features[key]
	if !exists {
		return fmt.Errorf("feature %s not found", key)
	}

	return feature.Deactivate(h.entity, bus)
}
