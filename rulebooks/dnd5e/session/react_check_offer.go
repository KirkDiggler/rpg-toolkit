// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// answerCheckOffer finishes a check that stopped after its d20 —
// [Manager.answerPostRoll]'s shape, for [resolution.ResumeCheck] instead of
// a resumed strike machine.
//
// # It resumes the SAME check rather than rolling again
//
// The window froze resolution's own state; this hands it straight back
// through [resolution.ResumeCheck] with the answer. Nothing is re-rolled and
// nothing re-folded.
//
// # The checker's sheet is reloaded fresh, not carried in the payload
//
// [resolution.ResumeCheck] attaches unconditionally whichever answer this
// is (its own doc explains why), and the sheet it attaches must be the
// checker's CURRENT record — a rest or another effect between the question
// and the answer is real state, not staleness to paper over.
//
// # Which verb it finishes is the payload's to say
//
// [checkOfferWindowPayload.Door] and [checkOfferWindowPayload.Target] name
// what to finish, and exactly one of them is ever set (that field's doc
// says why, and thawCheckOfferPayload refuses a payload that sets both or
// neither). A door finishes the Unlock the question paused; a target
// finishes the Intimidate.
func (m *Manager) answerCheckOffer(
	ctx context.Context, scope *writeScope, window interrupt.Window, choice ReactChoice,
) (*ReactOutput, error) {
	payload, err := thawCheckOfferPayload(window.Payload, string(window.Audience))
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	answer := resolution.OfferKeep
	if choice == ReactStrike {
		answer = resolution.OfferSpend
	}

	data, err := m.fetchCharacterData(ctx, "member", payload.Audience)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	out, err := resolution.ResumeCheck(ctx, &resolution.CheckResumeInput{
		Frozen:    payload.Frozen,
		Answer:    answer,
		Character: data,
		// The offered die is rolled with the HOST'S dice, through the same
		// seam every other roll in this package takes.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("react: %w", translateResolution(err))
	}

	if out.DirtyCharacter != nil {
		if err := m.characters.SaveCharacter(ctx, out.DirtyCharacter); err != nil {
			report := SaveReport{
				Written: append([]string(nil), scope.written...),
				Failed:  []string{"character:" + out.DirtyCharacter.ID},
			}
			return nil, &SaveError{Report: report, Err: fmt.Errorf("saving checker: %w", err)}
		}
		scope.written = append(scope.written, "character:"+out.DirtyCharacter.ID)
	}

	// Finish the verb this check was for. Each composition op records its
	// own beat — the ordinary (unposed) paths do not record separately
	// either, so the resumed half stays symmetric with them.
	//
	// The ACTION IS NOT CHARGED HERE. Intimidate charged it before it
	// rolled, precisely so that a member who pauses cannot answer the
	// question and then threaten somebody else with the same action.
	// THE RESUMED HALF APPENDS THE SAME BEATS, so it fails closed the same
	// way: a resumed check that lost its arithmetic would publish one number
	// through a door the unposed path has just been shut on.
	if err := requireCalculation("react", payload.Audience, rollCalculationFor(out.Calculation)); err != nil {
		return nil, err
	}

	if payload.Door != "" {
		if _, err := scope.enc.Unlock(&encounter.UnlockInput{
			Door:        payload.Door,
			Beaten:      out.Result.Success,
			Actor:       encounter.MemberID(payload.Audience),
			Total:       out.Result.Total,
			Applied:     out.Applied,
			Calculation: rollCalculationFor(out.Calculation),
		}); err != nil {
			return nil, fmt.Errorf("react: %w", translate(err))
		}
	} else if err := m.landResumedSocial(ctx, scope, payload, out); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	// Closed BEFORE the beat is committed, so a failure to save leaves a
	// session whose ledger and story disagree in the direction that fails
	// closed: the window stays open only if nothing was written at all.
	if err := answerWindow(scope, window, choice); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	return &ReactOutput{Saved: report, Delivery: delivery}, nil
}

// checkOfferDeclaration compiles one open check-offer window into the row
// its audience sees — [postRollDeclaration]'s shape, reusing the same
// candidate-free, slot-free presentation: answering costs no reaction, the
// price is the thing being offered.
func checkOfferDeclaration(session, member string, window interrupt.Window) (Declaration, error) {
	payload, err := thawCheckOfferPayload(window.Payload, string(window.Audience))
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

// landResumedSocial finishes the social verb a check-offer window paused,
// through the same composition op the unpaused path uses.
//
// THE PAYLOAD SAYS WHICH VERB, and this switch is why it has to
// ([checkOfferWindowPayload.Verb]). While Intimidate was the only social verb,
// "a target and no door" named it unambiguously; with two, a resumed Persuade
// that landed an Intimidate would put the wrong deed on a mind — the coward
// would take fear from a conversation it was talked round by. The payload is
// refused at the trust boundary when it names neither.
//
// NOTHING IS RE-ROLLED. The verdict handed in is the resumed machine's, offer
// included; this only carries it across the seam. The action, if one was
// spent, was spent before the question was asked.
func (m *Manager) landResumedSocial(
	ctx context.Context, scope *writeScope, payload checkOfferWindowPayload, out *resolution.CheckOutput,
) error {
	landing := &socialLanding{
		Actor:  encounter.MemberID(payload.Audience),
		Target: encounter.MemberID(payload.Target),
		Beaten: out.Result.Success,
		DC:     out.Applied.DC,
		Total:  out.Result.Total,
		// The RESUMED calculation, which is the pre-offer one plus whatever
		// the answer added — never the frozen one the window asked with. The
		// caller refused a nil one before reaching either branch.
		Calculation: rollCalculationFor(out.Calculation),
	}

	var spec socialVerb
	switch payload.Verb {
	case VerbIntimidate:
		spec = m.intimidateVerb()
	case VerbPersuade:
		spec = m.persuadeVerb()
	default:
		// Unreachable: thawCheckOfferPayload refuses a target under any other
		// verb. Refusing rather than defaulting keeps the day that stops being
		// true from silently landing a threat.
		return fmt.Errorf("%w: a paused check names target %q under verb %q",
			ErrInvalidSession, payload.Target, payload.Verb)
	}

	if _, _, err := spec.land(ctx, scope.enc, landing); err != nil {
		return translate(err)
	}

	return nil
}
