// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// FeatureData provides access to a feature's ref and raw JSON data.
// This allows consumers to identify features and route them to the appropriate
// loader without the mechanics package knowing about specific game implementations.
type FeatureData interface {
	Ref() *core.Ref        // The feature's identifier (e.g., "dnd5e:feature:rage")
	JSON() json.RawMessage // The raw JSON data for this feature
}

// featureData is the internal implementation of FeatureData.
type featureData struct {
	ref  *core.Ref
	json json.RawMessage
}

// Ref returns the feature's reference.
func (f *featureData) Ref() *core.Ref {
	return f.ref
}

// JSON returns the raw JSON data.
func (f *featureData) JSON() json.RawMessage {
	return f.json
}

// Load extracts feature data (ref + JSON) from raw JSON.
// This is the primary way to identify what type of feature you're dealing with
// before routing it to the appropriate game-specific loader.
//
// Example usage:
//
//	data, err := features.Load(jsonFromDB)
//	if err != nil {
//	    return err
//	}
//
//	switch data.Ref().Module {
//	case "dnd5e":
//	    feat, err = dnd5e.LoadFeature(data.JSON())
//	case "pathfinder":
//	    feat, err = pathfinder.LoadFeature(data.JSON())
//	}
func Load(data json.RawMessage) (FeatureData, error) {
	// Peek at just the ref field
	var peek struct {
		Ref string `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmarshalFailed, err)
	}

	if peek.Ref == "" {
		return nil, ErrInvalidRef
	}

	// Parse the ref string into a structured Ref
	ref, err := core.ParseString(peek.Ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRef, err)
	}

	return &featureData{
		ref:  ref,
		json: data,
	}, nil
}

// LoadAll extracts feature data for multiple features from a JSON array.
// Returns a slice of FeatureData that can be used to route each feature
// to its appropriate loader.
func LoadAll(data json.RawMessage) ([]FeatureData, error) {
	var dataArray []json.RawMessage
	if err := json.Unmarshal(data, &dataArray); err != nil {
		return nil, fmt.Errorf("%w: expected array", ErrUnmarshalFailed)
	}

	result := make([]FeatureData, 0, len(dataArray))
	for i, d := range dataArray {
		featData, err := Load(d)
		if err != nil {
			return nil, fmt.Errorf("failed to load feature %d: %w", i, err)
		}
		result = append(result, featData)
	}

	return result, nil
}
