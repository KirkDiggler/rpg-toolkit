// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// step.go is ONE STEP ON THE MAP, and everything that decides what one is.
//
// A step names a DUNGEON-ABSOLUTE cell and nothing else. Whether that cell is
// somewhere inside the stepper's own room or on the far side of a doorway is
// this file's business to work out — off this composition's own connection
// data, in one place, for every caller.
//
// It is one place because it was two (rpg-toolkit#1059). The session SDK
// resolved a walk itself: locate each cell, compare rooms, scan the projected
// Atlas doorway list, then choose between the Move verb and the Traverse verb.
// The pump made the same decision here, off the raw connection inputs. The two
// agreed only for as long as neither learned anything the other had not — and
// the first rule that tells doorways apart, a locked door, a one-way door, a
// doorway spanning more than one cell pair, would have had to land in both, in
// lockstep, forever. Until it missed once, and a player walked somewhere a
// monster could not follow.
//
// So there is one implementation, [Encounter.stepMember], and two ways in:
// [Encounter.Step], the public verb a host walks a party with, and
// [Encounter.stepTo], the silent one the pump moves monsters with. They differ
// in whether a refusal is REPORTED or simply means the monster failed to act
// this tick. They cannot differ in what is crossable.

// The two kinds of step the composition can carry out. Constants rather than
// literals because they are matched in several places — built in stepMember,
// then switched on for beats and for the pump's output — and a typo in any one
// of them silently drops an action from a transcript.
const (
	actionMove     = "move"
	actionTraverse = "traverse"
)

// executedAction is one step that actually happened: a move within a room, or
// a crossing through a doorway. Only [Encounter.stepMember] builds one.
//
// Every cell here is ROOM-LOCAL, because that is the frame the mechanisms
// underneath work in. Rooms do not cross the seam, so anything that reports
// one of these projects it through [Encounter.absoluteOf] first.
type executedAction struct {
	member     *memberRecord
	kind       string // actionMove or actionTraverse
	connection string // only meaningful for actionTraverse
	fromRoom   string // the room departed; for a move, the member's own room
	from       spatial.Position
	toRoom     string // the room arrived in; for a move, the member's own room
	to         spatial.Position
}

// Step moves a member ONE STEP to a dungeon-absolute cell, crossing a doorway
// if that is what the cell turns out to be, and says what happened.
//
// This is the movement verb to reach for. [Encounter.Move] is same-room only
// and takes a room-local cell, which is the composition's own bookkeeping and
// the source of the Locate→Move trap documented on it; [Encounter.Traverse]
// takes a connection id, which is the mechanism rather than the intent. Step
// takes the one thing a caller actually has — a cell on the map — and works
// the rest out.
//
// ONE STEP, not a walk. Walking is the seam's job, because anything that fires
// because a member entered a PARTICULAR CELL can only be noticed by something
// that visits each of them in turn; this verb is what such a loop calls.
//
// ADJACENCY IS NOT CHECKED, deliberately. Step is the reporting twin of the
// silent stepTo the pump moves monsters with, and a decider's IntentMoveTo has
// never carried an adjacency contract — the composition's Move accepts any
// cell of the room. What "one step" means for a WALK is a rule about walking,
// and it lives with the walk. The same goes for the zero-distance step
// (rpg-toolkit#1060): stepping onto the cell you already occupy is a legal, if
// pointless, placement here and is refused at the seam that knows it is a
// phantom.
//
// Validation order (R5 atomicity): nil input → empty member → closed → not a
// member → in a fight → the step itself. Returns ErrNilInput, ErrNoMember,
// ErrClosed, ErrNotMember, ErrInBubble for a member mid-fight (free roam is a
// world-clock verb), ErrBadPlacement for a cell no room owns or a placement
// the field rejects, or ErrNoCrossing for a cell in another room with no
// doorway joining it to where the member stands.
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

	// The same world-clock gate Move and Traverse carry. A member caught in a
	// bubble acts through the fight's own turn structure — there is no in-fight
	// movement verb yet, and until there is, a fight member cannot move at all
	// rather than moving outside initiative.
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
	// refreshSight). A step narrates exactly as the verb whose mechanism
	// carried it — "moved" within a room, "traversed" through a doorway — so a
	// host reading the story cannot tell which verb produced a movement.
	seq, err := e.appendMovementBeat(action, audience, at)
	if err != nil {
		return nil, fmt.Errorf("step append beat: %w", err)
	}

	intelDeltas, formed, err := e.refreshSight(audience)
	if err != nil {
		return nil, fmt.Errorf("step refresh sight: %w", err)
	}

	out := &StepOutput{
		Crossing:    action.connection,
		IntelDeltas: intelDeltas,
		Seq:         seq,
		Outcome:     e.firedReachedPosition(member, action.to, at),
		Formed:      formed,
	}
	out.Stepped.Member = in.Member
	out.Stepped.From = e.absoluteOf(action.fromRoom, action.from)
	out.Stepped.To = e.absoluteOf(action.toRoom, action.to)
	return out, nil
}

