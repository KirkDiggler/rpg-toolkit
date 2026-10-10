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
// changed nothing about it. The composition will call [Encounter.Resume]
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

// PauseVersion is the version written on every [PauseData] and the only one
// [LoadEncounter] reads back. A stored pause carrying any other version was
// written by another build, and is refused with [ErrStalePause] before
// anything is constructed rather than resumed onto a shape this build cannot
// vouch for (one pause envelope, ruling E5).
const PauseVersion = 1

// PauseKind names which walk an encounter's one pause is holding.
type PauseKind string

const (
	// PauseTurn is a driven turn stopped mid-walk (or just after a paused
	// strike) because a reactor is being asked. Finishing it finishes the
	// turn and drives on, as the EndTurn that started the drive would have.
	PauseTurn PauseKind = "turn"

	// PauseDirective is a DIRECTED walk held mid-route — nobody's turn, so
	// finishing it finishes the walk and reports what the walk did.
	PauseDirective PauseKind = "directive"
)

// pause is everything needed to finish the one walk this encounter is waiting
// on: a driven turn that stopped mid-walk, or a directed walk held mid-route.
//
// ONE PAUSE, TWO KINDS (one pause envelope, ruling E2). The two used to be two
// types in two fields, kept mutually exclusive by a load refusal and a door
// refusal that existed only because there were two slots. A fight waits on one
// answer at a time, so there is one slot, and the kind says which walk it is.
//
// THE ENCOUNTER OWNS THE PAUSE (rpg-project#316 rung 3, ruling R2). The walk
// is this composition's; so is its pause, and a host persists it the way it
// persists every other thing here — as bytes in the blob, through ToData.
//
// The walk's own fields are shared. `cause` is the effect moving the
// creature, which every beat of its walk names: required on a directive, and
// the zero Ref on a turn's own [Move] (a [Routed] turn names one). `forced` is
// the stance a directive's [Mover] was told about, and is a directive's only.
// `turn` is present exactly on a turn and carries what the turn interrupted.
//
// The bubble and the *memberRecord are deliberately NOT here. Both are
// re-derived on resume from the loaded encounter, and a stored copy of either
// could only ever agree with the roster or lie to it.
type pause struct {
	kind      PauseKind
	member    MemberID
	from      spatial.Position
	to        spatial.Position
	remaining []spatial.Position
	moved     int
	at        uint64
	audience  []MemberID
	cause     core.Ref
	turn      *turnPause
	forced    bool
}

// turnPause is what a [PauseTurn] carries beyond the walk: the turn the walk
// interrupted.
//
// The two values a reader will not expect are intent and bound, the driven
// turn's own anti-spin coordinates (see [Encounter.driveOneMonsterTurn]): a
// resume that restarted the inner loop at zero would hand a misbehaving driver
// a fresh budget of intents for every window it opened.
//
// `terminal` is whether finishing the walk finishes the TURN: a [Routed] turn
// is over when its walk is, so a resume that dropped the flag would hand the
// driver a second intent nobody's turn had left. `afterStrike` is a turn whose
// strike paused rather than its walk: there is nothing left to walk, only the
// rest of the turn. Neither is re-derivable from the cells that remain.
type turnPause struct {
	round       int
	budget      TurnBudget
	intent      int
	bound       int
	terminal    bool
	afterStrike bool
}

