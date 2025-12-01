// Package features provides D&D 5e class features implementation
package features

import (
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// CreateFromRefInput provides input for creating a feature from a reference
type CreateFromRefInput struct {
	Ref    string          // e.g., "dnd5e:features:rage"
	Config json.RawMessage // Type-specific configuration
	Level  int             // Character level (affects feature scaling)
}

// CreateFromRefOutput is the result of creating a feature from a reference
type CreateFromRefOutput struct {
	Feature Feature
}

// CreateFromRef creates a feature instance from a reference string.
// The reference format is "dnd5e:features:<feature_id>".
func CreateFromRef(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input is required")
	}

	// Parse the ref
	parts := strings.Split(input.Ref, ":")
	if len(parts) != 3 {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid feature ref format: %s", input.Ref)
	}

	if parts[0] != "dnd5e" || parts[1] != "features" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid feature ref: %s", input.Ref)
	}

	featureID := parts[2]

	// Default level to 1 if not specified
	level := input.Level
	if level == 0 {
		level = 1
	}

	switch featureID {
	case RageID:
		return createRage(level)
	case SecondWindID:
		return createSecondWind(level)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "unknown feature: %s", featureID)
	}
}

// createRage creates a rage feature for a barbarian
func createRage(level int) (*CreateFromRefOutput, error) {
	maxUses := calculateRageUses(level)

	rage := &Rage{
		id:       "rage",
		name:     "Rage",
		level:    level,
		resource: resources.NewResource("rage", maxUses),
	}

	return &CreateFromRefOutput{Feature: rage}, nil
}

// createSecondWind creates a second wind feature for a fighter
func createSecondWind(level int) (*CreateFromRefOutput, error) {
	secondWind := &SecondWind{
		id:       "second_wind",
		name:     "Second Wind",
		level:    level,
		resource: resources.NewResource("second_wind", 1), // 1 use per short rest
	}

	return &CreateFromRefOutput{Feature: secondWind}, nil
}

// CreateRageFromJSON is a helper that creates a rage feature from JSON data
// This is used by the loader for deserializing stored features
func CreateRageFromJSON(data json.RawMessage) (Feature, error) {
	var rageData RageData
	if err := json.Unmarshal(data, &rageData); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to unmarshal rage data")
	}

	rage := &Rage{
		id:       rageData.ID,
		name:     rageData.Name,
		level:    rageData.Level,
		resource: resources.NewResource("rage", rageData.MaxUses),
	}
	rage.resource.SetCurrent(rageData.Uses)

	return rage, nil
}

// CreateSecondWindFromJSON is a helper that creates a second wind feature from JSON data
func CreateSecondWindFromJSON(data json.RawMessage) (Feature, error) {
	var swData SecondWindData
	if err := json.Unmarshal(data, &swData); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to unmarshal second wind data")
	}

	secondWind := &SecondWind{
		id:       swData.ID,
		name:     swData.Name,
		level:    swData.Level,
		resource: resources.NewResource("second_wind", swData.MaxUses),
	}
	secondWind.resource.SetCurrent(swData.Uses)

	return secondWind, nil
}

// GetFeatureRef returns the core.Ref for a feature type
func GetFeatureRef(featureID string) core.Ref {
	return core.Ref{
		Module: "dnd5e",
		Type:   "features",
		ID:     featureID,
	}
}
