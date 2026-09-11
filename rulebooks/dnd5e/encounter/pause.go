// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BeatWindowOpened is the "beat" value of the story beat this composition
// appends when a step pauses to ask somebody (rpg-project#316 rung 3).
//
// EXPORTED BECAUSE A DECODER READS IT. Every other beat kind in this module
// is a bare string literal at its one append site, and the hosts that decode
// them carry their own copies — which is how a rename becomes a beat nobody
// renders and nothing fails. This one arrives with a decoder being written
// against it in the same wave, so it arrives as a name.
const BeatWindowOpened = "window_opened"

// PausedWindow names one reactor who is being asked about a step, and what
// they are being asked to react WITH.
//
// A step can ask several people at once — a pack passing a line — so this is
// the unit and [StepPausedError] carries a list. Ordered as the [Mover]
// produced it; this composition does not sort or dedupe, because who was
// asked in what order is the host's own record and inventing an order here
// would be a second answer to it.
type PausedWindow struct {
	// Audience is the member being asked. Always a member of this
	// encounter; a Mover that names anybody else has malfunctioned, and
	// this composition cannot tell the difference (C1) so it does not try.
	Audience MemberID

	// Reaction names what the audience would be reacting with — the same
	// identity the resulting struck/missed beat carries if they say yes.
	// See [ReactionIdentity].
	Reaction ReactionIdentity
}

// StepPausedError is the detail a [Mover] returns alongside [ErrStepPaused]:
// the step is announced, somebody is being asked about it, and it must not
// be taken until they answer.
//
// IT IS NEWS, NOT A FAILURE, and that is the whole reason it is an error at
// all. [Mover.Move] has no other channel: its one return is an error, and
// every other value in it aborts the caller's verb. Rather than widen the
// interface for one case — which would make ~100 existing construction sites
// and every implementation carry a second return they never use — the
// pause travels as a sentinel-wrapping error the composition recognizes and
// treats as a checkpoint. The interface is unchanged; the vocabulary grew.
//
// A Mover that returns this must have recorded nothing for the step and
// changed nothing about it. The composition will call [Encounter.ResumeTurn]
// once the answers are in, and the announced step is taken THEN, without
// being announced a second time — every reactor for it was already asked.
type StepPausedError struct {
	// Windows are the reactors being asked, in the order the Mover posed
	// them. Empty is legal but meaningless: a pause nobody was asked about
	// is a Mover defect this composition cannot detect, and it pauses
	// anyway rather than second-guessing the capability.
	Windows []PausedWindow
}

// Error names the pause and how many people it is waiting on.
func (e *StepPausedError) Error() string {
	return fmt.Sprintf("encounter: step paused: %d reactor(s) asked", len(e.Windows))
}

// Unwrap makes errors.Is(err, ErrStepPaused) true for this detail type — the
// same sentinel-plus-detail shape this module's other rich errors use.
func (e *StepPausedError) Unwrap() error { return ErrStepPaused }

// pausedTurn is everything needed to finish one driven turn that stopped
// mid-walk because a reactor is being asked about a step.
//
// THE ENCOUNTER OWNS THE PAUSE (rpg-project#316 rung 3, ruling R2). The
// alternative was threading a remainder out of a drive that is three loops
// deep through five entry points, and back in through a sixth. The turn is
// this composition's; so is its pause, and a host persists it the way it
// persists every other thing here — as bytes in the blob, through ToData.
//
// The two values a reader will not expect are the last two. Intent and Bound
// are the driven turn's own anti-spin coordinates (see
// [Encounter.driveOneMonsterTurn]): a resume that restarted the inner loop at
// zero would hand a misbehaving driver a fresh budget of intents for every
// window it opened.
//
// The bubble and the *memberRecord are deliberately NOT here. Both are
// re-derived on resume from the loaded encounter, and a stored copy of either
// could only ever agree with the roster or lie to it.
type pausedTurn struct {
	member    MemberID
	round     int
	from      spatial.Position
	to        spatial.Position
	remaining []spatial.Position
	moved     int
	budget    TurnBudget
	intent    int
	bound     int
	at        uint64
	audience  []MemberID
}

