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
// ([cellQuery.floodFrom], `opens`); the walk asks it again from where the
// creature actually stands so the door it opens is the one in front of it
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
// would open, or nil when the step opens nothing.
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
func (q *cellQuery) doorToOpen(walker MemberID, from, to spatial.Position) (*doorRecord, error) {
	prop, blocked, err := q.crossingBlocked(from, to)
	if err != nil {
		return nil, err
	}

	return q.doorPast(walker, from, to, prop, blocked)
}

// doorPast is [cellQuery.doorToOpen] for a caller that has already asked the
// crossing fold ([Encounter.crossingBlocked]) and holds its answer — the
// route flood, which asks it of every edge and must not pay for asking it
// twice. The fold NAMES what closes the crossing, which is what keeps this
// cheap where it is asked most: a wall's crossing names the wall, which is
// not a door, and is answered by one map read rather than a trace.
func (q *cellQuery) doorPast(walker MemberID, from, to spatial.Position, prop PropID, blocked bool) (*doorRecord, error) {
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
			return nil, nil
		}
		if _, across, err := q.field.crossingBlocks(from, to); err != nil || across {
			return nil, err
		}
		if q.field.doorStandingOn(to) != nil {
			return nil, nil
		}

		return d, nil

	case blocked:
		// A FOOTPRINT ACROSS THE CROSSING, named by the fold. Only a door's
		// name is a door; and with that door left out, nothing else may lie
		// across the crossing or stand on the cell as a second door.
		d := q.doorsByID[DoorID(prop)]
		if d == nil || d.placement == nil || !opens(d) {
			return nil, nil
		}
		if _, across, err := q.field.crossingBlocksExcept(from, to, d.id); err != nil || across {
			return nil, err
		}
		if on := q.field.doorStandingOn(to); on != nil && on != d {
			return nil, nil
		}

		return d, nil
	}

	// A FOOTPRINT DOOR STANDING ON THE DESTINATION, with the crossing clear
	// and nothing but the door blocking the cell.
	d := q.field.doorStandingOn(to)
	if !opens(d) {
		return nil, nil
	}
	fact, err := q.cellAt(CellAtInput{Cell: to, Mover: walker})
	if err != nil {
		return nil, err
	}
	if !blockedOnlyBy(fact, d) {
		return nil, nil
	}

	return d, nil
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

// openAhead opens the door in front of a walking creature, when the next
// cell of its walk is through one, and reports whether it did.
//
// ASKED FROM WHERE THE CREATURE STANDS, not where its route was planned
// from: the door opened is the one between it and the next cell, by the
// same [cellQuery.doorToOpen] the route read. Nothing to open — no door, a
// locked one, a hidden one, or something else in the way — answers false and
// opens nothing, and the walk stops there exactly as it stopped before doors
// could be opened.
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
	door, err := query.doorToOpen(walker, from, next)
	if err != nil {
		return false, fmt.Errorf("door ahead: %w", err)
	}
	if door == nil {
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

// walkOn is the part of a walk still to take after its first `taken` cells,
// when those cells ended in front of a door the walker has just opened — or
// nil when the walk is over.
//
// THE WALK IS TAKEN IN LEGS, DOOR TO DOOR. [Encounter.standableEnd] trims a
// walk where a shut crossing stops it, which is where a door is; so a walk
// that opens doors takes the trimmed leg, opens the door the leg stopped in
// front of, and asks for the next leg from there with the door now open. A
// leg that stopped short for any other reason — a budget cut onto an ally, a
// wall, a creature in the way — is the end of the walk, as it always was.
//
// `opens` is false for a walk nobody chose (a directive), which never opens
// anything.
func (e *Encounter) walkOn(walker MemberID, path []spatial.Position, taken int, cause core.Ref, opens bool) ([]spatial.Position, bool, error) {
	if !opens || taken >= len(path) {
		return nil, false, nil
	}
	opened, err := e.openAhead(walker, path[taken], cause)
	if err != nil || !opened {
		return nil, false, err
	}

	return path[taken:], true, nil
}
