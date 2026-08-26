// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// CapacityType identifies what capacity an action consumes.
//
// This is the toolkit's cost vocabulary. A shared action definition may carry
// a compiled [SpendProfile] that keys its costs, grants and requirements by it,
// and [Ledger] is where those keys are read and
// written. It is deliberately keyed rather than fielded: the action economy
// grew a named field per feature once already, and the persisted twin had to
// replace them with a map when the bridge between the two turned out to carry
// three of the five keys that existed.
//
// It lived in turn_manager.go until #1091 — beside a TurnManager that nothing
// outside its own tests has referenced in a long time — which made a live
// vocabulary look like part of a dead type. The move changed no identifier and
// no value.
//
// The set is closed on purpose: [SpendProfile.Validate] refuses a key that is
// not in it, because a capacity no ledger has agreed to store is a grant that
// vanishes and a cost that is free, and neither reports anything. Growing it is
// a constant here plus whatever each ledger needs to hold it — and the
// round-trip tests fail until every ledger does.
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

// capacities is the closed set, as a set. CapacityNone is not in it: "no
// capacity" is the answer an action gives about itself, not a currency
// anything can hold.
var capacities = map[CapacityType]struct{}{
	CapacityAttack:        {},
	CapacityMovement:      {},
	CapacityOffHandAttack: {},
	CapacityFlurryStrike:  {},
}

// CapacityTypes returns every capacity a profile may name and every capacity a
// ledger must be able to hold. Callers get a fresh slice, so the vocabulary
// cannot be edited by reading it.
//
// It exists so a ledger implementation can be pinned TOTAL rather than trusted
// to be: a test walks this list rather than the keys the rulebook happens to
// compile today, and a new member breaks every ledger that cannot store it.
func CapacityTypes() []CapacityType {
	return []CapacityType{
		CapacityAttack,
		CapacityMovement,
		CapacityOffHandAttack,
		CapacityFlurryStrike,
	}
}

// IsCapacity reports whether a key names a capacity that can be held.
func IsCapacity(key CapacityType) bool {
	_, ok := capacities[key]
	return ok
}

// AdjacentCells is "within 5 feet" expressed in the units a grid answers in:
// one cell.
//
// Every grid in tools/spatial reports distance in cells — Chebyshev on a square
// grid (SquareGrid.Distance is max(|dx|,|dy|), so a diagonal neighbour is 1),
// hex distance on a hex grid, Euclidean cell-widths when gridless — and a cell
// is 5 feet. Comparing against 1 is therefore the correct reading of the rule
// rather than an approximation of it.
//
// It lives here because three rules had independently written it down and one
// of them had it wrong: prone said 1.0, while sneak attack said 1.5 "to include
// diagonals" — a correction the square grid does not need and which reads as
// 7.5 feet on a gridless room. Two copies that agree are a coincidence waiting
// to end; this is the constant they should all have been reading
// (rpg-toolkit#1255 review).
const AdjacentCells = 1.0
