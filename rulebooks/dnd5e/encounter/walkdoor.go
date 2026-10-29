// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// walkdoor.go is A WALKING CREATURE OPENS THE DOOR IT WALKS INTO
// (rpg-project#715, Frank's list S1; the primitive question
// rpg-toolkit#2069).
//
// Until this, a driven walk routed only through open doors and no creature
// opened one, so every patrol, march and pursuit stopped at the first shut
// door and a castle had to leave its throne door open for the AI's sake. Now
// a creature's OWN walk — its table's `toward`, `dash`, `away`, a pursuit, a
// search, a patrol leg, on either clock — treats a closed, unlocked door as
// passable at the price of the step onto it, and the encounter opens the
// door before that step is taken.
//
// # One question, asked by the route and by the walk
//
// [cellQuery.doorToOpen] is the only place that decides which door a step
// would open. The route flood asks it so the plan goes through the door
// ([cellQuery.floodFrom], `opens`); the walk's end-of-walk trim asks it so the
// walk is cut where its walker can stand with those doors open
// ([Encounter.walkableEnd]); and each step asks it again from where the
// creature actually stands, so the door it opens is the one in front of it
// ([Encounter.openAhead]). A route and a walk that asked different questions
// would hand each other paths the other refuses — rpg-toolkit#1652, one noun
// later.
//
// # One open, the player's
//
// The open is [Encounter.changeDoorState] — the same state write, edge
// registration, door beat, revealed cells and `opened: { fact }` landing
// [Encounter.OpenDoor] performs for a player — and only the sight refresh is
// left to the walk, which already refreshes once after its steps (or, on the
// world clock, leaves it to the verb that raised the clock). There is no
// second open.
//
// # What it never opens
//
//   - A LOCKED door (lane rpg-project#722, D5: never, for S1; a creature
//     carrying a key is the Grant story's neighbour). It blocks the route and
//     the walk exactly as before, and the creature stops where it always did.
//   - A door HIDDEN to the walker: what it has not found is wall to it, and
//     the masquerade law already keeps its route off one.
//   - Anything on a walk that is nobody's choice: a push, a pull, a rout
//     ([Encounter.Direct]) and the public [Encounter.Route] stop at a shut
//     door as they always have. Being shoved into a door is not opening it.
//
// # What it costs
//
// Nothing beyond the step (this slice's ruling). The open is part of the
// walk; pricing it as an Interact is the player's verb's business, and this
// spends no world action and no turn budget.

// doorToOpen is the door a creature's own step from one cell to the next
// would open, and whether there is one: ok=false, with a nil door, when the
// step opens nothing — no door in the way, or one this walk may not open.
// The error is a failed geometry read, never "no door".
//
// A DOOR IS OPENED ONLY WHEN IT IS ALL THAT CLOSES THE STEP. Exactly one
// closed, unlocked door the walker can see as a door — an edge door standing
// in the crossing, a footprint door lying across it, or a footprint door
// standing on the destination — and nothing else in the way once it is open:
// no second door, no wall, no table across the same crossing, and nothing
// else on the destination's centre. A step that would still be refused with
// the door open is not a step through a door, and opening it would be the
// creature doing something for nothing.
//
// The destination's OTHER facts — a creature on it, scenery, void — are not
// this question's: the caller asks them of the cell as it always has, and a
// door standing open says nothing about them.
func (q *cellQuery) doorToOpen(walker MemberID, from, to spatial.Position) (*doorRecord, bool, error) {
	prop, blocked, err := q.crossingBlocked(from, to)
	if err != nil {
		return nil, false, err
	}

	return q.doorPast(walker, from, to, prop, blocked)
}

