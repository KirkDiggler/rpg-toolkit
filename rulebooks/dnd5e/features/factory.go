// Package features provides D&D 5e class features implementation
package features

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// CreateFromRefInput provides input for creating a feature from a ref string
type CreateFromRefInput struct {
	// Ref is the feature reference in "module:type:value" format
	// e.g., "dnd5e:features:rage"
	Ref string
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

// CreateFromRef creates a feature from a ref string and configuration.
// The ref is parsed to determine which feature type to create, and
// the config is parsed by each feature's specific factory logic.
func CreateFromRef(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input is nil")
	}

	if input.Ref == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "ref is required")
	}

	if input.CharacterID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "character_id is required")
	}

	// Parse the ref to get the feature type
	ref, err := core.ParseString(input.Ref)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse ref: %s", input.Ref)
	}

	// Validate module and type
	if ref.Module != refs.Module {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unsupported module: %s", ref.Module)
	}
	if ref.Type != refs.TypeFeatures {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unsupported type: %s (expected '%s')", ref.Type, refs.TypeFeatures)
	}

	// Create the feature based on the ID
	var feature Feature

	switch ref.ID {
	case refs.Features.Rage().ID:
		feature, err = createRage(input.Config, input.CharacterID)
	case refs.Features.SecondWind().ID:
		feature, err = createSecondWind(input.Config, input.CharacterID)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown feature: %s", ref.ID)
	}

	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to create feature: %s", ref.ID)
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
	resource := resources.NewResource(refs.Features.Rage().ID, uses)

	return &Rage{
		id:       refs.Features.Rage().ID,
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
func createSecondWind(config json.RawMessage, characterID string) (*SecondWind, error) {
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

	// Create recoverable resource for tracking uses
	resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          refs.Features.SecondWind().ID,
		Maximum:     uses,
		CharacterID: characterID,
		ResetType:   coreResources.ResetShortRest,
	})

	return &SecondWind{
		id:          refs.Features.SecondWind().ID,
		name:        "Second Wind",
		level:       level,
		characterID: characterID,
		resource:    resource,
	}, nil
}
