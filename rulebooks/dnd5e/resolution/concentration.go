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
	if len(c.facts) == 0 {
		return nil, nil
	}

	checks := followUpsOf(outcome)
	breaks := make([]encounter.ConcentrationBreak, 0, len(c.facts))
	for _, fact := range c.facts {
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
			Kind:   encounter.ResultConditionRemoved,
			Target: encounter.MemberID(address.MemberID),
			Ref:    ref,
			Name:   name,
			Reason: fact.Reason,
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
		result := check.Save.Result

		return &encounter.CastSave{
			Saver:     encounter.MemberID(check.SaverID),
			Ability:   string(check.Ability),
			Roll:      result.Roll,
			Total:     result.Total,
			DC:        result.DC,
			Succeeded: result.Success,
		}
	}

	return nil
}

// followUpsOf reads the checks off whichever outcome ran them.
//
// A CLOSED SWITCH over the two machines that apply damage, with no default
// beyond "none". The third damage source that calls reportDamage adds its arm
// here, which is the same one-line cost the shared step already asks of it.
func followUpsOf(outcome Outcome) []FollowUpOutcome {
	switch produced := outcome.(type) {
	case StrikeOutcome:
		return produced.FollowUps
	case CastOutcome:
		return produced.FollowUps
	case ContestOutcome:
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
					Reason:       removal.Reason,
				}); err != nil {
				return nil, fmt.Errorf("end %s on %q: %w",
					removal.Owner.ConditionRef, removal.Owner.MemberID, err)
			}

			return next(removalEffect(removal.Owner, removal.Reason))
		},
	}
}