// doorPast is [cellQuery.doorToOpen] for a caller that has already asked the
// crossing fold ([Encounter.crossingBlocked]) and holds its answer — the
// route flood, which asks it of every edge and must not pay for asking it
// twice. The fold NAMES what closes the crossing, which is what keeps this
// cheap where it is asked most: a wall's crossing names the wall, which is
// not a door, and is answered by one map read rather than a trace.
//
// Same answer shape as doorToOpen: the door and ok=true, or nil and
// ok=false when the step opens nothing; an error only for a failed read.
func (q *cellQuery) doorPast(
	walker MemberID, from, to spatial.Position, prop PropID, blocked bool,
) (*doorRecord, bool, error) {
	opens := func(d *doorRecord) bool {
		return d != nil && d.state.Kind() == DoorClosed && !q.hiddenDoorTo(walker, d.id)
	}

	switch {
	case blocked && prop == "":
		// A BOUNDARY, which for a door is AN EDGE DOOR IN ITS CROSSING. No
		// wall stands in a door's crossing (validateDoorInputs refuses one),
		// so a blocked boundary with a door on it is that door; what else
		// might close the crossing is a footprint across it or on the cell.
		d := q.doorOnEdge(from, to)
		if !opens(d) {
			return nil, false, nil
		}
		_, across, err := q.field.crossingBlocks(from, to)
		if err != nil {
			return nil, false, err
		}
		if across || q.field.doorStandingOn(to) != nil {
			return nil, false, nil
		}

		return d, true, nil

	case blocked:
		// A FOOTPRINT ACROSS THE CROSSING, named by the fold. Only a door's
		// name is a door; and with that door left out, nothing else may lie
		// across the crossing or stand on the cell as a second door.
		d := q.doorsByID[DoorID(prop)]
		if d == nil || d.placement == nil || !opens(d) {
			return nil, false, nil
		}
		_, across, err := q.field.crossingBlocksExcept(from, to, d.id)
		if err != nil {
			return nil, false, err
		}
		if across {
			return nil, false, nil
		}
		if on := q.field.doorStandingOn(to); on != nil && on != d {
			return nil, false, nil
		}

		return d, true, nil
	}

	// A FOOTPRINT DOOR STANDING ON THE DESTINATION, with the crossing clear
	// and nothing but the door blocking the cell.
	d := q.field.doorStandingOn(to)
	if !opens(d) {
		return nil, false, nil
	}
	fact, err := q.cellAt(CellAtInput{Cell: to, Mover: walker})
	if err != nil {
		return nil, false, err
	}
	if !blockedOnlyBy(fact, d) {
		return nil, false, nil
	}

	return d, true, nil
}

// blockedOnlyBy reports whether the one thing closing a cell to its mover
// is this door, standing on it as a footprint — the cell a creature's walk
// may enter because the step onto it opens the door ([cellQuery.doorToOpen]).
func blockedOnlyBy(fact CellFact, door *doorRecord) bool {
	for _, c := range fact.Contribs {
		if c.Blocks && (c.Kind != ContribDoor || c.ID != string(door.id)) {
			return false
		}
	}

	return true
}

// standsADoor reports whether a shut footprint door is among what closes a
// cell — the one case a cell's own fact can be a door this walk would open.
func standsADoor(fact CellFact) bool {
	for _, c := range fact.Contribs {
		if c.Blocks && c.Kind == ContribDoor {
			return true
		}
	}

	return false
}

