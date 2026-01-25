// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// StrikeInput provides input for making a weapon attack.
type StrikeInput struct {
	// TargetID is the combatant being attacked.
	TargetID string

	// Weapon is the weapon used for the attack.
	Weapon *weapons.Weapon
}

// MoveInput provides input for movement.
type MoveInput struct {
	// Path is the sequence of positions to move through.
	// Path[0] must be the entity's current position.
	// Each step is one grid unit (5 feet).
	Path []spatial.Position
}

// OffHandStrikeInput provides input for an off-hand attack (two-weapon fighting).
type OffHandStrikeInput struct {
	// TargetID is the combatant being attacked.
	TargetID string

	// Weapon is the off-hand weapon used for the attack.
	Weapon *weapons.Weapon
}

// FlurryStrikeInput provides input for a monk flurry unarmed strike.
type FlurryStrikeInput struct {
	// TargetID is the combatant being attacked.
	TargetID string

	// Weapon is the unarmed strike weapon.
	Weapon *weapons.Weapon
}

// Strike makes a weapon attack, consuming one attack from the economy.
// Requires the Attack ability to have been used first (to grant attack capacity).
func (tm *TurnManager) Strike(ctx context.Context, input *StrikeInput) (*AttackResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already ended")
	}
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "StrikeInput is nil")
	}
	if input.TargetID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "TargetID is required")
	}
	if input.Weapon == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Weapon is required")
	}

	if err := tm.economy.UseAttack(); err != nil {
		return nil, err
	}

	combatCtx := tm.buildContext(ctx)
	result, err := ResolveAttack(combatCtx, &AttackInput{
		AttackerID: tm.character.GetID(),
		TargetID:   input.TargetID,
		Weapon:     input.Weapon,
		EventBus:   tm.bus,
		Roller:     tm.roller,
		AttackHand: AttackHandMain,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Move executes movement along a path, consuming movement from the economy.
// Path[0] must be the entity's current position.
// Movement cost is (len(path) - 1) * 5 feet. If stopped early by an opportunity attack,
// unused movement is refunded.
func (tm *TurnManager) Move(ctx context.Context, input *MoveInput) (*MoveEntityResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already ended")
	}
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "MoveInput is nil")
	}
	if len(input.Path) < 2 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Path must have at least 2 positions")
	}

	// Validate that Path[0] is the entity's current position
	currentPos, exists := tm.room.GetEntityPosition(tm.character.GetID())
	if !exists {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "entity not found in room: %s", tm.character.GetID())
	}
	if input.Path[0].X != currentPos.X || input.Path[0].Y != currentPos.Y {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"Path[0] must be current position (%d,%d), got (%d,%d)",
			int(currentPos.X), int(currentPos.Y), int(input.Path[0].X), int(input.Path[0].Y))
	}

	// Calculate movement cost: each step between positions is 5 feet
	steps := len(input.Path) - 1
	cost := steps * int(FeetPerGridUnit)

	if err := tm.economy.UseMovement(cost); err != nil {
		return nil, err
	}

	combatCtx := tm.buildContext(ctx)
	result, err := MoveEntity(combatCtx, &MoveEntityInput{
		EntityID:   tm.character.GetID(),
		EntityType: "character",
		Path:       input.Path,
		EventBus:   tm.bus,
		Roller:     tm.roller,
	})
	if err != nil {
		return nil, err
	}

	// Refund unused movement if stopped early by OA
	if result.MovementStopped {
		unusedSteps := steps - result.StepsCompleted
		if unusedSteps > 0 {
			refund := unusedSteps * int(FeetPerGridUnit)
			tm.economy.AddMovement(refund)
		}
	}

	return result, nil
}

// OffHandStrike makes an off-hand attack using two-weapon fighting.
// Consumes one off-hand attack from the economy (granted by TwoWeaponGranter).
func (tm *TurnManager) OffHandStrike(ctx context.Context, input *OffHandStrikeInput) (*AttackResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already ended")
	}
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "OffHandStrikeInput is nil")
	}
	if input.TargetID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "TargetID is required")
	}
	if input.Weapon == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Weapon is required")
	}

	if err := tm.economy.UseOffHandAttack(); err != nil {
		return nil, err
	}

	combatCtx := tm.buildContext(ctx)
	result, err := ResolveAttack(combatCtx, &AttackInput{
		AttackerID: tm.character.GetID(),
		TargetID:   input.TargetID,
		Weapon:     input.Weapon,
		EventBus:   tm.bus,
		Roller:     tm.roller,
		AttackHand: AttackHandOff,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FlurryStrike makes an unarmed strike as part of Flurry of Blows.
// Consumes one flurry strike from the economy (granted by FlurryOfBlows feature).
func (tm *TurnManager) FlurryStrike(ctx context.Context, input *FlurryStrikeInput) (*AttackResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already ended")
	}
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "FlurryStrikeInput is nil")
	}
	if input.TargetID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "TargetID is required")
	}
	if input.Weapon == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Weapon is required")
	}

	if err := tm.economy.UseFlurryStrike(); err != nil {
		return nil, err
	}

	combatCtx := tm.buildContext(ctx)
	result, err := ResolveAttack(combatCtx, &AttackInput{
		AttackerID: tm.character.GetID(),
		TargetID:   input.TargetID,
		Weapon:     input.Weapon,
		EventBus:   tm.bus,
		Roller:     tm.roller,
		AttackHand: AttackHandMain,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
