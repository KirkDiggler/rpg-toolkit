// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ConditionData provides access to condition reference and raw JSON data
// for routing to specific implementations without knowing the concrete type
type ConditionData interface {
	// Ref returns the condition reference for routing
	Ref() *core.Ref

	// JSON returns the raw JSON data for the condition
	JSON() json.RawMessage
}

// conditionData implements ConditionData
type conditionData struct {
	ref  *core.Ref
	data json.RawMessage
}

// Ref returns the condition reference
func (c *conditionData) Ref() *core.Ref {
	return c.ref
}

// JSON returns the raw JSON data
func (c *conditionData) JSON() json.RawMessage {
	return c.data
}

// Load extracts condition reference and JSON from raw data
func Load(data json.RawMessage) (ConditionData, error) {
	// Peek at just the ref string for routing
	var peek struct {
		Ref string `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, &LoadError{
			Data: data,
			Err:  fmt.Errorf("failed to peek at condition ref: %w", err),
		}
	}

	if peek.Ref == "" {
		return nil, &LoadError{
			Data: data,
			Err:  ErrInvalidRef,
		}
	}

	// Let core handle ref parsing and evolution
	ref, err := core.ParseString(peek.Ref)
	if err != nil {
		return nil, &LoadError{
			Data: data,
			Err:  fmt.Errorf("failed to parse ref string '%s': %w", peek.Ref, err),
		}
	}

	return &conditionData{
		ref:  ref,
		data: data,  // Keep full JSON for the actual implementation
	}, nil
}

// LoadAll extracts multiple conditions from a JSON array
func LoadAll(data json.RawMessage) ([]ConditionData, error) {
	var rawConditions []json.RawMessage
	if err := json.Unmarshal(data, &rawConditions); err != nil {
		return nil, &LoadError{
			Data: data,
			Err:  fmt.Errorf("failed to unmarshal condition array: %w", err),
		}
	}

	conditions := make([]ConditionData, 0, len(rawConditions))
	for i, raw := range rawConditions {
		cond, err := Load(raw)
		if err != nil {
			return nil, &LoadError{
				Data: raw,
				Err:  fmt.Errorf("failed to load condition at index %d: %w", i, err),
			}
		}
		conditions = append(conditions, cond)
	}

	return conditions, nil
}

// LoadFromDatabase simulates loading conditions from a database
// In practice, this would query your actual database
func LoadFromDatabase(entityID string) ([]ConditionData, error) {
	// This is a placeholder - actual implementation would:
	// 1. Query database for conditions on this entity
	// 2. Convert database rows to JSON
	// 3. Use LoadAll to create ConditionData instances

	// Example structure:
	// SELECT condition_data FROM conditions WHERE entity_id = ?
	// Then for each row, parse the JSON and create ConditionData

	return nil, fmt.Errorf("database loading not implemented")
}
