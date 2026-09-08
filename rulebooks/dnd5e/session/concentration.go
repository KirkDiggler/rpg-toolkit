// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// concentrationBreaks projects the concentrations one interaction ended onto
// the composition's own carrier, in the order the rulebook ended them.
//
// # It decides nothing, and that is the whole slice
//
// Kirk's ruling on where the check lives: *"resolution is the place that
// should resolve things. it shouldn't have to leak out."* So this reads an
// outcome and writes it down. It asks for no saving throw, computes no DC,
// consults no sheet, and never learns that damage was taken — the numbers
// arrive already rolled, already compared, already spent. What it produces
// rides the SAME Record call the strike or the cast already makes, so one blow
// leaves one train in the story: struck, the check that was failed, the break,
// and one condition-removed per address the ending spell was holding.
// TestSessionConstructsNoCheck holds the half of that claim a test can hold.
//
// # A made check produces nothing here, and that is a gap rather than a rule
//
// A follow-up whose save succeeded carries no [resolution.ConcentrationEnded],
// and the composition's break carrier has nowhere to put a save that broke
// nothing — [encounter.ConcentrationBreak.Save] is the check that was FAILED,
// nested inside the break it caused. So a bard who takes 9 and makes the DC 10
// gets no saved beat, and the table sees a blow with no visible check behind
// the spell surviving. Named on the slice's gap thread rather than fixed by
// widening someone else's shape from here.
func concentrationBreaks(followUps []resolution.FollowUpOutcome) []encounter.ConcentrationBreak {
	breaks := make([]encounter.ConcentrationBreak, 0, len(followUps))
	for _, followUp := range followUps {
		if followUp.Ended == nil {
			// The check was made and the spell held. Nothing ended, so there
			// is no break to record — see this function's own doc for the
			// saved beat that goes with it.
			continue
		}

		broken := concentrationBreak(*followUp.Ended)
		// The CHECK THAT WAS FAILED, carried inside the break it caused rather
		// than beside it. The composition writes it as the saved beat
		// immediately before the break, which is the order it happened in.
		//
		// A NIL RESULT IS NOT GUARDED HERE. A break that ended a spell without
		// a roll behind it reaches the composition as a d20 reading zero, and
		// the composition refuses the whole transaction by name — before the
		// strike beat itself lands. That refusal is louder and lands in one
		// place; a guard here would be a second opinion about a beat this seam
		// does not validate, and it would have to say what to do with a break
		// it had just decided was unwritable.
		if result := followUp.Save.Result; result != nil {
			broken.Save = &encounter.CastSave{
				Saver:   encounter.MemberID(followUp.SaverID),
				Ability: string(followUp.Ability),
				Roll:    result.Roll,
				Total:   result.Total,
				DC:      result.DC,
				// Recorded rather than recomputed, the same reading castSave
				// keeps: the rules entry owns whether the total beat the DC,
				// including any exception this seam does not know about. It is
				// false on every break here by construction — a made check
				// ends nothing — and copied rather than written as a literal,
				// so a machine that ever disagreed with itself is visible in
				// the record instead of overwritten by this line.
				Succeeded: result.Success,
			}
		} else {
			broken.Save = &encounter.CastSave{
				Saver:   encounter.MemberID(followUp.SaverID),
				Ability: string(followUp.Ability),
			}
		}
		breaks = append(breaks, broken)
	}

	if len(breaks) == 0 {
		// NIL RATHER THAN AN EMPTY SLICE, the same spelling castOutcome uses
		// for a cast that delivered nothing: the composition reads an absent
		// list as "this interaction broke nobody's concentration", and that is
		// exactly what happened.
		return nil
	}
	return breaks
}

// concentrationBreak projects one ended concentration, check or no check.
//
// # Nothing here is validated, and that is deliberate
//
// A caster who is not in the roster, a spell with no ref, a break with no
// stated reason: the composition refuses every one of those by name, before a
// single beat of the transaction lands. A second set of guards here would be a
// second opinion about a rule this seam does not own, free to disagree with the
// first and certain to drift from it.
func concentrationBreak(ended resolution.ConcentrationEnded) encounter.ConcentrationBreak {
	var ref string
	if ended.Spell != nil {
		ref = ended.Spell.String()
	}

	removed := make([]encounter.ActivationResult, 0, len(ended.Removed))
	for _, child := range ended.Removed {
		removed = append(removed, encounter.ActivationResult{
			Kind:   encounter.ResultConditionRemoved,
			Target: encounter.MemberID(child.MemberID),
			Ref:    child.ConditionRef,
			// NAMED BY THE SPELL THAT WAS HOLDING IT, which is the same
			// reading imposedResult uses for a cast's damage and for the same
			// reason: the removal and the break beside it are one event in the
			// story, and a client saying "True Strike ended" on both lines
			// says one thing rather than two.
			//
			// The address the rulebook publishes is a member and a condition
			// ref and carries no display name — by the time anybody could look
			// one up, the condition that knew it is gone from the sheet. The
			// spell's name is the one that survives the strip, which is why
			// resolution carries it out.
			Name: ended.SpellName,
			// The same word the break itself gives, so a reader never has to
			// hold two causes for one event.
			Reason: ended.Reason,
		})
	}

	return encounter.ConcentrationBreak{
		Caster: encounter.MemberID(ended.CasterID),
		Spell:  encounter.SpellIdentity{Ref: ref, Name: ended.SpellName},
		Reason: ended.Reason,
		Removed: func() []encounter.ActivationResult {
			if len(removed) == 0 {
				// A spell holding nothing when it broke. Empty is legal and
				// nil is how this seam spells empty.
				return nil
			}
			return removed
		}(),
	}
}

// castConcentrationBreaks is [concentrationBreaks] for a cast, which can end a
// concentration two ways in one call.
//
// THE DISPLACED SPELL COMES FIRST, because it went first: a second
// concentration cast drops the first before the new spell resolves, while
// every follow-up here is a question the cast's own damage asked afterwards. A
// record written the other way round would show a bard losing Bless to their
// own Vicious Mockery after the mockery had already landed.
//
// The drop carries no save, and nil is the honest zero for one: casting again
// ends the first spell outright, so there is no roll and a save beat reading 0
// against DC 0 would say one happened.
func castConcentrationBreaks(cast resolution.CastOutcome) []encounter.ConcentrationBreak {
	breaks := concentrationBreaks(cast.FollowUps)
	if cast.Dropped == nil {
		return breaks
	}
	return append([]encounter.ConcentrationBreak{concentrationBreak(*cast.Dropped)}, breaks...)
}
