// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

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
// when a taken offer cannot be paid, ErrInvalidSession for a stored window
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
	if err := closeWindow(scope, window, a.ledger()); err != nil {
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
		World:        scope.enc.WorldView(),
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
			castOut, err = m.finishCast(ctx, scope, w.Story.Caster, w.Story.Spell, *spellRef, w.Story.Caught, out, true)
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
