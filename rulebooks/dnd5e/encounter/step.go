// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// step.go is ONE STEP ON THE MAP, and everything that decides what one is.
//
// A step names a DUNGEON-ABSOLUTE cell and nothing else. That used to be half
// the work: the cell had to be located to a room, compared against the
// stepper's own, and then carried out by one of two mechanisms — a move within
// a room or a traversal between two of them. There is one room now
// (rpg-toolkit#1106), so there is one mechanism, and the dispatch that used to
// live here is gone rather than centralized.
//
// It was centralized first, and that mattered: the session SDK resolved a walk
// itself while the pump made the same decision here, off the raw connection
// inputs (rpg-toolkit#1059). The two agreed only for as long as neither learned
// anything the other had not. Making it one decision is what made it possible
// to see that the decision itself was the artifact of a world model — and to
// delete it.
//
// So there is one implementation, [Encounter.stepMember], and two ways in:
// [Encounter.Step], the public verb a host walks a party with, and
// [Encounter.stepTo], the silent one the pump moves monsters with. They differ
// in whether a refusal is REPORTED or simply means the monster failed to act
// this tick. They cannot differ in what is crossable.

// executedAction is one step that actually happened.
//
// Every cell here is DUNGEON-ABSOLUTE, because that is the frame the canvas
// works in and the frame every report speaks. There is nothing left to project.
type executedAction struct {
	member *memberRecord
	// connection names the doorway this step went through, or is empty. A
	// NAME, for narration — it grants nothing and refuses nothing.
	connection string
	// door names the door this step went through, or is empty. Unlike
	// connection this one COULD have refused the step — it just did not,
	// because it was open (rpg-toolkit#1123).
	door DoorID
	from spatial.Position
	to   spatial.Position
}

// Step moves a member ONE STEP to a dungeon-absolute cell and says what
// happened. If that cell is on the far side of a doorway, the step goes through
// the doorway — not as a different kind of movement, but because the cell on
// the other side of an opening is simply the next cell along.
//
// ONE STEP, not a walk. Walking is the seam's job, because anything that fires
// because a member entered a PARTICULAR CELL can only be noticed by something
// that visits each of them in turn; this verb is what such a loop calls.
//
// ADJACENCY IS NOT CHECKED, deliberately. Step is the reporting twin of the
// silent stepTo the pump moves monsters with, and a decider's IntentMoveTo has
// never carried an adjacency contract. What "one step" means for a WALK is a
// rule about walking, and it lives with the walk. The same goes for the
// zero-distance step (rpg-toolkit#1060): stepping onto the cell you already
// occupy is a legal, if pointless, placement here and is refused at the seam
// that knows it is a phantom.
//
// Validation order (R5 atomicity): nil input → empty member → closed → not a
// member → in a fight → the step itself. Returns ErrNilInput, ErrNoMember,
// ErrClosed, ErrNotMember, ErrInBubble for a member mid-fight (free roam is a
// world-clock verb), or ErrBadPlacement — for a cell no authored room owns
// (void is not floor), for a wall in the way, or for any other placement the
// canvas refuses.
func (e *Encounter) Step(in *StepInput) (*StepOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("step: %w", ErrNilInput)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("step: %w", ErrNoMember)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("step: %w", ErrClosed)
	}

	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("step: %w", ErrNotMember)
	}

	// The same world-clock gate every free-roam verb carries. A member caught
	// in a bubble acts through the fight's own turn structure — there is no
	// in-fight movement verb yet, and until there is, a fight member cannot
	// move at all rather than moving outside initiative.
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("step: %w", err)
	}
	if bubble != nil {
		return nil, fmt.Errorf("step: member %q: %w", in.Member, ErrInBubble)
	}

	action, err := e.stepMember(member, in.To)
	if err != nil {
		return nil, fmt.Errorf("step: %w", err)
	}

	audience := e.rosterIDs()
	at := uint64(e.clock.ToData().HighWater)

	// The beat BEFORE the sight refresh: the step is the cause, anything
	// trigger detection appends is its effect (the law is stated at
	// refreshSight).
	seq, err := e.appendMovementBeat(action, audience, at)
	if err != nil {
		return nil, fmt.Errorf("step append beat: %w", err)
	}

	intelDeltas, formed, err := e.refreshSight(audience)
	if err != nil {
		return nil, fmt.Errorf("step refresh sight: %w", err)
	}

	out := &StepOutput{
		Door:        action.door,
		Crossing:    action.connection,
		IntelDeltas: intelDeltas,
		Seq:         seq,
		Outcome:     e.firedReachedPosition(member, action.to, at),
		Formed:      formed,
	}
	out.Stepped.Member = in.Member
	out.Stepped.From = action.from
	out.Stepped.To = action.to
	return out, nil
}

