// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Answer is one answer to an open window: [Take] or [Decline].
//
// It mirrors resolution's answer at this boundary (law S2: no inner type
// crosses), and it is a value rather than an enum because Take carries the
// option the offer listed. THE ZERO ANSWER IS REFUSED with [ErrNotOffered]: a
// caller that built a ReactInput and never said what it chose has no answer
// to give, and reading that as a decline would spend somebody's question on a
// forgotten field.
type Answer struct {
	given  bool // set by Take and Decline; the zero Answer is refused
	take   bool
	option string
}

// Take accepts the offer. option is one of the offer's choices — the row's
// [Declaration.Options] — when it lists any, and empty when it lists none.
// Anything else is refused with [ErrNotOffered] once the window is read.
func Take(option string) Answer { return Answer{given: true, take: true, option: option} }

// Decline refuses the offer. Declining costs nothing and carries no option.
func Decline() Answer { return Answer{given: true} }

// Taken reports whether this answer takes the offer.
func (a Answer) Taken() bool { return a.take }

// Option is the chosen option, or empty.
func (a Answer) Option() string { return a.option }

// resolution is this answer in resolution's vocabulary, rebuilt through its
// own constructors because its fields are unexported.
func (a Answer) resolution() resolution.Answer {
	if a.take {
		return resolution.Take(a.option)
	}
	return resolution.Decline()
}

// ledger is the ledger's bookkeeping word for this answer.
func (a Answer) ledger() interrupt.Option {
	if a.take {
		return ledgerTake
	}
	return ledgerDecline
}

// ReactInput answers one open interrupt window.
type ReactInput struct {
	// Session is the session the window was posed in.
	Session string

	// Member is who is answering. Required.
	//
	// THE HOST MUST BIND THIS TO THE AUTHENTICATED CALLER, exactly as
	// [Manager.Afford] requires: a client-supplied ID wired through unchecked
	// would let one player answer another's window, which is the one refusal
	// [ErrNotAudience] exists to make.
	Member string

	// DeclarationID is the opaque selector from the REACT declaration
	// [Manager.Afford] offered. Required, echoed verbatim, never parsed by a
	// client — and never parsed by this verb either: React matches it by
	// regenerating every open window's selector.
	DeclarationID string

	// Answer is [Take] or [Decline]. The zero Answer is [ErrNotOffered].
	Answer Answer
}

