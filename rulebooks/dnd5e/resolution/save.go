// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// SaveInput describes one saving throw.
//
// Note what is absent: the modifier. The saver's own bonus is read off the
// sheet resolution loaded, because asking a caller for it would mean the caller
// had to load the character too — and then "everything at the seam is data"
// would be true of the signature and false of the usage.
type SaveInput struct {
	// SaverID names the participant making the save.
	SaverID string

	// Ability is the ability score being tested.
	Ability abilities.Ability

	// DC is what the total must reach.
	DC int

	// Cause is what provoked the save, and is what effect predicates read to
	// decide whether they apply.
	Cause dnd5eEvents.SaveCause

	// Roller rolls the save. Nil takes the default roller.
	Roller dice.Roller

	// HasAdvantage and HasDisadvantage are advantage the *caller* already knows
	// about, before any effect has a say. Effects add their own during the
	// fold; these two do not replace that.
	HasAdvantage    bool
	HasDisadvantage bool
}

// SaveOutcome is what a saving throw produces.
type SaveOutcome struct {
	// Result is the roll, its total, and whether it beat the DC.
	Result *saves.SavingThrowResult

	// Folded is the chain event after every subscriber had its say — the record
	// of *which* effects granted advantage, imposed disadvantage, or added a
	// bonus. Kept because Result alone can say "there was advantage" but not
	// "Raging granted it", and the difference is the whole point of a bus.
	Folded *dnd5eEvents.SavingThrowChainEvent
}

func (SaveOutcome) isOutcome() {}

// NewSave returns the machine for a saving throw: one Gather, then Done.
//
// This is the smallest interaction that genuinely folds a chain, which is why
// it is the first. It is also not a mechanic being converted — saves already
// worked this way, with an optional bus threaded in from outside. What changes
// is custody: the bus is resolution's, and the machine never touches it.
func NewSave(in *SaveInput) Machine {
	return &saveMachine{in: in}
}

type saveMachine struct {
	in *SaveInput
}

// Start reads the saver's own modifier off its sheet, then asks resolution to
// fold the saving-throw chain.
func (m *saveMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}

	saver, ok := cast.Character(m.in.SaverID)
	if !ok {
		if _, isMonster := cast.Monster(m.in.SaverID); isMonster {
			return nil, fmt.Errorf("%w: %q", ErrSaverNotCharacter, m.in.SaverID)
		}

		return nil, fmt.Errorf("%w: %q", ErrNoSaver, m.in.SaverID)
	}

	modifier := saver.GetSavingThrowModifier(m.in.Ability)

	event := &dnd5eEvents.SavingThrowChainEvent{
		SaverID: m.in.SaverID,
		Ability: m.in.Ability,
		DC:      m.in.DC,
		Cause:   m.in.Cause,
	}

	return gatherSavingThrow(event, func(ctx context.Context, folded *dnd5eEvents.SavingThrowChainEvent) (Step, error) {
		return m.finish(ctx, modifier, folded)
	}), nil
}

// finish rolls the save with whatever the fold produced already folded in.
//
// It calls into the rules package with no bus, deliberately: the chain has
// already been folded by resolution, and passing a bus here would fold it a
// second time. Everything else about a saving throw — advantage cancellation,
// natural 1s and 20s, the totals — stays where it lives rather than being
// reimplemented on this side of the seam.
func (m *saveMachine) finish(
	ctx context.Context, modifier int, folded *dnd5eEvents.SavingThrowChainEvent,
) (Step, error) {
	result, err := saves.MakeSavingThrow(ctx, &saves.SavingThrowInput{
		Roller:          m.in.Roller,
		EventBus:        nil,
		SaverID:         m.in.SaverID,
		Cause:           m.in.Cause,
		Ability:         m.in.Ability,
		DC:              m.in.DC,
		Modifier:        modifier + folded.TotalBonus(),
		HasAdvantage:    m.in.HasAdvantage || folded.HasAdvantage(),
		HasDisadvantage: m.in.HasDisadvantage || folded.HasDisadvantage(),
	})
	if err != nil {
		return nil, fmt.Errorf("roll saving throw: %w", err)
	}

	return Done{Outcome: SaveOutcome{Result: result, Folded: folded}}, nil
}

// gatherSavingThrow builds the Gather step that folds the saving-throw chain.
//
// The closure is where the bus appears, and it is built here — in resolution —
// rather than by any machine. A machine names the fold it wants and receives a
// typed result; it never holds a bus, and cannot subscribe during its own
// resolution.
func gatherSavingThrow(
	event *dnd5eEvents.SavingThrowChainEvent,
	next func(context.Context, *dnd5eEvents.SavingThrowChainEvent) (Step, error),
) Gather {
	return Gather{
		name: "saving throw",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
			topic := dnd5eEvents.SavingThrowChain.On(bus)

			modified, err := topic.PublishWithChain(ctx, event, saveChain)
			if err != nil {
				return nil, fmt.Errorf("publish saving throw chain: %w", err)
			}

			folded, err := modified.Execute(ctx, event)
			if err != nil {
				return nil, fmt.Errorf("execute saving throw chain: %w", err)
			}

			return next(ctx, folded)
		},
	}
}
