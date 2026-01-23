// Package actions provides D&D 5e combat actions that can be activated
package actions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

	// CurrentPosition is the entity's current position (for Move action)
	CurrentPosition *spatial.Position `json:"-"`

	// Destination is the target position for movement (for Move action)
	Destination *spatial.Position `json:"-"`

	// MovementCostFt is the movement cost in feet to reach the destination
	// This should be calculated by the caller based on grid/terrain rules
	MovementCostFt int `json:"-"`
}
