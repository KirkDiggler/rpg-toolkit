// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

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

	// Roller reconstitutes runtime dice dependencies for both Character
	// conditions and Monster traits while attaching. REQUIRED, and refused when
	// absent rather than defaulted. Preflight only carries the value into the
	// generic attach APIs; it does not choose which effects bind or switch on
	// their refs. Silently using a real roller here would let a preflight consume
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
// So the whole cast goes through ONE [attachAll], which collects refusals
// instead of stopping at the first — the Refusals sink on [attachAllInput].
// That is not a second attach mechanism: it is the same one, an argument apart,
// the way DropUnreadable already works.
//
// # What the one-call shape is actually worth
//
// Not what an earlier version of this doc claimed. It said a cast of one could
// not observe a cast of many, and recorded that as a cost being knowingly paid.
// That cost was not real: this entry never installs game context — it is
// deliberately not a fold entry — so no cast is installed during its attach at
// ALL, and nothing a participant could read during Apply differs between one
// participant and twenty. Review caught the claim by restoring the previous
// implementation under the new tests and watching every one of them pass.
//
// What the single call is worth is smaller and true: ordering is decided in one
// place. The previous version sorted the participants itself and then called
// the attach once each — and the attach sorts too, so the R4 ordering rule
// lived in two copies that had to agree. They did agree, which is why nothing
// observable changed; they were still two. TestOrderingIsDecidedInOnePlace
// holds the absence.
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

	// ONE CAST, ONE PASS. Every participant is attached onto the same surface,
	// in the same sorted order an interaction uses, and the refusals are
	// collected rather than aborting on the first — see attachAllInput.Refusals.
	//
	// This is the whole cast on purpose. An earlier version attached a cast of
	// ONE at a time, which answered the same for every case anybody had, and
	// carried a doc admitting what it could not see: a participant that would
	// only fail in company. Nothing exercises that today — attaching is
	// per-sheet and the bus is fresh — but "we checked them one at a time" is
	// the kind of shortcut that is invisible until it is not, and the entry
	// exists to predict an interaction rather than to approximate one.
	refusals := make([]ParticipantRefusal, 0)
	if _, err := attachAll(ctx, surf, &attachAllInput{
		Participants: in.Participants,
		Roller:       in.Roller,
		Refusals:     &refusals,
	}); err != nil {
		// Unreachable while Refusals is set — the attach reports per
		// participant and returns no error of its own — so this refuses rather
		// than dropping an error nobody expected, the way the projection's
		// unreachable cast lookup does.
		return nil, errors.Join(err, surf.teardown(ctx))
	}

	if err := surf.teardown(ctx); err != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", err)
	}

	return &PreflightOutput{Unreadable: refusals}, nil
}
