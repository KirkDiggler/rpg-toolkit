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
// A POLICY ARRIVES WITH ITS EXECUTOR, and that is the rule rather than the
// schedule: a constant declared ahead of the thing that carries it out is a
// name callers can validate against and nothing can honour. [MoveAway] arrived
// with Dissonant Whispers and [MoveToward] with Command.
// [Encounter.Route]'s switch is closed on the policies that exist and refuses
// every other word with [ErrUnsupportedPolicy], so the day a fourth one is
// added, the place that must learn about it is the place that already refuses
// it.
const MoveLine MovePolicy = "line"

// MoveAway is the rout: of every cell the mover can reach within the budget
// and legally stop on, the one FARTHEST FROM THE ANCHOR BY THE RULER. It is
// Dissonant Whispers — a creature that must use its whole movement to run
// away from the caster.
//
// MEASURED BY THE RULER, NOT BY THE WALK (rpg-project#430, the
// directed-movement design §3, and its rejected alternative). "Away from you"
// means far from you, so a creature that could run six cells down a dead-end
// corridor and finish one cell from the caster by the crow does not run: it
// takes the open floor that ends farther away, even when that is a shorter
// walk. Ties go to the shorter walk, then to scan order, so the answer is the
// same every time it is asked (C8).
//
// It is a SEARCH, unlike [MoveLine]'s arithmetic, and it searches the same
// flood a monster's own route reads: it may cross a nonhostile creature and
// may not stop on one, and a wall, a pillar or a sealed cell closes a cell to
// it exactly as it closes one to a step.
const MoveAway MovePolicy = "away"

// MoveToward is the approach: the shortest walking route to a cell the mover
// may stop on WITHIN ONE CELL OF THE ANCHOR, and — when the budget reaches no
// such cell — the reached cell NEAREST THE ANCHOR BY THE RULER. It is
// Command's Approach: a creature that "moves toward you by the shortest and
// most direct route, ending its turn if it moves within 5 feet of you".
//
// IT STOPS BESIDE, NOT ON. The anchor's own cell is never a destination: a
// creature standing on it makes it unstandable anyway, and an empty anchor
// cell is still not where "within 5 feet" ends. So the goal is the ring
// around the anchor, and the route is the FEWEST STEPS to any cell of it —
// this is a walk, measured by walking, which is where it parts company with
// [MoveAway].
//
// THE RULER DECIDES ONLY WHEN THE WALK CANNOT. A door that is shut, a budget
// that runs out halfway down the hall, a pillar in the mouth of the only way
// in: none of those is "the creature stays put", because the spell says it
// closes. So the fallback is the reached cell that is NEAREST the anchor by
// the ruler, and STRICTLY nearer than the cell the mover already stands on —
// shuffling sideways to spend a budget is not approaching, exactly as it is
// not fleeing ([MoveAway]'s own strictly-farther rule, mirrored). Ties go to
// the shorter walk, then to scan order, so the answer is the same every time
// it is asked (C8).
//
// AN ANCHOR ON THE MOVER IS NOT A REFUSAL, unlike [MoveLine]'s: "get next to
// that" is already true of a creature standing there, so the answer is the
// empty route rather than [ErrBadReach]. Being already adjacent is the same
// answer for the same reason.
const MoveToward MovePolicy = "toward"

