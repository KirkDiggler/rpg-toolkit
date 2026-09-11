// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// directive.go is AN EFFECT MOVES A CREATURE — the half of that sentence this
// module owns (rpg-project#430, the directed-movement design).
//
// The split is the whole design. A rule says "this creature moves, this way,
// this far"; it does not say which cells, because which cells is geometry read
// through the fold, and that is this module's question and nobody else's. So
// there are two verbs here and they are deliberately separable:
//
//   - [Encounter.Route] READS. Given a policy, an anchor and a budget, it says
//     which cells the move would cross and — when it is shorter than the budget
//     — why it stops where it does. It changes nothing, which is what lets the
//     behavior lane ask it for a monster's own flee and then submit the answer
//     as an ordinary Move intent.
//   - [Encounter.Direct] WRITES. It walks cells somebody already chose, through
//     the same [Mover] seam, the same [Encounter.stepTo] and the same beats a
//     walk uses, and every beat it appends names the cause.
//
// # It is the walk, not a second one
//
// [Encounter.Direct] and the monster's own Move intent run the SAME body
// ([Encounter.walkPath]). A directed move that walked its own loop would be the
// second searcher rpg-toolkit#1652 was about, one layer up: two answers to
// "what stops a step", drifting apart the first time either learns something.
//
// Two things differ, and they are both about whose move it is. A directive does
// not require the mover to hold the active turn, because it happens during
// somebody else's action; and it charges no turn budget, because it is not the
// mover's turn being spent.

// MovePolicy names HOW a directive measures the move it describes — the closed
// set of shapes an effect may ask for.
//
// It is a string rather than an int so a persisted directive round-trips as the
// word its author wrote, the way [OrientationKind] and [ContribKind] do.
type MovePolicy string

// MoveLine continues the line from the anchor THROUGH the mover, past them, for
// as many cells as the budget pays for. It is the push: Thunderwave's blast
// blows a creature straight away from where the caster stands.
//
// IT IS THE ONLY ONE, and that is the rule rather than the schedule. A policy
// arrives with its executor — "away" comes with Dissonant Whispers, "toward"
// with Thorn Whip — because a constant declared ahead of the thing that carries
// it out is a name callers can validate against and nothing can honour.
// [Encounter.Route]'s switch is closed on this single value and refuses every
// other word with [ErrUnsupportedPolicy], so the day a second one is added, the
// place that must learn about it is the place that already refuses it.
const MoveLine MovePolicy = "line"

// RouteInput asks which cells a directed move would cross.
type RouteInput struct {
	// Mover is who is being moved. Their current cell is read off the
	// canvas, not supplied, for [Encounter.placementOf]'s reason: two reads
	// of where somebody stands is the dual-state defect this composition has
	// paid for before.
	Mover MemberID

	// Policy is how the move is measured. Required, and [MoveLine] is the
	// only one that exists: the zero value is not a default, it is a refusal
	// ([ErrUnsupportedPolicy]), and so is any other word.
	Policy MovePolicy

	// Anchor is the cell the policy is measured FROM — the caster's cell for
	// a push, the thing being fled for a rout. Dungeon-absolute, like every
	// other cell on this module.
	Anchor spatial.Position

	// Budget is how many cells the move pays for. Zero is a legal, pointless
	// directive that routes nowhere; negative is [ErrBadReach].
	Budget int
}

// RouteOutput is the cells a directed move would cross, and why it is short.
type RouteOutput struct {
	// Path EXCLUDES the mover's own cell and is in walking order. Empty when
	// the policy yields nothing — a mover with a wall at their back is not an
	// error, it is a creature that does not move.
	Path []spatial.Position

	// StoppedBy is why Path is shorter than Budget, as the fold's own refusal
	// phrase ("is blocked by dnd5e:props:pillar"). Empty when the whole
	// budget was routed.
	//
	// THE ROUTE OWNS THIS SENTENCE, not the walk. The route is what decided
	// to stop in front of the pillar; by the time [Encounter.Direct] walks
	// the cells the route handed it, the pillar is not in them and the walk
	// has nothing left to say about it. [DirectOutput.StoppedBy] answers the
	// different question of whether the WALK fell short of its own route.
	StoppedBy string
}