// walkableEnd is [Encounter.standableEnd] for a walk that OPENS DOORS: the
// same two cuts — at the first step the walk cannot take, then back to the
// last cell it may stop on — judged over the WHOLE path, with every door the
// walk would open on its way standing open.
//
// WHOLE PATH, NOT DOOR TO DOOR, which is the bug it replaced (review of
// rpg-toolkit#2150). Trimming each stretch between doors on its own cut a
// stretch back from the cell in front of a door whenever an ally stood
// there: passable, not standable, so the stretch ended a cell short, the
// step left to ask about was the step onto the ally, it opened nothing, and
// the march stood still every round with its route through the door. Judged
// whole, the ally's cell is crossed and the walk ends on the last cell it
// may stand on, past the door or before it.
//
// A door a step opens is judged as the open door: an edge or a door across
// the crossing leaves the far cell's own fact to answer; a footprint door
// standing on the cell is crossed and never stopped on — nobody ends a walk
// inside a door leaf. A walk that opens nothing (`opens` false: a directive)
// is exactly standableEnd.
//
// A sibling of standableEnd rather than a flag on it only because that file
// is in flight on a parallel slice; folding the two into one is a follow-up.
func (e *Encounter) walkableEnd(mover MemberID, path []spatial.Position, opens bool) ([]spatial.Position, error) {
	if !opens {
		return e.standableEnd(mover, path)
	}
	prev, placed := e.canvas.GetEntityPosition(string(mover))
	if !placed {
		return nil, nil
	}
	query, err := e.cellQuery()
	if err != nil {
		return nil, fmt.Errorf("walk end: %w", err)
	}
	end := -1
	for i, cell := range path {
		fact, crossed, err := query.walkInto(mover, prev, cell)
		if err != nil {
			return nil, fmt.Errorf("walk end at %v: %w", cell, err)
		}
		if !crossed || fact.Passage == PassageBlocked {
			break
		}
		if fact.Passage == PassageStandable {
			end = i
		}
		prev = cell
	}
	if end < 0 {
		return nil, nil
	}

	return path[:end+1], nil
}

// walkInto is [cellQuery.stepInto] for a walk that opens doors: a step
// through a door it would open is crossed, and the far cell is judged as it
// stands once the door is open.
func (q *cellQuery) walkInto(mover MemberID, prev, cell spatial.Position) (CellFact, bool, error) {
	door, ok, err := q.doorToOpen(mover, prev, cell)
	if err != nil {
		return CellFact{}, false, err
	}
	if !ok {
		return q.stepInto(mover, prev, cell)
	}
	fact, err := q.cellAt(CellAtInput{Cell: cell, Mover: mover})
	if err != nil {
		return CellFact{}, false, err
	}
	if fact.Passage == PassageBlocked && blockedOnlyBy(fact, door) {
		fact.Passage = PassagePassThrough
	}

	return fact, true, nil
}

// openAhead opens the door in front of a walking creature, when the next
// cell of its walk is through one, and reports whether it did.
//
// ASKED FROM WHERE THE CREATURE STANDS, not where its route was planned
// from: the door opened is the one between it and the next cell, by the
// same [cellQuery.doorToOpen] the route read. Nothing to open — no door, a
// locked one, a hidden one, or something else in the way — answers false and
// opens nothing, and the step goes on to meet whatever is there, as it did
// before doors could be opened.
//
// THE ACTOR IS THE WALKER and the beat names it, so the record reads "gate-1
// opens the kitchen door". The cause, when the walk has one (a table's
// `toward`, `away`, `search` or `dash`), rides the beat beside it, so a
// reader can tell which word walked the creature through; a walk that names
// no cause — a creature's own Move — leaves it off, as its moved beats do.
func (e *Encounter) openAhead(walker MemberID, next spatial.Position, cause core.Ref) (bool, error) {
	from, placed := e.canvas.GetEntityPosition(string(walker))
	if !placed {
		return false, nil
	}
	query, err := e.cellQuery()
	if err != nil {
		return false, fmt.Errorf("door ahead: %w", err)
	}
	door, ok, err := query.doorToOpen(walker, from, next)
	if err != nil {
		return false, fmt.Errorf("door ahead: %w", err)
	}
	if !ok {
		return false, nil
	}

	extra := map[string]interface{}{"actor": string(walker)}
	if cause.IsValid() == nil {
		extra["cause"] = cause.String()
	}
	if _, err := e.changeDoorState(door, DoorIsOpen(), walker, extra); err != nil {
		return false, fmt.Errorf("open door %q on the way: %w", door.id, err)
	}

	return true, nil
}
