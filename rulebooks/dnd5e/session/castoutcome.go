// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
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
//
// # It says nothing about concentration
//
// A cast can end a hold two ways that are neither its gate nor its delivery —
// displacing one by casting again, and breaking somebody else's with its
// damage — and neither passes through here. Both arrive on the interaction's
// own Output, already assembled, because a hold ends in whatever interaction
// happened to be running rather than as one machine's answer.
func castOutcome(
	outcome resolution.Outcome, actor string, spell SpellRef,
) ([]encounter.CastTargetResult, []castPush, error) {
	cast, ok := outcome.(resolution.CastOutcome)
	if !ok {
		return nil, nil, fmt.Errorf("%w: cast by %q produced %T", ErrInvalidWorld, actor, outcome)
	}

	targets := make([]encounter.CastTargetResult, 0, len(cast.Targets))
	var pushes []castPush
	for _, target := range cast.Targets {
		save, err := castSave(target, cast.Spell)
		if err != nil {
			return nil, nil, err
		}
		results := make([]encounter.ActivationResult, 0, len(target.Applied))
		for _, applied := range target.Applied {
			result, resultErr := imposedResult(applied, spell)
			if resultErr != nil {
				return nil, nil, resultErr
			}
			if applied.Kind == resolution.ImposedMove && applied.NotTaken == "" {
				// THE BEAT IS BUILT HERE AND FINISHED LATER. How far the push
				// went is the board's answer, not the contest's, and this
				// function has no board. It is named by index rather than by
				// pointer so nothing holds a reference into a slice that is
				// still growing.
				//
				// A MOVE THAT WAS NOT TAKEN IS NOT A PUSH TO FINISH. The
				// contest already answered it — the price went unpaid and
				// nothing moves — so there is nothing for the board to route
				// and nothing for the walk to walk. Its beat is already whole
				// (imposedResult writes the zero and the reason), and
				// collecting it here would ask the board how far a walk that
				// never happened would have gone.
				pushes = append(pushes, castPush{
					target:   encounter.MemberID(applied.RecipientID),
					move:     *applied.Move,
					targetAt: len(targets), resultAt: len(results),
				})
			}
			results = append(results, result)
		}
		targets = append(targets, encounter.CastTargetResult{
			Target: encounter.MemberID(target.TargetID), Missed: target.Missed, Save: save, Results: results,
		})
	}
	return targets, pushes, nil
}