// PausedTurnData is the persistent representation of a paused turn — see
// [pausedTurn] for what each value is for.
//
// PLAIN VALUES ONLY. Everything here marshals by inspection and reloads to
// the same thing; the two live objects a resume needs (the bubble, the
// member record) are re-derived rather than stored.
type PausedTurnData struct {
	// Member is whose turn is paused.
	Member MemberID `json:"member"`

	// Round is the bubble's round when the turn began, rebuilt into the
	// monster view on resume.
	Round int `json:"round"`

	// From is where the mover still stands: the cell the paused step was
	// announced FROM, and — if the reaction drops them — the cell they fall
	// in (ruling R6).
	From PositionData `json:"from"`

	// To is the cell the paused step was announced TO. Always Remaining[0];
	// carried by name because the beat and the host's window payload speak
	// of a step from-and-to, not of an index into a path.
	To PositionData `json:"to"`

	// Remaining is the rest of the walk, THE ANNOUNCED CELL FIRST. Never
	// empty: a pause with nothing left to walk is not a pause.
	Remaining []PositionData `json:"remaining"`

	// Moved is how many cells of this intent were already walked before the
	// pause. It is not budget arithmetic — Budget below is already charged
	// for them — it is what makes "the driver asked for a path that could
	// not even start from here" still answerable after a resume.
	Moved int `json:"moved,omitempty"`

	// Budget is the turn's remaining economy, ALREADY CHARGED for the cells
	// walked before the pause. The live loop decrements movement once after
	// its cell loop; a pause happens inside that loop, so it does the
	// arithmetic itself rather than resuming onto a budget it already spent.
	Budget TurnBudgetData `json:"budget"`

	// Intent is the inner loop's counter at the pause, and Bound its limit.
	// See [pausedTurn].
	Intent int `json:"intent"`
	Bound  int `json:"bound"`

	// At is the clock high-water the walk's beats are stamped at, so the
	// cells after the pause are stamped like the cells before it.
	At uint64 `json:"at,omitempty"`

	// Audience is the movement beats' audience, captured once for the whole
	// intent as the live loop captures it.
	Audience []MemberID `json:"audience,omitempty"`
}

// TurnBudgetData is the persistent representation of a [TurnBudget].
type TurnBudgetData struct {
	AttacksLeft  int `json:"attacks_left,omitempty"`
	MovementFeet int `json:"movement_feet,omitempty"`
}

// pausedTurnDataFrom renders a paused turn for the blob.
func pausedTurnDataFrom(p *pausedTurn) *PausedTurnData {
	if p == nil {
		return nil
	}
	remaining := make([]PositionData, len(p.remaining))
	for i, c := range p.remaining {
		remaining[i] = PositionData{X: c.X, Y: c.Y}
	}
	return &PausedTurnData{
		Member:    p.member,
		Round:     p.round,
		From:      PositionData{X: p.from.X, Y: p.from.Y},
		To:        PositionData{X: p.to.X, Y: p.to.Y},
		Remaining: remaining,
		Moved:     p.moved,
		Budget: TurnBudgetData{
			AttacksLeft:  p.budget.AttacksLeft,
			MovementFeet: p.budget.MovementFeet,
		},
		Intent:   p.intent,
		Bound:    p.bound,
		At:       p.at,
		Audience: append([]MemberID(nil), p.audience...),
	}
}

// pausedTurnFrom rebuilds the live paused turn from validated bytes.
func pausedTurnFrom(d *PausedTurnData) *pausedTurn {
	if d == nil {
		return nil
	}
	remaining := make([]spatial.Position, len(d.Remaining))
	for i, c := range d.Remaining {
		remaining[i] = spatial.Position{X: c.X, Y: c.Y}
	}
	return &pausedTurn{
		member:    d.Member,
		round:     d.Round,
		from:      spatial.Position{X: d.From.X, Y: d.From.Y},
		to:        spatial.Position{X: d.To.X, Y: d.To.Y},
		remaining: remaining,
		moved:     d.Moved,
		budget: TurnBudget{
			AttacksLeft:  d.Budget.AttacksLeft,
			MovementFeet: d.Budget.MovementFeet,
		},
		intent:   d.Intent,
		bound:    d.Bound,
		at:       d.At,
		audience: append([]MemberID(nil), d.Audience...),
	}
}

