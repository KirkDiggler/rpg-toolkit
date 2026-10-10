// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Machine is a rules package's contribution to an interaction: a sequence of
// steps over data.
//
// A machine is handed the cast that resolution loaded and attached, because
// rules are read off sheets. It is never handed the bus (R6): it says what it
// wants folded, and resolution folds it.
type Machine interface {
	// Start is pure preflight: it may validate and read attached sheets, but it
	// must not roll, spend, publish, or mutate. It yields the first step.
	Start(ctx context.Context, cast *Participants) (Step, error)
}

// Step is what a machine yields. The set is sealed — implementations live in
// this package and nowhere else — because every yield point is also a legal
// suspension point, and a case nobody drives is a case nobody can resume.
//
// The set is [Gather], [Request], [Pause] and [Done]. The suspension was
// named by ADR-0038 and deliberately unbuilt until the caller that forces it
// arrived; the strike machine was that caller (rpg-project#398 R1), and
// [Pause] is now the one envelope every suspension travels in.
type Step interface {
	isStep()
}

// Gather is "do this on the bus and hand me what happened" — folding a chain,
// first and mainly, and imposing a consequence a contest just decided on.
//
// It is opaque on purpose. A machine cannot construct one directly — it calls a
// constructor in this package naming what it wants — which is what keeps the
// bus on resolution's side of the seam while the machine still gets a typed
// result back. [Gather.Name] is what it asked for, so a fold and an imposition
// are told apart by reading rather than by type.
type Gather struct {
	name string
	run  func(ctx context.Context, bus events.EventBus) (Step, error)
}

func (Gather) isStep() {}

// Name identifies the chain being folded, for logs and for tests that want to
// assert what a machine asked for without reaching into it.
func (g Gather) Name() string { return g.name }

// Request is "run this interaction, then continue with what it produced".
//
// It is how one machine composes another without either knowing the other
// exists: a contest asks for a saving throw and resumes with its outcome,
// while the save machine remains a machine anyone can drive on its own. Like
// [Gather] it is opaque — a machine calls a constructor here naming the
// follow-up it wants, and the driver is what actually runs it, on the same bus
// and over the same cast. That sameness is load-bearing: the sub-machine folds
// its chains on the interaction's own bus, so an effect attached for this
// interaction contributes to the requested save exactly as it would to a
// direct one.
//
// By default the requested machine runs to Done inside this one's step loop,
// which is enough for every consumer that leaves [Request.onPause] unset —
// exactly today's behaviour, unchanged. [Request.onPause] is the opt-in for a
// requester whose sub-machine CAN suspend: a contest requesting a saving
// throw (Resistance's own die is offered on the save, not the contest), a
// cast requesting a contest per target, a sequence or a walk requesting a
// strike, all need it, because [drive]'s default refusal ("a requester cannot
// be suspended") is correct for every other requester and wrong for these.
// The callback turns the sub-machine's [Pause] into this machine's OWN pause
// — freezing whatever this machine needs to resume the request later — rather
// than the pause crossing the requester unexamined, which is what would
// strand it: nothing about a requester survives a suspension except what
// onPause chooses to freeze.
type Request struct {
	name    string
	machine Machine
	next    func(ctx context.Context, out Outcome) (Step, error)

	// onPause is nil for every requester that cannot compose with a pause —
	// [drive] keeps refusing those exactly as before. Set, it is called
	// instead of that refusal, with the sub-machine's raw pause, and its
	// return value becomes the step this request continues with (typically
	// a new [Pause] of the requester's own).
	onPause func(ctx context.Context, pause Pause) (Step, error)
}

func (Request) isStep() {}

// Name identifies the interaction being requested, for logs and for tests that
// want to assert what a machine asked for without reaching into it.
func (r Request) Name() string { return r.name }

// Done ends a machine and carries what the interaction produced.
type Done struct {
	Outcome Outcome
}

func (Done) isStep() {}

// Outcome is what an interaction produced. Sealed like [Step], and for the same
// reason: a caller switching on outcomes should get a compiler error when a new
// kind of interaction arrives, not a silent default branch.
type Outcome interface {
	isOutcome()
}

