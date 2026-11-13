// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// relocate.go is A MEMBER MOVED TO A PLACE (rpg-project#925, ruled
// 2026-11-14: "a primitive that we use here … the cut away while walking off
// and appearing in the cell"): an authored thing — a consequence, a passage,
// later a spell — sets members down somewhere else without walking the way
// between.
//
// # Not a walk
//
// A walk is [Encounter.Step] and [Encounter.Direct]: cell by cell, paying
// paces, meeting doors and posts, refusing two cells that are not neighbours.
// None of that is weakened. A relocation is its own verb with its own record:
// the cell write is spatial's [spatial.BasicRoom.RelocateEntity], which asks
// no boundary, so a wall, a locked or held door, or any distance between the
// two ends is no refusal. Reachability is not asked by design.
//
// # One fact, two ends, each told to who saw it
//
// One [BeatRelocated] per member, in per-audience versions, as speech is:
//
//   - the member themself: from, to, the cause and its WORDS, end "both";
//   - whoever saw them at the origin (on the live reach before the move) and
//     sees them at the destination (after it): from, to, the cause;
//   - whoever saw them only at the origin: from and the cause, end "left" —
//     never where they went;
//   - whoever sees them only at the destination: to and the cause, end
//     "arrived" — never where they came from;
//   - whoever saw neither end: nothing. Their sighting fades as for any lost
//     sight: the next refresh leaves it a ghost at the old cell.
//
// The authored words are the member's alone (the lead's call on #925);
// observers get the cause ref, which a client words. A journal fact on the
// truth grain, `relocated:<id>@<cell>`, records where the member was set down
// and why, beside `arrived:` (reserve.go).
//
// # What the move settles
//
// The member's sneak ends (a move shows you, as casting does). The address
// sweep runs at the host's reach ([Encounter.sweepAddresses]) and the
// reservation sweep ([Encounter.sweepHolds]) abandons a kept line its holder
// is now out of reach of, as after any step. The cause's rounds pass on the
// world clock, round by round, waking whoever is due. Then one sight refresh
// for everyone settles both ends: who lost them, who gained them, the cells
// they now learn, and a fight that forms where hostile sight meets.
// Concentration, conditions and sight areas are untouched: none of them is
// bound to where a member stands. Posts are bypassed: a relocation crosses
// nothing for a post to hold.
//
// # What it refuses
//
// A member in a fight (the lead's call: no relocating in a fight), a member
// the guards hold — unless the engine's own custody consequence asks, which
// no request can name ([Encounter.relocate]) — a held walk, the member's own
// cell, a cell nobody can stand on (not floor, or under a blocking
// footprint), a cell somebody stands on unless they leave it in the same
// call (a chain: the leaver goes first; a ring of members into each other's
// cells has no first and is refused), two members onto one cell, and one
// member twice. ALL OF IT IS ASKED BEFORE THE ROOM IS: spatial's
// [spatial.BasicRoom.RelocateEntity] knows nothing of void, unfloored cells
// or footprints, so this module never calls it bare.

// BeatRelocated is the "beat" value of a member moved to a place: one per
// member per audience version ([Encounter.Relocate]). EXPORTED BECAUSE A
// DECODER READS IT.
const BeatRelocated = "relocated"

// The ends a [BeatRelocated] version tells (its "end" key).
const (
	// RelocatedLeft: the reader saw the member at the origin only.
	RelocatedLeft = "left"
	// RelocatedArrived: the reader sees the member at the destination only.
	RelocatedArrived = "arrived"
	// RelocatedBoth: the reader saw both ends, or is the member.
	RelocatedBoth = "both"
)

// relocatedPrefix is the kind prefix of a relocation's journal fact.
const relocatedPrefix = "relocated:"

