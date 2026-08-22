// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TurnDriver decides what a member with no player does when a fight's clock
// lands on their turn. Required.
//
// A FIGHT NOW STARTS ON ITS OWN, and it can start — or a turn can end — with
// the clock landing on a member the host has no player for: initiative rolled
// the monster first, or the human ahead of it just ended their own turn.
// Nothing about that moment is a question this package is allowed to answer;
// "what does an unplayed member do" is exactly the D&D-shaped decision the
// Monster AI initiative (rpg-project#201) will eventually own. So the host is
// asked, the way Standing and Sight already are — required, refused at
// construction, never defaulted to a guess (rpg-toolkit#1033,
// rpg-toolkit#1162, ADR-0043).
//
// v1's whole answer fits in one shipped implementation: [Pass]. A host wires
// `TurnDriver: session.Pass{}` and every unplayed member's turn ends the
// moment the clock reaches it, with the pass recorded as an ordinary
// turn-ended beat. A real decider replaces that ONE line later, through the
// identical Config field — nothing in this package's shape has to change to
// receive it.
//
// It is an interface over this package's own vocabulary rather than
// encounter.TurnDriver directly, for the reason every other capability here
// is (S2): a host implementing encounter's interface would name a module this
// SDK intends to keep replaceable underneath it.
type TurnDriver interface {
	// Act decides what member does on their turn, or returns an error that
	// aborts the verb that discovered the unplayed turn — EndTurn, or
	// whichever verb's own Move/Attack/Spawn triggered a fight forming with
	// an unplayed member first in initiative. Nothing is persisted on that
	// error: this package's load-mutate-save shape means the in-memory world
	// is simply discarded, so a broken driver fails loudly on retry rather
	// than corrupting a fight's state.
	Act(member string) (TurnOutcome, error)
}

// TurnOutcome is a sealed vocabulary (unexported marker method) — this
// package's own twin of encounter.TurnOutcome, kept separate for the same
// reason ClockKind and every other wire enum here is: it is what lets the
// composition's own vocabulary change shape without a host source file
// changing.
//
// ONE CASE TODAY. See encounter.TurnOutcome's own doc for why a second case
// (an attack, a move) is real vocabulary growth rather than an oversight —
// the identical reasoning applies here, one layer up.
type TurnOutcome interface {
	isTurnOutcome()
}

// Pass is the only outcome a v1 driver may return: the member's turn ends
// with no other effect. The SDK's own ready-made implementation of
// TurnDriver — wire `TurnDriver: session.Pass{}` and every unplayed member
// passes.
type Pass struct{}

// isTurnOutcome marks Pass as a TurnOutcome.
func (Pass) isTurnOutcome() {}

// Act always passes, regardless of who is asked. Pass satisfies TurnDriver as
// well as being one of its outcomes, so a host that wants v1's whole behavior
// wires the same value to both jobs.
func (Pass) Act(string) (TurnOutcome, error) {
	return Pass{}, nil
}

// turnDriverSeam adapts the host's TurnDriver to the composition's.
//
// Unexported for the reason every seam in this file is: if the host had to
// satisfy encounter.TurnDriver directly, replacing the composition would
// break every host that implemented it.
type turnDriverSeam struct {
	driver TurnDriver
}

// Act translates one member's outcome across the boundary.
//
// The default arm is NOT purely defensive the way encounter's own sealed
// switch is: TurnOutcome here is this package's vocabulary, and a host
// implementing TurnDriver can only ever hand back a Pass today — but the day
// this package's own TurnOutcome grows a second case, an adapter that has not
// been updated to translate it is a real, reachable gap, not a compile-time
// impossibility. Refused by name (ErrBadTurnOutcome) rather than silently
// treated as a pass, which would let an unrecognised outcome quietly become a
// different one.
func (s turnDriverSeam) Act(member encounter.MemberID) (encounter.TurnOutcome, error) {
	outcome, err := s.driver.Act(string(member))
	if err != nil {
		return nil, err
	}

	switch outcome.(type) {
	case Pass:
		return encounter.Pass{}, nil
	default:
		return nil, fmt.Errorf("turn driver %q: %w: %T", member, ErrBadTurnOutcome, outcome)
	}
}

// compile-time proof the adapter satisfies what it is handed to.
var _ encounter.TurnDriver = turnDriverSeam{}