// stepMember executes one step to a DUNGEON-ABSOLUTE cell and says why not
// when it cannot.
//
// Two things decide a step, and neither is a room. FLOOR: some authored
// chamber's footprint has to hold the destination, because the canvas spans the
// field's bounding box and the space between chambers is not floor. WALLS: the
// canvas refuses a move whose ray crosses a movement-blocking boundary, which
// is what the seam between two chambers now is when nobody cut a doorway in it.
//
// That second one used to be the connection list's job — "W2 lets two rooms
// share an edge without a door, so absolute adjacency alone is not permission
// to cross". It was doing that job because a wall on a room's edge could not be
// expressed (see compileCanvas); it can be now, so permission is geometry like
// everything else, and a step into a wall is refused exactly as a step into any
// other wall is.
func (e *Encounter) stepMember(member *memberRecord, to spatial.Position) (executedAction, error) {
	// Hex fields require integral axial cells (interim tools/spatial#926
	// enforcement — see isIntegralAxialPosition). Asked BEFORE the floor
	// question so a fractional cell is named as itself: regionAt refuses one
	// too, and refuses it by this very check (rpg-toolkit#1108), but it would
	// report it as "not floor" — sending a caller to the map instead of to its
	// arithmetic.
	if !isIntegralAxialPosition(e.canvas.GetGrid(), to) {
		return executedAction{}, fmt.Errorf("target is not an integral axial cell: %w", ErrBadPlacement)
	}

	if _, owned := e.RegionAt(to); !owned {
		return executedAction{}, fmt.Errorf("cell %v is not floor: %w", to, ErrBadPlacement)
	}

	// A SHUT DOOR REFUSES BY NAME (rpg-toolkit#1123). The canvas would refuse
	// this step anyway — a closed door's edges are movement-blocking
	// boundaries and spatial stops on them like any wall — and "cannot cross
	// movement-blocking boundary" is the wrong sentence for a door. A wall is
	// a fact about the dungeon; a shut door is a thing with a state, and the
	// state is what a caller does something about. So the door is named, and
	// so is what state it is in.
	//
	// Asked BEFORE the move rather than mapped from its error, because the
	// member's cell is only knowable while they are still standing on it and
	// spatial's refusal does not say which crossing it stopped at.
	if here, placed := e.canvas.GetEntityPosition(string(member.ID)); placed {
		if door := e.doorAcross(here, to); door != nil && door.state.blocks() {
			return executedAction{}, fmt.Errorf("door %q is %s: %w", door.id, door.state.Kind(), ErrBadPlacement)
		}
	}

	from, err := e.moveMember(member, to)
	if err != nil {
		return executedAction{}, err
	}

	action := executedAction{
		member:     member,
		connection: e.crossingOf(from, to),
		from:       from,
		to:         to,
	}
	if door := e.doorAcross(from, to); door != nil {
		action.door = door.id
	}

	return action, nil
}

// stepTo is the pump's way in: the same step, refused SILENTLY.
//
// Every refusal is reported as not-stepped rather than as an error, which is
// the contract a spatially-rejected move already had — a monster that cannot
// act simply does not act this tick, and never aborts the pump.
//
// It decides nothing of its own. Sharing stepMember with the public verb is
// what makes "what is crossable" a single answer: a rule added there reaches a
// player's walk and a monster's pursuit in the same commit, and there is no
// second copy left to forget it.
func (e *Encounter) stepTo(member *memberRecord, to spatial.Position) (executedAction, bool) {
	action, err := e.stepMember(member, to)
	if err != nil {
		return executedAction{}, false
	}
	return action, true
}

// crossingOf names the doorway joining two absolute cells, in either direction,
// or returns empty.
//
// A NAME AND NOTHING MORE (rpg-toolkit#1106). Its predecessor, doorwayFrom, was
// the permission function: false meant a step between two touching rooms was
// refused, because a wall on a room's edge was inexpressible and the connection
// list had to stand in for one. Walls are boundary edges on the canvas now, so
// nothing about a step consults this — it runs AFTER the move succeeded, purely
// so a host narrating "she slips through the gate" does not have to rediscover
// which gate from the Atlas.
func (e *Encounter) crossingOf(from, to spatial.Position) string {
	for _, c := range e.connectionsInput {
		near := c.FromPosition.Add(roomOrigin(e.fieldInput, c.From))
		far := c.ToPosition.Add(roomOrigin(e.fieldInput, c.To))
		if (near == from && far == to) || (far == from && near == to) {
			return c.ID
		}
	}
	return ""
}