// ReactOutput is what answering a window did.
//
// DELIBERATELY THIN. Answering the last open window resumes a monster's turn,
// which can drive several more turns, form or dissolve a fight, and pause
// again — none of which is this member's business to be told in a return
// value. What happened reaches every member the way everything else does: as
// beats on their stream, read back through Story and Afford.
type ReactOutput struct {
	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`

	// RollKept is true when the answer reached a waiting line its speaker
	// had walked away from (rpg-project#938): the roll was kept and settled
	// DECLINED whatever the answer was — the held offer is not spent — and
	// the line's verdict landed. A caller shows "you walked away: your roll
	// stands, your die is kept".
	RollKept bool `json:"roll_kept,omitempty"`
}

// React answers an open interrupt window.
//
// # It is the one verb that reaches a frozen session
//
// Every other change verb opens through [Manager.openForChange] and is refused
// with [ErrWindowOpen] while any window is open. This one uses openForWrite,
// because a frozen session is exactly the state it exists to operate on.
//
// # One window, one dispatch
//
// Every window holds one stored payload and every answer goes through
// [Manager.answerWindow]: the window is read, consumed, and its pause handed
// back to resolution with the answer. What settled before the pause was told
// when it paused; the resume tells only what settled after. A taken offer is
// charged once, by resolution, at the price the pause stated.
//
// # The last answer resumes the table
//
// While any window is still open the fight stays frozen: two fighters asked
// about one step are two answers, and the first changes nothing but the
// ledger and its own swing. When the last one lands, a player's interrupted
// walk continues, or the encounter's paused walk is resumed — and may pause
// again on a later cell, which is not a failure but the next question.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrNotOffered for the zero Answer (before anything is loaded) or an answer
// the offer does not accept, ErrNoSession, ErrNoEncounter, ErrNoWindow when
// the declaration names no open window, ErrNotAudience when it names somebody
// else's, ErrStalePause for a window an earlier build posed, ErrCannotAfford
// when a taken offer cannot be paid, ErrStaleOption for a paused line whose
// conversation hold is gone (the window is closed declined and nothing the
// answer offered is spent; a paused line its speaker walked away from is not
// refused but settled declined, told as [ReactOutput.RollKept]), ErrInvalidSession for a stored window
// this build could not have written, or ErrSaveFailed with a populated report.
func (m *Manager) React(ctx context.Context, in *ReactInput) (*ReactOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("react: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Member == "" {
		return nil, fmt.Errorf("react: %w", ErrNoMemberID)
	}
	if in.DeclarationID == "" {
		return nil, fmt.Errorf("react: %w", ErrNoDeclarationID)
	}
	// Refused BEFORE anything is loaded: no answer was given, so there is no
	// window worth reading.
	if !in.Answer.given {
		return nil, fmt.Errorf("react: %w: no answer was given", ErrNotOffered)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	window, err := m.selectWindow(scope, in)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	out, err := m.answerWindow(ctx, scope, window, in.Answer)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	return out, nil
}

// answerWindow is the one dispatch: the only reader that resumes a window.
//
// # The window is consumed BEFORE the resume
//
// Resolution is stateless — a pause resumed twice runs, and charges, twice —
// so answering once is this package's job. The window is closed on the ledger
// first, then the pause is resumed; a failure anywhere after drops the whole
// scope, so the window is never answered without its resume landing, and a
// second answer finds no open window.
//
// # Everything else is the story's
//
// A check resumes through [resolution.ResumeCheck] and lands as the check's
// own verb. Every other story resumes through [resolution.Resume], resolves
// over the whole roster, and lands through [Manager.land] with that story's
// one record function over [resolution.Output.Outcome], a window for whatever
// the output poses, and the last-answer continuation when it poses nothing.
func (m *Manager) answerWindow(
	ctx context.Context, scope *writeScope, window interrupt.Window, a Answer,
) (*ReactOutput, error) {
	w, err := thawWindow(window.Payload, string(window.Audience))
	if err != nil {
		return nil, err
	}
	// A PAUSED UNLOCK ON A DOOR HELD SINCE (rpg-project#852, R7): the hold
	// answers before the resumed die is read, and the pause is CLOSED, never
	// left open. The window is declined whatever the answer was, so a taken
	// offer is not spent, and the decline is committed before the refusal
	// returns: a refusal that dropped the scope would leave the window in
	// the ledger and the session frozen behind it.
	if w.Story.Kind == storyCheck && w.Story.Door != "" {
		if held := refuseHeldDoor(scope.enc, w.Story.Door, w.Audience); held != nil {
			return nil, m.declineHeldUnlock(ctx, scope, window, held)
		}
	}
	// A PAUSED LINE WHOSE HOLD DID NOT STAND (rpg-project#924, #938), asked
	// BEFORE the resume, because the resume is what spends the held die:
	//
	//   - The speaker and the target lost reach or sight of each other: the
	//     roll is KEPT. It is settled declined whatever the answer was, so
	//     the held offer is not spent, committed, and the caller is told
	//     [ReactOutput.RollKept].
	//   - The target fell or left, or the conversation otherwise moved: the
	//     settle would be refused, so it is never attempted. The window is
	//     closed declined, nothing the answer offered is spent, the decline
	//     is committed, and the answer is told the conversation moved on.
	if w.Story.Kind == storyCheck && w.Story.Interaction != "" {
		state, err := waitingHold(scope.enc, w.Audience, w.Story)
		if err != nil {
			return nil, err
		}
		switch state {
		case holdGone:
			return nil, m.declineAbandonedLine(ctx, scope, window)
		case holdAbandoned, holdLeft:
			return m.keepAbandonedRoll(ctx, scope, window, w, state)
		}
	}
	if err := closeWindow(scope, window, a.ledger()); err != nil {
		return nil, err
	}
	// THE ANSWER IS ITS TRIGGER'S ACT (rpg-project#627, Law 2): the reaction's
	// beats, and the walk that continues after the last answer, carry the id
	// of the walk or swing the window interrupted. A window saved before acts
	// existed names none, and the answer mints a react act of its own.
	if err := scope.openAct(string(VerbReact), w.Story.ActID); err != nil {
		return nil, err
	}
	answer := a.resolution()

	if w.Story.Kind == storyCheck {
		return m.answerCheck(ctx, scope, w, answer)
	}

	machine, err := resolution.Resume(&resolution.ResumeInput{
		Pause:  w.Pause,
		Answer: answer,
		// The HOST'S dice, through the same seam every other roll takes —
		// required whichever the answer is.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, translateResolution(err)
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, translate(err)
	}
	// EVERYONE IS ATTACHED, as the first half was resolved: a resumed half
	// among fewer subscribers than its first half is a smaller world.
	var participants []resolution.Participant
	if w.Story.Kind == storyCast {
		// A cast loads every member fresh: nothing is paid on a resume, so
		// there is no readied ledger to thread through.
		var failures []resolutionDependencyFailure
		participants, failures = m.compileResolutionCast(ctx, scope.data, roster, nil)
		if len(failures) > 0 {
			return nil, fmt.Errorf("participant %q: %w: %v", failures[0].member, ErrBadCharacter, failures[0].err)
		}
	} else {
		participants = m.walkCast(ctx, scope, roster)
	}

	// No cost: a taken offer is charged by resolution at the price its pause
	// froze, and the declared action was paid for before the first pause.
	out, err := resolution.Resolve(ctx, m.resolutionInput(ctx, scope, resolutionAsk{
		Live:         scope.enc,
		Participants: participants,
		Machine:      machine,
	}))
	if err != nil {
		if w.Story.Kind == storyAttack {
			return nil, translateAttack(err)
		}
		return nil, translateResolution(err)
	}

	var l *landing
	posed := out.Posed != nil
	switch w.Story.Kind {
	case storyCast:
		var castOut *CastOutput
		if posed {
			castOut, err = m.poseCastWindow(ctx, scope, w.Story.Caster, w.Story.Spell, w.Story.Caught, out)
		} else {
			spellRef, perr := core.ParseString(w.Story.Spell.Ref)
			if perr != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidSession, perr)
			}
			castOut, err = m.finishCast(ctx, scope, w.Story.Caster, w.Story.Spell, *spellRef, w.Story.Caught, out)
		}
		if err != nil {
			return nil, err
		}
		return &ReactOutput{Saved: castOut.Persisted, Delivery: castOut.Delivery}, nil
	case storyAttack:
		l = m.attackLanding(scope, nil, w.Story, out, nil)
	case storyStep:
		var windows []encounter.PausedWindow
		l, windows, err = m.stepLanding(scope, nil, w.Story, out)
		if err != nil {
			return nil, err
		}
		posed = len(windows) > 0
	}

	if !posed {
		story := w.Story
		l.Continue = func(*encounter.Encounter) error {
			// THE MOVER IS DOWN AND THERE IS NOTHING LEFT TO REACT TO. The
			// remaining windows on the step are declined on their audiences'
			// behalf — the ledger allows By == Audience — because a swing at
			// a body is a question with one honest answer.
			if story.Kind == storyStep {
				down, err := discoveryStanding(scope)
				if err != nil {
					return err
				}
				if down[story.Target] {
					if err := holdRemainingWindows(scope); err != nil {
						return err
					}
				}
			}
			walker := ""
			if len(story.WalkPath) > 0 {
				walker = story.Target
			}
			return m.resumeAfterLastAnswer(ctx, scope, walker, story.WalkPath)
		}
	}

	result, err := m.land(ctx, scope, out, l)
	if err != nil {
		return nil, err
	}
	return &ReactOutput{Saved: result.Saved, Delivery: result.Delivery}, nil
}

// resumeAfterLastAnswer continues whatever the table was waiting on, once the
// window just answered was the last one open. Every answer that poses nothing
// ends here, so no two answers can disagree about what "the last answer"
// resumes (rpg-toolkit#1965).
//
// NOTHING RESUMES WHILE A QUESTION STANDS. Another audience still deciding is
// still holding the table, and continuing past them would take the announced
// step out from under their answer — the step the encounter's pause holds is
// taken once, here, after the last asked reactor answered (ruling E8).
//
// Resuming can pause AGAIN on a later cell — a new question, not a failure —
// and the pose that does it writes its own windows onto this same ledger.
func (m *Manager) resumeAfterLastAnswer(
	ctx context.Context, scope *writeScope, walker string, walkPath []spatial.Position,
) error {
	open, err := scope.ledger.Open()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	if len(open) > 0 {
		return nil
	}
	// Every continuation runs after the answer saved dirty sheets — and a
	// walk's earlier cells may have saved more — so whichever one refuses, the
	// refusal names what landed (S6).
	return saveErrorAfterWrites(scope, "", m.continueAfterLastAnswer(ctx, scope, walker, walkPath))
}

// continueAfterLastAnswer runs the one continuation, narrowest first:
//
//   - walkPath is a player's own walk a reaction interrupted, carried on the
//     window that stopped it.
//   - Otherwise the encounter's one paused walk — a driven turn or a directed
//     walk — is finished by [encounter.Encounter.Resume], which takes the
//     announced step and walks on.
//
// It does not report writes; its caller does.
func (m *Manager) continueAfterLastAnswer(
	ctx context.Context, scope *writeScope, walker string, walkPath []spatial.Position,
) error {
	switch {
	case len(walkPath) > 0:
		if _, err := m.runWalk(ctx, scope, walker, walkPath); err != nil {
			return err
		}
		return m.saveWalkProgress(ctx, scope)
	case scope.enc.Paused():
		if _, err := scope.enc.Resume(ctx); err != nil {
			return translate(err)
		}
	}
	return nil
}

// selectWindow finds the open window a declaration id names.
//
// BY REGENERATION, NEVER BY PARSING. The id is a hash over the session, the
// member, the verb, the slot and the window — the same construction every other
// verb's selector uses — so the only honest way to read one is to mint the
// candidates and compare. A scan over EVERY open window, not just this
// member's, tells "no such window" from "not yours".
func (m *Manager) selectWindow(scope *writeScope, in *ReactInput) (interrupt.Window, error) {
	open, err := scope.ledger.Open()
	if err != nil {
		return interrupt.Window{}, fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	for _, window := range open {
		id, derr := reactDeclarationID(scope.session, string(window.Audience), window.ID)
		if derr != nil {
			return interrupt.Window{}, fmt.Errorf("%w: %v", ErrInvalidSession, derr)
		}
		if id != in.DeclarationID {
			continue
		}
		if string(window.Audience) != in.Member {
			return interrupt.Window{}, ErrNotAudience
		}
		return window, nil
	}
	return interrupt.Window{}, ErrNoWindow
}

// closeWindow closes one window in the ledger and writes the ledger onto the
// session record.
func closeWindow(scope *writeScope, window interrupt.Window, choice interrupt.Option) error {
	if _, err := scope.ledger.Answer(&interrupt.AnswerInput{
		Window: window.ID,
		By:     window.Audience,
		Choice: choice,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true
	return nil
}

// holdRemainingWindows declines every still-open window on its own audience's
// behalf, resuming none: a declined window charges nothing and tells nothing.
// See [Manager.answerWindow] on when that is honest.
func holdRemainingWindows(scope *writeScope) error {
	open, err := scope.ledger.Open()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	for _, window := range open {
		if err := closeWindow(scope, window, ledgerDecline); err != nil {
			return err
		}
	}
	return nil
}

// reactDeclarationID is the selector for one open window's REACT row. Minted
// in exactly one place, so [Manager.Afford] and [Manager.React] cannot mint
// two different ids for one window.
func reactDeclarationID(session, member string, window interrupt.WindowID) (string, error) {
	id, _, err := selectorIDFor(session, member, VerbReact, SlotReaction, nil, nil, "", windowIDString(window))
	return id, err
}

// declineHeldUnlock closes a paused unlock's window declined, commits it,
// and answers with the held refusal: the attempt ends with no verdict and
// the lock as it was.
func (m *Manager) declineHeldUnlock(
	ctx context.Context, scope *writeScope, window interrupt.Window, held error,
) error {
	var refusal *DoorHeldError
	if !errors.As(held, &refusal) {
		// Not a hold: a door or member the composition refuses outright,
		// which the scope drops as any other failed answer.
		return held
	}
	if err := closeWindow(scope, window, ledgerDecline); err != nil {
		return err
	}
	if _, _, err := m.commit(ctx, scope); err != nil {
		return err
	}
	return refusal
}

// holdState is what became of the reservation a paused Speak check placed
// ([waitingHold]).
type holdState int

const (
	// holdStands: the line still waits on this roll and its settle would
	// land as the world stands.
	holdStands holdState = iota

	// holdAbandoned: the encounter abandoned the hold (rpg-project#938) — the
	// holder and the target lost reach or sight of each other — and the roll
	// is kept: settled declined ([Manager.settleKeptRoll]).
	holdAbandoned

	// holdLeft: the same, found by the session's own doors before the
	// encounter's next participation pass marked it; the hold is abandoned
	// ([encounter.Encounter.AbandonReservation]) and then settled declined.
	holdLeft

	// holdGone: nobody is left to answer, or the conversation moved — the
	// target down or gone, the holder unplaced, the line no longer held at
	// this revision. Ended with no verdict ([endWaitingLine]).
	holdGone
)

// waitingHold reports what became of the reservation a paused Speak check
// placed (rpg-project#924, #938). Asked of the composition's own offer to the
// holder: the line is still offered to them, pending, on the interaction the
// story names, at the revision the reservation moved it to
// ([windowStory.Held]). Every place, settle, release and advance moves that
// revision — abandoning does not — so an equal revision is THIS
// reservation, and anything else is a hold that is gone.
//
// A held line the encounter marked abandoned is [holdAbandoned]. Otherwise
// the settle's witness and reach doors are asked as Speak asks them, because
// a verb that runs no participation pass — a door closed between them —
// leaves the hold standing with a settle that would be refused: unwitnessed
// or out of reach is [holdLeft], a roll kept as the encounter's own sweep
// keeps it. A holder or target the composition no longer answers for —
// down, gone, unplaced, a prop no longer shown — is [holdGone], as the
// encounter's sweep releases exactly those. Any other refusal is a defect,
// surfaced.
func waitingHold(enc *encounter.Encounter, holder string, story windowStory) (holdState, error) {
	var (
		lines []encounter.OfferedOption
		err   error
	)
	if story.Prop != "" {
		lines, err = enc.PropOptionsFor(encounter.MemberID(holder), encounter.PropID(story.Prop))
	} else {
		lines, err = enc.OptionsFor(encounter.MemberID(holder), encounter.MemberID(story.Target))
	}
	for _, gone := range []error{
		encounter.ErrNotMember, encounter.ErrMemberDown, encounter.ErrBadPlacement, encounter.ErrNoProp,
	} {
		if errors.Is(err, gone) {
			return holdGone, nil
		}
	}
	if err != nil {
		return holdGone, translate(err)
	}
	var line *encounter.OfferedOption
	for i := range lines {
		if lines[i].ID == story.Option {
			line = &lines[i]
			break
		}
	}
	if line == nil || !line.Pending || string(line.Interaction) != story.Interaction || line.Revision != story.Held {
		return holdGone, nil
	}
	if line.Abandoned {
		return holdAbandoned, nil
	}
	if story.Target != "" {
		witnesses, err := enc.Witnesses(encounter.MemberID(holder))
		if err != nil {
			return holdGone, translate(err)
		}
		if !slices.Contains(witnesses, encounter.MemberID(story.Target)) {
			return holdLeft, nil
		}
	}
	if err := refuseSpeakOutOfReach(enc, holder, story.Target, story.Prop); err != nil {
		switch {
		case errors.Is(err, ErrOutOfRange):
			return holdLeft, nil
		case errors.Is(err, ErrStaleDeclaration) || errors.Is(err, ErrBadPosition):
			return holdGone, nil
		}
		return holdGone, err
	}
	return holdStands, nil
}

// settleRefusals are the encounter's refusals of a kept roll's settle that
// mean nobody is left to answer it as the world stands — the target down or
// gone, the line no longer offered or its revision moved — so the hold is
// released with no verdict instead ([Manager.settleKeptRoll]).
var settleRefusals = []error{
	encounter.ErrNotMember, encounter.ErrMemberDown, encounter.ErrNoProp, encounter.ErrBadPlacement,
	encounter.ErrNoOption, encounter.ErrStaleOption, encounter.ErrReserved,
}

// settleKeptRoll ends a waiting line whose holder and target lost reach or
// sight of each other (rpg-project#938, "a roll is a roll that is kept"):
// the window is closed declined; the stored pause is resumed DECLINED,
// whatever the holder asked, so the held benefit is never spent; and the
// line is said at the reserved revision with that verdict, so its answer,
// advance and grants land once and the encounter clears the hold. A hold the
// session found left before the encounter marked it is abandoned first
// ([encounter.Encounter.AbandonReservation]). A settle the encounter refuses
// because nobody is left to answer ([settleRefusals]) releases the hold
// with no verdict, as [endWaitingLine] does, so nothing strands. The
// caller's act must be open: the settle's beats are its.
func (m *Manager) settleKeptRoll(
	ctx context.Context, scope *writeScope, window interrupt.Window, w pendingWindow, state holdState,
) error {
	if state == holdLeft {
		if _, err := scope.enc.AbandonReservation(&encounter.AbandonReservationInput{
			Member: encounter.MemberID(w.Audience),
		}); err != nil {
			return translate(err)
		}
	}
	if err := closeWindow(scope, window, ledgerDecline); err != nil {
		return err
	}
	data, err := m.sheetsFor(nil).load(ctx, "member", w.Audience)
	if err != nil {
		return err
	}
	out, err := resolution.ResumeCheck(ctx, &resolution.CheckResumeInput{
		Pause: w.Pause, Answer: resolution.Decline(), Character: data, Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return translateResolution(err)
	}
	if out.Posed != nil {
		return fmt.Errorf("%w: a declined check asked again", ErrInvalidWorld)
	}
	if out.DirtyCharacter != nil {
		if err := m.sheetsFor(scope).save(ctx, out.DirtyCharacter); err != nil {
			return err
		}
	}
	if err := requireCalculation("react", w.Audience, rollCalculationFor(out.Calculation)); err != nil {
		return err
	}
	err = m.landResumedSocial(ctx, scope, w.Story, w.Audience, out)
	var refused bool
	for _, refusal := range settleRefusals {
		refused = refused || errors.Is(err, refusal)
	}
	if !refused {
		return err
	}
	_, err = scope.enc.ReleaseReservation(&encounter.ReleaseReservationInput{Member: encounter.MemberID(w.Audience)})
	if err != nil && !errors.Is(err, encounter.ErrNoReservation) && !errors.Is(err, encounter.ErrNotMember) {
		return translate(err)
	}
	return nil
}

// keepAbandonedRoll answers a React on a waiting line the encounter
// abandoned before the answer came (rpg-project#938): settled declined
// whatever the answer was ([Manager.settleKeptRoll]), committed, and told
// to the caller as [ReactOutput.RollKept].
func (m *Manager) keepAbandonedRoll(
	ctx context.Context, scope *writeScope, window interrupt.Window, w pendingWindow, state holdState,
) (*ReactOutput, error) {
	if err := scope.openAct(string(VerbReact), w.Story.ActID); err != nil {
		return nil, err
	}
	if settleErr := m.settleKeptRoll(ctx, scope, window, w, state); settleErr != nil {
		// Ended with no verdict, committed, and the one caller told why.
		if err := endFailedKeptRoll(scope, window, w, settleErr); err != nil {
			return nil, err
		}
		if _, _, err := m.commit(ctx, scope); err != nil {
			return nil, err
		}
		return nil, settleErr
	}
	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, err
	}
	return &ReactOutput{Saved: report, Delivery: delivery, RollKept: true}, nil
}

// endFailedKeptRoll ends a kept roll whose settle failed for a reason
// [settleRefusals] does not cover — a sheet that would not load, a pause
// resolution refused, a capability's error — as the nobody-can-answer path
// does: the window is closed declined if [Manager.settleKeptRoll] had not
// closed it yet, the hold is released with no verdict, and the held offer
// stays unspent (the declined resume spends nothing). What happened is
// recorded on the scope ([SaveReport.Faults]) so it is never silent.
func endFailedKeptRoll(scope *writeScope, window interrupt.Window, w pendingWindow, cause error) error {
	open, err := scope.ledger.Open()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	if slices.ContainsFunc(open, func(o interrupt.Window) bool { return o.ID == window.ID }) {
		if err := closeWindow(scope, window, ledgerDecline); err != nil {
			return err
		}
	}
	_, err = scope.enc.ReleaseReservation(&encounter.ReleaseReservationInput{Member: encounter.MemberID(w.Audience)})
	if err != nil && !errors.Is(err, encounter.ErrNoReservation) && !errors.Is(err, encounter.ErrNotMember) {
		return translate(err)
	}
	scope.faults = append(scope.faults, fmt.Sprintf(
		"%s's kept roll on line %q could not settle and was released with no verdict: %v", w.Audience, w.Story.Option, cause))
	return nil
}

// endWaitingLine closes a waiting line's window declined and, in the same
// commit, ends the encounter's hold for its speaker if it still stands
// ([encounter.Encounter.ReleaseReservation], with its `held: false` beat),
// so a question and its hold always end together: no hold outlives its
// question, holding lines pending for everyone with nobody to answer
// (rpg-project#924). A hold the encounter already let go is not released
// twice ([encounter.ErrNoReservation]), and a holder who left the run holds
// nothing ([encounter.ErrNotMember]). The speaker's own verbs are refused
// while their roll waits ([Manager.openForChangeBy]), so the only hold they
// can hold is this window's.
func endWaitingLine(scope *writeScope, window interrupt.Window) error {
	if err := closeWindow(scope, window, ledgerDecline); err != nil {
		return err
	}
	_, err := scope.enc.ReleaseReservation(&encounter.ReleaseReservationInput{Member: window.Audience})
	if err != nil && !errors.Is(err, encounter.ErrNoReservation) && !errors.Is(err, encounter.ErrNotMember) {
		return translate(err)
	}
	return nil
}

// declineAbandonedLine ends a paused Speak check whose settle could no
// longer land ([endWaitingLine]) and commits it, then answers
// [ErrStaleOption]: the attempt ends with no verdict and the held offer
// unspent ([Manager.declineHeldUnlock]'s shape). The action, spent before
// the question, stays spent.
func (m *Manager) declineAbandonedLine(ctx context.Context, scope *writeScope, window interrupt.Window) error {
	if err := endWaitingLine(scope, window); err != nil {
		return err
	}
	if _, _, err := m.commit(ctx, scope); err != nil {
		return err
	}
	return fmt.Errorf("the conversation moved on while the roll waited: %w", ErrStaleOption)
}