// relocatedKind mints the fact kind recording that a member was set down at a
// cell — `relocated:<id>@<x>,<y>`, [arrivedKind]'s spelling.
func relocatedKind(id MemberID, at spatial.Position) journal.Kind {
	return journal.Kind(fmt.Sprintf("%s%s@%g,%g", relocatedPrefix, id, at.X, at.Y))
}

// parseRelocated reads a relocated fact's kind back into the member and the
// cell, false for a kind this build did not mint.
func parseRelocated(kind string) (MemberID, spatial.Position, bool) {
	rest, ok := strings.CutPrefix(kind, relocatedPrefix)
	if !ok {
		return "", spatial.Position{}, false
	}
	id, at, ok := parseArrived(arrivedPrefix + rest)
	return MemberID(id), at, ok
}

// RelocationCause is why members are moved to a place: an authored ref, the
// words the moved member is told, and how many world rounds the move takes.
type RelocationCause struct {
	// Ref names the authored thing that moves them. REQUIRED, a valid ref.
	Ref core.Ref

	// Words is what the moved member is told ("The guards march you to the
	// cells."). REQUIRED. Observers are told the Ref, never the words.
	Words string

	// Rounds is how many world rounds pass with the move, paid after the
	// members are set down and before the sight refresh. Zero or more.
	Rounds int
}

// Relocation is one member and the cell they are set down on.
type Relocation struct {
	// Member is a member of the run, placed.
	Member MemberID

	// To is a dungeon-absolute cell: standable floor that nobody stands on.
	To spatial.Position
}

// RelocateInput is one relocation of one or more members, applied whole or
// not at all.
type RelocateInput struct {
	// Moves are the members and their cells. At least one; one per member;
	// one member per cell.
	Moves []Relocation

	// Cause is why they move.
	Cause RelocationCause

	// Reach is the host's interaction reach, as [StepInput.Reach]: the reach
	// the address sweep measures at once the members are set down. Zero
	// means adjacent.
	Reach int
}

// Relocated is one member a relocation set down.
type Relocated struct {
	// Member is who moved.
	Member MemberID
	// From is where they stood.
	From spatial.Position
	// To is where they stand now.
	To spatial.Position
}

// RelocateOutput is what a relocation did.
type RelocateOutput struct {
	// Moved is each member set down, in the input's order.
	Moved []Relocated

	// Seqs are the [BeatRelocated] beats appended, in order.
	Seqs []uint64

	// IntelDeltas are the percepts the sight refresh changed.
	IntelDeltas map[MemberID]*IntelDelta

	// Formed is the fight the refresh formed, if one did.
	Formed *FormedBubble
}

// Relocate sets members down at other cells without walking the way between
// (this file's doc). Applied whole: every move is validated before anything
// changes.
//
// It never moves a member the guards hold: the custody consequence is the
// engine's own and is not reachable through this verb ([Encounter.relocate]).
//
// Errors: ErrNilInput; ErrClosed; ErrNoMember (no moves); ErrInvalidData (an
// invalid cause ref, no words, negative rounds, a member twice, a negative
// reach); ErrNotMember; ErrBadPlacement (unplaced, its own cell, not
// standable, occupied by someone staying, a cell taken twice, a ring);
// ErrInBubble (in a fight);
// ErrTurnPaused (a walk is held); a [CustodyMoveError] (held by the guards).
func (e *Encounter) Relocate(in *RelocateInput) (*RelocateOutput, error) {
	return e.relocate(in, false)
}

