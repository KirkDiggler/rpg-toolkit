package features

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Feature represents a class feature that can be activated
type Feature interface {
	core.Action[FeatureInput] // Can be activated
	ToJSON() (json.RawMessage, error)
}

// LoadJSON loads a feature from JSON based on its ref
func LoadJSON(data json.RawMessage) (Feature, error) {
	// First, extract just the ID to determine feature type
	var metadata struct {
		Ref core.Ref `json:"ref"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to extract feature ID: %w", err)
	}

	// Route based on Ref value
	switch metadata.Ref.Value {
	case "rage":
		rage := &Rage{}
		if err := rage.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load rage: %w", err)
		}

		return rage, nil
	default:
		return nil, fmt.Errorf("unknown feature type: %s", metadata.Ref.Value)
	}
}
