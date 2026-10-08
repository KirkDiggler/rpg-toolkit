// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ShortRestInput is the persisted character to rest through the root D&D
// short-rest rules, and the hit dice it spends.
type ShortRestInput struct {
	// Character is a record, not a live sheet. The entry clones, strictly
	// loads and attaches it on its own transient interaction surface.
	Character *character.Data

	// HitDice is how many hit dice to spend. Zero rests without spending any
	// and still refills what a short rest refills; a negative count, or more
	// than remain, is the rulebook's refusal.
	HitDice int

	// Roller throws the hit dice: the world's roller, threaded to
	// character.ShortRest unchanged. Required when HitDice is above zero; no
	// default randomness is substituted for missing session wiring
	// (rpg-toolkit#1033).
	Roller dice.Roller

	// Others are the other sheets of the run the character rests in, every
	// one of them (R3). A concentration the rest ends takes its effects off
	// whoever carries them, and only a sheet passed in can hear that. A hold that
	// reaches a member left out refuses the rest with [ErrBadParticipant]
	// before anything ends.
	Others []Participant
}

// ShortRestOutput contains the rested record and the root rulebook's complete
// typed result.
type ShortRestOutput struct {
	// Character is an independently owned persistence snapshot taken after the
	// rest and before registration teardown.
	Character *character.Data

	// Result is character.ShortRest's answer by value. Result.Healing is the
	// trace: every hit die thrown, each naming the resting character as its
	// SourceID (rpg-project#462 R7), and the Constitution modifier for the
	// whole count. Nil when no die was spent.
	Result character.ShortRestOutput

	// ConcentrationBreaks are the holds the rest ended on the character, each
	// naming the effects that came off with it — the same record a Resolve
	// returns. An hour passes in a short rest and the encounter's clock jump
	// runs no durations, so the rest ends every hold whose remaining clock is
	// an hour or less (reason [conditions.ConcentrationEndedDuration]). The
	// host closes any runtime area the caster opened, as it does for a
	// Resolve's breaks.
	ConcentrationBreaks []encounter.ConcentrationBreak

	// Ended are the other effects the rest took off the character — those
	// that end on a rest of any kind — in the order they ended. A hold and its
	// effects are in [ShortRestOutput.ConcentrationBreaks], not here.
	Ended []encounter.ActivationResult

	// DirtyCharacters and DirtyMonsters are the [ShortRestInput.Others] the
	// rest changed, and only those: a sheet that lost an effect of an ended
	// hold. The resting character is [ShortRestOutput.Character].
	DirtyCharacters []*character.Data
	DirtyMonsters   []*monster.Data
}

// ShortRest strictly clones, loads, attaches, rests, snapshots, and tears
// down one character taking a short rest. It is the sibling of [LongRest].
//
// The rest is an hour. Before the rulebook's rest runs, the entry ends every
// concentration the character holds that cannot outlast that hour, through
// the hold's own removal — the one a recast or a failed check publishes — so
// the hold strips its effects from every sheet passed in. Then the rulebook's
// rest publishes the rest event, and every effect that ends on a rest ends
// itself.
//
// It installs no encounter world: whether the character may rest (not in a fight)
// and the hour the rest takes are the encounter's and the session's
// (rpg-project#542 R5). No runtime character or event bus crosses this data
// boundary.
//
// Returns [ErrNilInput], [ErrNoRoller] when dice are asked with no roller,
// [ErrBadParticipant] for a record that is not one sheet or a hold that
// reaches a member not passed in, or the rulebook's
// own refusal wrapped (a negative count, more dice than remain, a dead
// character), with nothing spent.
func ShortRest(ctx context.Context, in *ShortRestInput) (*ShortRestOutput, error) {
	return shortRestOn(ctx, in, newSurface(events.NewEventBus()))
}

// shortRestOn is ShortRest with its transient surface supplied for lifecycle
// tests. It stays unexported so a caller cannot retain the operation's bus.
func shortRestOn(
	ctx context.Context, in *ShortRestInput, surf *surface,
) (out *ShortRestOutput, err error) {
	// Teardown on every exit; errors.Join keeps operation and teardown
	// failures independently reachable.
	defer func() {
		tearErr := surf.teardown(ctx)
		if tearErr == nil {
			return
		}

		out = nil
		if err != nil {
			err = errors.Join(err, tearErr)
			return
		}
		err = fmt.Errorf("resolution: teardown: %w", tearErr)
	}()

	if in == nil {
		return nil, ErrNilInput
	}
	if in.HitDice > 0 && in.Roller == nil {
		return nil, fmt.Errorf("%w: a short rest rolls the hit dice it spends", ErrNoRoller)
	}

	one := Participant{Character: cloneCharacterData(in.Character)}
	if err := one.validate(); err != nil {
		return nil, err
	}
	participants := []Participant{one}
	for _, other := range in.Others {
		if err := other.validate(); err != nil {
			return nil, err
		}
		if other.ID() == one.ID() {
			return nil, fmt.Errorf("%w: %q rests and is also among the others", ErrBadParticipant, one.ID())
		}
		if other.Character != nil {
			other = Participant{Character: cloneCharacterData(other.Character)}
		}
		participants = append(participants, other)
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: participants,
		// The hit dice are thrown by in.Roller through the rulebook's own
		// operation; nothing attached is expected to roll during a rest.
		Roller: refusingRoller{},
		// Strict: this entry writes the sheet back, so a dropped effect would
		// be persisted as a deletion.
	})
	if err != nil {
		return nil, err
	}

	ctx = installTruth(ctx, nil, cast, nil)

	ch, ok := cast.Character(one.ID())
	if !ok {
		return nil, fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID())
	}

	// A rest the rulebook refuses (a negative count, more dice than remain, a
	// dead character) refuses after the holds were ended on this transient
	// surface, and returns no record: nothing that ended reaches a write.
	ends, err := collectConcentrationEnds(ctx, surf.inner)
	if err != nil {
		return nil, err
	}
	removals, err := collectRemovals(ctx, surf.inner, one.ID())
	if err != nil {
		return nil, errors.Join(err, ends.stop(ctx))
	}
	out, err = restAnHour(ctx, surf.inner, cast, ch, in, ends, removals)
	if stopErr := errors.Join(ends.stop(ctx), removals.stop(ctx)); stopErr != nil {
		return nil, errors.Join(err, stopErr)
	}
	if err != nil {
		return nil, err
	}

	dirty, err := dirtyCharacters(cast)
	if err != nil {
		return nil, err
	}
	for _, record := range dirty {
		if record.ID != one.ID() {
			out.DirtyCharacters = append(out.DirtyCharacters, record)
		}
	}
	out.DirtyMonsters = dirtyMonsters(cast)

	return out, nil
}

