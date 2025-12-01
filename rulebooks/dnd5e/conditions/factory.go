// Package conditions provides D&D 5e condition types and effects
package conditions

import (
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
)

// CreateFromRefInput provides input for creating a condition from a reference
type CreateFromRefInput struct {
	Ref         string          // e.g., "dnd5e:conditions:unarmored_defense"
	Config      json.RawMessage // Type-specific configuration
	CharacterID string          // The character this condition applies to
	SourceRef   string          // Where this condition came from (e.g., "dnd5e:classes:barbarian")
}

// CreateFromRefOutput is the result of creating a condition from a reference
type CreateFromRefOutput struct {
	Condition dnd5eEvents.ConditionBehavior
}

// CreateFromRef creates a condition instance from a reference string.
// The reference format is "dnd5e:conditions:<condition_id>".
func CreateFromRef(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input is required")
	}

	// Parse the ref
	parts := strings.Split(input.Ref, ":")
	if len(parts) != 3 {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid condition ref format: %s", input.Ref)
	}

	if parts[0] != "dnd5e" || parts[1] != "conditions" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid condition ref: %s", input.Ref)
	}

	conditionID := parts[2]

	switch conditionID {
	case UnarmoredDefenseID:
		return createUnarmoredDefense(input)
	case RagingID:
		return createRaging(input)
	case BrutalCriticalID:
		return createBrutalCritical(input)
	case FightingStyleID:
		return createFightingStyle(input)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "unknown condition: %s", conditionID)
	}
}

// createUnarmoredDefense creates an unarmored defense condition
func createUnarmoredDefense(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	// Parse config to get the type (barbarian or monk)
	var config struct {
		Type string `json:"type"`
	}

	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &config); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to parse unarmored defense config")
		}
	}

	var udType UnarmoredDefenseType
	switch config.Type {
	case "barbarian":
		udType = UnarmoredDefenseBarbarian
	case "monk":
		udType = UnarmoredDefenseMonk
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unarmored defense requires type (barbarian or monk), got: %s", config.Type)
	}

	condition := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: input.CharacterID,
		Type:        udType,
		Source:      input.SourceRef,
	})

	return &CreateFromRefOutput{Condition: condition}, nil
}

// createRaging creates a raging condition
func createRaging(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	// Parse config for level and damage bonus
	var config struct {
		Level       int `json:"level"`
		DamageBonus int `json:"damage_bonus"`
	}

	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &config); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to parse raging config")
		}
	}

	// Default to level 1 if not specified
	level := config.Level
	if level == 0 {
		level = 1
	}

	// Calculate damage bonus based on level if not specified
	damageBonus := config.DamageBonus
	if damageBonus == 0 {
		switch {
		case level < 9:
			damageBonus = 2
		case level < 16:
			damageBonus = 3
		default:
			damageBonus = 4
		}
	}

	condition := &RagingCondition{
		CharacterID: input.CharacterID,
		DamageBonus: damageBonus,
		Level:       level,
		Source:      input.SourceRef,
	}

	return &CreateFromRefOutput{Condition: condition}, nil
}

// createBrutalCritical creates a brutal critical condition
func createBrutalCritical(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	// Parse config for level
	var config struct {
		Level int `json:"level"`
	}

	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &config); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to parse brutal critical config")
		}
	}

	// Default to level 9 (when feature is gained) if not specified
	level := config.Level
	if level == 0 {
		level = 9
	}

	condition := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: input.CharacterID,
		Level:       level,
	})

	return &CreateFromRefOutput{Condition: condition}, nil
}

// createFightingStyle creates a fighting style condition
func createFightingStyle(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	// Parse config for style
	var config struct {
		Style string `json:"style"`
	}

	if len(input.Config) > 0 {
		if err := json.Unmarshal(input.Config, &config); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to parse fighting style config")
		}
	}

	if config.Style == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "fighting style config requires style field")
	}

	style := config.Style

	// Validate the fighting style is implemented
	if !fightingstyles.IsImplemented(style) {
		return nil, rpgerr.Newf(rpgerr.CodeNotAllowed,
			"fighting style %s is not yet implemented", style)
	}

	condition := NewFightingStyleCondition(FightingStyleConditionConfig{
		CharacterID: input.CharacterID,
		Style:       style,
	})

	return &CreateFromRefOutput{Condition: condition}, nil
}
