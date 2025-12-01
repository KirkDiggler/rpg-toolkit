// Package features provides D&D 5e class features implementation
package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Type is the content type for features within the dnd5e module
const Type core.Type = "features"

// Feature ID constants for type-safe references
const (
	RageID       core.ID = "rage"
	SecondWindID core.ID = "second_wind"
)

// Grant represents a feature granted to a character (e.g., from a class level)
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