// RouteInput asks which cells a directed move would cross.
type RouteInput struct {
	// Mover is who is being moved. Their current cell is read off the
	// canvas, not supplied, for [Encounter.placementOf]'s reason: two reads
	// of where somebody stands is the dual-state defect this composition has
	// paid for before.
	Mover MemberID

	// Policy is how the move is measured. Required, and one of [MoveLine],
	// [MoveAway] and [MoveToward]: the zero value is not a default, it is a
	// refusal ([ErrUnsupportedPolicy]), and so is any other word.
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
// AN EMPTY PATH IS NOT A REFUSAL. A mover with a wall at their back, and a
// mover with nowhere farther from the anchor to stand, both come back with no
// cells and a [RouteOutput.StoppedBy] saying which — not an error. That
// distinction is the whole reason an unknown policy is an error instead.
//
// Refusals: [ErrNilInput] is not reachable (the input is a value), but
// [ErrNoMember], [ErrClosed], [ErrNotMember], [ErrBadReach] for a negative
// budget or a [MoveLine] anchor standing on the mover, and
// [ErrUnsupportedPolicy] for any word that is not a policy, all are.
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
	from, err := e.cellOf(m)
	if err != nil {
		return RouteOutput{}, fmt.Errorf("route %q: %w", in.Mover, err)
	}

	// ONE SWITCH, CLOSED ON THE POLICIES THAT EXIST. The anchor-on-the-mover
	// refusal below belongs to the line and only to the line: a line through
	// two identical cells has no direction, while "as far from here as you
	// can get" is a perfectly good question asked from the cell itself.
	switch in.Policy {
	case MoveLine:
		if from == in.Anchor {
			return RouteOutput{}, fmt.Errorf(
				"route %q: the anchor stands on the mover, so there is no line through them: %w",
				in.Mover, ErrBadReach)
		}
		return e.routeLine(in.Mover, from, in.Anchor, in.Budget), nil
	case MoveAway:
		return e.routeAway(in.Mover, from, in.Anchor, in.Budget), nil
	case MoveToward:
		return e.routeToward(in.Mover, from, in.Anchor, in.Budget), nil
	default:
		return RouteOutput{}, fmt.Errorf("route %q: policy %q: %w", in.Mover, in.Policy, ErrUnsupportedPolicy)
	}
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

// routeAway is the [MoveAway] policy: flood as far as the budget pays for,
// then keep the reached cell that is farthest from the anchor by the ruler.
//
// IT IS THE SAME FLOOD A MONSTER'S OWN ROUTE READS ([Encounter.floodFrom]),
// with a Limit, which is the whole reason a speed-bounded rout needed no
// second searcher: the budget is a bound on a field that already existed
// (rpg-toolkit#1652's lesson, one policy later).
//
// MAY CROSS IS NOT MAY STOP, exactly as [Encounter.nearestStop] has it: the
// flood passes through a nonhostile creature's cell, and the cell the mover
// ends on must be Standable.
//
// STRICTLY FARTHER, OR NOWHERE. A cell the same distance from the anchor is
// not away from it, so the mover stays put rather than shuffling sideways to
// spend a budget. When nothing qualifies the path is EMPTY and StoppedBy says
// so — the pinned creature, and the case [ErrUnsupportedPolicy]'s doc insists
// must be distinguishable from a policy nobody wrote.
//
// TIES: the shorter walk first, then [beforeInScanOrder], so ranging over the
// flood's map cannot leak iteration order into the answer (C8).
//
// A ZERO BUDGET RETURNS FIRST, before the flood. Zero is unbounded to
// [spatial.FieldInput.Limit], so asking the field with it would flood the
// entire floor and hand a creature with no movement the far corner of the
// dungeon.
func (e *Encounter) routeAway(mover MemberID, from, anchor spatial.Position, budget int) RouteOutput {
	if budget == 0 {
		return RouteOutput{}
	}

	field, ok := e.floodFrom(mover, from, nil, budget)
	if !ok {
		return RouteOutput{StoppedBy: "the floor could not be flooded"}
	}

	here := e.Distance(anchor, from)

	var best spatial.Position
	bestFar, bestWalk, found := 0.0, 0, false
	for cell, walk := range field.Dist {
		if cell == from {
			continue
		}
		if e.CellAt(CellAtInput{Cell: cell, Mover: mover}).Passage != PassageStandable {
			continue
		}
		far := e.Distance(anchor, cell)
		if far <= here {
			continue
		}
		better := !found || far > bestFar ||
			(far == bestFar && (walk < bestWalk || (walk == bestWalk && beforeInScanOrder(cell, best))))
		if better {
			best, bestFar, bestWalk, found = cell, far, walk, true
		}
	}
	if !found {
		return RouteOutput{StoppedBy: fmt.Sprintf("nowhere farther from %v within %d cells", anchor, budget)}
	}

	return pathOrRefusal(field, best)
}

// routeToward is the [MoveToward] policy: flood as far as the budget pays
// for, then keep the reached cell beside the anchor that took the fewest
// steps — or, when the ring around the anchor is out of reach, the reached
// cell nearest it by the ruler.
//
// TWO SCANS, AND THE SECOND ONLY WHEN THE FIRST FOUND NOTHING. The first is
// [Encounter.nearestStop] over the goal "within one cell of the anchor",
// which is the same fewest-steps, may-cross-may-not-stop search a monster's
// own route already runs ([Encounter.routeTo]); the second is
// [Encounter.routeAway]'s scan with its comparison turned around. Keeping
// them apart rather than sharing a parameterised loop is deliberate: they
// answer different questions — "which way in" and "how close can I get" —
// and a reader of either should not have to hold the other in mind.
//
// STRICTLY NEARER, OR NOWHERE, in the fallback. A cell the same distance from
// the anchor is not toward it, so a mover ringed by cells no nearer than its
// own stays put and the path is EMPTY with StoppedBy saying so — the
// distinguishable-from-an-unimplemented-policy case [ErrUnsupportedPolicy]'s
// doc insists on.
//
// ALREADY THERE IS THE EMPTY ROUTE WITH NO SENTENCE, which covers an anchor
// standing on the mover: nothing stopped the move, it was already over. A
// fallback that succeeds says nothing either, exactly as [Encounter.routeAway]
// says nothing about a rout that spent less than its budget — StoppedBy is for
// the empty answer, and a caller that wants the geometry has the path.
//
// A ZERO BUDGET RETURNS FIRST, before the flood, for [Encounter.routeAway]'s
// reason: zero is unbounded to [spatial.FieldInput.Limit], so asking the field
// with it would flood the whole floor on behalf of a creature that cannot
// move.
func (e *Encounter) routeToward(mover MemberID, from, anchor spatial.Position, budget int) RouteOutput {
	if budget == 0 {
		return RouteOutput{}
	}

	beside := func(cell spatial.Position) bool { return e.Distance(cell, anchor) <= 1 }
	if beside(from) {
		return RouteOutput{}
	}

	field, ok := e.floodFrom(mover, from, nil, budget)
	if !ok {
		return RouteOutput{StoppedBy: "the floor could not be flooded"}
	}

	if best, found := e.nearestStop(field, mover, from, beside); found {
		return pathOrRefusal(field, best)
	}

	here := e.Distance(anchor, from)

	var best spatial.Position
	bestNear, bestWalk, found := 0.0, 0, false
	for cell, walk := range field.Dist {
		if cell == from {
			continue
		}
		if e.CellAt(CellAtInput{Cell: cell, Mover: mover}).Passage != PassageStandable {
			continue
		}
		near := e.Distance(anchor, cell)
		if near >= here {
			continue
		}
		better := !found || near < bestNear ||
			(near == bestNear && (walk < bestWalk || (walk == bestWalk && beforeInScanOrder(cell, best))))
		if better {
			best, bestNear, bestWalk, found = cell, near, walk, true
		}
	}
	if !found {
		return RouteOutput{StoppedBy: fmt.Sprintf("nowhere nearer to %v within %d cells", anchor, budget)}
	}

	return pathOrRefusal(field, best)
}

// pathOrRefusal reads one cell's route out of the flood that reached it.
//
// The not-reached branch is unreachable by construction — best came out of
// this same field's own Dist — and is refused rather than returned empty
// because an empty path here would be the pinned sentence about a cell that
// is not pinned. It is a function rather than a repeated four lines because
// two policies now end this way.
func pathOrRefusal(field spatial.FieldOutput, best spatial.Position) RouteOutput {
	path, reached := field.PathTo(best)
	if !reached {
		return RouteOutput{StoppedBy: fmt.Sprintf("no path to %v, which the flood reached", best)}
	}
	return RouteOutput{Path: path}
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

	// Paused is true when the walk is HELD on an open window: a reactor is
	// being asked about the next cell, and [Encounter.ResumeDirective]
	// finishes the rest once they have answered.
	//
	// AN ORDINARY OUTCOME, NOT A FAILURE, and the field exists so a caller
	// can tell it from a walk that simply ended. Moved is still true — it is
	// the cells taken so far, accumulated across every hold of this same
	// walk — and StoppedBy is empty, because nothing stopped it.
	Paused bool
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
// # A window mid-push is HELD, not refused
//
// This used to refuse with [ErrStepPaused], and its doc said why that was
// survivable: the only customer pushed without provoking, so nothing reached
// it, and "the day a directive provokes (Dissonant Whispers), this refusal is
// the thing that has to be answered." The day came. A [Mover] that pauses is
// asking a player about a step, and the rest of the route is now held beside
// the held turn (held.go): this returns [DirectOutput.Paused] with the cells
// taken so far, and [Encounter.ResumeDirective] finishes it once the answer is
// in. Nothing is dropped and nothing is refused.
//
// # A second held walk is refused at the door
//
// There is exactly one, and a verb that overwrote it would lose a walk the
// table is waiting on. So a directive is refused outright while anything is
// held — which subsumes the older, narrower refusal of pushing the currently
// paused member, and keeps it for its own reason: their walk is announced and
// not taken, and moving them off the cell the open window was announced from
// would leave the reaction it exists for checking reach against a body that is
// no longer there.
//
// Refusals: [ErrNoMember], [ErrClosed], [ErrNotMember], [ErrNoCause], and
// [ErrTurnPaused] while any walk is held.
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
	// A HELD TABLE TAKES NO SECOND DIRECTIVE — see this verb's own doc. It
	// covers the paused member themself, who could never be pushed, and
	// everybody else, whose push would need a second hold this composition
	// has no slot for and would refuse to read back.
	if e.Paused() {
		return DirectOutput{}, fmt.Errorf(
			"direct %q: %q is already waiting on an answer: %w", in.Mover, e.PausedMember(), ErrTurnPaused)
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
	out := DirectOutput{Moved: res.moved}
	if res.paused == nil && !res.dropped && res.moved < len(in.Route) {
		cell := in.Route[res.moved]
		out.StoppedBy = e.stoppedBy(e.CellAt(CellAtInput{Cell: cell, Mover: in.Mover}), cell)
	}

	// THE SAME SETTLE EVERY WALK RUNS, AND IT RUNS AT A HOLD TOO. A push
	// reveals what a step reveals: the mover is somewhere else now, and who
	// can see whom changed with them. A walk that stopped half way still
	// walked its half (clocks.go's own note on settling at a pause).
	deltas, serr := e.settleWalk(audience, res.moved)
	if serr != nil {
		return DirectOutput{}, fmt.Errorf("direct %q: %w", in.Mover, serr)
	}
	out.IntelDeltas = deltas

	if res.paused != nil {
		e.heldDirective = &heldDirective{
			member:    in.Mover,
			from:      res.from,
			to:        res.to,
			remaining: res.pending,
			moved:     res.moved,
			at:        at,
			audience:  audience,
			cause:     in.Cause,
			forced:    !in.Provokes,
		}
		if _, berr := e.appendWindowOpenedBeat(
			in.Mover, res.from, res.to, at, res.paused.Windows, in.Cause,
		); berr != nil {
			return DirectOutput{}, fmt.Errorf("direct %q: %w", in.Mover, berr)
		}
		out.Paused = true
	}

	return out, nil
}
