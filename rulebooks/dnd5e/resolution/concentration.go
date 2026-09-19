// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// concentrationCollector hears every hold that ended during one interaction.
//
// # One collector for every Resolve, and not one per machine
//
// A hold ends seven ways and only ONE of them happens inside a machine that
// knows about concentration: the failed check a strike or a cast ran. The other
// six are the condition's own business — its clock running out and the fight
// ending both land inside a boundary interaction, its last child ending and the
// caster dropping to zero land inside whatever blow did it, a recast lands in
// the cast that displaced it, and a long rest lands in a rest. A machine-by-
// machine collector would therefore have to be added to every machine that
// exists and to every machine that ever will, each of them learning what
// concentration is for no other reason.
//
// So the subscription lives where the interaction does. It is opened for the
// life of one Resolve on the driver's own bus, hears the fact whoever published
// it, and is closed with the rest of the interaction. Nothing here knows which
// machine ran.
type concentrationCollector struct {
	facts []dnd5eEvents.ConcentrationEndedEvent
	stop  func(context.Context) error
}

// collectConcentrationEnds opens the collector on the interaction's own bus.
func collectConcentrationEnds(ctx context.Context, bus events.EventBus) (*concentrationCollector, error) {
	collector := &concentrationCollector{}
	id, err := dnd5eEvents.ConcentrationEndedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConcentrationEndedEvent) error {
			collector.facts = append(collector.facts, event)

			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("resolution: collect concentration ends: %w", err)
	}
	collector.stop = func(inner context.Context) error { return bus.Unsubscribe(inner, id) }

	return collector, nil
}

// checks reads the checks that were MADE back in the shape the record takes.
//
// A made check and a break are the two halves of one question and they are
// recorded apart, because encounter refuses a check that changed nothing to be
// written as a break: a failed check carries its roll on the break it caused,
// and a made one has no break to ride.
//
// # Where the spell's identity comes from
//
// The hold itself, read off the caster's sheet — because the check was MADE, so
// the hold is still there. The fallback covers the one case where it is not:
// a caster who kept the spell against the damage and then lost it later in the
// same interaction, to the fight ending or its own clock, whose identity is on
// the ended fact that break published. If neither has it, that is a rule that
// did not run and it is refused rather than recorded half-named.
func (c *concentrationCollector) checks(
	cast *Participants, outcome Outcome,
) ([]encounter.ConcentrationCheck, error) {
	return c.checksFrom(cast, followUpsOf(outcome))
}

// checksFrom is [concentrationCollector.checks] over a follow-up list the
// caller chose, rather than over a whole outcome's.
//
// SPLIT OUT FOR THE SEQUENCE, which records per swing: every other machine
// asks about its one interaction and gets exactly what it got before, while a
// multiattack asks once per step and gets that step's own rolls. The body is
// unchanged — the only thing that moved is where the list comes from.
func (c *concentrationCollector) checksFrom(
	cast *Participants, followUps []FollowUpOutcome,
) ([]encounter.ConcentrationCheck, error) {
	made := make([]encounter.ConcentrationCheck, 0)
	for _, check := range followUps {
		if check.Save.Result == nil || !check.Save.Result.Success {
			continue
		}
		spell, err := c.spellHeldBy(cast, check.SaverID)
		if err != nil {
			return nil, err
		}
		made = append(made, encounter.ConcentrationCheck{
			Spell: spell,
			Save:  castSave(check),
		})
	}
	if len(made) == 0 {
		return nil, nil
	}

	return made, nil
}

// spellHeldBy names the spell a caster was holding when it rolled.
func (c *concentrationCollector) spellHeldBy(
	cast *Participants, casterID string,
) (encounter.SpellIdentity, error) {
	if held, holding := concentrationHeldBy(cast, casterID); holding {
		return encounter.SpellIdentity{Ref: held.SpellRef, Name: held.SpellName}, nil
	}
	for _, fact := range c.facts {
		if fact.CasterID == casterID {
			return encounter.SpellIdentity{Ref: fact.SpellRef, Name: fact.SpellName}, nil
		}
	}

	return encounter.SpellIdentity{}, fmt.Errorf(
		"%w: %q made a concentration check while holding nothing this interaction can name",
		ErrBadStep, casterID)
}

