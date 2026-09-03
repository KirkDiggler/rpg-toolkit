// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
)

// LongRestInput is the persisted character to recover through the root D&D
// long-rest rules.
type LongRestInput struct {
	// Character is a record, not a live sheet. The entry strictly loads and
	// attaches it on its own transient interaction surface.
	Character *character.Data
}

// LongRestOutput is the recovered character record ready for persistence.
type LongRestOutput struct {
	// Character is an independently owned snapshot of the attached sheet after
	// its long rest completed.
	Character *character.Data
}

// LongRest strictly loads and attaches one character, invokes the root D&D
// long-rest behavior, snapshots the result, and tears down every registration.
// No runtime character or event bus crosses this data boundary.
func LongRest(ctx context.Context, in *LongRestInput) (*LongRestOutput, error) {
	return longRestOn(ctx, in, newSurface(events.NewEventBus()))
}

// longRestOn is LongRest with the surface handed in so lifecycle tests can hold
// the real bus underneath. It stays unexported because callers must not supply
// or retain an interaction bus.
func longRestOn(
	ctx context.Context, in *LongRestInput, surf *surface,
) (out *LongRestOutput, err error) {
	// The surface is torn down on every exit, including validation, load,
	// attach, and rule failures. When both the operation and teardown fail,
	// keep both reachable by errors.Is as Resolve does.
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

	// Clone at the record boundary. character.Load owns the rules but retains
	// references to a few immutable-on-load maps and slices; putting an owned
	// copy through the cast guarantees neither the rest nor its output aliases
	// the caller's record.
	one := Participant{Character: cloneCharacterData(in.Character)}
	if err := one.validate(); err != nil {
		return nil, err
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants:   []Participant{one},
		Roller:         refusingRoller{},
		DropUnreadable: false,
	})
	if err != nil {
		return nil, err
	}

	// The same truth door as every other resolution entry, with the honest
	// answer for this record-only operation: a cast of one and no world.
	ctx = installTruth(ctx, nil, cast)

	ch, ok := cast.Character(one.ID())
	if !ok {
		return nil, fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID())
	}

	if err := ch.LongRest(ctx); err != nil {
		return nil, fmt.Errorf("resolution: long rest %q: %w", one.ID(), err)
	}

	// Snapshot before the deferred teardown. Character.Cleanup is deliberately
	// never called: resolution owns registration teardown, and Cleanup would
	// erase conditions before this persistence snapshot.
	rested := ch.ToData()

	return &LongRestOutput{Character: rested}, nil
}

// cloneCharacterData returns a deep-enough record copy for every mutable field
// on character.Data. The copy is the only record given to the live sheet, so
// shallow references retained by the root loader cannot reach the caller.
func cloneCharacterData(in *character.Data) *character.Data {
	if in == nil {
		return nil
	}

	out := *in
	out.Appearance = customization.CloneAppearance(in.Appearance)
	out.AbilityScores = maps.Clone(in.AbilityScores)
	if in.DeathSaveState != nil {
		state := *in.DeathSaveState
		out.DeathSaveState = &state
	}
	out.Skills = maps.Clone(in.Skills)
	out.SavingThrows = maps.Clone(in.SavingThrows)
	out.Languages = slices.Clone(in.Languages)
	out.ArmorProficiencies = slices.Clone(in.ArmorProficiencies)
	out.WeaponProficiencies = slices.Clone(in.WeaponProficiencies)
	out.ToolProficiencies = slices.Clone(in.ToolProficiencies)
	out.Inventory = slices.Clone(in.Inventory)
	out.EquipmentSlots = maps.Clone(in.EquipmentSlots)
	out.SpellSlots = maps.Clone(in.SpellSlots)
	out.ClassResources = maps.Clone(in.ClassResources)
	out.Resources = maps.Clone(in.Resources)
	out.Features = cloneRawMessages(in.Features)
	out.Conditions = cloneRawMessages(in.Conditions)
	if in.ActionEconomy != nil {
		economy := *in.ActionEconomy
		economy.Granted = maps.Clone(in.ActionEconomy.Granted)
		out.ActionEconomy = &economy
	}

	return &out
}

func cloneRawMessages(in []json.RawMessage) []json.RawMessage {
	if in == nil {
		return nil
	}

	out := make([]json.RawMessage, len(in))
	for i := range in {
		out[i] = slices.Clone(in[i])
	}
	return out
}
