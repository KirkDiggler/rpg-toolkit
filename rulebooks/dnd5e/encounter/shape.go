// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MembersWithinInput asks who is standing inside a round footprint.
type MembersWithinInput struct {
	// Origin is the cell the footprint is centred on, in the same
	// dungeon-absolute coordinates [Member.Position] reports. A caller holding a
	// Member already holds a legal Origin; it never has to convert anything.
	Origin spatial.Position

	// RadiusCells is how far the footprint reaches, in CELLS — the unit
	// [Encounter.Distance] answers in, not the feet a rulebook authors.
	//
	// NAMED FOR ITS UNIT rather than called Radius, because this composition has
	// paid for unit confusion more than once (rpg-toolkit#1141, #1150) and a
	// bare Radius on a module whose callers think in feet is an invitation to
	// hand it 5 and mean one cell.
	//
	// Zero is a legal footprint and means the origin cell alone. It is not
	// refused here: a rulebook that converts a sub-cell distance and lands on
	// zero has authored something that cannot reach, and that is a content
	// error worth refusing where the content is declared, not a malformed
	// question to this module.
	RadiusCells float64
}

// MembersWithin reports who is standing inside a round footprint, in the same
// stable order (and with the same placement) [Encounter.Members] reports them.
//
// A ROSTER READ, FILTERED — the sibling of [Encounter.MembersIn], and built the
// same way for the same reason. MembersIn names its footprint by pointing at an
// authored region; this one names one in the moment, by a centre and a reach.
// Two ways of saying WHERE, one answer to WHO, both folded from the projection
// every other member read uses, because two reads disagreeing about where
// somebody stands is the dual-state defect this composition has paid for
// before.
//
// # It says who is there, never who it is for
//
// The caster of an area spell is standing at its centre and IS RETURNED. So is
// an ally, so is a shopkeeper, so is anything else with a placement. This module
// has no idea why it was asked.
//
// That is the seam, not an oversight. "Every creature other than you" is a rule
// a spell states, and rules live above this module — which cannot even read one,
// since [encounter's go.mod] does not require the rulebook (law C1). A caller
// takes the projection it needs from a complete answer. An [Member.Kind]-tagged
// roster is what makes that possible: a caller that must treat a KindWorld
// member differently can see it, rather than discovering later that something
// standing in the blast was silently never mentioned.
//
// # An empty answer is an ordinary answer
//
// Nobody standing in the blast is a fact worth reporting, and a spell that
// catches nobody still happened. Only a malformed question is an error: nil
// input is [ErrNilInput], and a negative reach is [ErrBadReach] — refused
// rather than answered empty, so content that converted to a backwards
// footprint cannot look like a spell that simply missed.
//
// Returns [ErrNoField] if a member's cell cannot be resolved, for
// [Encounter.Members]' reason.
func (e *Encounter) MembersWithin(in *MembersWithinInput) ([]Member, error) {
	if in == nil {
		return nil, fmt.Errorf("members within: %w", ErrNilInput)
	}
	if in.RadiusCells < 0 {
		return nil, fmt.Errorf("members within: reach %v cells: %w", in.RadiusCells, ErrBadReach)
	}

	all, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("members within: %w", err)
	}

	caught := make([]Member, 0, len(all))
	for _, m := range all {
		if e.Distance(in.Origin, m.Position) <= in.RadiusCells {
			caught = append(caught, m)
		}
	}
	return caught, nil
}