// relocate is [Encounter.Relocate]; custody true lets it move a member the
// guards hold. ONLY THE ENGINE'S CUSTODY CONSEQUENCE PASSES TRUE: the
// exported verb passes false, and no field of any input can set it, so no
// host request — whatever cause it names — moves the held.
func (e *Encounter) relocate(in *RelocateInput, custody bool) (*RelocateOutput, error) {
	moves, order, err := e.validateRelocation(in, custody)
	if err != nil {
		return nil, err
	}
	at := uint64(e.clock.ToData().HighWater)

	before, err := e.placedSightReach()
	if err != nil {
		return nil, fmt.Errorf("relocate: %w", err)
	}
	down, err := e.downNow()
	if err != nil {
		return nil, fmt.Errorf("relocate: %w", err)
	}
	sawLeave := make([][]MemberID, len(moves))
	for i, m := range moves {
		sawLeave[i] = e.relocationWitnesses(before, down, m.Member)
	}

	// THE CANVAS IS ASKED ONLY AFTER THIS MODULE'S OWN LAW: validation has
	// checked floor, footprints and bodies, which spatial's room knows nothing
	// of. Moves apply in an order that frees each destination first
	// (validateRelocation's chain order).
	out := &RelocateOutput{Moved: make([]Relocated, len(moves))}
	for _, i := range order {
		m := moves[i]
		// A MOVE SHOWS YOU, as casting does (sneaking R7).
		e.endSneaking(m.Member)
		if err := e.canvas.RelocateEntity(string(m.Member), m.To); err != nil {
			return nil, fmt.Errorf("relocate %q: %w: %w", m.Member, ErrBadPlacement, err)
		}
		if _, err := e.holdings.log.Append(journal.Fact{
			Kind:    relocatedKind(m.Member, m.To),
			Actor:   journal.EntityID(m.Member),
			Subject: memberSubject(m.Member),
			Outcome: journal.Outcome{Detail: in.Cause.Ref.String()},
		}); err != nil {
			return nil, fmt.Errorf("relocate %q: %w", m.Member, err)
		}
		out.Moved[i] = Relocated{Member: m.Member, From: m.from, To: m.To}
	}

	after, err := e.placedSightReach()
	if err != nil {
		return nil, fmt.Errorf("relocate: %w", err)
	}
	for i, m := range moves {
		seqs, err := e.tellRelocation(m, in.Cause, sawLeave[i], e.relocationWitnesses(after, down, m.Member), at)
		if err != nil {
			return nil, err
		}
		out.Seqs = append(out.Seqs, seqs...)
	}

	if e.hasApproaches() {
		if err := e.sweepAddresses(in.Reach); err != nil {
			return nil, fmt.Errorf("relocate: %w", err)
		}
	}
	if err := e.sweepHolds(); err != nil {
		return nil, fmt.Errorf("relocate: %w", err)
	}
	for r := 0; r < in.Cause.Rounds; r++ {
		raised := false
		for _, m := range moves {
			rose, err := e.advanceWorld(m.Member, 1)
			if err != nil {
				return nil, fmt.Errorf("relocate: %w", err)
			}
			raised = raised || rose
		}
		if raised {
			if err := e.wake(); err != nil {
				return nil, fmt.Errorf("relocate: %w", err)
			}
		}
	}

	deltas, formed, err := e.refreshSight(e.rosterIDs())
	if err != nil {
		return nil, fmt.Errorf("relocate: refresh sight: %w", err)
	}
	out.IntelDeltas, out.Formed = deltas, formed
	return out, nil
}

// relocationMove is one validated move and where its member stood.
type relocationMove struct {
	Relocation
	from spatial.Position
}

