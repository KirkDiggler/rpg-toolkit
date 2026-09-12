// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// held.go is a DIRECTED walk waiting on an answer — pause.go's sibling, and
// deliberately its copy rather than its generalisation.
//
// [Encounter.Direct] used to refuse a pause outright, and its doc said why
// that was survivable: "Today nothing reaches it: the only customer pushes
// without provoking. The day a directive provokes (Dissonant Whispers), this
// refusal is the thing that has to be answered." The day came.
//
// # What a directive keeps and what it drops
//
// A [pausedTurn] carries the walk plus the turn it interrupted: a round, a
// budget, and the anti-spin intent/bound pair. A directive has none of those —
// it is nobody's turn and charges no turn budget — so a hold is the walk and
// only the walk.
//
// It gains the two fields a turn never needed. `cause` is the effect that is
// moving this creature, which every beat of a directed walk names; `forced` is
// the stance the [Mover] was told about. Both must survive the hold or the
// cells after it become a creature that walked by choice, one beat at a time.
//
// # Why not one held walk with an optional turn
//
// Because the two resume verbs are different verbs. [Encounter.ResumeTurn]
// finishes a turn — it drives the remaining intents and ends the turn; this
// one finishes a walk and reports what the walk did. Folding them would give
// the merged verb a branch on "was there a turn" at every step, which is the
// same question the two names already answer.
type heldDirective struct {
	member    MemberID
	from      spatial.Position
	to        spatial.Position
	remaining []spatial.Position
	moved     int
	at        uint64
	audience  []MemberID
	cause     core.Ref
	forced    bool
}

// HeldDirectiveData is the persistent representation of a held directive —
// see [heldDirective] for what each value is for.
//
// PLAIN VALUES ONLY, like [PausedTurnData]: everything here marshals by
// inspection and reloads to the same thing.
type HeldDirectiveData struct {
	// Member is who is being moved.
	Member MemberID `json:"member"`

	// From is where the mover still stands: the cell the held step was
	// announced FROM, and — if the reaction drops them — the cell they fall
	// in (ruling R6).
	From PositionData `json:"from"`

	// To is the cell the held step was announced TO. Always Remaining[0],
	// carried by name for [PausedTurnData.To]'s reason.
	To PositionData `json:"to"`

	// Remaining is the rest of the directed walk, THE ANNOUNCED CELL FIRST.
	// Never empty: a hold with nothing left to walk is not a hold.
	Remaining []PositionData `json:"remaining"`

	// Moved is how many cells of this directive were already walked before
	// the hold, across every earlier hold of the same walk. It is what
	// [DirectOutput.Moved] reports when the walk finally finishes, and the
	// only place that count can survive a restart.
	Moved int `json:"moved,omitempty"`

	// At is the clock high-water the walk's beats are stamped at, so the
	// cells after the hold are stamped like the cells before it.
	At uint64 `json:"at,omitempty"`

	// Audience is the movement beats' audience, captured once for the whole
	// directive as [Encounter.Direct] captured it.
	Audience []MemberID `json:"audience,omitempty"`

	// Cause is the effect that is moving them, as module:type:id. REQUIRED
	// and refused when it is not a ref: [DirectInput.Cause] is required for
	// the walk, and a resumed half of that walk is not entitled to be
	// vaguer about it than the first half was.
	Cause string `json:"cause"`

	// Forced is what the [Mover] was told about this step's stance —
	// [DirectInput.Provokes], inverted, exactly as the live walk carried it.
	// Zero is the ordinary walk that provokes, which is the safe default a
	// blob missing the key should get.
	Forced bool `json:"forced,omitempty"`
}

