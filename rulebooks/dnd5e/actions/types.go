// Package actions provides D&D 5e combat actions that can be activated
package actions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Type is the content type for actions within the dnd5e module
const Type core.Type = "actions"

// EntityTypeAction is the entity type for actions
const EntityTypeAction core.EntityType = "action"

// UnlimitedUses indicates an action can be used unlimited times
const UnlimitedUses = -1

// ActionInput provides input data for action activation.
// Actions typically need a target (unlike most features).
type ActionInput struct {
	// Bus is provided by the character/owner during activation
	Bus events.EventBus `json:"-"`

	// ActionEconomy tracks action/bonus action/reaction usage
	ActionEconomy *combat.ActionEconomy `json:"-"`

	// Target is the entity being targeted by this action
	Target core.Entity `json:"-"`

	// TODO: Add Position for AoE targeting when needed
	// Position *spatial.Position `json:"-"`
}