// castSave is one roll, in the shape both halves of the record take.
func castSave(check FollowUpOutcome) encounter.CastSave {
	result := check.Save.Result

	return encounter.CastSave{
		Saver:       encounter.MemberID(check.SaverID),
		Ability:     string(check.Ability),
		Roll:        result.Roll,
		Total:       result.Total,
		DC:          result.DC,
		Calculation: encounterRollCalculation(result.Calculation),
		Succeeded:   result.Success,
	}
}

func encounterRollCalculation(calculation *dnd5eEvents.RollCalculation) *encounter.RollCalculation {
	if calculation == nil {
		return nil
	}
	converted := &encounter.RollCalculation{Total: calculation.Total}
	converted.Components = make([]encounter.RollComponent, len(calculation.Components))
	for i, component := range calculation.Components {
		converted.Components[i] = encounter.RollComponent{
			Source: encounter.RollSource{
				Name: component.Source.Name, Label: component.Source.Label, SourceID: component.Source.SourceID,
			},
			SubtractDice: component.SubtractDice,
		}
		if component.Source.Ref != nil {
			converted.Components[i].Source.Ref = component.Source.Ref.String()
		}
		if component.Modifier != nil {
			modifier := *component.Modifier
			converted.Components[i].Modifier = &modifier
		}
		if component.Dice != nil {
			trace := &encounter.DiceTrace{
				Notation: component.Dice.Notation, DieSize: component.Dice.DieSize,
				OriginalRolls: append([]int(nil), component.Dice.OriginalRolls...),
				FinalRolls:    append([]int(nil), component.Dice.FinalRolls...),
				KeptIndices:   append([]int(nil), component.Dice.KeptIndices...),
				Subtotal:      component.Dice.Subtotal,
			}
			trace.Rerolls = make([]encounter.DiceReroll, len(component.Dice.Rerolls))
			for j, reroll := range component.Dice.Rerolls {
				trace.Rerolls[j] = encounter.DiceReroll{
					DieIndex: reroll.DieIndex, Before: reroll.Before, After: reroll.After,
					Source: encounter.RollSource{
						Name: reroll.Source.Name, Label: reroll.Source.Label, SourceID: reroll.Source.SourceID,
					},
				}
				if reroll.Source.Ref != nil {
					trace.Rerolls[j].Source.Ref = reroll.Source.Ref.String()
				}
			}
			// The keep record crosses with the dice it describes. Dropping it
			// here would put the pair of faces on the record with nothing to
			// say which one counted or why — the exact narrowing this slice
			// exists to remove (rpg-project#462 R1).
			trace.Keep = encounterDiceKeep(component.Dice.Keep)
			converted.Components[i].Dice = trace
		}
	}
	return converted
}

// encounterDiceKeep mirrors a keep record onto the persistence carrier, or nil
// when nothing touched the pool. Nil in, nil out: a straight roll has no rule
// to record, and inventing one would answer a question nobody asked.
func encounterDiceKeep(keep *dnd5eEvents.DiceKeep) *encounter.DiceKeep {
	if keep == nil {
		return nil
	}

	return &encounter.DiceKeep{
		Rule:    encounter.KeepRule(keep.Rule),
		Granted: encounterRollSources(keep.Granted),
		Imposed: encounterRollSources(keep.Imposed),
	}
}

func encounterRollSources(sources []dnd5eEvents.RollSource) []encounter.RollSource {
	if len(sources) == 0 {
		return nil
	}

	mapped := make([]encounter.RollSource, len(sources))
	for i, source := range sources {
		mapped[i] = encounter.RollSource{
			Name: source.Name, Label: source.Label, SourceID: source.SourceID,
		}
		if source.Ref != nil {
			mapped[i].Ref = source.Ref.String()
		}
	}
	return mapped
}

