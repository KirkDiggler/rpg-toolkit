// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// UseAbilityInput provides input for activating a combat ability.
type UseAbilityInput struct {
	// AbilityRef identifies which combat ability to activate (e.g., refs.CombatAbilities.Attack()).
	AbilityRef *core.Ref
}

// UseAbilityResult contains the outcome of activating a combat ability.
type UseAbilityResult struct {
	// Economy is the action economy state after ability activation.
	Economy *ActionEconomy
}

// UseAbility activates a combat ability by its Ref, consuming the appropriate action economy
// resource and granting capacity (attacks, movement, conditions, etc.).
func (tm *TurnManager) UseAbility(ctx context.Context, input *UseAbilityInput) (*UseAbilityResult, error) {
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "UseAbilityInput is nil")
	}
	if input.AbilityRef == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "AbilityRef is required")
	}

	err := tm.character.ActivateCombatAbility(ctx, &ActivateAbilityInput{
		AbilityRef:   input.AbilityRef,
		Bus:          tm.bus,
		Economy:      tm.economy,
		Speed:        tm.character.GetSpeed(),
		ExtraAttacks: tm.character.GetExtraAttacksCount(),
	})
	if err != nil {
		return nil, err
	}

	return &UseAbilityResult{
		Economy: tm.economy,
	}, nil
}
