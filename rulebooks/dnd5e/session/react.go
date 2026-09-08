// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ReactChoice is one answer to an open reaction window.
//
// A CLOSED ENUM OWNED HERE, like [Verb], and deliberately not the ledger's own
// interrupt.Option: the custody module's vocabulary is "whatever this window
// offered", and this seam's is the two answers a reaction window actually
// poses. A host that matched on the inner type would be pinned to a module
// this package intends to be able to replace (law S2).
type ReactChoice string

const (
	// ReactStrike takes the reaction: the swing that was offered is resolved
	// against the mover, from the cell they are still standing in, and the
	// beat records what it was taken AS.
	ReactStrike ReactChoice = "strike"

	// ReactHold declines it, and COSTS NOTHING. The reaction is spent only
	// when the movement machine reports one taken (ruling R1), so a member
	// who holds still has their reaction for the next skeleton — which is the
	// whole reason Kirk asked to be asked.
	ReactHold ReactChoice = "hold"
)

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

	// Choice is [ReactStrike] or [ReactHold]. Anything else is
	// [ErrNotOffered].
	Choice ReactChoice
}

// ReactOutput is what answering a window did.
//
// DELIBERATELY THIN. Answering the last open window resumes a monster's turn,
// which can drive several more turns, form or dissolve a fight, and pause
// again — none of which is this member's business to be told in a return
// value. What happened reaches every member the way everything else does: as
// beats on their stream, read back through Story and Afford. A second, partial
// account of a turn nobody at this seam owns would be a shape that has to stay
// true, and it is one this verb cannot keep.
type ReactOutput struct {
	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// React answers an open interrupt window: a monster's step stopped to ask this
// member whether they take their reaction, and this is the answer.
//
// # It is the one verb that reaches a frozen session
//
// Every other change verb opens through [Manager.openForChange] and is refused
// with [ErrWindowOpen] while any window is open. This one uses openForWrite,
// because a frozen session is exactly the state it exists to operate on. The
// split is structural rather than a comment: a new verb picks its policy by
// picking its opener.
//
// # Striking resolves the SAME step a second time
//
// Not a hand-rolled swing. The window froze a mover, two cells and the attack
// this member was offered; [ReactStrike] hands all four back to the movement
// machine with a ReactionAttacks capability that answers for this reactor and
// nobody else. So the fold runs again — Disengage still silences it, the
// concealment seams still apply, the condition still triggers because holding
// costs nothing and left it unspent — and the swing that lands is the one the
// question described. [ReactHold] resolves nothing at all.
//
// # The last answer resumes the turn
//
// While any window is still open the fight stays frozen: two fighters asked
// about one step are two answers, and the first of them changes nothing but
// the ledger. When the last one lands, the encounter's own
// [encounter.Encounter.ResumeTurn] takes the announced step and finishes the
// turn — and may pause again on a later cell, which is not a failure but the
// next question.
//
// A strike that DROPS the mover ends the step for everybody: the remaining
// windows are answered [ReactHold] on their audiences' behalf, because there
// is no longer a walk to react to, and the mover falls in the cell they were
// leaving (ruling R6).
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrNotOffered for a choice this window does not offer, ErrNoSession,
// ErrNoEncounter, ErrNoWindow when the declaration names no open window,
// ErrNotAudience when it names somebody else's, ErrInvalidSession for a stored
// window this build could not have written, or ErrSaveFailed with a populated
// report.
func (m *Manager) React(ctx context.Context, in *ReactInput) (*ReactOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("react: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("react: %w", ErrNoMemberID)
	}
	if in.DeclarationID == "" {
		return nil, fmt.Errorf("react: %w", ErrNoDeclarationID)
	}
	// The choice is checked against the enum BEFORE anything is loaded, and
	// against the WINDOW's own options below. Two checks because they answer
	// two questions: this build does not know the word at all, versus this
	// window does not offer it.
	switch in.Choice {
	case ReactStrike, ReactHold:
	default:
		return nil, fmt.Errorf("react: choice %q: %w", in.Choice, ErrNotOffered)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	window, err := m.selectWindow(scope, in)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	kind, err := windowKindOf(window.Payload)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	if kind == windowKindPostRoll {
		return m.answerPostRoll(ctx, scope, window, in.Choice)
	}

	payload, err := thawWindowPayload(window.Payload, string(window.Audience))
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	if in.Choice == ReactStrike {
		if err := m.strikeForWindow(ctx, scope, payload); err != nil {
			return nil, fmt.Errorf("react: %w", err)
		}
	}

	if err := answerWindow(scope, window, ReactChoice(in.Choice)); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	// THE MOVER IS DOWN AND THERE IS NOTHING LEFT TO REACT TO. Asking the
	// remaining audiences anyway would offer a swing at a body, and refusing
	// to resume until they answered it would freeze the table on a question
	// with one honest answer. The custodian may answer for them — the ledger
	// allows By == Audience — so it does, and says so here rather than in a
	// silent branch.
	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	if down[payload.Mover] {
		if err := holdRemainingWindows(scope); err != nil {
			return nil, fmt.Errorf("react: %w", err)
		}
	}

	open, err := scope.ledger.Open()
	if err != nil {
		return nil, fmt.Errorf("react: %w: %v", ErrInvalidSession, err)
	}
	if len(open) == 0 && scope.enc.Paused() {
		// Resuming can pause AGAIN on a later cell — a new question, not a
		// failure — and the pose that does it writes its own windows onto
		// this same ledger. Both outcomes are read off the ledger below.
		if _, err := scope.enc.ResumeTurn(ctx); err != nil {
			return nil, fmt.Errorf("react: %w", translate(err))
		}
	}

	// Written once, AFTER the resume, so a walk that stopped again on a later
	// cell persists the windows it just posed rather than the empty ledger it
	// briefly had.
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	return &ReactOutput{Saved: report, Delivery: delivery}, nil
}

// selectWindow finds the open window a declaration id names.
//
// BY REGENERATION, NEVER BY PARSING. The id is a hash over the session, the
// member, the verb, the slot and the window — the same construction every other
// verb's selector uses — so the only honest way to read one is to mint the
// candidates and compare. That is also what makes the three refusals distinct:
// a scan over EVERY open window, not just this member's, can tell "no such
// window" from "not yours", which a scan over the caller's own windows would
// collapse into the first.
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
		if !offers(window.Options, in.Choice) {
			return interrupt.Window{}, fmt.Errorf("choice %q: %w", in.Choice, ErrNotOffered)
		}
		return window, nil
	}
	return interrupt.Window{}, ErrNoWindow
}