// castSave reads the gate's saving throw, or nothing when there was no gate.
//
// NIL IS THE HONEST ZERO, and it is resolution's own spelling: a nil Save says
// "there is no saved beat to write" without a second boolean that could
// disagree with it. True Strike delivers a condition and rolls nothing, and a
// save beat reading 0 against DC 0 would say a roll happened that never did.
func castSave(target resolution.CastTargetOutcome, spell core.Ref) (*encounter.CastSave, error) {
	if target.Save == nil {
		return nil, nil
	}
	if target.Save.Save.Result == nil {
		// A contest reporting no saving throw result rolled nothing, and a beat
		// built from its zero values would narrate a roll that never happened.
		// A provider defect this fails closed on rather than records.
		return nil, fmt.Errorf("%w: cast %q contested a save with no result",
			ErrInvalidWorld, spell.String())
	}
	return &encounter.CastSave{
		// The saver is the creature the cast named. A contest is saver-centric
		// and this slice's casts have exactly one, so the beat's "who rolled"
		// and the cast's "who was named" are the same member by construction.
		Saver:       encounter.MemberID(target.TargetID),
		Ability:     string(target.Save.Ability),
		Roll:        target.Save.Save.Result.Roll,
		Total:       target.Save.Save.Result.Total,
		DC:          target.Save.DC,
		Calculation: rollCalculationFor(target.Save.Save.Result.Calculation),
		// The CASTER'S ruling, recorded rather than recomputed: the contest
		// owns whether the total beat the DC, including any exception to
		// Total >= DC this seam does not know about.
		Succeeded: target.Save.Succeeded,
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
// # A move is named by the spell too, and is only half filled in here
//
// A push is the one consequence whose outcome this function cannot know. What
// the contest decided is that the creature is moved and by what policy; how far
// it actually goes is the board's, and the board is not in scope. So this
// builds the beat and [castPush] finishes it.
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
		if imposed.Address.MemberID != imposed.RecipientID ||
			imposed.Address.ConditionRef != imposed.Ref.String() {
			return encounter.ActivationResult{}, fmt.Errorf(
				"%w: cast delivered a condition with a mismatched address", ErrInvalidWorld)
		}
		return encounter.ActivationResult{
			Kind: encounter.ResultConditionApplied,
			Address: &encounter.ConditionAddress{
				MemberID:     encounter.MemberID(imposed.Address.MemberID),
				ConditionRef: imposed.Address.ConditionRef,
				SourceID:     imposed.Address.SourceID,
			},
			Name: imposed.Description,
		}, nil
	case resolution.ImposedHealing:
		return encounter.ActivationResult{Kind: encounter.ResultHealingApplied, Target: encounter.MemberID(imposed.RecipientID), Ref: imposed.Ref.String(), Name: imposed.Description, Amount: imposed.Amount, Requested: imposed.Requested, Before: imposed.Before, After: imposed.After, Calculation: rollCalculationFor(imposed.Calculation)}, nil
	case resolution.ImposedDamage:
		if len(imposed.Components) == 0 {
			// Damage with no components has no type to report, and the
			// composition refuses a damage result without one. Failing closed
			// here names the defect where it can be read rather than letting
			// the record refuse a beat nobody can trace back.
			return encounter.ActivationResult{}, fmt.Errorf(
				"%w: cast delivered damage with no components", ErrInvalidWorld)
		}
		return encounter.ActivationResult{
			Kind:   encounter.ResultDamageApplied,
			Target: encounter.MemberID(imposed.RecipientID),
			Ref:    spell.Ref,
			Name:   spell.Name,
			// THE FIRST COMPONENT'S TYPE, which is the whole of it while a
			// cantrip declares one damage pool. Kirk's ruling: the cap is the
			// current use case. A spell that deals two types wants a
			// per-component carrier rather than a wider guess here, and it
			// arrives with that spell.
			DamageType: string(imposed.Components[0].DamageType),
			Amount:     imposed.Amount,
			Requested:  imposed.Requested,
			Before:     imposed.Before,
			After:      imposed.After,
			// Deep-cloned through rollCalculationFor so no captured trace
			// aliases the provider's own graph — the same copy
			// activationResults makes for a heal's.
			Calculation: rollCalculationFor(imposed.Calculation),
		}, nil
	case resolution.ImposedMove:
		if imposed.Move == nil {
			// resolution fills the directive for this kind and nothing else
			// produces one, so a nil here is a provider defect. Failing closed
			// names it where it can be read, rather than recording a push with
			// no policy and then finding nothing to walk.
			return encounter.ActivationResult{}, fmt.Errorf(
				"%w: cast delivered a move with no directive", ErrInvalidWorld)
		}
		// HOW FAR IT WENT IS NOT KNOWN YET, and the zero is not a claim that
		// it went nowhere. This function projects what the CONTEST decided;
		// the distance and the blocker are the board's answer and are written
		// onto this result by the route, before the record is taken. See
		// [castPush].
		//
		// UNLESS THE CONTEST ALREADY ANSWERED IT. A move priced at something
		// the mover could not pay is not a move waiting on the board — it is a
		// finished outcome the contest reached, and NotTaken is its reason. So
		// it is written here, in the one place a zero distance is a claim: no
		// cells, and the sentence saying why. Reporting it as an absent result
		// would make "nothing pushed you" and "you had nothing left to spend"
		// the same silence.
		return encounter.ActivationResult{
			Kind:      encounter.ResultMoved,
			Target:    encounter.MemberID(imposed.RecipientID),
			Ref:       spell.Ref,
			Name:      spell.Name,
			StoppedBy: imposed.NotTaken,
		}, nil
	default:
		// A delivered kind this build has no case for is a resolution newer
		// than this seam, and a beat that guessed at it would narrate
		// something that did not happen.
		return encounter.ActivationResult{}, fmt.Errorf(
			"%w: cast delivered an unknown effect kind %q", ErrInvalidWorld, imposed.Kind)
	}
}
