// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// economy_test.go drives the economy through the verb, which is where it has to
// be right. This file pins the merge underneath it directly, for the two reasons
// translate_internal_test.go exists: some of what asOnePayment guarantees has no
// path from a caller today, and a promise that is only true for the profiles v1
// happens to compile is not the promise the doc makes.
//
// The rulebook compiles exactly two profiles today, and between them they
// exercise one shape: a slot cost, a grant, and a capacity cost smaller than the
// grant. Pools, requirements, and a strike that costs MORE than its action banks
// are all expressible and all unexercised — so a merge that silently dropped any
// of them would ship green. That failure is the one combat.SpendProfile's own
// doc names as the expensive one: a cost keyed to something nobody charges is
// not a cheaper action, it is a free one.

// TestASwingCostsMoreThanItsActionBanks is the case the netting exists to get
// right in the other direction.
//
// An action that banks one swing cannot buy two, so the second is still owed
// from the bank — and if the merge dropped the remainder, the profile would say
// the action pays for both and the gate would charge for neither.
func TestASwingCostsMoreThanItsActionBanks(t *testing.T) {
	action := &combat.SpendProfile{
		Slots:  map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	strike := &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 2},
	}

	merged := asOnePayment(action, strike)

	require.NoError(t, merged.Validate(), "a merge must never produce a price the gate refuses")
	require.Equal(t, 1, merged.Capacity[combat.CapacityAttack],
		"one of the two swings came out of the grant; the other is still owed from the bank")
	require.Empty(t, merged.Grants,
		"and nothing is left banked — a grant of zero is a cost that is not a cost, "+
			"which Validate refuses outright")
	require.Equal(t, 1, merged.Slots[coreCombat.ActionStandard], "the action is still paid for")
}

// TestTheNettingLeavesNoZeroEntries is the trap this merge walks past on every
// level-1 character.
//
// combat.SpendProfile.Validate refuses a grant or a cost of zero by name — "a
// cost that is not a cost" — so a netting that wrote the key with a zero rather
// than leaving it out would make the commonest case in the game unpayable, and
// would report it as a malformed profile rather than as anything a player did.
func TestTheNettingLeavesNoZeroEntries(t *testing.T) {
	action := &combat.SpendProfile{
		Slots:  map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	strike := &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}

	merged := asOnePayment(action, strike)

	require.NoError(t, merged.Validate())
	require.NotContains(t, merged.Grants, combat.CapacityAttack,
		"the one swing the action banked is the one being taken, so nothing is left over")
	require.NotContains(t, merged.Capacity, combat.CapacityAttack,
		"and nothing is owed from a bank either — the grant paid for it")
	require.Equal(t, 1, merged.Slots[coreCombat.ActionStandard])
}

// TestEveryCurrencySurvivesTheMerge is the totality claim, asserted rather than
// described.
//
// Pools and requirements are the monk's and the monk is not v1, so nothing the
// rulebook compiles today puts anything in either map. A merge that dropped them
// would therefore pass every other test in this package and would be discovered
// by whoever writes Flurry of Blows, at which point the bonus strike costs no ki
// and nothing says so.
func TestEveryCurrencySurvivesTheMerge(t *testing.T) {
	const ki = coreResources.ResourceKey("ki")

	action := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools:    map[coreResources.ResourceKey]int{ki: 1},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	strike := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools:    map[coreResources.ResourceKey]int{ki: 2},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 3},
	}

	merged := asOnePayment(action, strike)

	require.NoError(t, merged.Validate())
	require.Equal(t, 1, merged.Slots[coreCombat.ActionStandard])
	require.Equal(t, 1, merged.Slots[coreCombat.ActionBonus],
		"two different slots are two different costs, not one")
	require.Equal(t, 3, merged.Pools[ki],
		"two prices in the same pool are owed together")
	require.Equal(t, 3, merged.Requires[combat.CapacityAttack],
		"a requirement is a floor rather than a bill, so the stricter one wins")
}
