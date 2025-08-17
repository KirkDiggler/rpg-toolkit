// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package features implements D&D 5e character features as Actions.
package features

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// LoadJSON creates a feature from JSON data
func LoadJSON(data []byte, bus *events.Bus) (Feature, error) {
	var input struct {
		Ref  string          `json:"ref"`
		ID   string          `json:"id"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feature: %w", err)
	}

	// Parse ref to get feature type
	ref, err := core.ParseString(input.Ref)
	if err != nil {
		return nil, fmt.Errorf("invalid ref: %w", err)
	}

	// ref.Value should be something like "rage"
	featureKey := FeatureKey(ref.Value)

	switch featureKey {
	case FeatureKeyRage:
		var rageData struct {
			Uses  int `json:"uses"`
			Level int `json:"level"`
		}
		if err := json.Unmarshal(input.Data, &rageData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rage data: %w", err)
		}

		return &Rage{
			id:          input.ID,
			uses:        rageData.Uses,
			level:       rageData.Level,
			currentUses: rageData.Uses, // Start with max uses
			bus:         bus,
		}, nil

	default:
		return nil, fmt.Errorf("unknown feature: %s", featureKey)
	}
}
