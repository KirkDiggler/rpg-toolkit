// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// BelievedAimInput asks where an observer believes a subject stands, and
// whether that believed point is within a range on a clear path.
type BelievedAimInput struct {
	// Observer is the member aiming: the one whose testimony is read.
	Observer MemberID
	// Subject is the member aimed at.
	Subject MemberID
	// RangeFeet is the reach of whatever is being aimed, in feet. Zero
	// reaches only the observer's own cell. Negative is refused (ErrBadReach).
	RangeFeet int
}

// BelievedAimOutput is the encounter's answer to a [BelievedAimInput]. The
// policy for what to do with it — refuse an aim at a stale point, or attempt
// it and miss — is the asker's, never this module's.
type BelievedAimOutput struct {
	// Held is whether the observer holds sight testimony about the subject at
	// all. False means the observer has no belief about where the subject is;
	// every other field is then zero and means nothing more.
	Held bool
	// State is the held testimony's location state: [LocationKnown] carries a
	// believed point, [LocationUnknown] knows the subject without one.
	State LocationState
	// InRange is whether the believed point is within RangeFeet of the
	// observer on a clear line. False whenever State is not LocationKnown.
	InRange bool
	// Displaced is whether the subject has moved off the believed point: its
	// placement now is not where the observer believes it stands. False
	// whenever State is not LocationKnown.
	Displaced bool
}

// BelievedAim answers an aim from the observer's OWN testimony — the point it
// believes the subject stands on, which a creature out of sight leaves behind
// as a remembered location — never from the subject's live position. Range
// and the clear line are measured from the observer's placement to the
// believed point, on this encounter's grid and walls (rpg-project#539, "What
// an observer believes and reaches"). Whether the subject has moved off that
// point is the one fact read live, and it is reported, not acted on.
//
// THE CLEAR LINE IS THE CANVAS'S, WALLS ONLY. A runtime sight area (fog)
// never blocks an aim: fog obscures what an observer sees, which is already
// in its testimony, and does not stop a point it believes in from being
// reached.
//
// An observer aiming at itself believes its own placement: held, known, in
// range and not displaced.
//
// Errors: ErrNilInput; ErrNoMember for an empty id; ErrNotMember for an id
// naming nobody here; ErrBadReach for a negative range; ErrInvalidData for
// held sight testimony that does not decode; placement errors for an observer
// or subject with no cell.
func (e *Encounter) BelievedAim(in *BelievedAimInput) (BelievedAimOutput, error) {
	if in == nil {
		return BelievedAimOutput{}, fmt.Errorf("believed aim: %w", ErrNilInput)
	}
	observer, err := e.memberFor("believed aim: observer", in.Observer)
	if err != nil {
		return BelievedAimOutput{}, err
	}
	subject, err := e.memberFor("believed aim: subject", in.Subject)
	if err != nil {
		return BelievedAimOutput{}, err
	}
	if in.RangeFeet < 0 {
		return BelievedAimOutput{}, fmt.Errorf("believed aim: range %d: %w", in.RangeFeet, ErrBadReach)
	}
	from, err := e.cellOf(observer)
	if err != nil {
		return BelievedAimOutput{}, fmt.Errorf("believed aim: observer %q: %w", in.Observer, err)
	}
	if in.Observer == in.Subject {
		return BelievedAimOutput{Held: true, State: LocationKnown, InRange: true}, nil
	}

	holdings, err := e.memberIntel(in.Observer)
	if err != nil {
		return BelievedAimOutput{}, fmt.Errorf("believed aim: held by %q: %w", in.Observer, err)
	}
	for _, h := range holdings {
		if h.Channel != perception.Sight || h.Subject != in.Subject {
			continue
		}
		testimony, ok := DecodeSightTestimony(h.Payload)
		if !ok {
			return BelievedAimOutput{}, fmt.Errorf("believed aim: %q about %q: %w", in.Observer, in.Subject, ErrInvalidData)
		}
		out := BelievedAimOutput{Held: true, State: testimony.State}
		if testimony.State != LocationKnown {
			return out, nil
		}
		aim := testimony.Position
		out.InRange = e.Distance(from, aim) <= float64(CellsFromFeet(in.RangeFeet)) &&
			!e.canvas.IsLineOfSightBlocked(from, aim)
		actual, err := e.cellOf(subject)
		if err != nil {
			return BelievedAimOutput{}, fmt.Errorf("believed aim: subject %q: %w", in.Subject, err)
		}
		out.Displaced = actual != aim
		return out, nil
	}
	return BelievedAimOutput{}, nil
}
