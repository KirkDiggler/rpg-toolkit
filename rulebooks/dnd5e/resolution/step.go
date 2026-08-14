// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Machine is a rules package's contribution to an interaction: a sequence of
// steps over data.
//
// A machine is handed the cast that resolution loaded and attached, because
// rules are read off sheets. It is never handed the bus (R6): it says what it
// wants folded, and resolution folds it.
type Machine interface {
	// Start yields the first step of the interaction.
	Start(ctx context.Context, cast *Participants) (Step, error)
}

// Step is what a machine yields. The set is sealed — implementations live in
// this package and nowhere else — because every yield point is also a legal
// suspension point, and a case nobody drives is a case nobody can resume.
//
// Today the set is [Gather] and [Done]. ADR-0038 also names Pose and Request;
// each lands with the caller that forces it rather than now, when it would be
// an enumeration against hypotheticals.
type Step interface {
	isStep()
}

// Gather is "fold this chain and hand me the result".
//
// It is opaque on purpose. A machine cannot construct one directly — it calls a
// constructor in this package naming the chain it wants — which is what keeps
// the fold, and therefore the bus, on resolution's side of the seam while the
// machine still gets a typed result back.
type Gather struct {
	name string
	run  func(ctx context.Context, bus events.EventBus) (Step, error)
}

func (Gather) isStep() {}

// Name identifies the chain being folded, for logs and for tests that want to
// assert what a machine asked for without reaching into it.
func (g Gather) Name() string { return g.name }

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

// drive runs a machine to completion on the surface's bus.
//
// The loop is the whole driver: a machine yields, resolution acts, the machine
// yields again. Nothing accumulates on the Go stack between yields, which is
// what makes a suspension expressible later — the machine's own state is the
// only state there is.
func drive(ctx context.Context, bus events.EventBus, machine Machine, cast *Participants) (Outcome, error) {
	step, err := machine.Start(ctx, cast)
	if err != nil {
		return nil, err
	}

	for {
		switch s := step.(type) {
		case Done:
			return s.Outcome, nil

		case Gather:
			if s.run == nil {
				// A Gather that did not come from this package's constructors.
				// Refusing is better than folding nothing and calling it a
				// result, which would look exactly like a chain no one
				// subscribed to.
				return nil, ErrBadStep
			}

			step, err = s.run(ctx, bus)
			if err != nil {
				return nil, err
			}

		default:
			return nil, ErrBadStep
		}
	}
}
