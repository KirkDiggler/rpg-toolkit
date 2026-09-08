// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// castOutcome projects one resolved cast onto the record shape the composition
// takes: the save its gate produced, and everything it delivered.
//
// # One outcome, both halves of the door
//
// A cast profile with a gate resolves as a contest and one without it as a
// delivery, and resolution wraps both in a single [resolution.CastOutcome] —
// so this switches once rather than carrying an arm per machine. That is the
// design's own reading (R2, R2a): neither half is a new machine, and a cast's
// DELIVERIES are one concept however they were arrived at.
//
// The word "cast" lives at the door and nowhere below it. Resolution hands over
// a gate that was contested or was not, and the two beats this writes are what
// call the whole of it a cast.
func castOutcome(
	outcome resolution.Outcome, actor string, spell SpellRef,
) (*encounter.CastSave, []encounter.ActivationResult, error) {
	cast, ok := outcome.(resolution.CastOutcome)
	if !ok {
		return nil, nil, fmt.Errorf("%w: cast by %q produced %T", ErrInvalidWorld, actor, outcome)
	}

	save, err := castSave(cast)
	if err != nil {
		return nil, nil, err
	}

	if len(cast.Applied) == 0 {
		// A made save delivers nothing. Nil rather than an empty slice: the
		// composition reads "no results" as a complete cast, and a save that
		// negated both halves is exactly that.
		return save, nil, nil
	}

	results := make([]encounter.ActivationResult, 0, len(cast.Applied))
	for _, applied := range cast.Applied {
		result, resultErr := imposedResult(applied, spell)
		if resultErr != nil {
			return nil, nil, resultErr
		}
		results = append(results, result)
	}
	return save, results, nil
}

// castSave reads the gate's saving throw, or nothing when there was no gate.
//
// NIL IS THE HONEST ZERO, and it is resolution's own spelling: a nil Save says
// "there is no saved beat to write" without a second boolean that could
// disagree with it. True Strike delivers a condition and rolls nothing, and a
// save beat reading 0 against DC 0 would say a roll happened that never did.
func castSave(cast resolution.CastOutcome) (*encounter.CastSave, error) {
	if cast.Save == nil {
		return nil, nil
	}
	if cast.Save.Save.Result == nil {
		// A contest reporting no saving throw result rolled nothing, and a beat
		// built from its zero values would narrate a roll that never happened.
		// A provider defect this fails closed on rather than records.
		return nil, fmt.Errorf("%w: cast %q contested a save with no result",
			ErrInvalidWorld, cast.Spell.String())
	}
	return &encounter.CastSave{
		// The saver is the creature the cast named. A contest is saver-centric
		// and this slice's casts have exactly one, so the beat's "who rolled"
		// and the cast's "who was named" are the same member by construction.
		Saver:   encounter.MemberID(cast.TargetID),
		Ability: string(cast.Save.Ability),
		Roll:    cast.Save.Save.Result.Roll,
		Total:   cast.Save.Save.Result.Total,
		DC:      cast.Save.DC,
		// The CASTER'S ruling, recorded rather than recomputed: the contest
		// owns whether the total beat the DC, including any exception to
		// Total >= DC this seam does not know about.
		Succeeded: cast.Save.Succeeded,
	}, nil
}

// imposedResult projects one delivered consequence onto the composition's
// activation-result carrier.
//
// # The result kinds are reused, and the message's name is the only wrong thing
//
// A condition applied by a spell and a condition applied by a feature are the
// same fact in the story, so both travel as the same beat with the same closed
// kinds (design R7). Damage adds one kind beside them rather than a second
// carrier.
//
// # The recipient travels with the effect
//
// It is never inferred here. True Strike's condition lands on the CASTER while
// the cast named a creature, and Vicious Mockery's lands on the target — so a
// reader that assumed "the target" would put half of this slice's conditions on
// the wrong sheet.
//
// # Damage is named by the SPELL
//
// A cast's damage names whatever the save's cause named, and for a cantrip that
// is the cantrip. Filling it from the offer's own identity keeps the result
// beat and the cast beat naming one thing, which is what a client needs to say
// "Vicious Mockery, 3 psychic" on one line.
func imposedResult(
	imposed resolution.ImposedEffect, spell SpellRef,
) (encounter.ActivationResult, error) {
	if imposed.RecipientID == "" {
		return encounter.ActivationResult{}, fmt.Errorf(
			"%w: cast delivered a %q effect to nobody", ErrInvalidWorld, imposed.Kind)
	}

	switch imposed.Kind {
	case resolution.ImposedCondition:
		if imposed.Ref == nil {
			return encounter.ActivationResult{}, fmt.Errorf(
				"%w: cast delivered a condition with no ref", ErrInvalidWorld)
		}
		return encounter.ActivationResult{
			Kind:   encounter.ResultConditionApplied,
			Target: encounter.MemberID(imposed.RecipientID),
			Ref:    imposed.Ref.String(),
			Name:   imposed.Description,
		}, nil
	case resolution.ImposedDamage:
		return encounter.ActivationResult{
			Kind:      encounter.ResultDamageApplied,
			Target:    encounter.MemberID(imposed.RecipientID),
			Ref:       spell.Ref,
			Name:      spell.Name,
			Amount:    imposed.Amount,
			Requested: imposed.Requested,
			Before:    imposed.Before,
			After:     imposed.After,
			// Deep-cloned through rollCalculationFor so no captured trace
			// aliases the provider's own graph — the same copy
			// activationResults makes for a heal's.
			Calculation: rollCalculationFor(imposed.Calculation),
		}, nil
	default:
		// A delivered kind this build has no case for is a resolution newer
		// than this seam, and a beat that guessed at it would narrate
		// something that did not happen.
		return encounter.ActivationResult{}, fmt.Errorf(
			"%w: cast delivered an unknown effect kind %q", ErrInvalidWorld, imposed.Kind)
	}
}
