// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// SpendProfile is what one action costs one actor: the price, compiled.
//
// It is DATA, and that is the whole design. A profile is compiled per actor by
// the rulebook — a level-5 fighter's Attack action banks two swings and a
// level-1 fighter's banks one — and by the time it reaches [Pay] the class
// table is gone. The gate charges a price; it never learns why the price is
// what it is. Nothing here executes, subscribes, or reaches a bus.
//
// # Keyed, never fielded
//
// Every currency is a map. The action economy already learned this the
// expensive way: it grew a named field per feature (AttacksRemaining, then
// OffHandAttacksRemaining, then FlurryStrikesRemaining) while its persisted
// twin moved to a keyed map, and the bridge between the two silently carried
// three of five keys. A profile with a field per capacity would rebuild that
// bridge, one feature at a time.
//
// # What is deliberately not here
//
// The per-unit cost of movement. A profile says "spend 25 feet"; what a
// particular path across a particular room costs is the path's business, and
// the profile is compiled before anyone has chosen one.
//
// # Expressible and unexercised
//
// [SpendProfile.Pools] and [SpendProfile.Requires] are the monk's, and the monk
// is not v1. Nothing this rulebook compiles today emits either one. They are
// here because the gate that will charge a ki point, or refuse a bonus strike
// from someone who did not take the Attack action, must not need a different
// shape than the gate that charges an action — and because the alternative to
// carrying them is discovering the shape after there are call sites. They are
// checked and charged exactly like the currencies that do get compiled: a
// field the gate did not read would be a profile that could lie about what it
// costs.
//
// # What Requires is, and what it is not
//
// [SpendProfile.Requires] is THE PRECONDITION SHAPE THE GATE CAN EVALUATE
// TODAY: a keyed quantity the ledger must already hold. It is not a general
// predicate language and must not be read as one. A precondition that needs to
// ask something the ledger does not keep — a declared trigger, a condition on
// the board, a fact about who is adjacent to whom — is a different shape, and
// deciding whether that vocabulary should be sealed is rpg-toolkit#1035's open
// question 8, deliberately left open. It arrives with the caller that forces
// it, the way every other shape in this package did.
//
// A nil *SpendProfile is a free action. So is an empty one.
type SpendProfile struct {
	// Slots is the per-turn cost in action, bonus action or reaction — the
	// three slots a turn actually holds. A free action has no entry here
	// rather than an entry costing nothing, and movement is not a slot: it is
	// keyed capacity, because movement is spent in feet across a turn rather
	// than taken once.
	Slots map[coreCombat.ActionType]int

	// Capacity is the keyed capacity this action consumes: one banked attack
	// for a swing, feet of movement for a step.
	Capacity map[CapacityType]int

	// Grants is the keyed capacity this action banks. The Attack action's
	// entire effect on the economy is here — it buys 1+ExtraAttacks attacks —
	// and it ADDS to whatever is already banked, so a second Attack action in
	// one turn (Action Surge) buys more swings rather than resetting the bank
	// to the same number.
	Grants map[CapacityType]int

	// Pools is the cost in keyed resource points: ki, sorcery points, a
	// feature's charges. Expressible and unexercised in v1 — see the type doc.
	Pools map[coreResources.ResourceKey]int

	// Requires is keyed capacity that must be PRESENT and is never spent — a
	// precondition rather than a price.
	//
	// It needs no new state, and that is the reason it is shaped this way. The
	// monk's bonus strike wants to know whether the Attack action was taken
	// this turn, and the sheet ALREADY RECORDS THAT as banked capacity: the
	// post-strike grant files it the moment the swing lands. So a requirement
	// reads evidence the ledger keeps for its own reasons, rather than asking
	// the ledger to remember something extra on a precondition's behalf.
	//
	// What it is not is a predicate vocabulary — see the type doc.
	// Expressible and unexercised in v1.
	Requires map[CapacityType]int
}

// Validate reports whether this profile names a price anything could actually
// charge.
//
// It refuses rather than trims, and it refuses before any currency has moved,
// because the failure it exists for is silent: a cost keyed to something no
// ledger holds is not an expensive action, it is a FREE one, and nothing
// downstream would ever say so. Three ways to write one:
//
//   - a key outside the closed capacity vocabulary, [CapacityNone] included
//   - a slot that is not one of the three a turn holds
//   - an amount that is zero or negative, which is a cost that is not a cost
//
// A nil profile is a free action and validates.
func (p *SpendProfile) Validate() error {
	if p == nil {
		return nil
	}

	for slot, amount := range p.Slots {
		if !isTurnSlot(slot) {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"slot cost keyed to %q, which is not one of the turn's three slots", slot)
		}
		if amount <= 0 {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"slot cost of %d for %q is not a cost", amount, slot)
		}
	}

	if err := validateCapacities("capacity cost", p.Capacity); err != nil {
		return err
	}
	if err := validateCapacities("grant", p.Grants); err != nil {
		return err
	}
	if err := validateCapacities("requirement", p.Requires); err != nil {
		return err
	}

	for key, amount := range p.Pools {
		if key == "" {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "pool cost with no pool named")
		}
		if amount <= 0 {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"pool cost of %d for %q is not a cost", amount, key)
		}
	}

	return nil
}

// validateCapacities checks one of the three keyed-capacity maps. They have
// identical rules and different names, and the name is what makes the error
// say which map was wrong.
func validateCapacities(name string, costs map[CapacityType]int) error {
	for key, amount := range costs {
		if !IsCapacity(key) {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"%s keyed to %q, which no ledger holds", name, key)
		}
		if amount <= 0 {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"%s of %d for %q is not a cost", name, amount, key)
		}
	}

	return nil
}

// movesNothing reports whether paying this profile would touch the ledger at
// all. A free action is payable by anyone, in combat or out of it, because
// there is nothing to take and nowhere for it to fail to land.
func (p *SpendProfile) movesNothing() bool {
	return len(p.Slots) == 0 &&
		len(p.Capacity) == 0 &&
		len(p.Grants) == 0 &&
		len(p.Pools) == 0 &&
		len(p.Requires) == 0
}

// isTurnSlot reports whether an action type is one of the three per-turn slots
// a ledger keeps. Free is the absence of a slot cost rather than a slot that
// costs nothing, and movement is keyed capacity rather than a slot.
func isTurnSlot(slot coreCombat.ActionType) bool {
	switch slot {
	case coreCombat.ActionStandard, coreCombat.ActionBonus, coreCombat.ActionReaction:
		return true
	case coreCombat.ActionFree, coreCombat.ActionMovement:
		return false
	default:
		return false
	}
}