// validateRelocation refuses a relocation before anything changes
// ([Encounter.Relocate]'s errors) and answers each move with its origin, in
// the input's order, and the order to apply them in.
//
// THE STANDABLE-CELL LAW IS THIS MODULE'S, asked here and never left to the
// room: floor that is not covered by a blocking footprint
// ([Encounter.arrivalStandable], reserve arrival's law), and no body on it —
// unless every body there is a member who leaves it in this same call. Such a
// CHAIN applies its leaver first; a RING (members into each other's cells all
// round) has no first and is refused.
func (e *Encounter) validateRelocation(in *RelocateInput, custody bool) ([]relocationMove, []int, error) {
	switch {
	case in == nil:
		return nil, nil, fmt.Errorf("relocate: %w", ErrNilInput)
	case e.outcome != nil:
		return nil, nil, fmt.Errorf("relocate: %w", ErrClosed)
	case len(in.Moves) == 0:
		return nil, nil, fmt.Errorf("relocate: no member moves: %w", ErrNoMember)
	case in.Cause.Words == "":
		return nil, nil, fmt.Errorf("relocate: the cause tells the member nothing: %w", ErrInvalidData)
	case in.Cause.Rounds < 0:
		return nil, nil, fmt.Errorf("relocate: %d rounds: %w", in.Cause.Rounds, ErrInvalidData)
	case in.Reach < 0:
		return nil, nil, fmt.Errorf("relocate: reach %d: %w", in.Reach, ErrInvalidData)
	case e.Paused():
		return nil, nil, fmt.Errorf("relocate: %q is already waiting on an answer: %w", e.PausedMember(), ErrTurnPaused)
	}
	if err := in.Cause.Ref.IsValid(); err != nil {
		return nil, nil, fmt.Errorf("relocate: cause: %w: %w", ErrInvalidData, err)
	}
	moves := make([]relocationMove, 0, len(in.Moves))
	members := map[MemberID]bool{}
	cells := map[spatial.Position]MemberID{}
	for _, m := range in.Moves {
		if _, ok := e.members[m.Member]; !ok {
			return nil, nil, fmt.Errorf("relocate %q: %w", m.Member, ErrNotMember)
		}
		if members[m.Member] {
			return nil, nil, fmt.Errorf("relocate %q: moved twice: %w", m.Member, ErrInvalidData)
		}
		members[m.Member] = true
		from, placed := e.canvas.GetEntityPosition(string(m.Member))
		if !placed {
			return nil, nil, fmt.Errorf("relocate %q: %w", m.Member, ErrBadPlacement)
		}
		bubble, err := e.bubbleFor(m.Member)
		if err != nil && !errors.Is(err, errNotInBubble) {
			return nil, nil, fmt.Errorf("relocate %q: %w", m.Member, err)
		}
		if bubble != nil {
			return nil, nil, fmt.Errorf("relocate %q: in a fight: %w", m.Member, ErrInBubble)
		}
		if !custody && e.inCustody(m.Member) {
			return nil, nil, fmt.Errorf("relocate: %w", &CustodyMoveError{Mover: m.Member})
		}
		switch {
		case m.To == from:
			return nil, nil, fmt.Errorf("relocate %q: already at %v: %w", m.Member, m.To, ErrBadPlacement)
		case !e.arrivalStandable(m.To):
			return nil, nil, fmt.Errorf("relocate %q: nobody can stand at %v: %w", m.Member, m.To, ErrBadPlacement)
		}
		if other, taken := cells[m.To]; taken {
			return nil, nil, fmt.Errorf("relocate %q: %v is %q's too: %w", m.Member, m.To, other, ErrBadPlacement)
		}
		cells[m.To] = m.Member
		moves = append(moves, relocationMove{Relocation: m, from: from})
	}
	// BODIES: a destination somebody stands on is free only when everybody
	// there leaves it in this call.
	for _, m := range moves {
		for _, there := range e.canvas.GetEntitiesAt(m.To) {
			if !members[MemberID(there.GetID())] {
				return nil, nil, fmt.Errorf("relocate %q: %q stands at %v: %w", m.Member, there.GetID(), m.To, ErrBadPlacement)
			}
		}
	}
	order, err := relocationOrder(moves)
	if err != nil {
		return nil, nil, err
	}
	return moves, order, nil
}

