// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"fmt"
)

// LoadFeatureFromJSON loads a feature from JSON data using a simple switch.
// Each game module adds their features to this switch.
// This is intentionally simple - no magic registries.
func LoadFeatureFromJSON(data json.RawMessage) (Feature, error) {
	// Peek at the ref to determine feature type
	var peek struct {
		Ref string `json:"ref"`
	}
	
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmarshalFailed, err)
	}
	
	// Simple switch - each game adds their features here
	switch peek.Ref {
	// Example features would be added here:
	// case "dnd5e:feature:rage":
	//     return barbarian.LoadRageFromJSON(data)
	// case "dnd5e:feature:second_wind":
	//     return fighter.LoadSecondWindFromJSON(data)
	default:
		return nil, NewLoadError(peek.Ref, ErrFeatureNotFound)
	}
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