// breaks reads the collected facts back in the shape the record takes.
//
// The SAVE is matched rather than carried, because the two halves are produced
// by different things: the check is a machine's roll and the break is the
// condition's fact, and neither may reach into the other. A break reasoned
// "damage" is the consequence of the check that same caster just rolled, and
// that pairing is the whole of the match. Every other reason is ungated and
// gets a nil save, which is the honest zero — a caster at zero hit points, or
// one whose spell ran out, rolled nothing.
func (c *concentrationCollector) breaks(outcome Outcome) ([]encounter.ConcentrationBreak, error) {
	return breaksFrom(c.facts, followUpsOf(outcome))
}

// breaksFrom is [concentrationCollector.breaks] over the facts and follow-ups
// the caller chose, for [concentrationCollector.checksFrom]'s reason.
func breaksFrom(
	facts []dnd5eEvents.ConcentrationEndedEvent, checks []FollowUpOutcome,
) ([]encounter.ConcentrationBreak, error) {
	if len(facts) == 0 {
		return nil, nil
	}

	breaks := make([]encounter.ConcentrationBreak, 0, len(facts))
	for _, fact := range facts {
		removed, err := removedResults(fact)
		if err != nil {
			return nil, err
		}
		breaks = append(breaks, encounter.ConcentrationBreak{
			Caster:  encounter.MemberID(fact.CasterID),
			Spell:   encounter.SpellIdentity{Ref: fact.SpellRef, Name: fact.SpellName},
			Reason:  fact.Reason,
			Save:    checkFor(checks, fact),
			Removed: removed,
		})
	}

	return breaks, nil
}

// removedResults turns the addresses that came off into the same
// activation-result beat every other removal in the stack uses.
//
// The identity goes through the display catalog, which is a hard error on an
// unknown ref: a removal the record cannot name is a beat that reads as a
// random drop on somebody else's sheet, which is the exact thing the break beat
// exists to explain.
func removedResults(fact dnd5eEvents.ConcentrationEndedEvent) ([]encounter.ActivationResult, error) {
	results := make([]encounter.ActivationResult, 0, len(fact.Removed))
	for _, address := range fact.Removed {
		parsed, err := core.ParseString(address.ConditionRef)
		if err != nil {
			return nil, fmt.Errorf("resolution: %s ended and removed an unusable ref %q: %w",
				fact.SpellName, address.ConditionRef, err)
		}
		ref, name, err := activationConditionIdentity(parsed)
		if err != nil {
			return nil, fmt.Errorf("resolution: %s ended and removed %s from %q: %w",
				fact.SpellName, address.ConditionRef, address.MemberID, err)
		}
		results = append(results, encounter.ActivationResult{
			Kind: encounter.ResultConditionRemoved,
			Address: &encounter.ConditionAddress{
				MemberID: encounter.MemberID(address.MemberID), ConditionRef: ref, SourceID: address.SourceID,
			},
			Name: name, Reason: fact.Reason,
		})
	}

	return results, nil
}

// checkFor finds the roll this break was the consequence of, or nil.
func checkFor(
	checks []FollowUpOutcome, fact dnd5eEvents.ConcentrationEndedEvent,
) *encounter.CastSave {
	if fact.Reason != conditions.ConcentrationEndedDamage {
		return nil
	}
	for _, check := range checks {
		if check.SaverID != fact.CasterID || check.Save.Result == nil {
			continue
		}
		save := castSave(check)

		return &save
	}

	return nil
}

// followUpsOf reads the checks off whichever outcome ran them.
//
// A CLOSED SWITCH over the machines that apply damage, with no default beyond
// "none". The next damage source that calls reportDamage adds its arm here,
// which is the same one-line cost the shared step already asks of it.
//
// A sequence pays that cost by flattening: its blows each come back with their
// own checks, and a defender who held a spell through the first and lost it to
// the second is only recorded correctly if both reach this list.
func followUpsOf(outcome Outcome) []FollowUpOutcome {
	switch produced := outcome.(type) {
	case StrikeOutcome:
		return produced.FollowUps
	case CastOutcome:
		return produced.FollowUps
	case ContestOutcome:
		return produced.FollowUps
	case SequenceOutcome:
		return produced.FollowUps
	default:
		return nil
	}
}

