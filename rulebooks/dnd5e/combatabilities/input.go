package combatabilities

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// CombatAbilityInput provides input data for combat ability activation.
// Combat abilities typically need the event bus and action economy to:
// 1. Consume action economy resources (action, bonus action, reaction)
// 2. Grant capacity (attacks, movement) via ActionEconomy
// 3. Grant actions (Strike, Move) via ActionHolder
// 4. Grant conditions (Dodging, Disengaging) via event bus
type CombatAbilityInput struct {
	// Bus is the event bus for publishing events and granting conditions.
	// Required for abilities that grant conditions or publish events.
	Bus events.EventBus `json:"-"`

	// ActionEconomy tracks action/bonus action/reaction usage and capacity.
	// Required for consuming action economy and setting capacity.
	ActionEconomy *combat.ActionEconomy `json:"-"`

	// ActionHolder is the entity that can hold granted actions.
	// Required for abilities that grant temporary actions (e.g., Strike).
	// Typically this is the Character using the ability.
	ActionHolder actions.ActionHolder `json:"-"`

	// Speed is the character's base movement speed in feet.
	// Required for abilities that add movement (e.g., Dash).
	Speed int `json:"-"`

	// ExtraAttacks is the number of additional attacks from features like Extra Attack.
	// 0 = normal (1 attack), 1 = Extra Attack (2 attacks), etc.
	// Required for the Attack ability to set correct attack capacity.
	ExtraAttacks int `json:"-"`
}
