// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
)

// AvailableAbility represents a combat ability the character can currently use.
type AvailableAbility struct {
	// Info contains the ability's metadata.
	Info AbilityInfo

	// CanUse indicates whether the ability can be activated right now.
	CanUse bool

	// Reason explains why the ability cannot be used (empty if CanUse is true).
	Reason string
}

// AvailableAction represents an action the character can currently take.
type AvailableAction struct {
	// Info contains the action's metadata.
	Info ActionInfo

	// CanUse indicates whether the action can be taken right now.
	CanUse bool

	// Reason explains why the action cannot be used (empty if CanUse is true).
	Reason string
}

// GetAvailableAbilities returns all combat abilities with their current availability.
// Checks the action economy to determine if each ability can be activated.
func (tm *TurnManager) GetAvailableAbilities(_ context.Context) []AvailableAbility {
	infos := tm.character.GetAbilityInfos()
	result := make([]AvailableAbility, 0, len(infos))

	for _, info := range infos {
		canUse, reason := tm.canUseAbility(info)
		result = append(result, AvailableAbility{
			Info:   info,
			CanUse: canUse,
			Reason: reason,
		})
	}

	return result
}

// GetAvailableActions returns all actions with their current availability.
// Checks the action economy to determine if each action can be taken.
func (tm *TurnManager) GetAvailableActions(_ context.Context) []AvailableAction {
	infos := tm.character.GetActionInfos()
	result := make([]AvailableAction, 0, len(infos))

	for _, info := range infos {
		canUse, reason := tm.canUseAction(info)
		result = append(result, AvailableAction{
			Info:   info,
			CanUse: canUse,
			Reason: reason,
		})
	}

	return result
}

// GetEconomy returns the current action economy state.
func (tm *TurnManager) GetEconomy() *ActionEconomy {
	return tm.economy
}

// canUseAbility checks if an ability can be activated based on its action type cost.
func (tm *TurnManager) canUseAbility(info AbilityInfo) (bool, string) {
	switch info.ActionType {
	case coreCombat.ActionStandard:
		if !tm.economy.CanUseAction() {
			return false, "no actions remaining"
		}
	case coreCombat.ActionBonus:
		if !tm.economy.CanUseBonusAction() {
			return false, "no bonus actions remaining"
		}
	case coreCombat.ActionReaction:
		if !tm.economy.CanUseReaction() {
			return false, "no reactions remaining"
		}
	}
	return true, ""
}

// canUseAction checks if an action can be taken based on its action type cost.
func (tm *TurnManager) canUseAction(info ActionInfo) (bool, string) {
	switch info.ActionType {
	case coreCombat.ActionStandard:
		if !tm.economy.CanUseAttack() {
			return false, "no attacks remaining"
		}
	case coreCombat.ActionBonus:
		if !tm.economy.CanUseOffHandAttack() {
			return false, "no off-hand attacks remaining"
		}
	case coreCombat.ActionMovement:
		if !tm.economy.CanUseMovement(int(FeetPerGridUnit)) {
			return false, "insufficient movement remaining"
		}
	}
	return true, ""
}
