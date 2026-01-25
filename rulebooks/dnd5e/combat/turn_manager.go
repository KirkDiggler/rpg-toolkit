// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ActivateAbilityInput provides input for activating a combat ability via CombatCharacter.
// This is defined in the combat package to avoid import cycles with combatabilities.
type ActivateAbilityInput struct {
	// AbilityRef identifies which combat ability to activate.
	AbilityRef *core.Ref

	// Bus is the event bus for publishing events and granting conditions.
	Bus events.EventBus

	// Economy is the action economy for consuming and setting capacity.
	Economy *ActionEconomy

	// Speed is the character's base movement speed in feet.
	Speed int

	// ExtraAttacks is the number of additional attacks from features like Extra Attack.
	ExtraAttacks int
}

// AbilityInfo provides metadata about an available combat ability.
type AbilityInfo struct {
	// Ref is the reference identifying this ability.
	Ref *core.Ref

	// Name is the display name of the ability.
	Name string

	// ActionType is the action economy cost to use this ability.
	ActionType coreCombat.ActionType
}

// CapacityType identifies what capacity an action consumes.
type CapacityType string

// CapacityType constants for different action capacity requirements.
const (
	// CapacityNone means the action has no capacity requirement.
	CapacityNone CapacityType = ""

	// CapacityAttack means the action consumes one attack from AttacksRemaining.
	CapacityAttack CapacityType = "attack"

	// CapacityMovement means the action consumes movement from MovementRemaining.
	CapacityMovement CapacityType = "movement"

	// CapacityOffHandAttack means the action consumes one off-hand attack.
	CapacityOffHandAttack CapacityType = "off_hand_attack"

	// CapacityFlurryStrike means the action consumes one flurry strike.
	CapacityFlurryStrike CapacityType = "flurry_strike"
)

// ActionInfo provides metadata about an available action.
type ActionInfo struct {
	// ID is the unique identifier of this action instance.
	ID string

	// ActionType is the action economy cost to use this action.
	ActionType coreCombat.ActionType

	// CapacityType indicates what capacity this action consumes when used.
	// For example, Strike consumes attack capacity, Move consumes movement capacity.
	CapacityType CapacityType

	// IsTemporary indicates if this action was granted for the current turn only.
	IsTemporary bool
}

// CombatCharacter combines the interfaces needed for turn orchestration.
// Character already satisfies this interface.
type CombatCharacter interface {
	Combatant
	GetSpeed() int
	GetExtraAttacksCount() int
	ActivateCombatAbility(ctx context.Context, input *ActivateAbilityInput) error
	GetAbilityInfos() []AbilityInfo
	GetActionInfos() []ActionInfo
	Cleanup(ctx context.Context) error
}

// NewTurnManagerInput provides configuration for creating a TurnManager.
type NewTurnManagerInput struct {
	// Character is the combatant whose turn is being managed.
	Character CombatCharacter

	// Combatants provides lookup of all combatants for attack resolution.
	Combatants CombatantLookup

	// Room is the spatial room for movement and threat detection.
	Room spatial.Room

	// EventBus is used for publishing turn lifecycle and combat events.
	EventBus events.EventBus

	// Roller is the dice roller for attack and damage rolls.
	// If nil, a default roller is used.
	Roller dice.Roller

	// MainHandWeapon provides main-hand weapon info for two-weapon fighting validation.
	MainHandWeapon *EquippedWeaponInfo

	// OffHandWeapon provides off-hand weapon info for two-weapon fighting validation.
	OffHandWeapon *EquippedWeaponInfo
}

// StartTurnResult contains the outcome of starting a turn.
type StartTurnResult struct {
	// Economy is the action economy state after turn start.
	Economy *ActionEconomy
}

// EndTurnResult contains the outcome of ending a turn.
type EndTurnResult struct {
	// CharacterID is the ID of the character whose turn ended.
	CharacterID string
}

