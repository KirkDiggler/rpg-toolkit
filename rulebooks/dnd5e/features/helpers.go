// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// RefToFeatureKey extracts the feature key from a ref string.
// Returns an error if the ref is not a supported dnd5e feature.
func RefToFeatureKey(refStr string) (FeatureKey, error) {
	ref, err := core.ParseString(refStr)
	if err != nil {
		return "", fmt.Errorf("invalid ref: %w", err)
	}

	// Check if it's a dnd5e feature
	if ref.Module != "dnd5e" || ref.Type != "features" {
		return "", fmt.Errorf("not a dnd5e feature ref: %s", refStr)
	}

	// Extract feature key from the value
	key := FeatureKey(ref.Value)

	// Check if it's supported
	switch key {
	case FeatureKeyRage:
		return key, nil
	default:
		return "", fmt.Errorf("unsupported feature: %s", key)
	}
}

// IsConditionRef checks if a ref string is a condition
func IsConditionRef(refStr string) bool {
	if ref, err := core.ParseString(refStr); err == nil {
		return ref.Module == "dnd5e" && ref.Type == "conditions"
	}
	return false
}

// ParseConditionType extracts the condition type from a ref
func ParseConditionType(refStr string) (string, error) {
	ref, err := core.ParseString(refStr)
	if err != nil {
		return "", err
	}

	if ref.Module != "dnd5e" || ref.Type != "conditions" {
		return "", fmt.Errorf("not a condition ref: %s", refStr)
	}

	return ref.Value, nil
}

// ExtractFeatureLevel attempts to extract level from feature ID
// e.g., "rage-lvl-5" -> 5
func ExtractFeatureLevel(featureID string) int {
	parts := strings.Split(featureID, "-")
	for i, part := range parts {
		if part == "lvl" && i+1 < len(parts) {
			var level int
			if _, err := fmt.Sscanf(parts[i+1], "%d", &level); err == nil {
				return level
			}
		}
	}
	return 1 // Default to level 1 if not specified
}
