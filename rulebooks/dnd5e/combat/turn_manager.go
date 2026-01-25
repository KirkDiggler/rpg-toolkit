// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

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

// ActionInfo provides metadata about an available action.
type ActionInfo struct {
	// ID is the unique identifier of this action instance.
	ID string

	// ActionType is the action economy cost to use this action.
	ActionType coreCombat.ActionType

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
type TurnManager struct {
	character      CombatCharacter
	economy        *ActionEconomy
	ctx            context.Context
	bus            events.EventBus
	roller         dice.Roller
	mainHandWeapon *EquippedWeaponInfo
	offHandWeapon  *EquippedWeaponInfo
	turnStarted    bool
}

// NewTurnManager creates a TurnManager for managing a combatant's turn.
// The context is pre-built with CombatantLookup and Room for use throughout the turn.
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

	// Pre-build context with combatant lookup and room
	ctx := context.Background()
	ctx = WithCombatantLookup(ctx, input.Combatants)
	ctx = WithRoom(ctx, input.Room)

	return &TurnManager{
		character:      input.Character,
		economy:        NewActionEconomy(),
		ctx:            ctx,
		bus:            input.EventBus,
		roller:         roller,
		mainHandWeapon: input.MainHandWeapon,
		offHandWeapon:  input.OffHandWeapon,
	}, nil
}

// StartTurn initializes the action economy and publishes a TurnStartEvent.
// Must be called before any other turn actions.
func (tm *TurnManager) StartTurn(ctx context.Context) (*StartTurnResult, error) {
	if tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already started")
	}

	tm.economy.SetMovement(tm.character.GetSpeed())
	tm.turnStarted = true

	// Publish turn start event
	topic := dnd5eEvents.TurnStartTopic.On(tm.bus)
	_ = topic.Publish(ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: tm.character.GetID(),
	})

	return &StartTurnResult{
		Economy: tm.economy,
	}, nil
}

// EndTurn publishes a TurnEndEvent and cleans up temporary actions/conditions.
// After calling EndTurn, the TurnManager should not be reused.
func (tm *TurnManager) EndTurn(ctx context.Context) (*EndTurnResult, error) {
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}

	// Publish turn end event
	topic := dnd5eEvents.TurnEndTopic.On(tm.bus)
	_ = topic.Publish(ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: tm.character.GetID(),
	})

	// Cleanup temporary actions/conditions
	if err := tm.character.Cleanup(ctx); err != nil {
		return nil, err
	}

	tm.turnStarted = false

	return &EndTurnResult{
		CharacterID: tm.character.GetID(),
	}, nil
}