// validatePausedTurn is the trust boundary for a persisted pause: reject,
// never crash, and never resume onto a shape this build could not have
// written.
//
// members is the load's own already-built index: the paused member must be a
// member, because there is nobody else the remainder could belong to.
//
// # A PAUSED MEMBER IN NO FIGHT IS LEGAL, and this is the check that is
// deliberately NOT here
//
// It was here, and it was wrong. The window's whole purpose is to let a
// player strike, and a strike that drops the LAST monster ends the fight:
// noticeDown dissolves the bubble and splices the body out, all of it
// recorded before anybody calls [Encounter.ResumeTurn]. The host then reloads
// mid-verb and finds a paused turn whose member is on no clock — the ordinary
// consequence of the answer it just wrote down, refused at the door as
// corruption.
//
// So the two halves of this file have to agree, and ResumeTurn's is the
// correct half: the turn is already over, ruling R6 already holds because the
// announced step never happened, and resuming has nothing left to do but
// clear the pause and let the rest of the run continue. A member who is down,
// spliced out, or gone entirely reloads fine and resumes to a no-op.
func validatePausedTurn(d *PausedTurnData, members map[core.EntityID]struct{}) error {
	if d == nil {
		return nil
	}
	if d.Member == "" {
		return fmt.Errorf("load encounter paused turn: names no member: %w", ErrInvalidData)
	}
	if _, ok := members[core.EntityID(d.Member)]; !ok {
		return fmt.Errorf("load encounter paused turn: %q is not a member: %w", d.Member, ErrInvalidData)
	}
	if len(d.Remaining) == 0 {
		return fmt.Errorf("load encounter paused turn %q: nothing left to walk: %w", d.Member, ErrInvalidData)
	}
	if d.To != d.Remaining[0] {
		return fmt.Errorf(
			"load encounter paused turn %q: the announced cell is not the first cell left to walk: %w",
			d.Member, ErrInvalidData)
	}
	if d.Bound <= 0 || d.Intent < 0 || d.Intent >= d.Bound {
		return fmt.Errorf(
			"load encounter paused turn %q: intent %d is outside the turn's bound %d: %w",
			d.Member, d.Intent, d.Bound, ErrInvalidData)
	}
	if d.Moved < 0 || d.Budget.AttacksLeft < 0 || d.Budget.MovementFeet < 0 {
		return fmt.Errorf("load encounter paused turn %q: negative budget or progress: %w", d.Member, ErrInvalidData)
	}
	if d.Round < 0 {
		return fmt.Errorf("load encounter paused turn %q: negative round: %w", d.Member, ErrInvalidData)
	}
	return nil
}

// Paused reports whether a walk is stopped mid-route waiting on an answer.
//
// THE ONE QUESTION every drive entry asks before it drives and every
// pause-carrying output answers. A host reads it to know whether the fight is
// waiting on somebody.
//
// TWO SOURCES, ONE ANSWER. A driven turn can be paused mid-walk and a DIRECTED
// walk can be held mid-route (held.go), and both freeze the table while a
// player decides — which is the right freeze for a directive too, even though
// the walk being held is nobody's turn. They are mutually exclusive by
// construction. [Encounter.HeldDirective] says which of the two it is, and the
// matching continue-verb is the only thing that makes this false again.
func (e *Encounter) Paused() bool { return e.pausedTurn != nil || e.heldDirective != nil }

// PausedMember names whose walk is held — the paused turn's member, or the
// held directive's — or "" when nothing is.
func (e *Encounter) PausedMember() MemberID {
	if e.pausedTurn != nil {
		return e.pausedTurn.member
	}
	if e.heldDirective != nil {
		return e.heldDirective.member
	}
	return ""
}

