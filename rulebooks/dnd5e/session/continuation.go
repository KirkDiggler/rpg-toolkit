// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrContinuationSpent means a continuation was continued twice, or after the
// runner that held it reported it would not run it. A continuation performs
// the rest of one verb exactly once; a second call has nothing left to do and
// no guard to give back.
var ErrContinuationSpent = errors.New("session continuation already run or abandoned")

// ContinuationRunner runs the part of a verb that follows its answer
// (rpg-project#879 R2, "the verb answers before the world runs"): for a Move,
// the step that pays a world round, the raise that round wakes, the remaining
// steps, the settlement, the save and the delivery.
//
// The SDK hands the runner a continuation while the session's guard is still
// held, and the continuation gives the guard back when it returns. The next
// verb on the session therefore waits for the world to finish, so the world
// still runs as one deterministic pass (the-world-runs R2): nothing a creature
// does moves relative to the player's steps, only the answer moves earlier.
//
// It is host-injected, beside [SessionLocker], and the SDK creates no
// goroutine of its own to run it: the host decides where the rest of the verb
// runs (its per-session write queue, R3's write tier) and with what context.
// A host that supplies none gets today's behaviour: the whole verb runs
// before the answer, and no answer continues.
type ContinuationRunner interface {
	// Run takes ownership of the continuation. It must arrange for
	// in.Continue to be called exactly once, and may do so before or after
	// returning. An error means Run did not and will not call Continue: the
	// verb then gives the guard back, saves nothing, and answers with that
	// error, exactly as a refusal before the hand-off would. A host that
	// queues the continuation and fails later still calls Continue; the
	// continuation's own report is where the rest's failure is told.
	Run(ctx context.Context, in *RunContinuationInput) error
}

// RunContinuationInput is one verb's continuation, handed to the host's
// [ContinuationRunner].
type RunContinuationInput struct {
	// Session is the session whose guard the continuation holds: the key a
	// host's per-session queue is ordered by.
	Session string

	// Verb is the verb being continued.
	Verb Verb

	// Continue performs the rest of the verb and gives the session's guard
	// back, whatever it returns. It must be called exactly once, with a
	// context the client's cancel cannot end (the save must land: use
	// [context.WithoutCancel] or the host's own lifetime). Every host
	// collaborator the rest of the verb calls is handed this context, not
	// the request's. A second call answers [ErrContinuationSpent].
	//
	// It returns what the answer could not carry: how the rest of the verb
	// ended, what it saved and what it delivered. An error is the rest's
	// refusal or failure, with what had already landed named in it (S6).
	Continue func(ctx context.Context) (*ContinuationReport, error)
}

// ContinuationReport is what the rest of a verb did after its answer.
type ContinuationReport struct {
	// Saved names what the rest of the verb persisted: the whole verb's
	// writes, since nothing is saved before the hand-off.
	Saved SaveReport

	// Delivery names what reached the event stream: the whole verb's acts,
	// the answered steps' included, since delivery waits for the save (S9).
	Delivery DeliveryReport

	// Move is the rest of a continued Move: the steps taken after
	// [MoveOutput.Through] and how the walk ended. Nil for any other verb.
	Move *MoveRest
}

// MoveRest is the part of a walk taken after its answer: the same facts a
// [MoveOutput] carries about the steps it answered with, for the steps it
// did not.
type MoveRest struct {
	// Status is how the whole walk ended: [MovementCompleted],
	// [MovementPaused] or [MovementStopped]. Never [MovementContinuing].
	Status MovementStatus `json:"status"`

	// StopReason is the observation-safe explanation for an obstruction.
	StopReason string `json:"stop_reason,omitempty"`

	// JoinedCombat reports entry into a visible fight before a step.
	JoinedCombat bool `json:"joined_combat,omitempty"`

	// Steps are the cells entered after the answer's, in order.
	Steps []Step `json:"steps,omitempty"`

	// Discovered is what changed in each observer's perception across these
	// steps, keyed by observer.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Outcome, Formed and Barred are [MoveOutput]'s, for a walk that ended
	// in the rest.
	Outcome *Outcome `json:"outcome,omitempty"`
	Formed  *Formed  `json:"formed,omitempty"`
	Barred  *Barred  `json:"barred,omitempty"`
}

