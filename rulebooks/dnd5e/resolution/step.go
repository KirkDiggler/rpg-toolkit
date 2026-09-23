// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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
// The set is [Gather], [Request], [Pose] and [Done]. Pose was named by
// ADR-0038 and deliberately unbuilt until the caller that forces it arrived;
// the strike machine is that caller (rpg-project#398 R1), and the
// hypothetical is over.
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
// which is enough for every consumer that leaves [Request.onPose] unset —
// exactly today's behaviour, unchanged. [Request.onPose] is the opt-in for a
// requester whose sub-machine CAN suspend: a contest requesting a saving
// throw (Resistance's own die is offered on the save, not the contest) and a
// cast requesting a contest per target both need it, because [drive]'s
// default refusal ("a requester cannot be suspended") is correct for every
// other requester and wrong for exactly these two. The callback turns the
// sub-machine's [Pose] into this machine's OWN pose — freezing whatever this
// machine needs to resume the request later — rather than the pose crossing
// the requester unexamined, which is what would strand it: nothing about a
// requester survives a suspension except what onPose chooses to freeze.
type Request struct {
	name    string
	machine Machine
	next    func(ctx context.Context, out Outcome) (Step, error)

	// onPose is nil for every requester that cannot compose with a pose —
	// [drive] keeps refusing those exactly as before. Set, it is called
	// instead of that refusal, with the sub-machine's raw pose, and its
	// return value becomes the step this request continues with (typically
	// a new [Pose] of the requester's own).
	onPose func(ctx context.Context, pose Pose) (Step, error)
}

func (Request) isStep() {}

// Name identifies the interaction being requested, for logs and for tests that
// want to assert what a machine asked for without reaching into it.
func (r Request) Name() string { return r.name }

// Ask is what a posed machine wants answered: who is being asked, what they
// hold that could join the roll, and the numbers they need to decide with.
//
// It is DATA and it is the whole question. A caller renders it, stores it,
// restarts the process, and answers it later; nothing about the machine that
// posed it survives except [Pose.Frozen].
type Ask struct {
	// Audience is the member being asked.
	Audience string

	// Offer is what they hold — ref, display name and die notation, exactly as
	// the effect that offered it named itself.
	Offer dnd5eEvents.Offer

	// Options are the answers this pose accepts, as opaque strings the caller
	// echoes back. The machine names them so a caller cannot answer a question
	// that was not asked.
	Options []string

	// Choices carries provider-authored option labels for reaction windows.
	Choices []Choice

	// Roll is the d20 as rolled and Total the number the offer would join.
	// TARGET AC IS DELIBERATELY ABSENT: a player who could see it would be
	// deciding "does this close the gap" rather than "is this worth spending",
	// which is a different question and a different game.
	Roll  int
	Total int

	// Calculation is the settled arithmetic the offered die would join — the
	// same numbers Roll and Total summarise, with the faces and the keep
	// record behind them. Carried for the beat the seam writes when it poses
	// the question: the paused window is where an untrained roll is FIRST
	// seen, and a seam with only two scalars can only show one face
	// (rpg-project#462 R5).
	//
	// It is the pre-offer calculation. Whatever the answer adds lands on the
	// resumed outcome's own, not here.
	Calculation *dnd5eEvents.RollCalculation
}

// Pose is a machine stopping mid-run to be answered from outside the process.
//
// It is the suspension every other step's doc has been pointing at. [Request]
// runs its sub-machine to Done inline because the answer is available in the
// same call; this one's answer is a person, so the machine's state leaves as
// bytes and comes back as a new machine.
//
// # Frozen is opaque on purpose
//
// The bytes are authored by the machine and never read by this package.
// Resolution drives steps over data and holds no rulebook, so a typed
// frozen-strike field here would put the dnd5e attack inside the driver. What
// a caller does with them is store them and hand them back.
//
// # One pose per run
//
// A machine that poses twice in one call is not designed here and is not
// refused here: the driver returns the FIRST pose and stops, and the second
// simply never happens because the run is over.
type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Pose struct {
	// Movement carries completed opportunity attacks at an interrupted step.
	Movement *MovementOutcome `json:"movement,omitempty"`
	// Sequence carries already completed swings at a sequence pause.
	Sequence *SequenceOutcome `json:"sequence,omitempty"`
	// BeforeRoll reports an attack reaction posed before rolling the attack.
	BeforeRoll bool `json:"before_roll,omitempty"`
	// SettledStrike is populated only for the initial post-hit reaction pose.
	SettledStrike *StrikeOutcome `json:"settled_strike,omitempty"`

	// Ask is the question.
	Ask Ask

	// Frozen is the machine's own state, serialized by the machine.
	Frozen []byte
}

func (Pose) isStep() {}

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
// no pose, because a requested machine that suspended would strand the machine
// that requested it: the requester's continuation is a Go closure on this
// stack, and nothing serializes it. driveStep refuses that case by name rather
// than dropping the pose.
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
// It returns EITHER an outcome or a pose, never both: a posed machine has not
// finished, and a zero-valued outcome beside a pose would read as an
// interaction that produced nothing rather than one that is waiting.
func driveStep(
	ctx context.Context, bus events.EventBus, step Step, cast *Participants,
) (Outcome, *Pose, error) {
	var err error
	for {
		switch s := step.(type) {
		case Done:
			return s.Outcome, nil, nil

		case Pose:
			// Returned rather than looped on. The answer is not in this
			// process, so there is nothing to continue with — the caller
			// stores Frozen, asks somebody, and starts a resumed machine.
			posed := s
			return nil, &posed, nil

		case Request:
			if s.machine == nil || s.next == nil {
				// A Request that did not come from this package's
				// constructors. Refusing beats running nothing and feeding the
				// requester a nil outcome, which would look exactly like an
				// interaction that produced nothing to say.
				return nil, nil, fmt.Errorf("%w: Request built outside this package", ErrBadStep)
			}

			if s.onPose == nil {
				// The same bus and the same cast: a requested interaction
				// happens inside this one, not beside it. Byte-identical to
				// every build before onPose existed — a requester that never
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

			// s.onPose is set: this requester can compose with a sub-machine
			// that suspends, so the refusal [drive] would apply does not run
			// here. Preflight and drive the sub-machine inline instead,
			// exactly as [drive] does, but hand a pose to onPose rather than
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
				step, err = s.onPose(ctx, *posed)
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