// relocationOrder is the order to apply moves in so each destination is left
// before it is taken: a move whose destination no unapplied mover still
// stands on goes next, input order breaking ties. A ring has none and is
// refused with ErrBadPlacement.
func relocationOrder(moves []relocationMove) ([]int, error) {
	done := make([]bool, len(moves))
	order := make([]int, 0, len(moves))
	for len(order) < len(moves) {
		next := -1
		for i, m := range moves {
			if done[i] {
				continue
			}
			blocked := false
			for j, o := range moves {
				if !done[j] && j != i && o.from == m.To {
					blocked = true
					break
				}
			}
			if !blocked {
				next = i
				break
			}
		}
		if next < 0 {
			return nil, fmt.Errorf("relocate: members move into each other's cells all round, and none can go first: %w", ErrBadPlacement)
		}
		done[next] = true
		order = append(order, next)
	}
	return order, nil
}

// relocationWitnesses is every member up who sees member on the reach given,
// member excluded, in id order.
func (e *Encounter) relocationWitnesses(reach sightReach, down map[MemberID]bool, member MemberID) []MemberID {
	var out []MemberID
	for _, id := range e.rosterIDs() {
		if id != member && !down[id] && reach.Reaches(perception.Sight, id, member) {
			out = append(out, id)
		}
	}
	return out
}

// tellRelocation appends one move's [BeatRelocated] versions (this file's
// doc): the member's own, then both ends, the leaving, the arriving — each
// only to its readers, none when it has none.
func (e *Encounter) tellRelocation(m relocationMove, cause RelocationCause, left, arrived []MemberID, at uint64) ([]uint64, error) {
	var both, leftOnly, arrivedOnly []MemberID
	for _, id := range left {
		if slices.Contains(arrived, id) {
			both = append(both, id)
		} else {
			leftOnly = append(leftOnly, id)
		}
	}
	for _, id := range arrived {
		if !slices.Contains(left, id) {
			arrivedOnly = append(arrivedOnly, id)
		}
	}
	versions := []struct {
		audience []MemberID
		end      string
		from, to bool
		words    bool
	}{
		{[]MemberID{m.Member}, RelocatedBoth, true, true, true},
		{both, RelocatedBoth, true, true, false},
		{leftOnly, RelocatedLeft, true, false, false},
		{arrivedOnly, RelocatedArrived, false, true, false},
	}
	var seqs []uint64
	for _, v := range versions {
		if len(v.audience) == 0 {
			continue
		}
		payload := map[string]interface{}{
			"beat":   BeatRelocated,
			"member": string(m.Member),
			"cause":  cause.Ref.String(),
			"end":    v.end,
		}
		if v.from {
			payload["from"] = m.from
		}
		if v.to {
			payload["to"] = m.To
		}
		if v.words {
			payload["words"] = cause.Words
		}
		beat, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("relocated beat: %w", err)
		}
		appended, err := e.appendBeat(&record.AppendInput{
			At: at, Audience: v.audience, Tags: map[string]string{"tag": BeatRelocated}, Payload: beat,
		})
		if err != nil {
			return nil, fmt.Errorf("relocated beat: %w", err)
		}
		seqs = append(seqs, appended.Seq)
	}
	return seqs, nil
}

// FreeCellsIn is the first n cells of a region a member could be set down on
// now — standable floor nobody stands on — in the region's own coordinate
// order, so every run of one save picks the same cells (C8).
//
// Errors: ErrInvalidData for n below one; ErrBadPlacement when the region is
// unknown or has fewer than n free cells.
func (e *Encounter) FreeCellsIn(region RegionID, n int) ([]spatial.Position, error) {
	if n < 1 {
		return nil, fmt.Errorf("free cells in %q: %d: %w", region, n, ErrInvalidData)
	}
	cells, ok := e.field.regionCells[region]
	if !ok {
		return nil, fmt.Errorf("free cells in %q: no such region: %w", region, ErrBadPlacement)
	}
	out := make([]spatial.Position, 0, n)
	for _, c := range cells {
		if e.arrivalStandable(c) && !e.canvas.IsPositionOccupied(c) {
			out = append(out, c)
			if len(out) == n {
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("free cells in %q: %d wanted, %d free: %w", region, n, len(out), ErrBadPlacement)
}
