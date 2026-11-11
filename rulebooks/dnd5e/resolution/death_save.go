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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// DeathSaveInput is the persisted character and authoritative d20 for one
// explicit tabletop death saving throw.
type DeathSaveInput struct {
	// Others supplies the remaining affected holders for linked effects. Empty
	// is valid only when the character's current application graph is local.
	Others []Participant
	// Character is a record, never a live sheet. DeathSave clones it before
	// strict loading so neither the rule nor its snapshot can alias the caller.
	Character *character.Data

	// Roller is required and is threaded to character.MakeDeathSave unchanged.
	// No default randomness is substituted for missing session wiring.
	Roller dice.Roller
}

// DeathSaveOutput contains the changed record and the root rulebook's complete
// typed result.
type DeathSaveOutput struct {
	// DirtyCharacters excludes Character; DirtyMonsters carries changed peer sheets.
	DirtyCharacters []*character.Data
	DirtyMonsters   []*monster.Data
	// Character is an independently owned persistence snapshot taken after the
	// accepted save and before registration teardown.
	Character *character.Data

	// Result is character.MakeDeathSave's answer by value. Its outcome and
	// continuation are provider vocabulary, never reclassified here.
	Result character.MakeDeathSaveOutput
}

// DeathSave strictly clones, loads, attaches, executes, snapshots, and tears
// down the character and every affected linked holder. It installs no encounter
// world: the caller supplies the affected sheets and d20, and a future effect
// that needs a world must add that fact to the input
// rather than receiving an invented empty room.
func DeathSave(ctx context.Context, in *DeathSaveInput) (*DeathSaveOutput, error) {
	return deathSaveOn(ctx, in, newSurface(events.NewEventBus()))
}

// deathSaveOn is DeathSave with its transient surface supplied for lifecycle
// tests. It stays unexported so a caller cannot retain the operation's bus.
func deathSaveOn(
	ctx context.Context, in *DeathSaveInput, surf *surface,
) (out *DeathSaveOutput, err error) {
	// Own teardown from the first instruction. Validation, strict loading,
	// partial attachment, provider refusal, and success all leave by this path;
	// errors.Join keeps operation and teardown failures independently reachable.
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
	if in.Roller == nil {
		return nil, fmt.Errorf("%w: a death save rolls a d20", ErrNoRoller)
	}

	one := Participant{Character: cloneCharacterData(in.Character)}
	if err := one.validate(); err != nil {
		return nil, err
	}

	participants, err := castWithOthers(one, in.Others)
	if err != nil {
		return nil, err
	}
	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: participants,
		Roller:       refusingRoller{},
		// The zero value is deliberately strict. This operation writes the
		// sheet back, so dropping an unreadable effect would persist deletion.
	})
	if err != nil {
		return nil, err
	}

	// The shared truth door, with the honest world answer for this record-only
	// operation: no encounter exists, while the attached cast does.
	ctx = installTruth(ctx, nil, cast, nil)

	ch, ok := cast.Character(one.ID())
	if !ok {
		return nil, fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID())
	}

	result, err := ch.MakeDeathSave(ctx, &character.MakeDeathSaveInput{Roller: in.Roller})
	if err != nil {
		return nil, fmt.Errorf("resolution: death save for %q: %w", one.ID(), err)
	}

	// Snapshot before deferred teardown. Cleanup is never called: registration
	// ownership belongs to the surface, and Cleanup would erase conditions from
	// the record about to cross the persistence boundary.
	changed, err := ch.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: death save for %q: %w", one.ID(), err)
	}

	out = &DeathSaveOutput{Character: changed, Result: *result, DirtyMonsters: dirtyMonsters(cast)}
	dirty, err := dirtyCharacters(cast)
	if err != nil {
		return nil, err
	}
	for _, sheet := range dirty {
		if sheet.ID != one.ID() {
			out.DirtyCharacters = append(out.DirtyCharacters, sheet)
		}
	}
	return out, nil
}
