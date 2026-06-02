package character

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
)

// EconomySlot identifies which action-economy slot a menu entry draws from, so
// a UI can group the action menu by slot without knowing the rules. It is the
// toolkit-authored, rulebook-native enum the game server (rpg-api) projects
// onto the wire field-for-field.
type EconomySlot string

const (
	// EconomySlotUnspecified means the slot was not set — a defect, not a value
	// a caller should reach. Present so the zero value is distinguishable.
	EconomySlotUnspecified EconomySlot = ""
	// EconomySlotAction draws from the standard action (e.g. Attack, Dodge).
	EconomySlotAction EconomySlot = "action"
	// EconomySlotBonusAction draws from the bonus action (e.g. the Monk
	// Martial Arts unarmed strike, Off-Hand Attack).
	EconomySlotBonusAction EconomySlot = "bonus_action"
	// EconomySlotReaction draws from the reaction.
	EconomySlotReaction EconomySlot = "reaction"
	// EconomySlotMovement draws from movement (e.g. the Move action).
	EconomySlotMovement EconomySlot = "movement"
	// EconomySlotFree costs no action-economy slot (a free action).
	EconomySlotFree EconomySlot = "free"
)

// economySlotForActionType maps the two-level model's primary ActionType to the
// menu EconomySlot. Reaction and Free pass through; Standard is the "action"
// slot. This is the single place the mapping lives so abilities and actions
// share it.
func economySlotForActionType(at coreCombat.ActionType) EconomySlot {
	switch at {
	case coreCombat.ActionStandard:
		return EconomySlotAction
	case coreCombat.ActionBonus:
		return EconomySlotBonusAction
	case coreCombat.ActionReaction:
		return EconomySlotReaction
	case coreCombat.ActionFree:
		return EconomySlotFree
	case coreCombat.ActionMovement:
		return EconomySlotMovement
	default:
		return EconomySlotUnspecified
	}
}

// TargetKind tells a UI what kind of target an action needs, so it raises the
// right prompt (self / single entity / position / area / none) without knowing
// the rules. It is toolkit-authored; the game server projects it field-for-field.
//
// Unspecified (zero value) means the toolkit did not set it — a defect a UI
// should treat as a bug. None means the action is deliberately untargeted (e.g.
// Dash): the UI fires it without raising a target prompt. None is therefore
// distinct from Self, which still names a target (the actor).
type TargetKind string

const (
	// TargetKindUnspecified means the kind was not set — a defect, not a value.
	TargetKindUnspecified TargetKind = ""
	// TargetKindSelf targets the actor itself (e.g. Dodge).
	TargetKindSelf TargetKind = "self"
	// TargetKindSingleEntity targets one other entity (e.g. Attack, Strike).
	TargetKindSingleEntity TargetKind = "single_entity"
	// TargetKindPosition targets a position on the map (e.g. Move).
	TargetKindPosition TargetKind = "position"
	// TargetKindArea targets an area of effect.
	TargetKindArea TargetKind = "area"
	// TargetKindNone is a deliberately untargeted action (e.g. Dash): the UI
	// fires it without a target prompt.
	TargetKindNone TargetKind = "none"
)

// AvailableAbility represents something a character can do that consumes
// primary action economy resources (action, bonus action, reaction).
type AvailableAbility struct {
	Ref             *core.Ref             // e.g. "dnd5e:combat_abilities:attack"
	Name            string                // e.g. "Attack"
	ActionType      coreCombat.ActionType // ActionStandard, ActionBonus, ActionReaction
	EconomySlot     EconomySlot           // which slot this draws from (menu grouping)
	TargetKind      TargetKind            // what target the UI must prompt for
	CanUse          bool                  // computed from current action economy state
	Reason          string                // why CanUse is false (empty when true)
	ResourceCurrent int                   // current charges (0 if no resource)
	ResourceMax     int                   // max charges (0 if no resource)
}

// AvailableAction represents something a character can do that consumes
// granted capacity (attacks remaining, movement remaining, etc.).
type AvailableAction struct {
	Ref         *core.Ref   // e.g. "dnd5e:actions:strike"
	Name        string      // e.g. "Strike"
	EconomySlot EconomySlot // which slot this draws from (menu grouping)
	TargetKind  TargetKind  // what target the UI must prompt for
	CanUse      bool        // computed from granted capacity
	Reason      string      // why CanUse is false (empty when true)
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
