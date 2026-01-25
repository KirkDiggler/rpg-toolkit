package actions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Action represents something a character can do (attack, dash, flurry strike).
// Actions can be permanent (always available) or temporary (granted by features).
// Unlike features which grant actions/conditions, actions DO things.
type Action interface {
	core.Action[ActionInput] // CanActivate + Activate

	// Event lifecycle - actions subscribe to events for self-removal
	Apply(ctx context.Context, bus events.EventBus) error
	Remove(ctx context.Context, bus events.EventBus) error

	// Lifecycle metadata
	IsTemporary() bool
	UsesRemaining() int // UnlimitedUses (-1) for permanent actions

	// Persistence
	ToJSON() (json.RawMessage, error)

	// Action economy cost
	ActionType() coreCombat.ActionType

	// CapacityType returns what capacity this action consumes (attacks, movement, etc.)
	CapacityType() combat.CapacityType
}

// ActionHolder is implemented by entities that can hold actions.
// Features type-assert to this interface to grant temporary actions.
type ActionHolder interface {
	// AddAction adds an action to the entity's available actions
	AddAction(action Action) error

	// RemoveAction removes an action by ID
	RemoveAction(actionID string) error

	// GetActions returns all available actions
	GetActions() []Action

	// GetAction returns a specific action by ID, or nil if not found
	GetAction(id string) Action
}
