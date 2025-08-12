// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"fmt"
)

// FeatureLoader is a function that can load a specific feature type from JSON.
type FeatureLoader func(data json.RawMessage) (Feature, error)

// featureRegistry maps feature refs to their loaders.
// In a real implementation, this would be populated by game modules.
var featureRegistry = make(map[string]FeatureLoader)

// RegisterFeatureLoader registers a loader for a specific feature ref.
// Game modules call this to register their features.
func RegisterFeatureLoader(ref string, loader FeatureLoader) {
	featureRegistry[ref] = loader
}

// LoadFeatureFromJSON loads a feature from JSON data.
// It peeks at the ref field to determine which loader to use.
func LoadFeatureFromJSON(data json.RawMessage) (Feature, error) {
	// Peek at the ref to determine feature type
	var peek struct {
		Ref string `json:"ref"`
	}
	
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("failed to peek at feature ref: %w", err)
	}
	
	// Look up the loader for this ref
	loader, exists := featureRegistry[peek.Ref]
	if !exists {
		return nil, fmt.Errorf("no loader registered for feature: %s", peek.Ref)
	}
	
	// Use the specific loader
	return loader(data)
}

// LoadFeaturesFromJSON loads multiple features from a JSON array.
func LoadFeaturesFromJSON(data json.RawMessage) ([]Feature, error) {
	var featuresData []json.RawMessage
	if err := json.Unmarshal(data, &featuresData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal features array: %w", err)
	}
	
	features := make([]Feature, 0, len(featuresData))
	for i, featureData := range featuresData {
		feature, err := LoadFeatureFromJSON(featureData)
		if err != nil {
			return nil, fmt.Errorf("failed to load feature %d: %w", i, err)
		}
		features = append(features, feature)
	}
	
	return features, nil
}