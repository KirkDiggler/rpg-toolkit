// Package features provides D&D 5e class features implementation
package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
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
