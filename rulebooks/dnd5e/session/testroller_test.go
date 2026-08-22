// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// testDice is the deterministic entropy most fixtures wire.
//
// It returns the same number for every die, so every member of a fight ties
// and the order falls to the ID tie-break the seam documents. That is a
// legibility choice, not a workaround: a scene asserting on who acts first
// reads better when the answer is alphabetical than when it depends on which
// d20 landed where. TestTheDiceDecideTheOrder wires varying rolls to prove the
// dice are actually what decides.
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

// sequenceDice hands out a scripted series of rolls, in order.
//
// It is what makes the seam's determinism assertable. Members are asked in
// sorted order, one at a time, so the Nth roll always lands on the Nth member
// alphabetically — which is a claim a test can check exactly, and which would
// be untestable if the rulebook's map iteration decided who got what.
type sequenceDice struct {
	rolls []int
	next  int
}

func (d *sequenceDice) Roll(_ context.Context, _ int) (int, error) {
	if d.next >= len(d.rolls) {
		return 0, fmt.Errorf("sequenceDice: asked for roll %d of %d", d.next+1, len(d.rolls))
	}
	roll := d.rolls[d.next]
	d.next++
	return roll, nil
}

// brokenDice is a host whose randomness is unavailable.
type brokenDice struct{}

var errNoRandomness = errors.New("the entropy source is down")

func (brokenDice) Roll(_ context.Context, _ int) (int, error) { return 0, errNoRandomness }

// encOrderAsGiven orders a fight in the order it was handed, for the fixtures
// that build an authored world with the composition directly — before any
// session exists to route the host's dice through the rulebook.
type encOrderAsGiven struct{}

func (encOrderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

// encEveryoneStanding is the standing capability the authored-world fixtures
// build with: nobody is ever down.
//
// It answers for CONSTRUCTION only. These fixtures use the composition directly
// to author a blob, and Setup consults the capability at first light — a scene
// can open with a body already on the floor. Every one of them opens with
// everybody upright, which is what they were all assuming before death existed.
//
// The session under test never uses this. It supplies its own capability,
// backed by the sheets it holds, at each of its own load sites — which is the
// thing rpg-toolkit#1079 is about, and the reason this fake must stay dumb
// enough that no test can accidentally lean on it for an answer.
type encEveryoneStanding struct{}

func (encEveryoneStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// encPassDriver is the TurnDriver capability the authored-world fixtures
// build with: every unplayed member always passes. Same reasoning as
// encEveryoneStanding — construction-only, and the session under test
// supplies its own at each of its own load sites (session.Pass wrapped in
// turnDriverSeam), so this fake must stay dumb enough that no test can
// accidentally lean on it for an answer.
type encPassDriver struct{}

func (encPassDriver) Act(encounter.MemberID) (encounter.TurnOutcome, error) {
	return encounter.Pass{}, nil
}