// restAnHour ends the holds the hour outlasts, rests, and snapshots the
// resting character with what ended.
func restAnHour(
	ctx context.Context, bus events.EventBus, cast *Participants, ch *character.Character, in *ShortRestInput,
	ends *concentrationCollector, removals *removalCollector,
) (*ShortRestOutput, error) {
	// The holds are read once, before any is ended: ending one removes it
	// from the sheet's condition list as it goes.
	var holds []*conditions.ConcentratingCondition
	for _, condition := range ch.GetConditions() {
		hold, ok := condition.(*conditions.ConcentratingCondition)
		if ok && hold.TurnEndsLeft <= encounter.RoundsPerHour {
			holds = append(holds, hold)
		}
	}

	// A hold strips its effects from every sheet that carries one, and only a
	// sheet in the cast hears it. An effect on a member who was not passed in
	// would be reported as removed and never written, so the rest refuses
	// before anything ends.
	for _, hold := range holds {
		for _, child := range hold.Children {
			_, isCharacter := cast.Character(child.MemberID)
			_, isMonster := cast.Monster(child.MemberID)
			if !isCharacter && !isMonster {
				return nil, fmt.Errorf("%w: %s held by %q reaches %q, who was not passed in",
					ErrBadParticipant, hold.SpellName, ch.GetID(), child.MemberID)
			}
		}
	}

	for _, hold := range holds {
		address := hold.ConditionAddress()
		if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
			MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID,
			Reason: conditions.ConcentrationEndedDuration,
		}); err != nil {
			return nil, fmt.Errorf("resolution: short rest %q: end %s: %w", ch.GetID(), hold.SpellName, err)
		}
	}

	result, err := ch.ShortRest(ctx, &character.ShortRestInput{HitDice: in.HitDice, Roller: in.Roller})
	if err != nil {
		return nil, fmt.Errorf("resolution: short rest %q: %w", ch.GetID(), err)
	}

	// Snapshot before the deferred teardown. Cleanup is never called: it would
	// erase conditions from the record about to cross the persistence boundary.
	rested, err := ch.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: short rest %q: %w", ch.GetID(), err)
	}

	breaks, err := breaksFrom(ends.facts, nil)
	if err != nil {
		return nil, err
	}
	ended, err := removals.endedOutside(ends.facts)
	if err != nil {
		return nil, err
	}

	return &ShortRestOutput{
		Character:           rested,
		Result:              *result,
		ConcentrationBreaks: breaks,
		Ended:               ended,
	}, nil
}

// removalCollector hears every effect that came off one member during an
// entry.
type removalCollector struct {
	memberID string
	facts    []dnd5eEvents.ConditionRemovedEvent
	stop     func(context.Context) error
}

// collectRemovals opens the collector on the entry's own bus.
func collectRemovals(ctx context.Context, bus events.EventBus, memberID string) (*removalCollector, error) {
	collector := &removalCollector{memberID: memberID}
	id, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.MemberID == collector.memberID {
				collector.facts = append(collector.facts, event)
			}
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("resolution: collect removals: %w", err)
	}
	collector.stop = func(inner context.Context) error { return bus.Unsubscribe(inner, id) }

	return collector, nil
}

// endedOutside is every removal heard that no ended hold accounts for — the
// hold itself and the effects it stripped are the breaks' to report — as the
// activation-result beat every removal in the stack uses.
func (c *removalCollector) endedOutside(
	holds []dnd5eEvents.ConcentrationEndedEvent,
) ([]encounter.ActivationResult, error) {
	owned := map[dnd5eEvents.ConditionAddress]bool{}
	for _, hold := range holds {
		for _, child := range hold.Removed {
			owned[child] = true
		}
	}

	var results []encounter.ActivationResult
	for _, fact := range c.facts {
		address := fact.Address()
		if owned[address] || address.ConditionRef == refs.Conditions.Concentrating().String() {
			continue
		}
		parsed, err := core.ParseString(address.ConditionRef)
		if err != nil {
			return nil, fmt.Errorf("resolution: rest removed an unusable ref %q: %w", address.ConditionRef, err)
		}
		ref, name, err := activationConditionIdentity(parsed)
		if err != nil {
			return nil, fmt.Errorf("resolution: rest removed %s from %q: %w", address.ConditionRef, address.MemberID, err)
		}
		results = append(results, encounter.ActivationResult{
			Kind: encounter.ResultConditionRemoved,
			Address: &encounter.ConditionAddress{
				MemberID:     encounter.MemberID(address.MemberID),
				ConditionKey: encounter.ConditionKey{ConditionRef: ref, SourceID: address.SourceID},
			},
			Name: name, Reason: fact.Reason,
		})
	}

	return results, nil
}
