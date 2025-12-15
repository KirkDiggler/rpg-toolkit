// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// CreateFromRefInput provides input for creating a condition from a ref string
type CreateFromRefInput struct {
	// Ref is the condition reference in "module:type:value" format
	// e.g., "dnd5e:conditions:unarmored_defense"
	Ref string
	// Config is condition-specific configuration as JSON
	Config json.RawMessage
	// CharacterID is the ID of the character this condition applies to
	CharacterID string
	// SourceRef is the ref of what granted this condition in "module:type:value" format
	// e.g., "dnd5e:classes:barbarian" for class-granted conditions
	// e.g., "dnd5e:features:rage" for feature-activated conditions
	SourceRef string
}

// CreateFromRefOutput provides the result of creating a condition from a ref
type CreateFromRefOutput struct {
	// Condition is the created condition
	Condition dnd5eEvents.ConditionBehavior
}

// CreateFromRef creates a condition from a ref string and configuration.
// The ref is parsed to determine which condition type to create, and
// the config is parsed by each condition's specific factory logic.
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

	// Parse the ref to get the condition type
	ref, err := core.ParseString(input.Ref)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse ref: %s", input.Ref)
	}

	// Validate module and type
	if ref.Module != refs.Module {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unsupported module: %s", ref.Module)
	}
	if ref.Type != refs.TypeConditions {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unsupported type: %s (expected '%s')", ref.Type, refs.TypeConditions)
	}

	// Create the condition based on the ID
	var condition dnd5eEvents.ConditionBehavior

	switch ref.ID {
	case refs.Conditions.UnarmoredDefense().ID:
		condition, err = createUnarmoredDefense(input.Config, input.CharacterID, input.SourceRef)
	case refs.Conditions.Raging().ID:
		condition, err = createRaging(input.Config, input.CharacterID, input.SourceRef)
	case refs.Conditions.BrutalCritical().ID:
		condition, err = createBrutalCritical(input.Config, input.CharacterID)
	case refs.Conditions.FightingStyle().ID:
		condition, err = createFightingStyle(input.Config, input.CharacterID)
	case refs.Conditions.ImprovedCritical().ID:
		condition, err = createImprovedCritical(input.Config, input.CharacterID)
	case refs.Conditions.MartialArts().ID:
		condition, err = createMartialArts(input.Config, input.CharacterID)
	case refs.Conditions.UnarmoredMovement().ID:
		condition, err = createUnarmoredMovement(input.Config, input.CharacterID)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown condition: %s", ref.ID)
	}

	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to create condition: %s", ref.ID)
	}

	return &CreateFromRefOutput{Condition: condition}, nil
}

// unarmoredDefenseConfig is the config structure for unarmored defense
type unarmoredDefenseConfig struct {
	Variant string `json:"variant"` // "barbarian" or "monk"
}

// createUnarmoredDefense creates an unarmored defense condition from config
func createUnarmoredDefense(config json.RawMessage, characterID, sourceRef string) (*UnarmoredDefenseCondition, error) {
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

	return NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: characterID,
		Type:        variant,
		Source:      sourceRef,
	}), nil
}

// ragingConfig is the config structure for raging condition
type ragingConfig struct {
	DamageBonus int `json:"damage_bonus"`
	Level       int `json:"level"`
}

// createRaging creates a raging condition from config
func createRaging(config json.RawMessage, characterID, sourceRef string) (*RagingCondition, error) {
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

	// Default to rage feature ref if not specified
	source := sourceRef
	if source == "" {
		source = refs.Features.Rage().String()
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

// improvedCriticalConfig is the config structure for improved critical
type improvedCriticalConfig struct {
	Threshold int `json:"threshold"` // Critical threshold (default 19)
}

// createImprovedCritical creates an improved critical condition from config
func createImprovedCritical(config json.RawMessage, characterID string) (*ImprovedCriticalCondition, error) {
	var cfg improvedCriticalConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse improved critical config")
		}
	}

	// Default to 19 if not specified
	threshold := cfg.Threshold
	if threshold == 0 {
		threshold = 19
	}

	return NewImprovedCriticalCondition(ImprovedCriticalInput{
		CharacterID: characterID,
		Threshold:   threshold,
	}), nil
}

// martialArtsConfig is the config structure for martial arts
type martialArtsConfig struct {
	MonkLevel int `json:"monk_level"`
}

// createMartialArts creates a martial arts condition from config
func createMartialArts(config json.RawMessage, characterID string) (*MartialArtsCondition, error) {
	var cfg martialArtsConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse martial arts config")
		}
	}

	// Monk level is required
	if cfg.MonkLevel == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "martial arts config requires 'monk_level' field")
	}

	return NewMartialArtsCondition(MartialArtsInput{
		CharacterID: characterID,
		MonkLevel:   cfg.MonkLevel,
	}), nil
}

// unarmoredMovementConfig is the config structure for unarmored movement
type unarmoredMovementConfig struct {
	MonkLevel int `json:"monk_level"`
}

// createUnarmoredMovement creates an unarmored movement condition from config
func createUnarmoredMovement(config json.RawMessage, characterID string) (*UnarmoredMovementCondition, error) {
	var cfg unarmoredMovementConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse unarmored movement config")
		}
	}

	// Monk level is required
	if cfg.MonkLevel == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "unarmored movement config requires 'monk_level' field")
	}

	return NewUnarmoredMovementCondition(UnarmoredMovementInput{
		CharacterID: characterID,
		MonkLevel:   cfg.MonkLevel,
	}), nil
}
