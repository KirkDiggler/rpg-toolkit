// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// answerCastOffer finishes a cast that stopped on one target's saving
// throw — [Manager.answerCheckOffer]'s shape, for [resolution.NewCastResumed]
// instead of a resumed check.
//
// # It resumes the SAME cast rather than rolling again
//
// The window froze resolution's own state; this hands it straight back
// through [resolution.NewCastResumed] with the answer. Nothing is re-rolled
// and nothing re-folded, and NOTHING IS CHARGED AGAIN: [Manager.Cast] pays
// before the door yields its first step, whichever step that turns out to
// be, so this call carries no [resolution.Cost] at all.
//
// # Everyone is in the cast, again
//
// R3, restated for a resume: a resumed cast still runs on the whole roster,
// exactly as the original attempt did, because a condition it publishes is
// applied by the owner's keeper and that keeper is only attached for a
// participant.
//
// # A resumed cast can pose AGAIN
//
// Bane can ask several targets, one after another — resuming past the
// first still leaves the rest to roll their own saves, and one of THEM may
// hold an offer too. When that happens this reaches for the exact function
// the fresh cast door does, [Manager.poseCastWindow], rather than a second
// copy of it.
func (m *Manager) answerCastOffer(
	ctx context.Context, scope *writeScope, window interrupt.Window, choice ReactChoice,
) (*ReactOutput, error) {
	payload, err := thawCastOfferPayload(window.Payload, string(window.Audience))
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	answer := resolution.OfferKeep
	if choice == ReactStrike {
		answer = resolution.OfferSpend
	}

	resumed, err := resolution.NewCastResumed(&resolution.CastResumeInput{
		Frozen: payload.Frozen,
		Answer: answer,
		// The offered die is rolled with the HOST'S dice, through the same
		// seam every other roll in this package takes.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("react: %w", translateResolution(err))
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("react: %w", translate(err))
	}
	// readied is nil: nothing is paid on a resume, so there is no readied
	// turn-economy ledger to thread through — every roster member, the
	// original caster included, is loaded fresh off its current record.
	participants, failures := m.compileResolutionCast(ctx, scope.data, roster, nil)
	if len(failures) > 0 {
		return nil, fmt.Errorf("react: participant %q: %w: %v",
			failures[0].member, ErrBadCharacter, failures[0].err)
	}

	world := scope.enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:         world,
		Participants:  participants,
		Initiative:    m.initiative,
		Standing:      scope.standing,
		Sight:         &sightSeam{members: worldMembers(world)},
		Equipment:     equipmentBeside(scope.standing),
		TurnDriver:    scope.driver,
		CheckResolver: checkSeam{m: m, scope: scope},
		Witness:       witnessSeam{scope: scope},
		Machine:       resumed,
		Roller:        &diceSeam{roller: m.dice},
		// Cost is deliberately absent — see this function's own doc.
	})
	if err != nil {
		return nil, fmt.Errorf("react: %w", translateResolution(err))
	}

	// Closed BEFORE either tail runs, [answerCheckOffer]'s own ordering: a
	// failure to save leaves a session whose ledger and story disagree in
	// the direction that fails closed, and a re-pose below opens its OWN
	// new window rather than leaving this one both closed and open.
	if err := answerWindow(scope, window, choice); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true

	if out.Posed != nil {
		// ANOTHER target holds an offer. The same function the fresh cast
		// door calls, because a re-pose is not a different question — it is
		// the same one, asked of somebody else.
		castOut, err := m.poseCastWindow(ctx, scope, payload.Caster, payload.Spell, payload.Caught, out)
		if err != nil {
			return nil, fmt.Errorf("react: %w", err)
		}
		return &ReactOutput{Saved: castOut.Persisted, Delivery: castOut.Delivery}, nil
	}

	spellRef, err := core.ParseString(payload.Spell.Ref)
	if err != nil {
		return nil, fmt.Errorf("react: %w: %v", ErrInvalidSession, err)
	}
	castOut, err := m.finishCast(ctx, scope, payload.Caster, payload.Spell, *spellRef, payload.Caught, out)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	return &ReactOutput{Saved: castOut.Persisted, Delivery: castOut.Delivery}, nil
}

// castOfferDeclaration compiles one open cast-offer window into the row its
// audience sees — [checkOfferDeclaration]'s shape, reusing the same
// candidate-free, slot-free presentation.
func castOfferDeclaration(session, member string, window interrupt.Window) (Declaration, error) {
	payload, err := thawCastOfferPayload(window.Payload, string(window.Audience))
	if err != nil {
		return Declaration{}, err
	}
	id, err := reactDeclarationID(session, member, window.ID)
	if err != nil {
		return Declaration{}, err
	}
	offer := payload.Offer
	return Declaration{
		Verb:       VerbReact,
		Slot:       SlotNone,
		Available:  true,
		ID:         id,
		Reaction:   &offer,
		TargetKind: TargetNone,
		Candidates: []TargetCandidate{},
	}, nil
}
