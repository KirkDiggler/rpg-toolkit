// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"
)

// held.go is the directive arm of [Encounter.Resume]: a DIRECTED walk waiting
// on an answer, finished.
//
// [Encounter.Direct] used to refuse a pause outright, and its doc said why
// that was survivable: "Today nothing reaches it: the only customer pushes
// without provoking. The day a directive provokes (Dissonant Whispers), this
// refusal is the thing that has to be answered." The day came.
//
// # What a directive keeps and what it drops
//
// A [PauseTurn] carries the walk plus the turn it interrupted: a round, a
// budget, and the anti-spin intent/bound pair. A directive has none of those —
// it is nobody's turn and charges no turn budget — so its [pause] is the walk
// and only the walk, with no turn arm. It always names a cause and carries
// the stance the [Mover] was told about; both must survive the pause or the
// cells after it become a creature that walked by choice, one beat at a time.
//
// # Why the two arms stay two bodies
//
// The pause is one value with a kind; the bodies that finish it are not
// folded. The turn arm finishes a turn — it drives the remaining intents and
// ends the turn; this one finishes a walk and reports what the walk did.
// Folding them would give the merged body a branch on "was there a turn" at
// every step, which is the question the kind already answered once, in
// [Encounter.Resume].

// resumeDirective continues the directed walk a [Mover] held, taking the
// announced step and walking whatever is left of the route.
// [Encounter.Resume] has already proved the encounter open and the pause a
// directive's, and its doc carries the three rules both arms keep.
//
// What differs from the turn arm is what a directive is: there is no turn to
// finish, no budget to charge and no driver to run on, so this reports what
// the WALK did and stops. The cause and the stance travel with every cell, the
// hand-stepped one included — a resumed flee that lost its cause would be
// recorded as a creature that walked away by choice.
//
// A LATER CELL MAY HOLD AGAIN, and that is ordinary: a route is several
// windows, one per step. The new pause carries the accumulated count, so
// [ResumeOutput.Moved] on the final resume is the whole walk rather than its
// last leg.
func (e *Encounter) resumeDirective(ctx context.Context) (*ResumeOutput, error) {
	h := e.pause
	m, ok := e.members[h.member]
	if !ok {
		return nil, fmt.Errorf("resume directive %q: %w", h.member, ErrNotMember)
	}

	// Cleared before the first step — see [Encounter.Resume].
	e.pause = nil

	downNow, derr := e.downNow()
	if derr != nil {
		return nil, fmt.Errorf("resume directive %q standing: %w", h.member, derr)
	}
	if downNow[h.member] {
		// Nothing to walk and nothing to report but what was already
		// walked before the window opened.
		return &ResumeOutput{Kind: PauseDirective, Moved: h.moved}, nil
	}

	res, werr := e.walkHeld(ctx, h, m)
	if werr != nil {
		return nil, werr
	}

	out := &ResumeOutput{Kind: PauseDirective, Moved: h.moved + res.moved}
	deltas, serr := e.settleWalk(h.audience, res.moved)
	if serr != nil {
		return nil, fmt.Errorf("resume directive %q: %w", h.member, serr)
	}
	out.IntelDeltas = deltas

	if res.paused != nil {
		e.pause = &pause{
			kind:      PauseDirective,
			member:    h.member,
			from:      res.from,
			to:        res.to,
			remaining: res.pending,
			moved:     h.moved + res.moved,
			at:        h.at,
			audience:  h.audience,
			cause:     h.cause,
			forced:    h.forced,
		}
		if _, berr := e.appendWindowOpenedBeat(
			h.member, res.from, res.to, h.at, res.paused.Windows, h.cause,
		); berr != nil {
			return nil, fmt.Errorf("resume directive %q: %w", h.member, berr)
		}
		out.Paused = true
		return out, nil
	}

	// Why the WALK fell short of the route it was given, which is
	// [ResumeOutput.StoppedBy]'s question and not the route's.
	if !res.dropped && res.moved < len(h.remaining) {
		cell := h.remaining[res.moved]
		fact, factErr := e.CellAt(CellAtInput{Cell: cell, Mover: h.member})
		if factErr != nil {
			return nil, factErr
		}
		out.StoppedBy = e.stoppedBy(fact, cell)
	}

	return out, nil
}

// walkHeld takes the announced cell by hand and walks the rest of the held
// route, returning the walk's own result for the resumed half alone.
//
// The hand-taken step is [Encounter.finishPausedIntent]'s, and since [Routed]
// gave a TURN's walk a cause too, the two now do the same thing with it: a
// first resumed cell missing the cause is a single beat in N claiming the
// creature walked away of its own accord. What still differs is that a
// directive's cause is required and a turn's is the zero Ref unless something
// routed it.
func (e *Encounter) walkHeld(ctx context.Context, h *pause, m *memberRecord) (walkResult, error) {
	action, stepped, stepErr := e.stepTo(m, h.to)
	if stepErr != nil {
		return walkResult{}, stepErr
	}
	action.cause = h.cause
	if !stepped {
		// The floor changed under the announced cell while the window was
		// open. The walk stops here, exactly as walkPath stops for a wall.
		return walkResult{}, nil
	}
	if _, berr := e.appendMovementBeat(action, h.audience, h.at); berr != nil {
		return walkResult{}, fmt.Errorf("resume directive %q move beat: %w", h.member, berr)
	}

	res := walkResult{moved: 1}
	if len(h.remaining) > 1 {
		rest, werr := e.walkPath(ctx, h.member, m, h.remaining[1:], h.audience, h.at, h.cause, h.forced)
		if werr != nil {
			return walkResult{}, fmt.Errorf("resume directive %q: %w", h.member, werr)
		}
		res.moved += rest.moved
		res.dropped = rest.dropped
		res.paused = rest.paused
		res.from, res.to, res.pending = rest.from, rest.to, rest.pending
	}

	return res, nil
}
