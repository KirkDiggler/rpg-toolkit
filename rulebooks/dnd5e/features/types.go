// Package features provides D&D 5e class features implementation
package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Type is the content type for features within the dnd5e module
// NOTE: Prefer using refs.TypeFeatures for consistency with the refs package
const Type core.Type = "features"

// EntityTypeFeature is the entity type for features
const EntityTypeFeature core.EntityType = "feature"

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

	// ActionEconomy is provided for features that grant extra actions (e.g., Action Surge)
	ActionEconomy *combat.ActionEconomy `json:"-"`

	// Action is provided for features with action choices (e.g., Step of the Wind: "disengage" or "dash")
	Action string `json:"action,omitempty"`

	// Roller is the interaction-scoped dice roller for features that roll
	// (e.g., Second Wind's healing d10). Nil retains the production default
	// roller; resolution supplies the interaction's roller so exact traces
	// never depend on process-global randomness.
	Roller dice.Roller `json:"-"`
}
