// Package features provides D&D 5e class features implementation
package features

import "github.com/KirkDiggler/rpg-toolkit/events"

// FeatureInput provides input data for feature activation.
// Most features need no input (rage, second wind, etc).
// Some features will need choices (wild shape forms, channel divinity options).
type FeatureInput struct {
	// Bus is provided by the character/owner during activation
	Bus events.EventBus `json:"-"`

	// Possible Future: Choice string `json:"choice,omitempty"` for features with options
}