// PauseData is the persistent representation of the encounter's one pause —
// see [pause] for what each value is for.
//
// PLAIN VALUES ONLY. Everything here marshals by inspection and reloads to
// the same thing; the two live objects a resume needs (the bubble, the
// member record) are re-derived rather than stored.
type PauseData struct {
	// Version is [PauseVersion] when this build wrote it. Anything else is
	// refused at load with [ErrStalePause].
	Version int `json:"version"`

	// Kind is which walk is paused: [PauseTurn] or [PauseDirective].
	Kind PauseKind `json:"kind"`

	// Member is whose walk is paused.
	Member MemberID `json:"member"`

	// From is where the mover still stands: the cell the paused step was
	// announced FROM, and — if the reaction drops them — the cell they fall
	// in (ruling R6).
	From PositionData `json:"from"`

	// To is the cell the paused step was announced TO. Always Remaining[0]
	// when anything remains; carried by name because the beat and the host's
	// window payload speak of a step from-and-to, not of an index into a
	// path.
	To PositionData `json:"to"`

	// Remaining is the rest of the walk, THE ANNOUNCED CELL FIRST. Never
	// empty, except on a turn whose STRIKE paused ([TurnPauseData.AfterStrike]):
	// a walk pause with nothing left to walk is not a pause.
	Remaining []PositionData `json:"remaining"`

	// Moved is how many cells of this walk were already taken before the
	// pause, across every earlier pause of the same walk. On a directive it
	// is what [ResumeOutput.Moved] reports when the walk finally finishes; on
	// a turn it is what makes "the driver asked for a path that could not
	// even start from here" still answerable after a resume.
	Moved int `json:"moved,omitempty"`

	// At is the clock high-water the walk's beats are stamped at, so the
	// cells after the pause are stamped like the cells before it.
	At uint64 `json:"at,omitempty"`

	// Audience is the movement beats' audience, captured once for the whole
	// walk as the live loop captured it.
	Audience []MemberID `json:"audience,omitempty"`

	// Cause is the effect moving them, as its Ref string. REQUIRED on a
	// directive — [DirectInput.Cause] is required for the walk, and a resumed
	// half of it is not entitled to be vaguer than the first half was — and
	// empty on a turn the creature chose ([Move]).
	Cause string `json:"cause,omitempty"`

	// Turn is present EXACTLY on a [PauseTurn]: the turn the walk interrupted.
	Turn *TurnPauseData `json:"turn,omitempty"`

	// Forced is what a directive's [Mover] was told about this step's stance —
	// [DirectInput.Provokes], inverted, exactly as the live walk carried it.
	// A directive's only; refused on a turn.
	Forced bool `json:"forced,omitempty"`
}

// TurnPauseData is the persistent representation of what a paused TURN
// carries beyond its walk — see [turnPause].
type TurnPauseData struct {
	// Round is the bubble's round when the turn began, rebuilt into the
	// monster view on resume.
	Round int `json:"round"`

	// Budget is the turn's remaining economy, ALREADY CHARGED for the cells
	// walked before the pause. The live loop decrements movement once after
	// its cell loop; a pause happens inside that loop, so it does the
	// arithmetic itself rather than resuming onto a budget it already spent.
	Budget TurnBudgetData `json:"budget"`

	// Intent is the inner loop's counter at the pause, and Bound its limit.
	// See [turnPause].
	Intent int `json:"intent"`
	Bound  int `json:"bound"`

	// Terminal is whether finishing the walk finishes the turn — true for a
	// [Routed] intent and false for a [Move].
	Terminal bool `json:"terminal,omitempty"`

	// AfterStrike is true when the turn's STRIKE paused rather than its walk:
	// nothing remains to walk, and the resume finishes the turn.
	AfterStrike bool `json:"after_strike,omitempty"`
}

// TurnBudgetData is the persistent representation of a [TurnBudget].
type TurnBudgetData struct {
	AttacksLeft  int `json:"attacks_left,omitempty"`
	MovementFeet int `json:"movement_feet,omitempty"`
}

// pauseDataFrom renders the pause for the blob, stamped with [PauseVersion].
func pauseDataFrom(p *pause) *PauseData {
	if p == nil {
		return nil
	}
	remaining := make([]PositionData, len(p.remaining))
	for i, c := range p.remaining {
		remaining[i] = PositionData{X: c.X, Y: c.Y}
	}
	d := &PauseData{
		Version:   PauseVersion,
		Kind:      p.kind,
		Member:    p.member,
		From:      PositionData{X: p.from.X, Y: p.from.Y},
		To:        PositionData{X: p.to.X, Y: p.to.Y},
		Remaining: remaining,
		Moved:     p.moved,
		At:        p.at,
		Audience:  append([]MemberID(nil), p.audience...),
		Cause:     causeString(p.cause),
		Forced:    p.forced,
	}
	if p.turn != nil {
		d.Turn = &TurnPauseData{
			Round: p.turn.round,
			Budget: TurnBudgetData{
				AttacksLeft:  p.turn.budget.AttacksLeft,
				MovementFeet: p.turn.budget.MovementFeet,
			},
			Intent:      p.turn.intent,
			Bound:       p.turn.bound,
			Terminal:    p.turn.terminal,
			AfterStrike: p.turn.afterStrike,
		}
	}
	return d
}