func start(ctx context.Context, machine Machine, cast *Participants) (Step, error) {
	return machine.Start(ctx, cast)
}

// drive runs a machine to completion on the surface's bus.
//
// It is the SUB-MACHINE entry — [Request] is its only caller — and it returns
// no pause, because a requested machine that suspended would strand the machine
// that requested it: the requester's continuation is a Go closure on this
// stack, and nothing serializes it. driveStep refuses that case by name rather
// than dropping the pause.
func drive(ctx context.Context, bus events.EventBus, machine Machine, cast *Participants) (Outcome, error) {
	first, err := start(ctx, machine, cast)
	if err != nil {
		return nil, err
	}
	outcome, posed, err := driveStep(ctx, bus, first, cast)
	if err != nil {
		return nil, err
	}
	if posed != nil {
		return nil, fmt.Errorf("%w: a requested machine posed, and a requester cannot be suspended", ErrBadStep)
	}
	return outcome, nil
}

// driveStep continues from an already preflighted first step.
//
// It returns EITHER an outcome or a pause: a paused machine has not finished,
// and what it settled before stopping travels on the pause itself, for the
// entry that reports it.
func driveStep(
	ctx context.Context, bus events.EventBus, step Step, cast *Participants,
) (Outcome, *Pause, error) {
	var err error
	for {
		switch s := step.(type) {
		case Done:
			return s.Outcome, nil, nil

		case Pause:
			// Returned rather than looped on. The answer is not in this
			// process, so there is nothing to continue with — the caller
			// stores the pause, asks somebody, and calls [Resume]. What the
			// machine settled before it stopped rides the pause to the
			// driver, which reports it as the run's outcome.
			paused := s
			return nil, &paused, nil

		case Request:
			if s.machine == nil || s.next == nil {
				// A Request that did not come from this package's
				// constructors. Refusing beats running nothing and feeding the
				// requester a nil outcome, which would look exactly like an
				// interaction that produced nothing to say.
				return nil, nil, fmt.Errorf("%w: Request built outside this package", ErrBadStep)
			}

			if s.onPause == nil {
				// The same bus and the same cast: a requested interaction
				// happens inside this one, not beside it. Byte-identical to
				// every build before onPause existed — a requester that never
				// opted in cannot tell the capability was added.
				out, runErr := drive(ctx, bus, s.machine, cast)
				if runErr != nil {
					return nil, nil, fmt.Errorf("requested %s: %w", s.name, runErr)
				}

				step, err = s.next(ctx, out)
				if err != nil {
					return nil, nil, err
				}
				continue
			}

			// s.onPause is set: this requester can compose with a sub-machine
			// that suspends, so the refusal [drive] would apply does not run
			// here. Preflight and drive the sub-machine inline instead,
			// exactly as [drive] does, but hand a pause to onPause rather than
			// erroring on it.
			sub, startErr := start(ctx, s.machine, cast)
			if startErr != nil {
				return nil, nil, fmt.Errorf("requested %s: %w", s.name, startErr)
			}
			out, posed, runErr := driveStep(ctx, bus, sub, cast)
			if runErr != nil {
				return nil, nil, fmt.Errorf("requested %s: %w", s.name, runErr)
			}
			if posed != nil {
				step, err = s.onPause(ctx, *posed)
				if err != nil {
					return nil, nil, err
				}
				continue
			}

			step, err = s.next(ctx, out)
			if err != nil {
				return nil, nil, err
			}

		case Gather:
			if s.run == nil {
				// A Gather that did not come from this package's constructors.
				// Refusing is better than folding nothing and calling it a
				// result, which would look exactly like a chain no one
				// subscribed to.
				return nil, nil, fmt.Errorf("%w: Gather built outside this package", ErrBadStep)
			}

			step, err = s.run(ctx, bus)
			if err != nil {
				return nil, nil, err
			}

		default:
			// Names the concrete type, because the likeliest way here is a
			// machine returning *Done or *Gather — pointer forms satisfy Step
			// (value receiver on isStep) but the vocabulary is the value
			// forms, one spelling per case. %T turns that mistake from a
			// riddle into a one-character diff.
			return nil, nil, fmt.Errorf("%w: %T", ErrBadStep, step)
		}
	}
}