// heldDirectiveDataFrom renders a held directive for the blob.
func heldDirectiveDataFrom(h *heldDirective) *HeldDirectiveData {
	if h == nil {
		return nil
	}
	remaining := make([]PositionData, len(h.remaining))
	for i, c := range h.remaining {
		remaining[i] = PositionData{X: c.X, Y: c.Y}
	}
	cause := h.cause
	return &HeldDirectiveData{
		Member:    h.member,
		From:      PositionData{X: h.from.X, Y: h.from.Y},
		To:        PositionData{X: h.to.X, Y: h.to.Y},
		Remaining: remaining,
		Moved:     h.moved,
		At:        h.at,
		Audience:  append([]MemberID(nil), h.audience...),
		Cause:     cause.String(),
		Forced:    h.forced,
	}
}

// heldDirectiveFrom rebuilds the live hold from validated bytes.
//
// It parses the cause a second time rather than carrying it out of
// [validateHeldDirective], because the two run at different moments: the
// validation is the trust boundary and runs before anything is constructed
// (R5), and this runs after the world is built. A parse that succeeded there
// succeeds here, and the error arm is kept rather than discarded so a future
// change to either cannot make this one lie by silence.
func heldDirectiveFrom(d *HeldDirectiveData) (*heldDirective, error) {
	if d == nil {
		return nil, nil
	}
	cause, err := core.ParseString(d.Cause)
	if err != nil {
		return nil, fmt.Errorf(
			"load encounter held directive %q: cause %q: %w: %w", d.Member, d.Cause, ErrInvalidData, err)
	}
	remaining := make([]spatial.Position, len(d.Remaining))
	for i, c := range d.Remaining {
		remaining[i] = spatial.Position{X: c.X, Y: c.Y}
	}
	return &heldDirective{
		member:    d.Member,
		from:      spatial.Position{X: d.From.X, Y: d.From.Y},
		to:        spatial.Position{X: d.To.X, Y: d.To.Y},
		remaining: remaining,
		moved:     d.Moved,
		at:        d.At,
		audience:  append([]MemberID(nil), d.Audience...),
		cause:     *cause,
		forced:    d.Forced,
	}, nil
}

// validateHeldDirective is the trust boundary for a persisted hold: reject,
// never crash, and never resume onto a shape this build could not have
// written.
//
// It is [validatePausedTurn]'s list minus the turn coordinates a directive has
// none of, plus the cause. A HELD MOVER IN NO FIGHT IS LEGAL here for the same
// reason it is legal there — a strike through the window can drop the last
// monster and dissolve the bubble before anybody resumes — and a directed walk
// never needed a clock in the first place.
func validateHeldDirective(d *HeldDirectiveData, members map[core.EntityID]struct{}) error {
	if d == nil {
		return nil
	}
	if d.Member == "" {
		return fmt.Errorf("load encounter held directive: names no member: %w", ErrInvalidData)
	}
	if _, ok := members[core.EntityID(d.Member)]; !ok {
		return fmt.Errorf("load encounter held directive: %q is not a member: %w", d.Member, ErrInvalidData)
	}
	if len(d.Remaining) == 0 {
		return fmt.Errorf("load encounter held directive %q: nothing left to walk: %w", d.Member, ErrInvalidData)
	}
	if d.To != d.Remaining[0] {
		return fmt.Errorf(
			"load encounter held directive %q: the announced cell is not the first cell left to walk: %w",
			d.Member, ErrInvalidData)
	}
	if d.Moved < 0 {
		return fmt.Errorf("load encounter held directive %q: negative progress: %w", d.Member, ErrInvalidData)
	}
	if _, err := core.ParseString(d.Cause); err != nil {
		return fmt.Errorf(
			"load encounter held directive %q: cause %q: %w: %w", d.Member, d.Cause, ErrInvalidData, err)
	}
	return nil
}

// HeldDirective reports whether the thing this encounter is waiting on is a
// DIRECTED walk rather than a driven turn.
//
// IT IS THE VERB SELECTOR, and that is the only reason it is exported.
// [Encounter.Paused] says the table is waiting; this says which of the two
// continue-verbs finishes it — [Encounter.ResumeDirective] when true,
// [Encounter.ResumeTurn] when false and Paused is true. A host that guessed
// would get [ErrNotPaused] from the wrong one, which is a refusal rather than
// a corruption, but it is a refusal nobody has to risk.
func (e *Encounter) HeldDirective() bool { return e.heldDirective != nil }

