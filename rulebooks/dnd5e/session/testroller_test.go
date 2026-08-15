// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// testDice is the deterministic entropy every fixture wires.
//
// It returns the same number for every die, which makes every member of a
// fight tie and sends the order to the ID tie-break the seam documents. That
// is deliberate: the rulebook's RollForOrder iterates a map, so WHICH member
// gets WHICH roll is not reproducible (its own TODO #285), and a fixture with
// varying rolls would produce a different order run to run. Equal rolls are
// the only shape that is reproducible today.
//
// The cost of that choice is that a constant roll makes the dice invisible —
// so TestTheDiceReachTheRule pins separately that they are rolled at all.
type testDice struct {
	// calls, when non-nil, counts every die rolled through this roller.
	calls *int
}

func (d testDice) Roll(_ context.Context, _ int) (int, error) {
	if d.calls != nil {
		*d.calls++
	}
	return 10, nil
}

// encOrderAsGiven orders a fight in the order it was handed, for the fixtures
// that build an authored world with the composition directly — before any
// session exists to route the host's dice through the rulebook.
type encOrderAsGiven struct{}

func (encOrderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}