// causeString renders a walk's cause for the blob — the empty string for the
// ZERO Ref, which is how a turn's own Move says it has none. [core.Ref.String]
// would spell that as a pair of colons, and a decoder reading it back would
// have to know to treat that shape as absent.
//
// THE ZERO REF, NOT ANY INVALID ONE, and the difference is the whole reason
// this reads three fields instead of calling IsValid. A Ref that is malformed
// rather than absent is a defect somewhere above; rendering it as "" would
// silently turn a compelled walk into a chosen one in the story of every cell
// after a reload, which is the one lie the cause exists to prevent. Written
// out as whatever it is, it comes back through [parseCause] as ErrInvalidData
// and the load says so by name. Unreachable today — [Routed] and
// [Encounter.Direct] validate their cause before the walk starts and [Move]'s
// is the zero Ref — and kept that way deliberately rather than left to be
// discovered.
func causeString(cause core.Ref) string {
	if cause == (core.Ref{}) {
		return ""
	}
	return cause.String()
}

// pauseFrom rebuilds the live pause from validated bytes.
//
// It parses the cause a second time rather than carrying it out of
// [validatePause], because the two run at different moments: the validation is
// the trust boundary and runs before anything is constructed (R5), and this
// runs after the world is built. A parse that succeeded there succeeds here,
// and the error arm is kept rather than discarded so a future change to either
// cannot make this one lie by silence.
func pauseFrom(d *PauseData) (*pause, error) {
	if d == nil {
		return nil, nil
	}
	cause, cerr := parseCause(d.Cause)
	if cerr != nil {
		return nil, fmt.Errorf(
			"load encounter pause %q: cause %q: %w: %w", d.Member, d.Cause, ErrInvalidData, cerr)
	}
	remaining := make([]spatial.Position, len(d.Remaining))
	for i, c := range d.Remaining {
		remaining[i] = spatial.Position{X: c.X, Y: c.Y}
	}
	p := &pause{
		kind:      d.Kind,
		member:    d.Member,
		from:      spatial.Position{X: d.From.X, Y: d.From.Y},
		to:        spatial.Position{X: d.To.X, Y: d.To.Y},
		remaining: remaining,
		moved:     d.Moved,
		at:        d.At,
		audience:  append([]MemberID(nil), d.Audience...),
		cause:     cause,
		forced:    d.Forced,
	}
	if d.Turn != nil {
		p.turn = &turnPause{
			round: d.Turn.Round,
			budget: TurnBudget{
				AttacksLeft:  d.Turn.Budget.AttacksLeft,
				MovementFeet: d.Turn.Budget.MovementFeet,
			},
			intent:      d.Turn.Intent,
			bound:       d.Turn.Bound,
			terminal:    d.Turn.Terminal,
			afterStrike: d.Turn.AfterStrike,
		}
	}
	return p, nil
}

// parseCause is [causeString]'s inverse: the empty string is the zero Ref, a
// walk with no cause, and anything else must parse.
func parseCause(s string) (core.Ref, error) {
	if s == "" {
		return core.Ref{}, nil
	}
	ref, err := core.ParseString(s)
	if err != nil {
		return core.Ref{}, err
	}
	return *ref, nil
}

