// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// FollowUpOutcome is one check a damage fact came back with, and what failing
// it cost.
//
// It rides [StrikeOutcome] and [CastOutcome] rather than [Output], because the
// follow-up happened INSIDE the interaction that caused it and the record is
// written from the interaction's own outcome. Nothing about it reaches the
// session as a decision: the session reads what happened and writes it down.
type FollowUpOutcome struct {
	// SaverID and Ability are who rolled and what they rolled, carried here
	// because [SaveOutcome] holds the roll and not the question. A saved beat
	// names both, and neither can be recovered from the result.
	SaverID string
	Ability abilities.Ability

	// Save is the check, whole — roll, total, DC and success.
	Save SaveOutcome

	// Ended is what a failed check ended, and NIL on a success. Nil is what
	// says "nothing came off", without a second boolean to disagree with it.
	Ended *ConcentrationEnded
}

// ConcentrationEnded is one spell that stopped being held together, and why.
//
// Named for its only customer rather than generically, because that is what it
// is: this slice's one consequence kind is a concentration break, and a
// "consequence outcome" with a reason string would be a vocabulary pretending
// to be a type. The day a second consequence exists, this is what has to widen.
type ConcentrationEnded struct {
	// CasterID is who was holding it.
	CasterID string

	// Spell is what they were holding, as the follow-up's cause named it.
	Spell *core.Ref

	// SpellName is what to call it, read off the hold BEFORE the strip so the
	// record can name a spell that no longer exists on any sheet.
	//
	// Carried rather than looked up downstream, because by the time anybody
	// downstream reads this the condition that knew the name is gone. A ref is
	// not a name, and turning one into English is the rulebook's job, not the
	// record's.
	SpellName string

	// Reason is why it ended, in the rulebook's vocabulary.
	Reason string

	// Removed are the addresses that came off with it, in publication order.
	Removed []dnd5eEvents.ChildRef
}

// ConditionRemoval is a contest's THIRD consequence: these conditions come off
// those sheets, and the owner that asked for the check ends with them.
//
// Addresses rather than conditions, for the reason [dnd5eEvents.ChildRef]
// exists: the delivery publishes one removal FACT per address and each
// member's own keeper honours it. Nothing here reaches across to another
// member's sheet, because nothing in the toolkit does.
type ConditionRemoval struct {
	// Addresses are the children to strip.
	Addresses []dnd5eEvents.ChildRef

	// Owner is the condition that asked for the check.
	Owner dnd5eEvents.ChildRef

	// Reason is why, carried onto every removal fact this delivers.
	Reason string
}

// reportDamageInput is the fact one damage-applying machine has to state.
type reportDamageInput struct {
	MemberID      string
	Amount        int
	DamageType    damage.Type
	DroppedToZero bool
	Cause         dnd5eEvents.SaveCause
}

// reportDamage says what landed, on the interaction's own bus, and hands back
// what came of it.
//
// # Why the machine publishes and the sheet does not
//
// Applying damage is bus-free by design — "the sheet's own business" — and the
// bus parked on a sheet is a CAPTURED bus, which [movementMachine.bill] refuses
// by name. Only the machine holds the driver's bus, and only the machine can
// run what comes back. A sheet publish would additionally force the follow-ups
// back through combat.ApplyDamageResult, putting a field shaped like a
// resolution step on an interface both Character and Monster implement.
//
// The invariant that makes this complete rather than partial: DAMAGE REACHES A
// SHEET ONLY INSIDE AN INTERACTION. ApplyDamage has exactly two callers, both
// machines, and both call this. The day something applies damage outside an
// interaction, that is the defect to fix rather than a missing publish to add.
//
// # The pointer is the return channel
//
// [dnd5eEvents.DamageTakenTopic] is defined over *DamageTakenEvent, so what a
// subscriber appends to FollowUps reaches this closure. Publishing a value
// would hand each subscriber a copy and discard everything it wrote.
func reportDamage(
	in reportDamageInput, next func(context.Context, []dnd5eEvents.FollowUp) (Step, error),
) Gather {
	return Gather{
		name: fmt.Sprintf("report %d damage taken by %s", in.Amount, in.MemberID),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			fact := &dnd5eEvents.DamageTakenEvent{
				MemberID:      in.MemberID,
				Amount:        in.Amount,
				DamageType:    in.DamageType,
				DroppedToZero: in.DroppedToZero,
				Cause:         in.Cause,
			}
			if err := dnd5eEvents.DamageTakenTopic.On(bus).Publish(ctx, fact); err != nil {
				return nil, fmt.Errorf("report damage taken by %q: %w", in.MemberID, err)
			}

			return next(ctx, fact.FollowUps)
		},
	}
}