// TurnManager orchestrates a single combatant's turn in combat.
// It manages the action economy, delegates to ResolveAttack and MoveEntity,
// and publishes events for multiplayer broadcasting.
// After EndTurn is called, the TurnManager must not be reused.
type TurnManager struct {
	character      CombatCharacter
	economy        *ActionEconomy
	combatants     CombatantLookup
	room           spatial.Room
	bus            events.EventBus
	roller         dice.Roller
	mainHandWeapon *EquippedWeaponInfo
	offHandWeapon  *EquippedWeaponInfo
	turnStarted    bool
	turnEnded      bool
}

// NewTurnManager creates a TurnManager for managing a combatant's turn.
// Dependencies are stored and used to build context per-call.
func NewTurnManager(input *NewTurnManagerInput) (*TurnManager, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "NewTurnManagerInput is nil")
	}
	if input.Character == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Character is required")
	}
	if input.Combatants == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Combatants is required")
	}
	if input.Room == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Room is required")
	}
	if input.EventBus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "EventBus is required")
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	return &TurnManager{
		character:      input.Character,
		economy:        NewActionEconomy(),
		combatants:     input.Combatants,
		room:           input.Room,
		bus:            input.EventBus,
		roller:         roller,
		mainHandWeapon: input.MainHandWeapon,
		offHandWeapon:  input.OffHandWeapon,
	}, nil
}

// buildContext creates an operation context with combat dependencies.
// Wraps the caller's context with CombatantLookup, Room, and TwoWeaponContext.
func (tm *TurnManager) buildContext(ctx context.Context) context.Context {
	ctx = WithCombatantLookup(ctx, tm.combatants)
	ctx = WithRoom(ctx, tm.room)
	ctx = WithTwoWeaponContext(ctx, &turnManagerTwoWeaponContext{
		characterID:    tm.character.GetID(),
		mainHandWeapon: tm.mainHandWeapon,
		offHandWeapon:  tm.offHandWeapon,
		economy:        tm.economy,
	})
	return ctx
}

// turnManagerTwoWeaponContext implements TwoWeaponContext for a single character's turn.
type turnManagerTwoWeaponContext struct {
	characterID    string
	mainHandWeapon *EquippedWeaponInfo
	offHandWeapon  *EquippedWeaponInfo
	economy        *ActionEconomy
}

func (t *turnManagerTwoWeaponContext) GetMainHandWeapon(characterID string) *EquippedWeaponInfo {
	if characterID != t.characterID {
		return nil
	}
	return t.mainHandWeapon
}

func (t *turnManagerTwoWeaponContext) GetOffHandWeapon(characterID string) *EquippedWeaponInfo {
	if characterID != t.characterID {
		return nil
	}
	return t.offHandWeapon
}

func (t *turnManagerTwoWeaponContext) GetActionEconomy(characterID string) *ActionEconomy {
	if characterID != t.characterID {
		return nil
	}
	return t.economy
}

// StartTurn initializes the action economy and publishes a TurnStartEvent.
// Must be called before any other turn actions.
func (tm *TurnManager) StartTurn(ctx context.Context) (*StartTurnResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn manager cannot be reused after EndTurn")
	}
	if tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already started")
	}

	tm.economy.SetMovement(tm.character.GetSpeed())
	tm.turnStarted = true

	// Publish turn start event
	topic := dnd5eEvents.TurnStartTopic.On(tm.bus)
	if err := topic.Publish(ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: tm.character.GetID(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish turn start event: %w", err)
	}

	return &StartTurnResult{
		Economy: tm.economy,
	}, nil
}

// EndTurn publishes a TurnEndEvent and cleans up temporary actions/conditions.
// After calling EndTurn, the TurnManager must not be reused.
func (tm *TurnManager) EndTurn(ctx context.Context) (*EndTurnResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already ended")
	}
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}

	// Publish turn end event
	topic := dnd5eEvents.TurnEndTopic.On(tm.bus)
	if err := topic.Publish(ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: tm.character.GetID(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish turn end event: %w", err)
	}

	// Cleanup temporary actions/conditions
	if err := tm.character.Cleanup(ctx); err != nil {
		return nil, err
	}

	tm.turnEnded = true

	return &EndTurnResult{
		CharacterID: tm.character.GetID(),
	}, nil
}
