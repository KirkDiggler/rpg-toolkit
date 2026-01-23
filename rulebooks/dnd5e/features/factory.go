// Package features provides D&D 5e class features implementation
package features

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
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
	case refs.Features.ActionSurge().ID:
		feature, err = createActionSurge(input.Config, input.CharacterID)
	case refs.Features.FlurryOfBlows().ID:
		feature, err = createFlurryOfBlows(input.Config, input.CharacterID)
	case refs.Features.PatientDefense().ID:
		feature, err = createPatientDefense(input.Config, input.CharacterID)
	case refs.Features.StepOfTheWind().ID:
		feature, err = createStepOfTheWind(input.Config, input.CharacterID)
	case refs.Features.DeflectMissiles().ID:
		feature, err = createDeflectMissiles(input.Config, input.CharacterID)
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
	Level int `json:"level"` // Barbarian level (optional, for calculating damage bonus)
}

// createRage creates a rage feature from config.
// Note: The rage resource (rage_charges) should be registered on the Character,
// not on the feature itself.
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

	return &Rage{
		id:    refs.Features.Rage().ID,
		name:  "Rage",
		level: level,
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

// actionSurgeConfig is the config structure for action surge feature
type actionSurgeConfig struct {
	Uses int `json:"uses"` // Number of uses (default 1)
}

// createActionSurge creates an action surge feature from config
func createActionSurge(config json.RawMessage, characterID string) (*ActionSurge, error) {
	var cfg actionSurgeConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse action surge config")
		}
	}

	// Default uses to 1 (restores on short rest)
	uses := cfg.Uses
	if uses == 0 {
		uses = 1
	}

	// Create recoverable resource for tracking uses
	resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          refs.Features.ActionSurge().ID,
		Maximum:     uses,
		CharacterID: characterID,
		ResetType:   coreResources.ResetShortRest,
	})

	return &ActionSurge{
		id:          refs.Features.ActionSurge().ID,
		name:        "Action Surge",
		characterID: characterID,
		resource:    resource,
	}, nil
}

// flurryOfBlowsConfig is the config structure for flurry of blows feature
type flurryOfBlowsConfig struct {
	// Flurry of Blows doesn't have its own uses - it consumes Ki from the character
	// No config needed, but we keep the struct for consistency
}

// createFlurryOfBlows creates a flurry of blows feature from config
func createFlurryOfBlows(config json.RawMessage, characterID string) (*FlurryOfBlows, error) {
	var cfg flurryOfBlowsConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse flurry of blows config")
		}
	}

	return &FlurryOfBlows{
		id:          refs.Features.FlurryOfBlows().ID,
		name:        "Flurry of Blows",
		characterID: characterID,
	}, nil
}

// patientDefenseConfig is the config structure for patient defense feature
type patientDefenseConfig struct {
	// Patient Defense doesn't have its own uses - it consumes Ki from the character
	// No config needed, but we keep the struct for consistency
}

// createPatientDefense creates a patient defense feature from config
func createPatientDefense(config json.RawMessage, characterID string) (*PatientDefense, error) {
	var cfg patientDefenseConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse patient defense config")
		}
	}

	return &PatientDefense{
		id:          refs.Features.PatientDefense().ID,
		name:        "Patient Defense",
		characterID: characterID,
	}, nil
}

// stepOfTheWindConfig is the config structure for step of the wind feature
type stepOfTheWindConfig struct {
	// Step of the Wind doesn't have its own uses - it consumes Ki from the character
	// No config needed, but we keep the struct for consistency
}

// createStepOfTheWind creates a step of the wind feature from config
func createStepOfTheWind(config json.RawMessage, characterID string) (*StepOfTheWind, error) {
	var cfg stepOfTheWindConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse step of the wind config")
		}
	}

	return &StepOfTheWind{
		id:          refs.Features.StepOfTheWind().ID,
		name:        "Step of the Wind",
		characterID: characterID,
	}, nil
}

// deflectMissilesConfig is the config structure for deflect missiles feature
type deflectMissilesConfig struct {
	MonkLevel   int `json:"monk_level"`   // Monk level (for damage reduction calculation)
	DexModifier int `json:"dex_modifier"` // Dexterity modifier (for damage reduction calculation)
}

// createDeflectMissiles creates a deflect missiles feature from config
func createDeflectMissiles(config json.RawMessage, characterID string) (*DeflectMissiles, error) {
	var cfg deflectMissilesConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse deflect missiles config")
		}
	}

	// Default monk level to 3 (when feature is gained) if not specified
	monkLevel := cfg.MonkLevel
	if monkLevel == 0 {
		monkLevel = 3
	}

	// Default dex modifier to +3 if not specified
	dexModifier := cfg.DexModifier
	if dexModifier == 0 {
		dexModifier = 3
	}

	return &DeflectMissiles{
		id:          refs.Features.DeflectMissiles().ID,
		name:        "Deflect Missiles",
		characterID: characterID,
		monkLevel:   monkLevel,
		dexModifier: dexModifier,
	}, nil
}