// appendWindowOpenedBeat narrates a step stopping to ask.
//
// IT TAKES THE WALK'S FIELDS RATHER THAN THE PAUSE, because there are two
// kinds of held walk now and only one beat: a driven turn's pause (this file)
// and a directed walk's hold (held.go) are the same news to a client.
//
// cause is ADDED TO THE PAYLOAD ONLY WHEN IT IS A REF, which is what keeps the
// change additive for the decoders that already read [BeatWindowOpened]. A
// turn's walk has no cause and passes the zero value, so the beat it writes is
// byte-identical to the one it always wrote; a directive names the effect that
// is moving the creature, for [DirectInput.Cause]'s own reason — an observer
// who cannot tell a rout from a stroll was told something false by omission.
//
// AUDIENCE IS EVERYONE (subjectBeat, subject the mover), which is the pre-v1
// full-data rule this composition applies to every other beat: the client
// decides what to show, and rpg-toolkit#940 is where per-recipient beats
// arrive for all of them at once. A window is plausibly the reactor's own
// business, and when that day comes it becomes a beatClass rather than a
// special case here.
func (e *Encounter) appendWindowOpenedBeat(
	member MemberID, from, to spatial.Position, at uint64, windows []PausedWindow, cause core.Ref,
) (uint64, error) {
	posed := make([]map[string]interface{}, 0, len(windows))
	for _, w := range windows {
		posed = append(posed, map[string]interface{}{
			"audience": string(w.Audience),
			"reaction": map[string]interface{}{
				"ref":  w.Reaction.Ref,
				"name": w.Reaction.Name,
			},
		})
	}

	payload := map[string]interface{}{
		"beat":    BeatWindowOpened,
		"member":  string(member),
		"from":    from,
		"to":      to,
		"windows": posed,
	}
	if cause.IsValid() == nil {
		payload["cause"] = cause.String()
	}
	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal window opened beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(subjectBeat, member),
		Tags:     map[string]string{"tag": "window"},
		Payload:  beatBytes,
	})
	if err != nil {
		return 0, err
	}
	return out.Seq, nil
}

// ResumeTurnOutput reports what continuing a paused turn did. It mirrors
// [EndTurnOutput] field for field, because it answers the same question the
// EndTurn that started the drive was going to answer before it stopped.
type ResumeTurnOutput struct {
	// Next is whose turn it now is — always a member with a player, on the
	// same terms EndTurnOutput.Next states, EXCEPT when Paused below is
	// true: the walk stopped again on a later cell, and Next is the paused
	// member, whose turn has not ended.
	Next MemberID

	// RoundWrapped is true when finishing this turn, or any unplayed
	// member's turn driven after it, closed the round.
	RoundWrapped bool

	// Seq is the story sequence of the LAST beat this call recorded.
	Seq uint64

	// IntelDeltas maps member IDs to their updated percepts.
	IntelDeltas map[MemberID]*IntelDelta

	// Paused is true when a later cell of the same walk asked somebody
	// else — an ordinary outcome, not a failure. The fight is waiting again
	// and this verb is called again once that answer is in.
	Paused bool
}