// stepMember executes one step to a DUNGEON-ABSOLUTE cell and says why not
// when it cannot.
//
// THE ONE PLACE the same-room-or-through-a-doorway decision is made, for every
// caller. The dispatch that used to be the decider's job lives here, and it is
// the whole reason IntentTraverse could retire: W3 makes a doorway's two
// endpoints ADJACENT ABSOLUTE CELLS, so "walk to the cell on the other side of
// the doorway" and "walk to the cell next to me" are the same sentence. Which
// of the two mechanisms carries it out is bookkeeping, and bookkeeping belongs
// on this side of the seam.
//
// Four ways to be refused, and they are told apart because the remedies
// differ: a cell no room owns (void is not floor — ErrBadPlacement), a cell in
// another room with no doorway joining it to where the member stands
// (ErrNoCrossing — the cell is real and there is simply no way through), the
// spatial rejections the move or the crossing itself raises (ErrBadPlacement),
// and this composition being unable to say where the member is standing at all
// (ErrNoField, from cellOf — an invariant break, not a caller mistake, and it
// must not arrive wearing a doorway's name).
func (e *Encounter) stepMember(member *memberRecord, to spatial.Position) (executedAction, error) {
	located, err := e.Locate(&LocateInput{Position: to})
	if err != nil {
		return executedAction{}, err
	}

	if located.Room == member.Room {
		fromPos, moveErr := e.moveMember(member, located.Position)
		if moveErr != nil {
			return executedAction{}, moveErr
		}
		return executedAction{
			member: member, kind: actionMove,
			fromRoom: member.Room, from: fromPos,
			toRoom: member.Room, to: located.Position,
		}, nil
	}

	// Where the member stands, resolved BEFORE the doorway lookup so that a
	// failure to answer that question is reported as itself. Folding it into
	// the lookup let "this member is not placed in any room I know" come back
	// as ErrNoCrossing — a refusal that names a door, for a member who has no
	// position at all, sending whoever reads it to the map instead of to the
	// roster.
	local, cerr := e.cellOf(member)
	if cerr != nil {
		return executedAction{}, cerr
	}

	connection, ok := e.doorwayFrom(e.absoluteOf(member.Room, local), to)
	if !ok {
		return executedAction{}, fmt.Errorf(
			"member %s: no doorway joins where they stand to %v: %w", member.ID, to, ErrNoCrossing)
	}

	result, travErr := e.traverseMember(member, connection)
	if travErr != nil {
		return executedAction{}, travErr
	}
	return executedAction{
		member: member, kind: actionTraverse, connection: connection,
		fromRoom: result.fromRoom, from: result.fromPos,
		toRoom: result.toRoom, to: result.toPos,
	}, nil
}

// stepTo is the pump's way in: the same step, refused SILENTLY.
//
// Every refusal is reported as not-stepped rather than as an error, which is
// the contract a spatially-rejected move already had and the one IntentTraverse
// documented for an illegal crossing — a monster that cannot act simply does
// not act this tick, and never aborts the pump.
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

// doorwayFrom finds the connection joining two absolute cells, in either
// direction — a doorway is crossable both ways.
//
// Returns false when no connection joins the two, and that is the ONLY thing
// it can mean. It used to take the member and read their cell itself, which
// gave it a second way to answer false — the roster and the field disagreeing
// about where somebody is — indistinguishable from the first at the call site.
// A lookup with nothing to fail at cannot conflate anything.
//
// False is what makes a step between rooms that merely TOUCH refuse: W2 lets
// two rooms share an edge without a door, so absolute adjacency alone is not
// permission to cross.
func (e *Encounter) doorwayFrom(from, to spatial.Position) (string, bool) {
	for _, c := range e.connectionsInput {
		near := e.absoluteOf(c.From, c.FromPosition)
		far := e.absoluteOf(c.To, c.ToPosition)
		if (near == from && far == to) || (far == from && near == to) {
			return c.ID, true
		}
	}
	return "", false
}
