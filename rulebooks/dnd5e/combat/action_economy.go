// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import "github.com/KirkDiggler/rpg-toolkit/rpgerr"

// ActionEconomy tracks the available actions, bonus actions, and reactions for a combatant
// Purpose: Manages the action economy system for D&D 5e combat, ensuring combatants can only
// take actions they have available in their turn.
type ActionEconomy struct {
	ActionsRemaining      int // Usually 1, Action Surge gives +1
	BonusActionsRemaining int // Usually 1
	ReactionsRemaining    int // Usually 1
}

// NewActionEconomy creates a new ActionEconomy with default values (1/1/1)
// Purpose: Standard constructor for initializing a combatant's action economy at the start of their turn
func NewActionEconomy() *ActionEconomy {
	return &ActionEconomy{
		ActionsRemaining:      1,
		BonusActionsRemaining: 1,
		ReactionsRemaining:    1,
	}
}

// CanUseAction returns whether an action is available
// Purpose: Allows checking action availability without consuming it
func (ae *ActionEconomy) CanUseAction() bool {
	return ae.ActionsRemaining > 0
}

// CanUseBonusAction returns whether a bonus action is available
// Purpose: Allows checking bonus action availability without consuming it
func (ae *ActionEconomy) CanUseBonusAction() bool {
	return ae.BonusActionsRemaining > 0
}

// CanUseReaction returns whether a reaction is available
// Purpose: Allows checking reaction availability without consuming it
func (ae *ActionEconomy) CanUseReaction() bool {
	return ae.ReactionsRemaining > 0
}

// UseAction consumes an action if available
// Returns CodeResourceExhausted if no actions remain
func (ae *ActionEconomy) UseAction() error {
	if ae.ActionsRemaining <= 0 {
		return rpgerr.ResourceExhausted("action")
	}
	ae.ActionsRemaining--
	return nil
}

// UseBonusAction consumes a bonus action if available
// Returns CodeResourceExhausted if no bonus actions remain
func (ae *ActionEconomy) UseBonusAction() error {
	if ae.BonusActionsRemaining <= 0 {
		return rpgerr.ResourceExhausted("bonus action")
	}
	ae.BonusActionsRemaining--
	return nil
}

// UseReaction consumes a reaction if available
// Returns CodeResourceExhausted if no reactions remain
func (ae *ActionEconomy) UseReaction() error {
	if ae.ReactionsRemaining <= 0 {
		return rpgerr.ResourceExhausted("reaction")
	}
	ae.ReactionsRemaining--
	return nil
}

// Reset restores all actions to default values (1/1/1)
// Purpose: Called at the start of a combatant's turn to restore their action economy
func (ae *ActionEconomy) Reset() {
	ae.ActionsRemaining = 1
	ae.BonusActionsRemaining = 1
	ae.ReactionsRemaining = 1
}

// GrantExtraAction grants an additional action
// Purpose: Used by features like Action Surge that provide extra actions beyond the normal limit
func (ae *ActionEconomy) GrantExtraAction() {
	ae.ActionsRemaining++
}

// GrantExtraBonusAction grants an additional bonus action
// Purpose: Future-proofing for potential features that grant extra bonus actions
func (ae *ActionEconomy) GrantExtraBonusAction() {
	ae.BonusActionsRemaining++
}
