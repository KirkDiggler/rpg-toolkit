// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"
)

// LoadConditionFromJSON loads a condition from its JSON representation.
// This is the big switch that knows how to unmarshal each condition type.
func LoadConditionFromJSON(data json.RawMessage) (ConditionBehavior, error) {
	// Peek at the ref to determine condition type
	var peek struct {
		Ref string `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("failed to peek at condition ref: %w", err)
	}

	// Big switch for all condition types
	switch peek.Ref {
	case "dnd5e:conditions:raging":
		var raging RagingCondition
		if err := json.Unmarshal(data, &raging); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raging condition: %w", err)
		}
		return &raging, nil

	// Future conditions...
	// case "dnd5e:conditions:poisoned":
	//     var poisoned PoisonedCondition
	//     if err := json.Unmarshal(data, &poisoned); err != nil {
	//         return nil, err
	//     }
	//     return &poisoned, nil

	default:
		return nil, fmt.Errorf("unknown condition ref: %s", peek.Ref)
	}
}

// IsValidConditionRef checks if a ref string is a supported condition
func IsValidConditionRef(ref string) bool {
	switch ref {
	case "dnd5e:conditions:raging":
		return true
	// Add more as we implement them
	default:
		return false
	}
}
