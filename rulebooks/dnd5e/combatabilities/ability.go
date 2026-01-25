// Package combatabilities provides D&D 5e universal combat abilities.
// These are actions every character can take during combat: Attack, Dash, Dodge, etc.
// They consume action economy (action, bonus action, reaction) to grant capacity or effects.
//
// Abilities are distinct from Features (class/race granted) and Actions (the doing).
// The flow is: Ability/Feature consumes action economy -> grants capacity/actions/conditions.
package combatabilities

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Type is the content type for combat abilities within the dnd5e module
const Type core.Type = "combat_abilities"

// EntityTypeCombatAbility is the entity type for combat abilities
const EntityTypeCombatAbility core.EntityType = "combat_ability"

// CombatAbility represents a universal combat action that every character can take.
// Examples include Attack, Dash, Dodge, Disengage, Help, Hide, and Ready.
// These consume action economy to grant capacity, actions, or conditions.
//
// Purpose: Provides the interface for standard combat options available to all characters,
// implementing the two-level action economy where abilities grant capacity that actions consume.
type CombatAbility interface {
	core.Action[CombatAbilityInput] // CanActivate + Activate

	// Event lifecycle - combat abilities may subscribe to events
	// Apply subscribes to any events the ability needs (e.g., turn end for cleanup)
	Apply(ctx context.Context, bus events.EventBus) error
	// Remove unsubscribes from events and cleans up
	Remove(ctx context.Context, bus events.EventBus) error

	// Metadata
	// Name returns the display name of this ability (e.g., "Attack", "Dash")
	Name() string
	// Description returns a brief description of what this ability does
	Description() string
	// ActionType returns the action economy cost to use this ability
	ActionType() coreCombat.ActionType
	// Ref returns the reference identifying this ability type
	Ref() *core.Ref

	// Persistence
	// ToJSON converts the ability to JSON for persistence
	ToJSON() (json.RawMessage, error)
}

// CombatAbilityHolder is implemented by entities that can hold combat abilities.
// Characters implement this to receive and use standard combat abilities.
type CombatAbilityHolder interface {
	// AddCombatAbility adds a combat ability to the entity
	AddCombatAbility(ability CombatAbility) error

	// RemoveCombatAbility removes a combat ability by ID
	RemoveCombatAbility(abilityID string) error

	// GetCombatAbilities returns all combat abilities
	GetCombatAbilities() []CombatAbility

	// GetCombatAbility returns a specific combat ability by ID, or nil if not found
	GetCombatAbility(id string) CombatAbility
}