// offers reports whether a window posed this choice.
func offers(options []interrupt.Option, choice ReactChoice) bool {
	for _, option := range options {
		if string(option) == string(choice) {
			return true
		}
	}
	return false
}

// answerWindow closes one window in the ledger.
func answerWindow(scope *writeScope, window interrupt.Window, choice ReactChoice) error {
	if _, err := scope.ledger.Answer(&interrupt.AnswerInput{
		Window: window.ID,
		By:     window.Audience,
		Choice: interrupt.Option(choice),
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	scope.touched = true
	return nil
}

// holdRemainingWindows answers every still-open window [ReactHold] on its own
// audience's behalf. See [Manager.React] on when that is honest.
func holdRemainingWindows(scope *writeScope) error {
	open, err := scope.ledger.Open()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	for _, window := range open {
		if err := answerWindow(scope, window, ReactHold); err != nil {
			return err
		}
	}
	return nil
}

// strikeForWindow resolves the swing one accepted window offered.
//
// It re-offers the frozen step to the rules with a capability that answers for
// this reactor alone — see [Manager.React] on why the machine runs twice rather
// than the swing being compiled here.
func (m *Manager) strikeForWindow(ctx context.Context, scope *writeScope, payload windowPayload) error {
	roster, err := scope.enc.Members()
	if err != nil {
		return translate(err)
	}
	return moverSeam{m: m, scope: scope}.offerStep(
		ctx, scope.enc, encounter.MemberID(payload.Mover), payload.From, payload.To, roster,
		&reactionAttacks{only: payload.Reactor, definition: payload.Definition},
	)
}

// reactDeclarationID is the selector for one open window's REACT row. Minted
// in exactly one place, so [Manager.Afford] and [Manager.React] cannot mint
// two different ids for one window.
func reactDeclarationID(session, member string, window interrupt.WindowID) (string, error) {
	id, _, err := selectorIDFor(session, member, VerbReact, SlotReaction, nil, nil, "", windowIDString(window))
	return id, err
}
