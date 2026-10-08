// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
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
}

// ShortRest strictly clones, loads, attaches, rests, snapshots, and tears
// down one character taking a short rest. It is the sibling of [LongRest] and
// installs no encounter world: whether the character may rest (not in a fight)
// and the hour the rest takes are the encounter's and the session's
// (rpg-project#542 R5). No runtime character or event bus crosses this data
// boundary.
//
// Returns [ErrNilInput], [ErrNoRoller] when dice are asked with no roller,
// [ErrBadParticipant] for a record that is not one sheet, or the rulebook's
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

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: []Participant{one},
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

	result, err := ch.ShortRest(ctx, &character.ShortRestInput{HitDice: in.HitDice, Roller: in.Roller})
	if err != nil {
		return nil, fmt.Errorf("resolution: short rest %q: %w", one.ID(), err)
	}

	// Snapshot before the deferred teardown. Cleanup is never called: it would
	// erase conditions from the record about to cross the persistence boundary.
	rested, err := ch.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: short rest %q: %w", one.ID(), err)
	}

	return &ShortRestOutput{Character: rested, Result: *result}, nil
}
