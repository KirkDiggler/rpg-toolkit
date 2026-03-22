package character

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
)

// AvailableAbility represents something a character can do that consumes
// primary action economy resources (action, bonus action, reaction).
type AvailableAbility struct {
	Ref             *core.Ref             // e.g. "dnd5e:combat_abilities:attack"
	Name            string                // e.g. "Attack"
	ActionType      coreCombat.ActionType // ActionStandard, ActionBonus, ActionReaction
	CanUse          bool                  // computed from current action economy state
	Reason          string                // why CanUse is false (empty when true)
	ResourceCurrent int                   // current charges (0 if no resource)
	ResourceMax     int                   // max charges (0 if no resource)
}

// AvailableAction represents something a character can do that consumes
// granted capacity (attacks remaining, movement remaining, etc.).
type AvailableAction struct {
	Ref    *core.Ref // e.g. "dnd5e:actions:strike"
	Name   string    // e.g. "Strike"
	CanUse bool      // computed from granted capacity
	Reason string    // why CanUse is false (empty when true)
}

// StartTurnInput provides input for initializing the action economy at turn start.
type StartTurnInput struct {
	Speed      int // character's movement speed (30ft default)
	TurnNumber int // current turn number (used to detect stale state)
}

// StartTurnOutput contains the initialized action economy state.
type StartTurnOutput struct {
	Abilities []AvailableAbility
	Actions   []AvailableAction
}

// ActivateAbilityInput provides input for activating a combat ability or feature.
type ActivateAbilityInput struct {
	AbilityRef *core.Ref // which ability to activate
}

// ActivateAbilityOutput contains the result of activating an ability.
type ActivateAbilityOutput struct {
	Success         bool
	Error           string
	GrantedCapacity string // e.g. "1 attack", "30ft movement"
	Abilities       []AvailableAbility
	Actions         []AvailableAction
}

// ExecuteActionInput provides input for executing an action that consumes capacity.
type ExecuteActionInput struct {
	ActionRef *core.Ref // which action to execute
	TargetID  string    // target entity ID (for strikes)
}

// ExecuteActionOutput contains the result of executing an action.
// Note: actual attack resolution (damage, hit/miss) happens in the API
// orchestrator. The Character only manages action economy feasibility.
type ExecuteActionOutput struct {
	Success   bool
	Error     string
	Abilities []AvailableAbility
	Actions   []AvailableAction
}

// EndTurnInput provides input for ending a turn.
type EndTurnInput struct{}

// EndTurnOutput contains the result of ending a turn.
type EndTurnOutput struct{}

// ExitCombatInput provides input for exiting combat entirely.
type ExitCombatInput struct{}

// ExitCombatOutput contains the result of exiting combat.
type ExitCombatOutput struct{}

// GrantedActionKey identifies a type of granted capacity in the action economy.
type GrantedActionKey string

const (
	// GrantedAttacks tracks the number of attacks remaining (from Attack ability).
	GrantedAttacks GrantedActionKey = "attacks"

	// GrantedOffHandStrikes tracks off-hand strikes remaining (from two-weapon fighting).
	GrantedOffHandStrikes GrantedActionKey = "off_hand_strikes"

	// GrantedFlurryStrikes tracks flurry strikes remaining (from Flurry of Blows).
	GrantedFlurryStrikes GrantedActionKey = "flurry_strikes"

	// GrantedMartialArtsBonus tracks martial arts bonus strike (from Martial Arts).
	GrantedMartialArtsBonus GrantedActionKey = "martial_arts_bonus"

	// GrantedOffHandAttack tracks off-hand attack ability availability.
	GrantedOffHandAttack GrantedActionKey = "off_hand_attack"
)

// ActionEconomyData is the serializable form of the action economy state.
// Lives on Character.Data.ActionEconomy (nil outside combat, omitempty).
type ActionEconomyData struct {
	TurnNumber            int                      `json:"turn_number"`
	ActionsRemaining      int                      `json:"actions_remaining"`
	BonusActionsRemaining int                      `json:"bonus_actions_remaining"`
	ReactionsRemaining    int                      `json:"reactions_remaining"`
	MovementRemaining     int                      `json:"movement_remaining"`
	Granted               map[GrantedActionKey]int `json:"granted,omitempty"`
}
