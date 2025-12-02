// Package features provides D&D 5e class features implementation
package features

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// CreateFromRefInput provides input for creating a feature from a ref
type CreateFromRefInput struct {
	// Ref is the typed feature reference (e.g., dnd5e:features:rage)
	Ref *core.Ref
	// Config is feature-specific configuration as JSON
	Config json.RawMessage
	// CharacterID is the ID of the character this feature belongs to
	CharacterID string
}

// CreateFromRefOutput provides the result of creating a feature from a ref
type CreateFromRefOutput struct {
	// Feature is the created feature
	Feature Feature
}

// CreateFromRef creates a feature from a ref and configuration.
// The ref determines which feature type to create, and
// the config is parsed by each feature's specific factory logic.
func CreateFromRef(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input is nil")
	}

	if input.Ref == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "ref is required")
	}

	if input.CharacterID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "character_id is required")
	}

	// Validate module and type
	if input.Ref.Module != "dnd5e" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unsupported module: %s", input.Ref.Module)
	}
	if input.Ref.Type != "features" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unsupported type: %s (expected 'features')", input.Ref.Type)
	}

	// Create the feature based on the ID
	var feature Feature
	var err error

	switch input.Ref.ID {
	case RageID:
		feature, err = createRage(input.Config, input.CharacterID)
	case SecondWindID:
		feature, err = createSecondWind(input.Config, input.CharacterID)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown feature: %s", input.Ref.ID)
	}

	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to create feature: %s", input.Ref.ID)
	}

	return &CreateFromRefOutput{Feature: feature}, nil
}

// rageConfig is the config structure for rage feature
type rageConfig struct {
	Uses        int `json:"uses"`         // Number of uses (default based on level)
	DamageBonus int `json:"damage_bonus"` // Damage bonus (default based on level)
	Level       int `json:"level"`        // Barbarian level (optional, for calculating defaults)
}

// createRage creates a rage feature from config
func createRage(config json.RawMessage, _ string) (*Rage, error) {
	var cfg rageConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse rage config")
		}
	}

	// Default level to 1 if not specified
	level := cfg.Level
	if level == 0 {
		level = 1
	}

	// Calculate uses based on level if not explicitly set
	uses := cfg.Uses
	if uses == 0 {
		uses = calculateRageUses(level)
	}

	// Create resource for tracking uses
	resource := resources.NewResource("rage", uses)

	return &Rage{
		id:       "rage",
		name:     "Rage",
		level:    level,
		resource: resource,
	}, nil
}

// secondWindConfig is the config structure for second wind feature
type secondWindConfig struct {
	Uses  int `json:"uses"`  // Number of uses (default 1)
	Level int `json:"level"` // Fighter level (for healing calculation)
}

// createSecondWind creates a second wind feature from config
func createSecondWind(config json.RawMessage, _ string) (*SecondWind, error) {
	var cfg secondWindConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse second wind config")
		}
	}

	// Default level to 1 if not specified
	level := cfg.Level
	if level == 0 {
		level = 1
	}

	// Default uses to 1 (restores on short rest)
	uses := cfg.Uses
	if uses == 0 {
		uses = 1
	}

	// Create resource for tracking uses
	resource := resources.NewResource("second_wind", uses)

	return &SecondWind{
		id:       "second_wind",
		name:     "Second Wind",
		level:    level,
		resource: resource,
	}, nil
}
