// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// FrozenError reports that a verb was refused because a window is open.
//
// It names the window and its audience rather than only saying "no", because a
// caller who is blocked needs to know who to go ask. A bare sentinel would send
// them to Pending to find out what this error already knew.
type FrozenError struct {
	// Window is the open window blocking the verb.
	Window string

	// Audience is the member who owes the answer.
	Audience string
}

// Error describes the open window blocking the verb.
func (e *FrozenError) Error() string {
	return fmt.Sprintf("world frozen: window %s awaits %s", e.Window, e.Audience)
}

// Unwrap returns ErrFrozen so callers can match the condition with errors.Is
// while still recovering the details with errors.As.
func (e *FrozenError) Unwrap() error { return ErrFrozen }

// AnswerInput resolves an open window.
type AnswerInput struct {
	// Session is the session holding the window.
	Session string

	// Window is the window being answered, as reported in Pending.Window.
	Window string

	// Member must be the window's audience. Answering someone else's window is
	// a rejection.
	Member string

	// Option is the Token of the chosen option.
	Option string
}

// AnswerOutput reports what resuming produced.
//
// It covers the resumption only: the steps taken before the world froze were
// already reported to whoever called Move. The continuous version of the story
// is the story itself, which Story returns in one unbroken sequence regardless
// of how many calls it took to produce.
type AnswerOutput struct {
	// Steps are the cells entered after resuming, in order. Empty if the answer
	// stopped the walk.
	Steps []Step `json:"steps,omitempty"`

	// Discovered is what changed in each observer's perception while resuming.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Outcome is present if an ending fired underfoot after resuming.
	Outcome *Outcome `json:"outcome,omitempty"`

	// Pending is present if resuming hit another checkpoint. A walk may
	// suspend as many times as it has things to notice.
	Pending *Pending `json:"pending,omitempty"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// PendingInput asks what windows are open in a session.
type PendingInput struct {
	// Session is the session to inspect.
	Session string
}

// PendingOutput lists the open windows.
type PendingOutput struct {
	// Windows are the open windows in the order they were opened. Empty means
	// the world is running.
	Windows []Pending `json:"windows,omitempty"`
}

// Answer resolves an open window and resumes whatever was waiting on it.
//
// This is the one verb that reaches a frozen session, because it is the only
// thing that can unfreeze one. Choosing OptionContinue resumes the suspended
// resolution from exactly the cell it stopped at; choosing OptionStop abandons
// what remained. Neither undoes what already happened — a walk that stopped
// after three steps stopped there, and the member stands where they stood.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoWindowID,
// ErrNoSession, ErrNoEncounter, ErrNoWindow if the window is not open,
// ErrNotAudience if the member does not owe it, ErrNotOffered if the choice was
// not on the menu, ErrInvalidSession if the frozen resolution is unreadable, or
// ErrSaveFailed with a populated report.
func (m *Manager) Answer(ctx context.Context, in *AnswerInput) (*AnswerOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("answer: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("answer: %w", ErrNoMemberID)
	}

	windowID, err := parseWindow(in.Window)
	if err != nil {
		return nil, fmt.Errorf("answer: %w", err)
	}

	// openForWrite, not openForChange: a frozen session is exactly the state
	// this verb exists to operate on.
	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("answer: %w", err)
	}

	closed, err := scope.ledger.Answer(&interrupt.AnswerInput{
		Window: windowID,
		By:     core.EntityID(in.Member),
		Choice: interrupt.Option(in.Option),
	})
	if err != nil {
		return nil, fmt.Errorf("answer: %w", translateWindow(err))
	}
	scope.touched = true

	frozen, err := thaw(closed.Window.Payload)
	if err != nil {
		return nil, fmt.Errorf("answer: %w", err)
	}

	res := &walkResult{}
	if OptionKind(closed.Choice) == OptionContinue {
		res, err = m.resume(scope, frozen)
		if err != nil {
			return nil, fmt.Errorf("answer: %w", err)
		}
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("answer: %w", err)
	}

	return &AnswerOutput{
		Steps:      res.steps,
		Discovered: nilIfEmpty(res.discovered),
		Outcome:    res.outcome,
		Pending:    res.pending,
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// resume re-validates a thawed walk against the world as it actually loaded,
// then continues it.
//
// Re-validation is not paranoia about our own freeze. The window and the world
// are two stored aggregates, and persist writes them in an order chosen so that
// the survivable half-failure is "the world moved but the window did not" — so
// a window can legitimately outlive a save that failed after it. Checking that
// the remaining path still starts adjacent to where the member actually stands
// turns that case into a clean rejection instead of a walk that teleports over
// the cells the failed save swallowed.
func (m *Manager) resume(scope *writeScope, frozen *frozenResolution) (*walkResult, error) {
	remaining := frozen.Path[frozen.Index:]
	if len(remaining) == 0 {
		return &walkResult{}, nil
	}
	if err := validatePath(scope.enc, encounter.MemberID(frozen.Member), remaining); err != nil {
		return nil, fmt.Errorf("resuming: %w", err)
	}
	return m.runWalk(scope, frozen)
}

// Pending reports the open windows in a session: who owes an answer, what they
// may choose, and what they are looking at.
//
// A read verb, and available while the world is frozen — a client that cannot
// ask what it is waiting for has no way to render the wait.
//
// It reads session state only. The prompt was recorded when the window opened,
// because a perception delta is a difference at a moment: by the time anyone
// asks, it has been folded into what the observer knows and cannot be
// recomputed. Storing what the audience was shown is the only way to show them
// the same thing twice.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, or ErrInvalidSession.
func (m *Manager) Pending(ctx context.Context, in *PendingInput) (*PendingOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("pending: %w", ErrNilInput)
	}
	if in.Session == "" {
		return nil, fmt.Errorf("pending: %w", ErrNoSessionID)
	}

	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("pending: %w", err)
	}

	ledger, err := interrupt.LoadLedger(data.Windows)
	if err != nil {
		return nil, fmt.Errorf("pending: session %q: %w: %w", in.Session, ErrInvalidSession, err)
	}

	open, err := ledger.Open()
	if err != nil {
		return nil, fmt.Errorf("pending: %w: %w", ErrInvalidSession, err)
	}
	if len(open) == 0 {
		return &PendingOutput{}, nil
	}

	windows := make([]Pending, 0, len(open))
	for _, w := range open {
		frozen, err := thaw(w.Payload)
		if err != nil {
			return nil, fmt.Errorf("pending: window %d: %w", w.ID, err)
		}
		windows = append(windows, *projectPending(w, frozen.Prompt))
	}
	return &PendingOutput{Windows: windows}, nil
}

// parseWindow converts an SDK window identifier back to a ledger ID.
//
// The identifier is documented as opaque, so this is the only place that knows
// it is currently a decimal number — which is what makes it safe to change into
// something composite or signed later without touching a caller.
func parseWindow(id string) (interrupt.WindowID, error) {
	if id == "" {
		return 0, ErrNoWindowID
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%q: %w", id, ErrNoWindowID)
	}
	return interrupt.WindowID(n), nil
}

// translateWindow maps the custody module's sentinels onto this module's.
//
// The same reason the boundary test exists: a host that matched on
// interrupt.ErrNotOpen would be coupled to the module we most want to stay free
// to replace. Sentinels are not types in a signature, so nothing mechanical
// would catch that leak — which is exactly why it is done deliberately here.
func translateWindow(err error) error {
	switch {
	case errors.Is(err, interrupt.ErrNotOpen):
		return fmt.Errorf("%w: %w", ErrNoWindow, err)
	case errors.Is(err, interrupt.ErrNotAudience):
		return fmt.Errorf("%w: %w", ErrNotAudience, err)
	case errors.Is(err, interrupt.ErrNotOffered):
		return fmt.Errorf("%w: %w", ErrNotOffered, err)
	case errors.Is(err, interrupt.ErrNoOption):
		return fmt.Errorf("%w: %w", ErrNotOffered, err)
	case errors.Is(err, interrupt.ErrNoAudience):
		return fmt.Errorf("%w: %w", ErrNoMemberID, err)
	default:
		return err
	}
}