// ResumeDirective continues the directed walk a [Mover] held, taking the
// announced step and walking whatever is left of the route.
//
// It is [Encounter.ResumeTurn]'s sibling and copies its continuation
// deliberately, because the reasons are the same ones:
//
//   - STANDING IS ASKED FIRST (ruling R6). A reaction that dropped the mover
//     while the window was open means the body is in the cell it was LEAVING,
//     and the announced step never happens.
//   - THE ANNOUNCED STEP IS NOT ANNOUNCED AGAIN. Every reactor for it was
//     already asked; asking twice would pose the same window again and, for a
//     reaction the host has since spent, refuse it the second time and
//     silently lose the swing.
//   - THE HOLD IS CLEARED BEFORE THE FIRST STEP. A stale one left standing
//     would freeze a table that is running again.
//
// What differs is what a directive is: there is no turn to finish, no budget
// to charge and no driver to run on, so this reports what the WALK did and
// stops. The cause and the stance travel with every cell, the hand-stepped one
// included — a resumed flee that lost its cause would be recorded as a
// creature that walked away by choice.
//
// A LATER CELL MAY HOLD AGAIN, and that is ordinary: a route is several
// windows, one per step. The new hold carries the accumulated count, so
// [DirectOutput.Moved] on the final resume is the whole walk rather than its
// last leg.
//
// Errors: [ErrClosed] on a closed encounter, [ErrNotPaused] when no directive
// is held, [ErrNotMember] when the mover is gone from the roster entirely, and
// whatever the walk itself can return.
func (e *Encounter) ResumeDirective(ctx context.Context) (DirectOutput, error) {
	if e.outcome != nil {
		return DirectOutput{}, fmt.Errorf("resume directive: %w", ErrClosed)
	}
	if e.heldDirective == nil {
		return DirectOutput{}, fmt.Errorf("resume directive: %w", ErrNotPaused)
	}

	h := e.heldDirective
	m, ok := e.members[h.member]
	if !ok {
		return DirectOutput{}, fmt.Errorf("resume directive %q: %w", h.member, ErrNotMember)
	}

	// Cleared before the first step — see this verb's own doc.
	e.heldDirective = nil

	downNow, derr := e.standingNow()
	if derr != nil {
		return DirectOutput{}, fmt.Errorf("resume directive %q standing: %w", h.member, derr)
	}
	if downNow[h.member] {
		// Nothing to walk and nothing to report but what was already
		// walked before the window opened.
		return DirectOutput{Moved: h.moved}, nil
	}

	res, werr := e.walkHeld(ctx, h, m)
	if werr != nil {
		return DirectOutput{}, werr
	}

	out := DirectOutput{Moved: h.moved + res.moved}
	deltas, serr := e.settleWalk(h.audience, res.moved)
	if serr != nil {
		return DirectOutput{}, fmt.Errorf("resume directive %q: %w", h.member, serr)
	}
	out.IntelDeltas = deltas

	if res.paused != nil {
		e.heldDirective = &heldDirective{
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
			return DirectOutput{}, fmt.Errorf("resume directive %q: %w", h.member, berr)
		}
		out.Paused = true
		return out, nil
	}

	// Why the WALK fell short of the route it was given, which is
	// [DirectOutput.StoppedBy]'s question and not the route's.
	if !res.dropped && res.moved < len(h.remaining) {
		cell := h.remaining[res.moved]
		out.StoppedBy = e.stoppedBy(e.CellAt(CellAtInput{Cell: cell, Mover: h.member}), cell)
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
func (e *Encounter) walkHeld(ctx context.Context, h *heldDirective, m *memberRecord) (walkResult, error) {
	action, stepped := e.stepTo(m, h.to)
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