// ResumeTurn continues the turn a [Mover] paused, taking the announced step
// and finishing whatever the turn had left.
//
// # The announced step is NOT announced again
//
// Every reactor for it was already asked; asking again would pose the same
// window twice and, for a reaction the host has since spent, would refuse it
// the second time and silently lose the swing. So the first cell is stepped
// directly. Every LATER cell is announced normally, and may pause again —
// the pack passing the line is several windows, one per step.
//
// # Standing is asked FIRST
//
// Before the step, not after, and this is ruling R6 arriving through the
// front door: a reaction that dropped the mover means the turn is over in the
// cell they were LEAVING. Stepping first would carry a body out of the square
// it fell in — the exact thing the announce-before-step contract exists to
// prevent, half a verb later.
//
// # Then it drives on
//
// Once the paused member's turn ends, this call reassesses the boundary and
// drives consecutive unplayed members exactly as [Encounter.EndTurn] does, so
// the caller receives a Next somebody can actually act for. It IS the rest of
// the EndTurn that stopped.
//
// Errors: ErrNotPaused when nothing is paused, ErrClosed on a closed
// encounter, and whatever the drive itself can return.
func (e *Encounter) ResumeTurn(ctx context.Context) (*ResumeTurnOutput, error) {
	if e.outcome != nil {
		return nil, fmt.Errorf("resume turn: %w", ErrClosed)
	}
	if e.pausedTurn == nil {
		return nil, fmt.Errorf("resume turn: %w", ErrNotPaused)
	}

	p := e.pausedTurn
	m, ok := e.members[p.member]
	if !ok {
		return nil, fmt.Errorf("resume turn %q: %w", p.member, ErrNotMember)
	}

	// THE MOVER MAY HAVE LEFT THE FIGHT WHILE THE WINDOW WAS OPEN. The
	// strike a player chose is recorded before this verb is reached, and a
	// killing one reaches noticeDown, which splices the body out of the
	// bubble — which IS that member's turn ending. Ruling R6 already holds
	// without anything happening here, because the announced step never
	// happened and the body is still in the cell it was leaving. So there is
	// no turn left to finish; there is only the rest of the fight to drive.
	bubble, berr := e.bubbleFor(p.member)
	if berr != nil {
		return nil, fmt.Errorf("resume turn %q: %w", p.member, berr)
	}
	moverLeft := bubble == nil
	if moverLeft && len(e.bubbles) > 0 {
		bubble = e.bubbles[0]
	}

	// CLEARED BEFORE THE FIRST STEP. Everything below either finishes this
	// turn or stores a fresh pause of its own, and a stale one left standing
	// would make every drive entry — including this call's own tail — a
	// no-op on a fight that is running again.
	e.pausedTurn = nil

	var (
		seq     uint64
		wrapped bool
		deltas  map[MemberID]*IntelDelta
	)

	if !moverLeft {
		// The re-entrancy guard, held by hand for the half of this verb that
		// is mid-turn. driveTurnsWithParticipation sets it for a drive it
		// owns; this stretch is a drive nobody owns yet — one member's turn
		// being finished outside any loop — and a reaction landing during it
		// can reach noticeDown -> Transfer -> driveIfStillRunning, which
		// would hand this still-running member a second turn under a second
		// budget (rpg-toolkit#1207, one verb over).
		e.driving = true
		paused, rerr := e.finishTurnFromPause(ctx, bubble, p, m, &seq, &wrapped, &deltas)
		e.driving = false
		if rerr != nil {
			return nil, rerr
		}
		if paused {
			// A later cell of the same walk asked somebody. Nothing else
			// about the turn moves; the fight waits again.
			return &ResumeTurnOutput{
				Next:        p.member,
				Seq:         seq,
				IntelDeltas: deltas,
				Paused:      true,
			}, nil
		}
	} else {
		seq = e.lastRecordedSeq()
	}

	// The turn is over. From here this is EndTurn's own tail, verbatim in
	// shape: reassess the boundary just crossed, drive whatever unplayed
	// members follow, and report the first slot that genuinely waits for a
	// player.
	if e.outcome != nil || bubble == nil {
		return &ResumeTurnOutput{Next: p.member, RoundWrapped: wrapped, Seq: seq, IntelDeltas: deltas}, nil
	}

	participation, moreDeltas, nerr := e.noticeDown(participationPassInput{
		newlyActive: []*clock.Turn{bubble},
	})
	if nerr != nil {
		return nil, fmt.Errorf("resume turn %q: %w", p.member, nerr)
	}
	wrapped = wrapped || participation.scheduledWrapped
	deltas = mergeIntelDeltas(deltas, moreDeltas)
	if participation.scheduledLastSeq != 0 {
		seq = participation.scheduledLastSeq
	}

	next := p.member
	remaining, oerr := bubble.Order()
	if oerr != nil {
		return nil, fmt.Errorf("resume turn %q: %w", p.member, oerr)
	}
	if len(remaining) > 0 && e.outcome == nil {
		active, aerr := bubble.Active()
		if aerr != nil {
			return nil, fmt.Errorf("resume turn %q: %w", p.member, aerr)
		}
		next = MemberID(active)
	}

	return &ResumeTurnOutput{
		Next:         next,
		RoundWrapped: wrapped,
		Seq:          seq,
		IntelDeltas:  deltas,
		Paused:       e.Paused(),
	}, nil
}