// publishRemoval ends a hold by publishing the OWNER's removal, and only that.
//
// The owner does the rest: it hears the fact addressed to itself, strips its
// children with the reason that arrived, and publishes one concentration-ended
// fact naming every address that came off. Publishing the children here would
// take them off the list before the owner heard anything, and the fact would
// come out naming none of them.
//
// A Gather rather than a bare publish, for the reason every other publish in
// this package is one: the bus belongs to the driver.
func publishRemoval(removal *ConditionRemoval, next func(ImposedEffect) (Step, error)) Gather {
	return Gather{
		name: fmt.Sprintf("end %s on %s (%s)",
			removal.Owner.ConditionRef, removal.Owner.MemberID, removal.Reason),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(
				ctx, dnd5eEvents.ConditionRemovedEvent{
					MemberID:     removal.Owner.MemberID,
					ConditionRef: removal.Owner.ConditionRef,
					SourceID:     removal.Owner.SourceID,
					Reason:       removal.Reason,
				}); err != nil {
				return nil, fmt.Errorf("end %s on %q: %w",
					removal.Owner.ConditionRef, removal.Owner.MemberID, err)
			}

			return next(removalEffect(removal.Owner, removal.Reason))
		},
	}
}

// attributeToSteps moves one interaction's concentration record onto the SWING
// that caused it (Kirk's ruling, 2026-09-20).
//
// # Why per swing rather than per action
//
// A beat is a roll, and a concentration check is a roll a defender made
// against ONE blow. Folding a multiattack's checks onto its last beat would
// say a defender rolled twice at the end of an action rather than once against
// each swing — and worse, it would hide the ordering that matters: a hold that
// broke on the first blow was already gone when the second landed, and a
// record that reports both at the end cannot show that.
//
// # How a fact finds its swing
//
// By its own save. A break reasoned "damage" is the consequence of a check
// some caster just rolled, and that check is on exactly one step's follow-ups
// — the step whose damage forced it. [checkFor] is the same pairing the
// interaction-level projection already uses; all this does is ask it per step
// and claim the fact for the first step that answers.
//
// # The tail, and why it rides the last swing
//
// A hold can also end mid-sequence for a reason no swing's own check explains
// — a caster dropped to zero hit points by the blow, say, whose fact carries
// no save to match. Nothing in the record says which swing it happened on, and
// this package will not guess: those ride the LAST step, where "by the end of
// this action, this also ended" is true rather than invented. Claiming is
// first-come and one fact is claimed once, so nothing is ever recorded twice.
func (c *concentrationCollector) attributeToSteps(
	cast *Participants, sequence SequenceOutcome,
) (SequenceOutcome, error) {
	if len(sequence.Steps) == 0 {
		return sequence, nil
	}

	steps := make([]SequenceStepOutcome, len(sequence.Steps))
	copy(steps, sequence.Steps)
	claimed := make([]bool, len(c.facts))

	for i := range steps {
		followUps := steps[i].Strike.FollowUps

		made, err := c.checksFrom(cast, followUps)
		if err != nil {
			return SequenceOutcome{}, err
		}
		steps[i].ConcentrationChecks = made

		var caused []dnd5eEvents.ConcentrationEndedEvent
		for index, fact := range c.facts {
			if claimed[index] || checkFor(followUps, fact) == nil {
				continue
			}
			claimed[index] = true
			caused = append(caused, fact)
		}
		broke, err := breaksFrom(caused, followUps)
		if err != nil {
			return SequenceOutcome{}, err
		}
		steps[i].ConcentrationBreaks = broke
	}

	var unclaimed []dnd5eEvents.ConcentrationEndedEvent
	for index, fact := range c.facts {
		if !claimed[index] {
			unclaimed = append(unclaimed, fact)
		}
	}
	if len(unclaimed) > 0 {
		last := len(steps) - 1
		broke, err := breaksFrom(unclaimed, steps[last].Strike.FollowUps)
		if err != nil {
			return SequenceOutcome{}, err
		}
		steps[last].ConcentrationBreaks = append(steps[last].ConcentrationBreaks, broke...)
	}

	sequence.Steps = steps

	return sequence, nil
}