// heldGuard is a verb's session guard that may change hands. The verb defers
// verbRelease on every path, as every verb does; once the guard is handed to
// a continuation that deferred release does nothing and the continuation
// gives the guard back instead. The guard is given back once, whichever gets
// there, so a runner that panics after continuing cannot release it twice.
type heldGuard struct {
	release   func()
	handedOff bool
}

func newHeldGuard(release func()) *heldGuard {
	return &heldGuard{release: sync.OnceFunc(release)}
}

// verbRelease is the verb's own deferred release: the guard, unless it was
// handed off. Read only on the verb's goroutine, after the hand-off.
func (g *heldGuard) verbRelease() {
	if !g.handedOff {
		g.release()
	}
}

// handOff gives the rest of a verb to the host's runner. rest performs it
// under ctx. On a nil error the guard belongs to the continuation, which
// releases it when it ends. On an error the continuation never ran and never
// will, and the guard is still the verb's to release.
func (m *Manager) handOff(
	ctx context.Context, session string, verb Verb, vctx *verbContext, guard *heldGuard,
	rest func(ctx context.Context) (*ContinuationReport, error),
) error {
	var claimed atomic.Bool
	in := &RunContinuationInput{
		Session: session,
		Verb:    verb,
		Continue: func(ctx context.Context) (*ContinuationReport, error) {
			if !claimed.CompareAndSwap(false, true) {
				return nil, ErrContinuationSpent
			}
			defer guard.release()
			if ctx == nil {
				return nil, ErrNilInput
			}
			vctx.switchTo(ctx)
			return rest(ctx)
		},
	}
	if err := m.continuations.Run(ctx, in); err != nil {
		if claimed.CompareAndSwap(false, true) {
			// Never continued and never will be: the guard is still the
			// verb's, and nothing the rest would have saved was saved.
			return errRunnerRefused{err: err}
		}
		// The runner continued and then reported an error of its own: the
		// guard is the continuation's, and the answer still stands.
	}
	guard.handedOff = true
	return nil
}

// errRunnerRefused marks a runner that declined the continuation, so the verb
// knows the guard is still its own to release.
type errRunnerRefused struct{ err error }

func (e errRunnerRefused) Error() string { return "continuation runner: " + e.err.Error() }
func (e errRunnerRefused) Unwrap() error { return e.err }

// verbContext is the context a continuing verb's collaborators are handed.
//
// The host seams an encounter is loaded with keep the context they were
// built under, and a continued verb outlives its request: the rest runs
// after the answer, when the request's context may already be canceled. So
// a verb that may continue is opened under a verbContext whose parent is the
// request's until the hand-off, and the continuation's own context from then
// on. The switch happens while nothing runs (between the answer and
// Continue), so no call ever sees its context change under it.
type verbContext struct {
	parent atomic.Pointer[context.Context]
}

func newVerbContext(ctx context.Context) *verbContext {
	v := &verbContext{}
	v.parent.Store(&ctx)
	return v
}

func (v *verbContext) switchTo(ctx context.Context) { v.parent.Store(&ctx) }

func (v *verbContext) current() context.Context { return *v.parent.Load() }

// Deadline implements [context.Context] over the current parent.
func (v *verbContext) Deadline() (time.Time, bool) { return v.current().Deadline() }

// Done implements [context.Context] over the current parent.
func (v *verbContext) Done() <-chan struct{} { return v.current().Done() }

// Err implements [context.Context] over the current parent.
func (v *verbContext) Err() error { return v.current().Err() }

// Value implements [context.Context] over the current parent.
func (v *verbContext) Value(key any) any { return v.current().Value(key) }