// validatePause is the trust boundary for a persisted pause: reject, never
// crash, and never resume onto a shape this build could not have written.
//
// THE ORDER IS PART OF THE CONTRACT. The version is asked first, so a pause
// another build wrote is refused as stale by name ([ErrStalePause]) rather
// than as whatever field happened to disagree. Then the envelope — a known
// kind, the turn arm present exactly on a turn, Forced only on a directive, a
// directive's cause a ref — and only then the walk itself.
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
// recorded before anybody calls [Encounter.Resume]. The host then reloads
// mid-verb and finds a paused walk whose member is on no clock — the ordinary
// consequence of the answer it just wrote down, refused at the door as
// corruption. The resume is the correct half: the announced step never
// happened, ruling R6 already holds, and resuming has nothing left to do but
// clear the pause. A directed walk never needed a clock in the first place.
func validatePause(d *PauseData, members map[core.EntityID]struct{}) error {
	if d == nil {
		return nil
	}
	if d.Version != PauseVersion {
		return fmt.Errorf("load encounter pause: version %d, this build reads %d: %w",
			d.Version, PauseVersion, ErrStalePause)
	}
	switch d.Kind {
	case PauseTurn, PauseDirective:
	default:
		return fmt.Errorf("load encounter pause: kind %q: %w", d.Kind, ErrInvalidData)
	}
	if d.Kind == PauseTurn && d.Turn == nil {
		return fmt.Errorf("load encounter pause %q: a turn pause carries no turn: %w", d.Member, ErrInvalidData)
	}
	if d.Kind == PauseDirective && d.Turn != nil {
		return fmt.Errorf("load encounter pause %q: a directive carries a turn: %w", d.Member, ErrInvalidData)
	}
	if d.Kind == PauseTurn && d.Forced {
		return fmt.Errorf("load encounter pause %q: a turn is never forced: %w", d.Member, ErrInvalidData)
	}
	if d.Kind == PauseDirective {
		if _, err := core.ParseString(d.Cause); err != nil {
			return fmt.Errorf(
				"load encounter pause %q: a directive's cause %q: %w: %w", d.Member, d.Cause, ErrInvalidData, err)
		}
	}

	if d.Member == "" {
		return fmt.Errorf("load encounter pause: names no member: %w", ErrInvalidData)
	}
	if _, ok := members[core.EntityID(d.Member)]; !ok {
		return fmt.Errorf("load encounter pause: %q is not a member: %w", d.Member, ErrInvalidData)
	}
	if d.Moved < 0 {
		return fmt.Errorf("load encounter pause %q: negative progress: %w", d.Member, ErrInvalidData)
	}
	if _, err := parseCause(d.Cause); err != nil {
		return fmt.Errorf(
			"load encounter pause %q: cause %q: %w: %w", d.Member, d.Cause, ErrInvalidData, err)
	}

	afterStrike := d.Turn != nil && d.Turn.AfterStrike
	if afterStrike {
		if len(d.Remaining) != 0 || d.Turn.Budget.AttacksLeft != 0 || d.Moved != 0 {
			return fmt.Errorf("load encounter paused strike %q: invalid continuation: %w", d.Member, ErrInvalidData)
		}
	} else {
		if len(d.Remaining) == 0 {
			return fmt.Errorf("load encounter pause %q: nothing left to walk: %w", d.Member, ErrInvalidData)
		}
		if d.To != d.Remaining[0] {
			return fmt.Errorf(
				"load encounter pause %q: the announced cell is not the first cell left to walk: %w",
				d.Member, ErrInvalidData)
		}
	}

	if t := d.Turn; t != nil {
		if t.Bound <= 0 || t.Intent < 0 || t.Intent >= t.Bound {
			return fmt.Errorf(
				"load encounter paused turn %q: intent %d is outside the turn's bound %d: %w",
				d.Member, t.Intent, t.Bound, ErrInvalidData)
		}
		if t.Budget.AttacksLeft < 0 || t.Budget.MovementFeet < 0 {
			return fmt.Errorf("load encounter paused turn %q: negative budget: %w", d.Member, ErrInvalidData)
		}
		if t.Round < 0 {
			return fmt.Errorf("load encounter paused turn %q: negative round: %w", d.Member, ErrInvalidData)
		}
	}
	return nil
}

// Paused reports whether a walk is stopped mid-route waiting on an answer.
//
// THE ONE QUESTION every drive entry asks before it drives and every
// pause-carrying output answers. A host reads it to know whether the fight is
// waiting on somebody — a driven turn paused mid-walk, or a directed walk
// held mid-route; [Encounter.PauseKind] says which, and [Encounter.Resume] is
// the only thing that makes this false again.
func (e *Encounter) Paused() bool { return e.pause != nil }

// PausedMember names whose walk is paused, or "" when nothing is.
func (e *Encounter) PausedMember() MemberID {
	if e.pause == nil {
		return ""
	}
	return e.pause.member
}

// PauseKind reports which walk the encounter is waiting on, and false when
// nothing is paused. A host does not need it to continue — [Encounter.Resume]
// dispatches on it — but it is the honest answer to "what is the table
// waiting for".
func (e *Encounter) PauseKind() (PauseKind, bool) {
	if e.pause == nil {
		return "", false
	}
	return e.pause.kind, true
}

