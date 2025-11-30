// Package features provides D&D 5e class features implementation
package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// Type is the content type for features in refs (e.g., "dnd5e:features:rage")
const Type core.Type = "features"

// Feature IDs - these are the known feature identifiers
const (
	RageID       core.ID = "rage"
	SecondWindID core.ID = "second_wind"
)

// Grant represents a feature grant from a class, race, or background.
// Grant = what you get (the feature ID to be granted)
type Grant struct {
	ID core.ID
}

// FeatureInput provides input data for feature activation.
// Most features need no input (rage, second wind, etc).
// Some features will need choices (wild shape forms, channel divinity options).
type FeatureInput struct {
	// Bus is provided by the character/owner during activation
	Bus events.EventBus `json:"-"`

	// Possible Future: Choice string `json:"choice,omitempty"` for features with options
}

// NewInput provides parameters for creating a new feature instance
type NewInput struct {
	OwnerID string // Character or entity that owns this feature
	Level   int    // Class level for scaling features
}

// New creates a new feature instance based on the grant ID.
// Returns nil if the feature ID is not recognized (fail loudly pattern).
func New(grant Grant, input NewInput) Feature {
	switch grant.ID {
	case RageID:
		maxUses := calculateRageUses(input.Level)
		return &Rage{
			id:       input.OwnerID + "_rage",
			name:     "Rage",
			level:    input.Level,
			resource: resources.NewResource("rage", maxUses),
		}
	case SecondWindID:
		return &SecondWind{
			id:       input.OwnerID + "_second_wind",
			name:     "Second Wind",
			level:    input.Level,
			resource: resources.NewResource("second_wind", 1),
		}
	default:
		return nil // Unmigrated feature - fail loudly
	}
}
