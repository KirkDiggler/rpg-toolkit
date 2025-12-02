// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// CreateFromRefInput provides input for creating a condition from a ref
type CreateFromRefInput struct {
	// Ref is the typed condition reference (e.g., dnd5e:conditions:unarmored_defense)
	Ref *core.Ref
	// Config is condition-specific configuration as JSON
	Config json.RawMessage
	// CharacterID is the ID of the character this condition applies to
	CharacterID string
	// SourceRef is the ref of what granted this condition
	// e.g., dnd5e:classes:barbarian for class-granted conditions
	// e.g., dnd5e:features:rage for feature-activated conditions
	SourceRef *core.Ref
}

// CreateFromRefOutput provides the result of creating a condition from a ref
type CreateFromRefOutput struct {
	// Condition is the created condition
	Condition dnd5eEvents.ConditionBehavior
}

// CreateFromRef creates a condition from a ref and configuration.
// The ref determines which condition type to create, and
// the config is parsed by each condition's specific factory logic.
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
	if input.Ref.Type != "conditions" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unsupported type: %s (expected 'conditions')", input.Ref.Type)
	}

	// Create the condition based on the ID
	var condition dnd5eEvents.ConditionBehavior
	var err error

	switch input.Ref.ID {
	case UnarmoredDefenseID:
		condition, err = createUnarmoredDefense(input.Config, input.CharacterID, input.SourceRef)
	case RagingID:
		condition, err = createRaging(input.Config, input.CharacterID, input.SourceRef)
	case BrutalCriticalID:
		condition, err = createBrutalCritical(input.Config, input.CharacterID)
	case FightingStyleID:
		condition, err = createFightingStyle(input.Config, input.CharacterID)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown condition: %s", input.Ref.ID)
	}

	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to create condition: %s", input.Ref.ID)
	}

	return &CreateFromRefOutput{Condition: condition}, nil
}

// unarmoredDefenseConfig is the config structure for unarmored defense
type unarmoredDefenseConfig struct {
	Variant string `json:"variant"` // "barbarian" or "monk"
}

// createUnarmoredDefense creates an unarmored defense condition from config
func createUnarmoredDefense(config json.RawMessage, characterID string, sourceRef *core.Ref) (*UnarmoredDefenseCondition, error) {
	var cfg unarmoredDefenseConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse unarmored defense config")
		}
	}

	// Default to barbarian variant if not specified
	variant := UnarmoredDefenseBarbarian
	if cfg.Variant == "monk" {
		variant = UnarmoredDefenseMonk
	}

	// Use provided source ref, or default based on variant
	source := sourceRef
	if source == nil {
		if variant == UnarmoredDefenseBarbarian {
			source = &core.Ref{Module: "dnd5e", Type: "classes", ID: "barbarian"}
		} else {
			source = &core.Ref{Module: "dnd5e", Type: "classes", ID: "monk"}
		}
	}

	return NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: characterID,
		Type:        variant,
		Source:      source,
	}), nil
}

// ragingConfig is the config structure for raging condition
type ragingConfig struct {
	DamageBonus int `json:"damage_bonus"`
	Level       int `json:"level"`
}

// createRaging creates a raging condition from config
func createRaging(config json.RawMessage, characterID string, sourceRef *core.Ref) (*RagingCondition, error) {
	var cfg ragingConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse raging config")
		}
	}

	// Default damage bonus to 2 if not specified
	damageBonus := cfg.DamageBonus
	if damageBonus == 0 {
		damageBonus = 2
	}

	// Use provided source ref, or default to rage feature ref
	source := sourceRef
	if source == nil {
		source = &core.Ref{Module: "dnd5e", Type: "features", ID: "rage"}
	}

	return &RagingCondition{
		CharacterID: characterID,
		DamageBonus: damageBonus,
		Level:       cfg.Level,
		Source:      source,
	}, nil
}

// brutalCriticalConfig is the config structure for brutal critical
type brutalCriticalConfig struct {
	Level int `json:"level"` // Barbarian level (9+ for 1 die, 13+ for 2, 17+ for 3)
}

// createBrutalCritical creates a brutal critical condition from config
func createBrutalCritical(config json.RawMessage, characterID string) (*BrutalCriticalCondition, error) {
	var cfg brutalCriticalConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse brutal critical config")
		}
	}

	// Level determines extra dice via calculateExtraDice in the constructor
	// Default to level 9 if not specified (minimum level for brutal critical)
	level := cfg.Level
	if level == 0 {
		level = 9
	}

	return NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: characterID,
		Level:       level,
	}), nil
}

// fightingStyleConfig is the config structure for fighting style
type fightingStyleConfig struct {
	Style string `json:"style"`
}

// createFightingStyle creates a fighting style condition from config
func createFightingStyle(config json.RawMessage, characterID string) (*FightingStyleCondition, error) {
	var cfg fightingStyleConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse fighting style config")
		}
	}

	if cfg.Style == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "fighting style config requires 'style' field")
	}

	// FightingStyle is a string type alias, so we can assign directly
	return NewFightingStyleCondition(FightingStyleConditionConfig{
		CharacterID: characterID,
		Style:       cfg.Style,
	}), nil
}
