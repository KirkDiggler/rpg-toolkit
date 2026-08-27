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

// CombatCharacter combines the interfaces needed for turn orchestration.
// Character already satisfies this interface.
type CombatCharacter interface {
	Combatant
	GetSpeed() int
	GetExtraAttacksCount() int
	ActivateCombatAbility(ctx context.Context, input *ActivateAbilityInput) error
	GetAbilityInfos() []AbilityInfo
	Cleanup(ctx context.Context) error
}

// NewTurnManagerInput provides configuration for creating a TurnManager.
type NewTurnManagerInput struct {
	// Character is the combatant whose turn is being managed.
	Character CombatCharacter

	// Room is the spatial room for movement and threat detection.
	Room spatial.Room

	// EventBus is used for publishing turn lifecycle and combat events.
	EventBus events.EventBus

	// Roller is the dice roller for attack and damage rolls.
	// If nil, a default roller is used.
	Roller dice.Roller
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
// It manages the action economy, delegates to the canonical Strike boundary
// and MoveEntity,
// and publishes events for multiplayer broadcasting.
// After EndTurn is called, the TurnManager must not be reused.
type TurnManager struct {
	character   CombatCharacter
	economy     *ActionEconomy
	room        spatial.Room
	bus         events.EventBus
	roller      dice.Roller
	turnStarted bool
	turnEnded   bool
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
		character: input.Character,
		economy:   NewActionEconomy(),
		room:      input.Room,
		bus:       input.EventBus,
		roller:    roller,
	}, nil
}

// buildContext creates an operation context with the movement room.
func (tm *TurnManager) buildContext(ctx context.Context) context.Context {
	ctx = WithRoom(ctx, tm.room)
	return ctx
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
		SubjectID: tm.character.GetID(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish turn start event: %w", err)
	}

	return &StartTurnResult{
		Economy: tm.economy,
	}, nil
}

// EndTurn publishes a TurnEndEvent and cleans up attached conditions.
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
		SubjectID: tm.character.GetID(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish turn end event: %w", err)
	}

	// Cleanup attached conditions and subscriptions.
	if err := tm.character.Cleanup(ctx); err != nil {
		return nil, err
	}

	tm.turnEnded = true

	return &EndTurnResult{
		CharacterID: tm.character.GetID(),
	}, nil
}
