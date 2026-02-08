package features

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Feature represents a class feature that can be activated
type Feature interface {
	core.Action[FeatureInput] // Can be activated
	ToJSON() (json.RawMessage, error)
	ActionType() combat.ActionType // Returns action economy cost to activate
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
	case refs.Features.PatientDefense().ID:
		patientDefense := &PatientDefense{}
		if err := patientDefense.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load patient defense: %w", err)
		}

		return patientDefense, nil
	case refs.Features.StepOfTheWind().ID:
		stepOfTheWind := &StepOfTheWind{}
		if err := stepOfTheWind.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load step of the wind: %w", err)
		}

		return stepOfTheWind, nil
	case refs.Features.RecklessAttack().ID:
		recklessAttack := &RecklessAttack{}
		if err := recklessAttack.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load reckless attack: %w", err)
		}

		return recklessAttack, nil
	case refs.Features.DeflectMissiles().ID:
		deflectMissiles := &DeflectMissiles{}
		if err := deflectMissiles.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load deflect missiles: %w", err)
		}

		return deflectMissiles, nil
	default:
		return nil, fmt.Errorf("unknown feature type: %s", metadata.Ref.ID)
	}
}