// turnPaused reports whether the one pause is a driven TURN's. The drive loops
// ask this rather than [Encounter.Paused] because a held directive is not
// theirs to care about: the door guard already refused entry on one, and
// nothing inside a driven turn can create one.
func (e *Encounter) turnPaused() bool { return e.pause != nil && e.pause.kind == PauseTurn }

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

// ResumeOutput reports what continuing the paused walk did. Kind says which
// arm ran; the fields marked for the other arm are zero.
type ResumeOutput struct {
	// Kind is which walk was resumed: [PauseTurn] or [PauseDirective].
	Kind PauseKind

	// Next is whose turn it now is — a turn's. Always a member with a
	// player, on the same terms [EndTurnOutput.Next] states, EXCEPT when
	// Paused below is true: the walk stopped again on a later cell, and Next
	// is the paused member, whose turn has not ended.
	Next MemberID

	// RoundWrapped is true when finishing this turn, or any unplayed
	// member's turn driven after it, closed the round — a turn's.
	RoundWrapped bool

	// Moved is how many cells the directed walk took IN ALL, across every
	// pause of it, so the final resume reports the whole walk rather than
	// its last leg — a directive's, with [DirectOutput.Moved]'s meaning.
	Moved int

	// StoppedBy is why the directed walk fell short of its route, with
	// [DirectOutput.StoppedBy]'s meaning — a directive's.
	StoppedBy string

	// Seq is the story sequence of the LAST beat a resumed TURN recorded. A
	// directive reports none, as [DirectOutput] never has.
	Seq uint64

	// IntelDeltas maps member IDs to their updated percepts.
	IntelDeltas map[MemberID]*IntelDelta

	// Paused is true when a later cell of the same walk asked somebody
	// else — an ordinary outcome, not a failure. The fight is waiting again
	// and Resume is called again once that answer is in.
	Paused bool
}

// Resume finishes whichever walk is paused: a driven turn, then the rest of
// the drive, or a directed walk, then nothing.
//
// ONE WAY BACK (one pause envelope, ruling E2). The encounter holds one pause
// and its kind says which walk it is, so the host no longer chooses a verb —
// a host that guessed used to get [ErrNotPaused] from the wrong one. Both arms
// share the three things that make a resume safe:
//
//   - STANDING IS ASKED FIRST (ruling R6). A reaction that dropped the mover
//     while the window was open means the body is in the cell it was LEAVING,
//     and the announced step never happens.
//   - THE ANNOUNCED STEP IS NOT ANNOUNCED AGAIN. Every reactor for it was
//     already asked; asking twice would pose the same window again and, for a
//     reaction the host has since spent, refuse it the second time and
//     silently lose the swing. Every LATER cell is announced normally and may
//     pause again — a route is several windows, one per step.
//   - THE PAUSE IS CLEARED BEFORE THE FIRST STEP. A stale one left standing
//     would freeze a table that is running again.
//
// Errors: [ErrClosed] on a closed encounter, [ErrNotPaused] when nothing is
// paused, [ErrNotMember] when the mover is gone from the roster entirely, and
// whatever the walk or the drive itself can return.
func (e *Encounter) Resume(ctx context.Context) (*ResumeOutput, error) {
	if e.outcome != nil {
		return nil, fmt.Errorf("resume: %w", ErrClosed)
	}
	if e.pause == nil {
		return nil, fmt.Errorf("resume: %w", ErrNotPaused)
	}
	switch e.pause.kind {
	case PauseTurn:
		return e.resumeTurn(ctx)
	case PauseDirective:
		return e.resumeDirective(ctx)
	default:
		return nil, fmt.Errorf("resume: pause kind %q: %w", e.pause.kind, ErrInvalidData)
	}
}

