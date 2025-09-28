// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// LoadJSON loads a condition from its JSON representation.
// This is the big switch that knows how to unmarshal each condition type.
func LoadJSON(data json.RawMessage) (ConditionBehavior, error) {
	// Peek at the ref to determine condition type
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("failed to peek at condition ref: %w", err)
	}

	// Big switch for all condition types
	switch peek.Ref.Value {
	case "raging":
		var raging RagingCondition
		if err := json.Unmarshal(data, &raging); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raging condition: %w", err)
		}

		return &raging, nil
	default:
		return nil, fmt.Errorf("unknown condition ref: %s", peek.Ref)
	}
}
