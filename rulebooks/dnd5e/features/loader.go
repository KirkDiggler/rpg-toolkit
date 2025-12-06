package features

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

	// Route based on Ref ID
	switch metadata.Ref.ID {
	case refs.Features.Rage().ID:
		rage := &Rage{}
		if err := rage.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load rage: %w", err)
		}

		return rage, nil
	case refs.Features.SecondWind().ID:
		secondWind := &SecondWind{}
		if err := secondWind.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load second wind: %w", err)
		}

		return secondWind, nil
	case refs.Features.ActionSurge().ID:
		actionSurge := &ActionSurge{}
		if err := actionSurge.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load action surge: %w", err)
		}

		return actionSurge, nil
	case refs.Features.FlurryOfBlows().ID:
		flurryOfBlows := &FlurryOfBlows{}
		if err := flurryOfBlows.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load flurry of blows: %w", err)
		}

		return flurryOfBlows, nil
	default:
		return nil, fmt.Errorf("unknown feature type: %s", metadata.Ref.ID)
	}
}