// resumeTurn continues the turn a [Mover] paused, taking the announced step
// and finishing whatever the turn had left. [Encounter.Resume] has already
// proved the encounter open and the pause a turn's.
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
func (e *Encounter) resumeTurn(ctx context.Context) (*ResumeOutput, error) {
	p := e.pause
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
	e.pause = nil

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
			return &ResumeOutput{
				Kind:        PauseTurn,
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
		return &ResumeOutput{
			Kind: PauseTurn, Next: p.member, RoundWrapped: wrapped, Seq: seq, IntelDeltas: deltas,
		}, nil
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

	return &ResumeOutput{
		Kind:         PauseTurn,
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
	ctx context.Context, bubble *clock.Turn, p *pause, m *memberRecord,
	seq *uint64, wrapped *bool, deltas *map[MemberID]*IntelDelta,
) (bool, error) {
	walkSeq, walkDeltas, done, rerr := e.finishPausedIntent(ctx, p, m)
	*deltas = mergeIntelDeltas(*deltas, walkDeltas)
	if rerr != nil {
		return false, fmt.Errorf("resume turn %q: %w", p.member, rerr)
	}
	if e.turnPaused() {
		*seq = walkSeq
		return true, nil
	}

	// TERMINAL ENDS THE TURN INSTEAD OF ASKING AGAIN. A [Routed] intent's
	// whole turn is its walk, so once the walk is finished there is nothing
	// left to run — and a resume that fell into runTurnIntents here would ask
	// the driver for an intent the turn never had, which is exactly the
	// second Act a commanded creature must never get.
	if !done && !p.turn.terminal {
		intentSeq, intentWrapped, moreDeltas, ierr := e.runTurnIntents(
			bubble, core.EntityID(p.member), m, p.turn.round, &p.turn.budget, p.turn.intent+1, p.turn.bound)
		*deltas = mergeIntelDeltas(*deltas, moreDeltas)
		if ierr != nil {
			return false, fmt.Errorf("resume turn %q: %w", p.member, ierr)
		}
		*seq, *wrapped = intentSeq, intentWrapped
		if e.turnPaused() {
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
	ctx context.Context, p *pause, m *memberRecord,
) (seq uint64, deltas map[MemberID]*IntelDelta, done bool, err error) {
	// STANDING FIRST (ruling R6). A reaction that dropped the mover during
	// the window ends the turn where they stood, and the announced step
	// never happens.
	downNow, derr := e.downNow()
	if derr != nil {
		return 0, nil, false, fmt.Errorf("resume standing: %w", derr)
	}
	if downNow[p.member] {
		return 0, nil, true, nil
	}
	if p.turn.afterStrike {
		return 0, nil, false, nil
	}

	moved := 0
	// The announced cell, stepped WITHOUT a second announcement — see this
	// verb's own doc for why asking twice is worse than not asking at all.
	//
	// THE CAUSE TRAVELS WITH IT, and for a [Move] pause it is the zero Ref
	// that says there is none. This used to be the one line held.go's own
	// resume needed and this one did not, back when a turn's walk could never
	// have a cause; [Routed] made that false, and a first resumed cell
	// missing it would be a single beat in N claiming the creature walked
	// away of its own accord.
	action, stepped, stepErr := e.stepTo(m, p.to)
	if stepErr != nil {
		return 0, nil, false, stepErr
	}
	action.cause = p.cause
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
		rest, werr := e.walkPath(ctx, p.member, m, p.remaining[1:], p.audience, p.at, p.cause, false)
		if werr != nil {
			return 0, nil, false, fmt.Errorf("resume move: %w", werr)
		}
		res.moved += rest.moved
		res.dropped = rest.dropped
		res.paused = rest.paused
		res.from, res.to, res.pending = rest.from, rest.to, rest.pending
	}

	p.turn.budget.MovementFeet -= res.moved * FeetPerCell
	deltas, serr := e.settleWalk(p.audience, res.moved)
	if serr != nil {
		return 0, deltas, false, serr
	}

	if res.paused != nil {
		e.pause = &pause{
			kind:      PauseTurn,
			member:    p.member,
			from:      res.from,
			to:        res.to,
			remaining: res.pending,
			moved:     p.moved + res.moved,
			at:        p.at,
			audience:  p.audience,
			cause:     p.cause,
			turn: &turnPause{
				round:    p.turn.round,
				budget:   p.turn.budget,
				intent:   p.turn.intent,
				bound:    p.turn.bound,
				terminal: p.turn.terminal,
			},
		}
		wseq, berr := e.appendWindowOpenedBeat(
			p.member, res.from, res.to, p.at, res.paused.Windows, p.cause)
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
