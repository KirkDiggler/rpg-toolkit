// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// PreflightInput is the cast an interaction would be given.
type PreflightInput struct {
	// Participants is the whole cast, exactly as [Resolve] would receive it.
	// Asking about a subset would answer a question nobody has: resolution
	// attaches everyone, so a cast that attaches in halves may still not
	// attach whole.
	Participants []Participant

	// Roller is the dice source a monster's traits may reach for while
	// attaching. REQUIRED, and refused when absent rather than defaulted:
	// silently rolling on a real roller here would let a preflight consume
	// randomness the interaction it is predicting has not spent yet.
	Roller dice.Roller
}

// PreflightOutput is what would go wrong.
type PreflightOutput struct {
	// Unreadable names every participant this cast could not attach, in cast
	// order, each with the reason.
	//
	// Empty means [Resolve] would attach all of them. That is the ordinary
	// answer and the safe zero value: a caller reading an empty list proceeds,
	// which is correct, because the only way to get an empty list is to have
	// attached every participant successfully.
	Unreadable []ParticipantRefusal
}

// ParticipantRefusal is one participant resolution would not accept.
type ParticipantRefusal struct {
	// Member is the participant's ID.
	Member string

	// Reason is why it could not be attached, as the loader reported it.
	Reason error
}

// Preflight reports which participants an interaction would refuse, without
// running one.
//
// # Why the answer is a list rather than an error
//
// [attachAll] stops at the first participant it cannot attach, which is right
// for an interaction: one unreadable sheet means the interaction does not
// happen, and the rest is not worth computing. It is wrong for the caller this
// entry serves. An offer menu shows a row per candidate, and each row carries
// its own verdict — so a caller told only "somebody is unreadable" would have
// to grey out the whole menu or guess which row to blame.
//
// So this attaches one participant at a time and keeps going, collecting every
// refusal. That is not a second attach mechanism: each participant goes through
// the same [attachAll] with a cast of one, so what refuses here refuses there,
// for the same reason and with the same message.
//
// # What it costs, said plainly
//
// A cast of one cannot observe a cast of many. If some future attach were to
// fail only in company — a condition that refuses when another is already on
// the bus — this would miss it, and the interaction would refuse where the
// preflight said it would not. No such attach exists today: attaching is
// per-sheet and the bus is a fresh one. It is recorded because "we checked
// them one at a time" is exactly the kind of shortcut that is invisible until
// the day it is not.
//
// # Nothing is left alive
//
// One surface for the whole pass, torn down once at the end, whatever
// happened. The seam this replaces built an ephemeral bus and dropped it
// without revoking anything; a subscription that outlives its purpose is the
// leak this package exists to prevent, and a preflight is a very short
// interaction.
func Preflight(ctx context.Context, in *PreflightInput) (*PreflightOutput, error) {
	return preflightOn(ctx, in, newSurface(events.NewEventBus()))
}

// preflightOn is [Preflight] with the surface handed in, so a test can hold the
// bus underneath and check what is left on it afterwards.
func preflightOn(ctx context.Context, in *PreflightInput, surf *surface) (*PreflightOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Roller == nil {
		return nil, ErrNoRoller
	}

	for _, p := range in.Participants {
		if err := p.validate(); err != nil {
			return nil, err
		}
	}

	// Cast order, so two preflights over identical data produce identical
	// reports — the same R4 argument attachAll makes, and the reason a caller
	// can compare one report against the next.
	ordered := append([]Participant(nil), in.Participants...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID() < ordered[j].ID() })

	refusals := make([]ParticipantRefusal, 0)
	for _, p := range ordered {
		_, err := attachAll(ctx, surf, &attachAllInput{
			Participants: []Participant{p},
			Roller:       in.Roller,
		})
		if err != nil {
			refusals = append(refusals, ParticipantRefusal{Member: p.ID(), Reason: err})
		}
	}

	if err := surf.teardown(ctx); err != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", err)
	}

	return &PreflightOutput{Unreadable: refusals}, nil
}
