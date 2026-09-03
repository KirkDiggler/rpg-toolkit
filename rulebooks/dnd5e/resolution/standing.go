// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// StandingInput is the cast to ask about.
type StandingInput struct {
	// Participants is who to ask about, as records. The same type Resolve and
	// Participation take because all three seams ask about the same people.
	// A member with no record is simply absent from this list; NO SHEET, NO
	// DEATH means there is no hit-point state to classify.
	Participants []Participant
}

// StandingOutput is the binary compatibility projection of Participation.
type StandingOutput struct {
	// Down names every member whose root participation answer reports Down, in
	// the same input order as ParticipationOutput.Members.
	Down []string
}

// Standing reports the Down subset of the richer [Participation] answer.
//
// Kept as a compatibility entry while encounter and its callers migrate from a
// binary standing question. It owns no second life-state rule: character death
// saves, stabilization, death, and monster defeat are all classified by the
// root rulebook in Participation, then this function projects one field.
//
// It asks only the root character's narrow ParticipationView, never its full
// display projection. A condition such as Shield that loads successfully but
// intentionally has no status-display catalog entry therefore still answers.
// That is distinct from the established lenient record policy: a truly
// unreadable character condition is audibly dropped because this entry never
// writes the sheet back, while an unreadable monster trait still refuses
// because monstertraits has no lenient loader.
func Standing(ctx context.Context, in *StandingInput) (*StandingOutput, error) {
	return standingOn(ctx, in, newSurface(events.NewEventBus()))
}

// standingOn is Standing with the surface handed in, so tests can hold the bus
// underneath. Participation owns attachment, truth installation, and teardown
// on that same surface; Standing only projects its answer.
func standingOn(ctx context.Context, in *StandingInput, surf *surface) (*StandingOutput, error) {
	if in == nil {
		// Delegate the refusal too, so the compatibility entry has no lifecycle
		// policy of its own.
		_, err := participationOn(ctx, nil, surf)
		return nil, err
	}

	participation, err := participationOn(ctx, &ParticipationInput{
		Participants: in.Participants,
	}, surf)
	if err != nil {
		return nil, err
	}

	down := make([]string, 0, len(participation.Members))
	for _, member := range participation.Members {
		if member.Participation.Down {
			down = append(down, member.Member)
		}
	}

	return &StandingOutput{Down: down}, nil
}