// finishTurnFromPause finishes the paused member's own turn: the rest of the
// interrupted walk, then whatever intents the turn had left within its stored
// bound, then the turn's end. It reports whether the walk paused AGAIN.
//
// seq, wrapped and deltas are written through because this is the middle of
// one verb split for readability, not a seam: every one of them is the
// caller's own return value being filled in.
func (e *Encounter) finishTurnFromPause(
	ctx context.Context, bubble *clock.Turn, p *pausedTurn, m *memberRecord,
	seq *uint64, wrapped *bool, deltas *map[MemberID]*IntelDelta,
) (bool, error) {
	walkSeq, walkDeltas, done, rerr := e.finishPausedIntent(ctx, p, m)
	*deltas = mergeIntelDeltas(*deltas, walkDeltas)
	if rerr != nil {
		return false, fmt.Errorf("resume turn %q: %w", p.member, rerr)
	}
	if e.pausedTurn != nil {
		*seq = walkSeq
		return true, nil
	}

	if !done {
		intentSeq, intentWrapped, moreDeltas, ierr := e.runTurnIntents(
			bubble, core.EntityID(p.member), m, p.round, &p.budget, p.intent+1, p.bound)
		*deltas = mergeIntelDeltas(*deltas, moreDeltas)
		if ierr != nil {
			return false, fmt.Errorf("resume turn %q: %w", p.member, ierr)
		}
		*seq, *wrapped = intentSeq, intentWrapped
		if e.pausedTurn != nil {
			return true, nil
		}
		return false, nil
	}

	endSeq, endWrapped, eerr := e.endDrivenTurn(bubble, core.EntityID(p.member))
	if eerr != nil {
		return false, fmt.Errorf("resume turn %q: %w", p.member, eerr)
	}
	*seq, *wrapped = endSeq, endWrapped
	return false, nil
}

// finishPausedIntent takes the announced step and walks whatever is left of
// the paused Move intent, reporting whether that intent ended the turn.
//
// done is true when the turn is over regardless of intents left: the mover
// went down, or the whole walk (before and after the pause) moved nobody —
// the same two answers executeTurnIntent's own Move case gives.
func (e *Encounter) finishPausedIntent(
	ctx context.Context, p *pausedTurn, m *memberRecord,
) (seq uint64, deltas map[MemberID]*IntelDelta, done bool, err error) {
	// STANDING FIRST (ruling R6). A reaction that dropped the mover during
	// the window ends the turn where they stood, and the announced step
	// never happens.
	downNow, derr := e.standingNow()
	if derr != nil {
		return 0, nil, false, fmt.Errorf("resume standing: %w", derr)
	}
	if downNow[p.member] {
		return 0, nil, true, nil
	}

	moved := 0
	// The announced cell, stepped WITHOUT a second announcement — see this
	// verb's own doc for why asking twice is worse than not asking at all.
	action, stepped := e.stepTo(m, p.to)
	if stepped {
		if _, berr := e.appendMovementBeat(action, p.audience, p.at); berr != nil {
			return 0, nil, false, fmt.Errorf("resume move beat: %w", berr)
		}
		moved++
	}

	res := walkResult{moved: moved}
	if stepped && len(p.remaining) > 1 {
		// Every LATER cell is an ordinary announced step, and may pause
		// again.
		rest, werr := e.walkCells(ctx, p.member, m, p.remaining[1:], p.audience, p.at)
		if werr != nil {
			return 0, nil, false, fmt.Errorf("resume move: %w", werr)
		}
		res.moved += rest.moved
		res.dropped = rest.dropped
		res.paused = rest.paused
		res.from, res.to, res.pending = rest.from, rest.to, rest.pending
	}

	p.budget.MovementFeet -= res.moved * FeetPerCell
	deltas, serr := e.settleWalk(p.member, p.audience, res.moved)
	if serr != nil {
		return 0, deltas, false, serr
	}

	if res.paused != nil {
		e.pausedTurn = &pausedTurn{
			member:    p.member,
			round:     p.round,
			from:      res.from,
			to:        res.to,
			remaining: res.pending,
			moved:     p.moved + res.moved,
			budget:    p.budget,
			intent:    p.intent,
			bound:     p.bound,
			at:        p.at,
			audience:  p.audience,
		}
		wseq, berr := e.appendWindowOpenedBeat(
			p.member, res.from, res.to, p.at, res.paused.Windows, core.Ref{})
		if berr != nil {
			return 0, deltas, false, fmt.Errorf("resume window beat: %w", berr)
		}
		return wseq, deltas, false, nil
	}

	if res.dropped {
		return 0, deltas, true, nil
	}
	// Zero cells over the WHOLE intent, its paused half included — the
	// answer executeTurnIntent's Move case gives, asked once across the
	// pause rather than twice on either side of it.
	return 0, deltas, p.moved+res.moved == 0, nil
}