// Route says which cells a directed move would cross, and why it stops where
// it does.
//
// IT READS [Encounter.CellAt] FOR EVERY CELL, exactly as [Encounter.routeTo]
// does and for the same reason (rpg-toolkit#1652): a route and a step that read
// different folds hand each other paths the other refuses. A creature in the
// way stops a push — nothing passes through anyone — so a cell the fold calls
// anything but Standable ends the route before it.
//
// IT CHANGES NOTHING. A caller may ask, trim the answer to what was actually
// paid for, and walk it with [Encounter.Direct], or ask and never walk at all.
//
// Refusals: [ErrNilInput] is not reachable (the input is a value), but
// [ErrNoMember], [ErrClosed], [ErrNotMember], [ErrBadReach] for a negative
// budget or an anchor standing on the mover, and [ErrUnsupportedPolicy] for
// anything that is not [MoveLine], all are.
func (e *Encounter) Route(in RouteInput) (RouteOutput, error) {
	if in.Mover == "" {
		return RouteOutput{}, fmt.Errorf("route: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return RouteOutput{}, fmt.Errorf("route: %w", ErrClosed)
	}
	m, ok := e.members[in.Mover]
	if !ok {
		return RouteOutput{}, fmt.Errorf("route %q: %w", in.Mover, ErrNotMember)
	}
	if in.Budget < 0 {
		return RouteOutput{}, fmt.Errorf("route %q: budget %d cells: %w", in.Mover, in.Budget, ErrBadReach)
	}
	if in.Policy != MoveLine {
		return RouteOutput{}, fmt.Errorf("route %q: policy %q: %w", in.Mover, in.Policy, ErrUnsupportedPolicy)
	}

	from, err := e.cellOf(m)
	if err != nil {
		return RouteOutput{}, fmt.Errorf("route %q: %w", in.Mover, err)
	}
	if from == in.Anchor {
		return RouteOutput{}, fmt.Errorf(
			"route %q: the anchor stands on the mover, so there is no line through them: %w",
			in.Mover, ErrBadReach)
	}

	return e.routeLine(in.Mover, from, in.Anchor, in.Budget), nil
}

// routeLine is the [MoveLine] policy: the grid's own line from the anchor
// through the mover, continued past them.
//
// THE CONTINUATION IS ARITHMETIC, NOT A SEARCH. The far end is
// `mover + (mover - anchor) * budget` in axial coordinates, and the cells
// between are whatever [spatial.Grid.GetLineOfSight] draws — the same line
// sight uses, so a push goes where a sightline would. On hex that is the only
// answer available that a table would recognise, and picking a different one
// here would mean this module held a second opinion about what "straight" means.
//
// ADJACENCY IS CHECKED RATHER THAN ASSUMED. The grid drops line cells outside
// its own span, so a push aimed off the edge of the field comes back with a
// gap in it; a route that stepped across that gap would hand [Encounter.Direct]
// two cells that are not neighbours, and the walk would teleport.
func (e *Encounter) routeLine(mover MemberID, from, anchor spatial.Position, budget int) RouteOutput {
	if budget == 0 {
		return RouteOutput{}
	}

	far := spatial.Position{
		X: from.X + (from.X-anchor.X)*float64(budget),
		Y: from.Y + (from.Y-anchor.Y)*float64(budget),
	}

	var out RouteOutput
	prev := from
	for _, cell := range e.canvas.GetGrid().GetLineOfSight(from, far) {
		if cell == from {
			continue // the line starts where they stand; the path does not
		}
		if len(out.Path) == budget {
			return out // the budget is spent, and a spent budget stops nothing
		}
		if !e.canvas.GetGrid().IsAdjacent(prev, cell) {
			out.StoppedBy = fmt.Sprintf("cell %v is off the edge of the field", cell)
			return out
		}
		if e.canvas.IsBoundaryMovementBlocked(prev, cell) {
			out.StoppedBy = fmt.Sprintf("the crossing from %v into %v is blocked", prev, cell)
			return out
		}
		if fact := e.CellAt(CellAtInput{Cell: cell, Mover: mover}); fact.Passage != PassageStandable {
			out.StoppedBy = e.stoppedBy(fact, cell)
			return out
		}
		out.Path = append(out.Path, cell)
		prev = cell
	}

	return out
}

// stoppedBy is WHY a cell ends a directed move, as a phrase for
// [RouteOutput.StoppedBy].
//
// It is [Encounter.blockedBy] plus the one case a push cares about that a step
// does not: a cell the fold calls PassThrough. A nonhostile creature's space is
// crossable on your own turn and is NOT crossable by a shove — "a creature in
// the way stops a push; nothing passes through anyone" — and blockedBy would
// have nothing to say about it, because nothing on that cell Blocks. Naming the
// field's own not-standable sentence there would be the story lying about which
// thing stopped the push.
func (e *Encounter) stoppedBy(fact CellFact, cell spatial.Position) string {
	if fact.Passage == PassageBlocked {
		return e.blockedBy(fact, cell)
	}
	for _, c := range fact.Contribs {
		if c.Kind == ContribMember {
			return fmt.Sprintf("is occupied by %s", c.ID)
		}
	}

	return e.blockedBy(fact, cell)
}

// DirectInput walks a creature along cells an effect chose for them.
type DirectInput struct {
	// Mover is who is being moved. They need not hold the active turn — that
	// is the whole point of this door.
	Mover MemberID

	// Cause is the effect that moved them, and it travels on every beat this
	// walk appends. REQUIRED ([ErrNoCause]): a movement beat with no cause is
	// a creature that walked, and an observer who cannot tell a shove from a
	// step is an observer this module lied to.
	//
	// Opaque here, like every other [core.Ref] this composition carries (C1).
	Cause core.Ref

	// Route is the cells to walk, in order, excluding the mover's own — an
	// [Encounter.Route] answer, trimmed by the caller to what was actually
	// paid for. It is supplied rather than recomputed so the caller that
	// priced the move is the caller that decided how much of it happens.
	Route []spatial.Position

	// Provokes is whether this move offers opportunity attacks. It reaches
	// the [Mover] inverted, as [MoveStep.Forced], because THAT zero value is
	// the safe one: a step nobody filled a field in for is an ordinary walk
	// that provokes, and no amount of forgetting can switch a reaction off.
	//
	// False is the push — the least permissive directive there is, and
	// Thunderwave's. True is the rout: Dissonant Whispers sends a creature
	// fleeing and it IS struck on the way out.
	Provokes bool
}

// DirectOutput is what the directed walk did.
type DirectOutput struct {
	// Moved is how many cells were actually stepped.
	Moved int

	// StoppedBy is why the WALK fell short of the Route it was given — a
	// different question from [RouteOutput.StoppedBy], which is why the route
	// was short of the budget. Empty when every cell was walked, and empty
	// when the mover was dropped mid-walk, which [DirectOutput.Moved] and the
	// standing capability both already report.
	//
	// It is populated at all because the map can change between the route and
	// the walk: a directive resolved after another creature moved may find
	// somebody standing in the cell it was routed through.
	StoppedBy string

	// IntelDeltas is what this move changed about who can see whom, in the
	// shape [StepOutput.IntelDeltas] reports it. Nil when nobody moved.
	IntelDeltas map[MemberID]*IntelDelta
}

// Direct walks a creature along cells an effect chose for them, off their own
// turn, and every beat it writes says what moved them.
//
// # It is the walk, with the turn taken out
//
// Per cell it announces through the [Mover] seam, stops if the mover was
// dropped, steps through [Encounter.stepTo] so the fold and the canvas refuse
// exactly what they would refuse a chosen step, and appends the same movement
// beat. A pushed creature that would land on a pillar stops in front of it.
// That is not a claim about this function being careful — it is the same body
// the monster's own Move intent runs ([Encounter.walkPath]).
//
// What it does NOT do is charge a turn budget or check whose turn it is.
// Neither is a fact about walking; both are facts about a turn, and this is
// nobody's turn.
//
// # Provoking, and who decides
//
// [DirectInput.Provokes] reaches the [Mover] as [MoveStep.Forced], inverted, so
// the zero value on the wire is the ordinary walk. This module carries the flag
// and does not act on it: what a forced step means for a reaction is a rule,
// and rules live above a module whose go.mod cannot import the rulebook (C1).
//
// # A window mid-push is refused, loudly
//
// A [Mover] that pauses is asking a player about a step, and the machinery that
// holds the rest of a paused walk is a TURN's ([Encounter.ResumeTurn]) — there
// is nowhere to put the remainder of a push. Rather than drop those cells
// silently, this refuses with [ErrStepPaused] and the caller is told the
// directive could not be carried out. Today nothing reaches it: the only
// customer pushes without provoking. The day a directive provokes
// (Dissonant Whispers), this refusal is the thing that has to be answered.
//
// Refusals: [ErrNoMember], [ErrClosed], [ErrNotMember], [ErrNoCause],
// [ErrTurnPaused] for a mover whose own walk is half-taken, and
// [ErrStepPaused] above.
func (e *Encounter) Direct(ctx context.Context, in DirectInput) (DirectOutput, error) {
	if in.Mover == "" {
		return DirectOutput{}, fmt.Errorf("direct: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return DirectOutput{}, fmt.Errorf("direct: %w", ErrClosed)
	}
	m, ok := e.members[in.Mover]
	if !ok {
		return DirectOutput{}, fmt.Errorf("direct %q: %w", in.Mover, ErrNotMember)
	}
	if err := in.Cause.IsValid(); err != nil {
		return DirectOutput{}, fmt.Errorf("direct %q: %w: %w", in.Mover, ErrNoCause, err)
	}
	// THE PAUSED MEMBER CANNOT BE PUSHED, for [Encounter.Step]'s own reason:
	// their walk is announced and not taken, and moving them off the cell the
	// open window was announced from would leave the reaction it exists for
	// checking reach against a body that is no longer there.
	if e.PausedMember() == in.Mover {
		return DirectOutput{}, fmt.Errorf("direct %q: %w", in.Mover, ErrTurnPaused)
	}
	if len(in.Route) == 0 {
		return DirectOutput{}, nil
	}

	// subjectBeat, subject is the mover — v1 still sends everyone
	// (audienceFor's doc).
	audience := e.audienceFor(subjectBeat, in.Mover)
	at := uint64(e.clock.ToData().HighWater)

	res, err := e.walkPath(ctx, in.Mover, m, in.Route, audience, at, in.Cause, !in.Provokes)
	if err != nil {
		return DirectOutput{}, fmt.Errorf("direct %q: %w", in.Mover, err)
	}
	if res.paused != nil {
		return DirectOutput{}, fmt.Errorf(
			"direct %q: a directed move has no turn to hold the rest of the walk on: %w",
			in.Mover, ErrStepPaused)
	}

	out := DirectOutput{Moved: res.moved}
	if !res.dropped && res.moved < len(in.Route) {
		cell := in.Route[res.moved]
		out.StoppedBy = e.stoppedBy(e.CellAt(CellAtInput{Cell: cell, Mover: in.Mover}), cell)
	}

	// THE SAME SETTLE EVERY WALK RUNS. A push reveals what a step reveals:
	// the mover is somewhere else now, and who can see whom changed with them.
	deltas, serr := e.settleWalk(in.Mover, audience, res.moved)
	if serr != nil {
		return DirectOutput{}, fmt.Errorf("direct %q: %w", in.Mover, serr)
	}
	out.IntelDeltas = deltas

	return out, nil
}