// runFollowUps answers follow-up i, or hands back to done when they are
// exhausted.
//
// ONE STEP PER FOLLOW-UP rather than one step resolving all of them, for the
// reason [movementMachine.react] yields one per trigger: each is a separate
// thing that happened, and every yield point is a legal suspension point. Two
// concentrating members damaged by one interaction are two follow-ups and two
// nested checks, with no special case.
//
// A nested check cannot suspend — drive refuses a posed sub-machine by name,
// because the requester's continuation is a Go closure on this stack and
// nothing serializes it — so the check is structurally automatic, which is
// also what RAW 2014 says it is.
func runFollowUps(
	ctx context.Context, cast *Participants, ups []dnd5eEvents.FollowUp, i int, roller dice.Roller,
	record func(FollowUpOutcome), done func(context.Context) (Step, error),
) (Step, error) {
	if i >= len(ups) {
		return done(ctx)
	}

	up := ups[i]
	in, err := followUpContest(up, roller)
	if err != nil {
		return nil, err
	}

	// The name is read NOW, while the hold is still on the sheet. A failed
	// check strips it before the outcome is built, and a record that could not
	// say which spell ended would be a beat nobody can read.
	name := heldSpellName(cast, up.OnFailure.Owner.MemberID)

	return requestContest(in, func(inner context.Context, contest ContestOutcome) (Step, error) {
		record(followUpOutcome(up, name, contest))

		return runFollowUps(inner, cast, ups, i+1, roller, record, done)
	}), nil
}

// heldSpellName is what the owner calls the spell it is holding, or empty when
// the owner is not a concentrating condition — which is what a second
// consequence kind would look like the day one exists.
func heldSpellName(cast *Participants, memberID string) string {
	if held, holding := concentrationHeldBy(cast, memberID); holding {
		return held.SpellName
	}

	return ""
}

// followUpContest turns a follow-up into the contest that answers it.
//
// The gate is the common one — one ability, negated on a success, no
// recurrence — with the DC the follow-up already settled riding as a static
// number. That is slice two's rule applied again: a DC known before the machine
// runs travels as [saves.DCStatic] rather than as a formula the machine
// evaluates, because a DCSource exists for a DC DERIVED at resolution time.
func followUpContest(up dnd5eEvents.FollowUp, roller dice.Roller) (*ContestInput, error) {
	if up.SaverID == "" {
		return nil, fmt.Errorf("%w: a follow-up names nobody to roll it", ErrBadAction)
	}
	if up.OnFailure.Owner.MemberID == "" || up.OnFailure.Owner.ConditionRef == "" {
		return nil, fmt.Errorf(
			"%w: a follow-up by %q names no owner for its failure to end", ErrBadAction, up.SaverID)
	}

	in := &ContestInput{
		Gate: &saves.SaveGate{
			Abilities:  []abilities.Ability{up.Ability},
			DC:         saves.DCStatic(up.DC),
			OnSuccess:  saves.Negated,
			Recurrence: saves.RecurrenceNone,
		},
		SaverID: up.SaverID,
		Cause:   up.Cause,
		Roller:  roller,
		Removal: &ConditionRemoval{
			Addresses: append([]dnd5eEvents.ChildRef(nil), up.OnFailure.Remove...),
			Owner:     up.OnFailure.Owner,
			Reason:    up.OnFailure.Reason,
		},
	}
	if err := refuseFollowUpDamage(in); err != nil {
		return nil, err
	}

	return in, nil
}

// refuseFollowUpDamage is R2's recursion bound, STATED rather than discovered.
//
// A follow-up's contest that applied damage would republish the damage fact
// from inside a nested contest and collect more follow-ups, without limit.
// It cannot: a consequence in this slice is removal only, and this refuses the
// construction that would make it otherwise. DEPTH IS EXACTLY ONE.
//
// The day a consequence damages — a retaliation aura, a shield of thorns —
// this refusal is what has to move, and it must be REPLACED WITH AN EXPLICIT
// BOUND rather than deleted.
func refuseFollowUpDamage(in *ContestInput) error {
	if len(in.Damage) > 0 {
		return fmt.Errorf(
			"%w: a contest built from a follow-up may not declare damage (%s), "+
				"because a consequence that damages has no depth bound",
			ErrBadAction, describeDamage(in.Damage))
	}

	return nil
}

// followUpOutcome reads the check back as the record's line for it.
func followUpOutcome(
	up dnd5eEvents.FollowUp, spellName string, contest ContestOutcome,
) FollowUpOutcome {
	out := FollowUpOutcome{SaverID: up.SaverID, Ability: contest.Ability, Save: contest.Save}
	if contest.Succeeded {
		// A made check removes nothing, and the roll is still the record.
		return out
	}

	out.Ended = &ConcentrationEnded{
		CasterID:  up.OnFailure.Owner.MemberID,
		Spell:     cloneCoreRef(up.Cause.EffectRef),
		SpellName: spellName,
		Reason:    up.OnFailure.Reason,
		Removed:   append([]dnd5eEvents.ChildRef(nil), up.OnFailure.Remove...),
	}

	return out
}

// primaryDamageType is what the fact calls a blow that landed as several typed
// instances: the first one.
//
// One type on the fact and several on the blow is a real narrowing, and it is
// named rather than hidden. Nothing in this slice reads DamageType — the
// concentration check does not care what hurt — so a richer shape would be a
// field with no reader. The day a subscriber breaks on fire and not on cold,
// this is the line that has to grow into the typed breakdown.
func primaryDamageType(instances []damage.Instance) damage.Type {
	if len(instances) == 0 {
		return ""
	}

	return instances[0].Type
}

// primaryComponentType is [primaryDamageType] for the other shape damage
// arrives in — a contest holds folded components where a strike holds typed
// instances — and it narrows the same way, for the same reason.
func primaryComponentType(components []dnd5eEvents.DamageComponent) damage.Type {
	if len(components) == 0 {
		return ""
	}

	return components[0].DamageType
}
