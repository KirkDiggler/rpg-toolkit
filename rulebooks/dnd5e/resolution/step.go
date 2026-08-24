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
// Today the set is [Gather], [Request] and [Done]. ADR-0038 also names Pose,
// which lands with the caller that forces it — the walk machine — rather than
// now, when it would be an enumeration against a hypothetical.
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
// No suspension yet. The requested machine runs to Done inside this one's
// step loop, which is enough for every consumer today, and the boundary is
// self-describing data — a machine, and a continuation taking its outcome — so
// the later case where the answer comes from outside the process (Pose) is a
// new step rather than a redesign of this one.
type Request struct {
	name    string
	machine Machine
	next    func(ctx context.Context, out Outcome) (Step, error)
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
func drive(ctx context.Context, bus events.EventBus, machine Machine, cast *Participants) (Outcome, error) {
	first, err := start(ctx, machine, cast)
	if err != nil {
		return nil, err
	}
	return driveStep(ctx, bus, first, cast)
}

// driveStep continues from an already preflighted first step.
func driveStep(ctx context.Context, bus events.EventBus, step Step, cast *Participants) (Outcome, error) {
	var err error
	for {
		switch s := step.(type) {
		case Done:
			return s.Outcome, nil

		case Request:
			if s.machine == nil || s.next == nil {
				// A Request that did not come from this package's
				// constructors. Refusing beats running nothing and feeding the
				// requester a nil outcome, which would look exactly like an
				// interaction that produced nothing to say.
				return nil, fmt.Errorf("%w: Request built outside this package", ErrBadStep)
			}

			// The same bus and the same cast: a requested interaction happens
			// inside this one, not beside it.
			out, runErr := drive(ctx, bus, s.machine, cast)
			if runErr != nil {
				return nil, fmt.Errorf("requested %s: %w", s.name, runErr)
			}

			step, err = s.next(ctx, out)
			if err != nil {
				return nil, err
			}

		case Gather:
			if s.run == nil {
				// A Gather that did not come from this package's constructors.
				// Refusing is better than folding nothing and calling it a
				// result, which would look exactly like a chain no one
				// subscribed to.
				return nil, fmt.Errorf("%w: Gather built outside this package", ErrBadStep)
			}

			step, err = s.run(ctx, bus)
			if err != nil {
				return nil, err
			}

		default:
			// Names the concrete type, because the likeliest way here is a
			// machine returning *Done or *Gather — pointer forms satisfy Step
			// (value receiver on isStep) but the vocabulary is the value
			// forms, one spelling per case. %T turns that mistake from a
			// riddle into a one-character diff.
			return nil, fmt.Errorf("%w: %T", ErrBadStep, step)
		}
	}
}
