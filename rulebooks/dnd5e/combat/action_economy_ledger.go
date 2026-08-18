// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
)

// ActionEconomy is a [Ledger].
//
// This is the economy a monster's turn is driven from: the caller builds one,
// hands it to the monster's action loop, and drops it when the turn ends. It
// keeps its capacity in named fields rather than a map, which is exactly the
// shape the persisted twin moved away from — so the keyed methods below are a
// VIEW over those fields rather than a second copy of them. A keyed write that
// the named accessors could not see would be a spend the surrounding code
// overwrites without noticing.
//
// It holds no pools, and says so honestly: [ActionEconomy.PoolLeft] answers
// zero for everything, so a profile that costs ki is refused rather than
// charged to nothing.
var _ Ledger = (*ActionEconomy)(nil)

// InCombat reports whether there is an economy here at all.
//
// An economy that does not exist is not in combat, and the nil receiver is
// deliberate: a caller holding a *ActionEconomy it never built gets a refusal
// from the gate rather than a panic.
func (ae *ActionEconomy) InCombat() bool {
	return ae != nil
}

// SlotsLeft reports the per-turn slots remaining.
//
// Only the three slots this economy keeps can answer. Free and movement report
// zero — free because a free action costs no slot and so can have no slot cost,
// movement because it is keyed capacity here rather than a slot.
func (ae *ActionEconomy) SlotsLeft(slot coreCombat.ActionType) int {
	if ae == nil {
		return 0
	}

	switch slot {
	case coreCombat.ActionStandard:
		return ae.ActionsRemaining
	case coreCombat.ActionBonus:
		return ae.BonusActionsRemaining
	case coreCombat.ActionReaction:
		return ae.ReactionsRemaining
	case coreCombat.ActionFree, coreCombat.ActionMovement:
		return 0
	default:
		return 0
	}
}

// SpendSlots debits per-turn slots. The gate checks affordability in full
// first, so this cannot fail; a slot this economy does not keep is not
// reachable through the gate, because the profile that named it would not have
// validated.
func (ae *ActionEconomy) SpendSlots(slot coreCombat.ActionType, n int) {
	if ae == nil {
		return
	}

	switch slot {
	case coreCombat.ActionStandard:
		ae.ActionsRemaining -= n
	case coreCombat.ActionBonus:
		ae.BonusActionsRemaining -= n
	case coreCombat.ActionReaction:
		ae.ReactionsRemaining -= n
	case coreCombat.ActionFree, coreCombat.ActionMovement:
	default:
	}
}

// CapacityLeft reports how much of a keyed capacity remains, reading the field
// that has always held it.
func (ae *ActionEconomy) CapacityLeft(key CapacityType) int {
	if ae == nil {
		return 0
	}

	switch key {
	case CapacityAttack:
		return ae.AttacksRemaining
	case CapacityMovement:
		return ae.MovementRemaining
	case CapacityOffHandAttack:
		return ae.OffHandAttacksRemaining
	case CapacityFlurryStrike:
		return ae.FlurryStrikesRemaining
	case CapacityNone:
		return 0
	default:
		return 0
	}
}

// SpendCapacity debits keyed capacity, writing the field that has always held
// it.
func (ae *ActionEconomy) SpendCapacity(key CapacityType, n int) {
	ae.BankCapacity(key, -n)
}

// BankCapacity adds keyed capacity. Adding rather than assigning is what lets
// an action taken twice in one turn bank twice — the fielded setters this
// economy also carries (SetAttacks and friends) assign, which is why the
// Attack ability had to know what was already there.
func (ae *ActionEconomy) BankCapacity(key CapacityType, n int) {
	if ae == nil {
		return
	}

	switch key {
	case CapacityAttack:
		ae.AttacksRemaining += n
	case CapacityMovement:
		ae.MovementRemaining += n
	case CapacityOffHandAttack:
		ae.OffHandAttacksRemaining += n
	case CapacityFlurryStrike:
		ae.FlurryStrikesRemaining += n
	case CapacityNone:
	default:
	}
}

// PoolLeft reports zero for every pool, because this economy has none.
//
// That is not a gap to be filled later with a silent zero: a cost in a pool
// nobody holds must be REFUSED, and reporting nothing left is how the gate
// refuses it.
func (ae *ActionEconomy) PoolLeft(_ coreResources.ResourceKey) int {
	return 0
}

// SpendPool cannot happen: the gate checks [ActionEconomy.PoolLeft] first and
// every answer is zero, so every pool cost is refused before anything is spent.
func (ae *ActionEconomy) SpendPool(_ coreResources.ResourceKey, _ int) {
}
