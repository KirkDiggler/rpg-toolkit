// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
)

// board.go is THE WHOLE BOARD AT ONCE (rpg-project#542, "Launch"): every
// member a launch places goes in, and only then does anybody look.
//
// # Why Join one at a time is the wrong shape for a launch
//
// [Encounter.Join] looks the moment its member lands — sight refreshes, and a
// fight forms if the joiner is in sight of an enemy. That is right for one
// member arriving into a run. For a launch it means the fight is decided by
// placement ORDER: two authored camps hostile to each other form their fight
// as soon as the second camp's first creature lands, before the party has
// been placed, and the party then joins a fight already under way rather than
// being in the one that forms. The design's law is the opposite: the whole
// board stands before anyone is seated, and the fight that forms holds
// everybody it should.
//
// # One look, after everybody
//
// Board admits every member exactly as Join does — the same refusals, the
// same reserve hold, the same arrival beat in order, the same temperament
// dealt at the door — and then refreshes sight ONCE over the whole roster,
// which is one formation pass: every contact on the board is seen at the
// same moment.

// BoardInput is every member a launch places, in the order they arrive.
type BoardInput struct {
	// Members is every joiner, in order. Each is exactly what [Encounter.Join]
	// takes. Never empty (ErrNoMember).
	Members []JoinInput
}

// BoardOutput is what placing the board produced.
type BoardOutput struct {
	// Joined is one answer per member, in the order given — the reserve
	// answer for a member held in reserve, or the placement and join beat for
	// a member who landed. The look's results are on the board, not on any
	// one member: Formed and IntelDeltas below.
	Joined []*JoinOutput

	// IntelDeltas is each observer's intel movement from the one look.
	IntelDeltas map[MemberID]*IntelDelta

	// Formed is every fight the one look formed. At most one today: an
	// encounter runs one fight at a time (rpg-toolkit#963), and every contact
	// the look finds is in it.
	Formed []*FormedBubble
}

// Board places every member of a launch in order, then looks once: one sight
// refresh and one formation pass over the whole roster.
//
// ALL OR NOTHING. Every member is validated — each as [Encounter.Join]
// validates one, plus what only a batch can get wrong: the same id twice, and
// two members on one cell where the first blocks it — before the first
// write. A board that refuses has placed nobody, written no beat and held
// nobody in reserve.
//
// A REACHED-POSITION ENDING is evaluated after the look, member by member in
// order, as Join evaluates it after its own; the first that fires closes the
// encounter and is reported on that member's answer.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember (an empty board, an empty or
// repeated id, or one already in the encounter), ErrBadPlacement, and every
// refusal [Encounter.Join] makes, wrapped with the member's position on the
// board; or a failure of the look (drop the encounter unsaved — doc.go's
// caller rule).
func (e *Encounter) Board(in *BoardInput) (*BoardOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("board: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("board: %w", ErrClosed)
	}
	if len(in.Members) == 0 {
		return nil, fmt.Errorf("board: nobody to place: %w", ErrNoMember)
	}

	seen := make(map[MemberID]int, len(in.Members))
	blocked := make(map[[2]float64]MemberID, len(in.Members))
	for i := range in.Members {
		m := &in.Members[i]
		if err := e.validateJoin(m); err != nil {
			return nil, fmt.Errorf("board: members[%d]: %w", i, err)
		}
		if prev, twice := seen[m.Member]; twice {
			return nil, fmt.Errorf("board: members[%d]: %q is members[%d] too: %w", i, m.Member, prev, ErrNoMember)
		}
		seen[m.Member] = i
		if m.Arrives != nil {
			continue
		}
		// The canvas refuses a cell a blocking entity already stands on, and
		// Join would learn that at its first write. A board learns it here,
		// against the run and against every member placed before this one.
		entity := &memberEntity{id: string(m.Member), kind: m.Kind, blocksMovement: m.BlocksMovement}
		if !e.canvas.CanPlaceEntity(entity, m.Cell) {
			return nil, fmt.Errorf("board: members[%d]: cell %v is taken: %w", i, m.Cell, ErrBadPlacement)
		}
		cell := [2]float64{m.Cell.X, m.Cell.Y}
		if by, taken := blocked[cell]; taken {
			return nil, fmt.Errorf("board: members[%d]: cell %v is taken by %q: %w", i, m.Cell, by, ErrBadPlacement)
		}
		if m.BlocksMovement {
			blocked[cell] = m.Member
		}
	}

	joined := make([]*JoinOutput, 0, len(in.Members))
	records := make([]*memberRecord, 0, len(in.Members))
	for i := range in.Members {
		record, out, err := e.admitMember(&in.Members[i])
		if err != nil {
			return nil, fmt.Errorf("board: members[%d]: %w", i, err)
		}
		joined = append(joined, out)
		records = append(records, record)
	}

	// THE ONE LOOK: every contact on the board, seen at the same moment.
	deltas, formed, err := e.refreshSight(e.rosterIDs())
	if err != nil {
		return nil, fmt.Errorf("board: refresh sight: %w", err)
	}

	at := uint64(e.clock.ToData().HighWater)
	for i, out := range joined {
		if out.Reserved {
			continue
		}
		m := &in.Members[i]
		if e.outcome == nil {
			fired, err := e.firedReachedPosition(records[i], m.Cell, at)
			if err != nil {
				return nil, fmt.Errorf("board: members[%d] ending: %w", i, err)
			}
			out.Outcome = fired
		}
		placement, err := e.placementOf(records[i])
		if err != nil {
			return nil, fmt.Errorf("board: members[%d]: %w", i, err)
		}
		out.Member = placement
	}

	result := &BoardOutput{Joined: joined, IntelDeltas: deltas}
	if formed != nil {
		result.Formed = []*FormedBubble{formed}
	}

	return result, nil
}
