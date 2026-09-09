// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// SaveInput describes one saving throw.
//
// Note what is absent: the modifier. The saver's own bonus is read off the
// sheet resolution loaded, because asking a caller for it would mean the caller
// had to load the participant too — and then "everything at the seam is data"
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

	// D20Source is the canonical rule or content source that caused this save.
	D20Source dnd5eEvents.RollSource

	// Roller rolls the save. It is required; resolution never substitutes hidden randomness.
	Roller dice.Roller

	// HasAdvantage and HasDisadvantage are advantage the *caller* already knows
	// about, before any effect has a say. Effects add their own during the
	// fold; these two do not replace that.
	HasAdvantage    bool
	HasDisadvantage bool
}

// SaveOutcome is what a saving throw produces.
type SaveOutcome struct {
	// Result is the roll, its total, whether it beat the DC — and the record of
	// *which* effects granted advantage, imposed disadvantage, or added a
	// bonus, in its source lists. Kept per-source because a total alone can say
	// "there was advantage" but not "Raging granted it", and the difference is
	// the whole point of a bus.
	//
	// This used to travel alongside a second field, Folded, carrying the chain
	// event resolution folded itself before rolling bus-free. The fold lives
	// inside [saves.MakeSavingThrow] now — see [NewSave] — and the result's
	// source lists are that fold's own record, so a second copy of it would be
	// the dual representation this repo's rules name as a defect.
	Result *saves.SavingThrowResult
}

func (SaveOutcome) isOutcome() {}

// NewSave returns the machine for a saving throw: one Gather, then Done.
//
// This is the smallest interaction that genuinely folds a chain, which is why
// it was the first. Custody of the fold has moved once since: this machine
// used to fold the SavingThrowChain itself and then roll through the rules
// package bus-free. rpg-toolkit#1382 removed the bus-free entry — for a real
// character nobody can prove no condition applies, so a save that skips the
// chain must not be expressible — which made "fold here, roll there" a
// double-fold waiting to happen. So the Gather now hands resolution's bus to
// [saves.MakeSavingThrow] and the chain folds exactly once, inside the rules
// entry that owns the arithmetic. The machine still never touches the bus
// (R6): the closure receives it from the driver, the same way every Gather
// does. The precedent is the damage chain's slice-1 shape — hand the lawful
// bus to the rules entry that requires one (see "The bus-effect tally" in this
// package's doc).
func NewSave(in *SaveInput) Machine {
	return &saveMachine{in: in}
}

type saveMachine struct {
	in *SaveInput
}

func describeRollContributions(
	cast *Participants, memberID string, kind dnd5eEvents.RollKind,
) ([]dnd5eEvents.DiceContribution, error) {
	var owner dnd5eEvents.RollConditionOwner
	if character, ok := cast.Character(memberID); ok {
		owner = character
	} else if monster, ok := cast.Monster(memberID); ok {
		owner = monster
	} else {
		return nil, fmt.Errorf("%w: %q", ErrNoSaver, memberID)
	}
	output, err := owner.DescribeRollContributions(&dnd5eEvents.DescribeRollContributionsInput{Kind: kind})
	if err != nil {
		return nil, err
	}
	if output == nil {
		return nil, fmt.Errorf("%w: %q returned no contribution description", ErrBadStep, memberID)
	}
	return append([]dnd5eEvents.DiceContribution(nil), output.Contributions...), nil
}

// Start reads the saver's own modifier off its sheet, then asks resolution to
// run the saving throw on its bus.
func (m *saveMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}

	modifier := 0
	if saver, ok := cast.Character(m.in.SaverID); ok {
		modifier = saver.GetSavingThrowModifier(m.in.Ability)
	} else if saver, ok := cast.Monster(m.in.SaverID); ok {
		modifier = saver.GetSavingThrowModifier(m.in.Ability)
	} else {
		return nil, fmt.Errorf("%w: %q", ErrNoSaver, m.in.SaverID)
	}
	if m.in.Roller == nil {
		return nil, fmt.Errorf("%w: a saving throw rolls with no roller", ErrNoRoller)
	}
	if m.in.D20Source.Ref == nil {
		return nil, fmt.Errorf("%w: a saving throw needs its d20 source", ErrBadAction)
	}

	return gatherSavingThrow(m.in, cast, modifier), nil
}

// gatherSavingThrow builds the Gather step that makes the saving throw.
//
// The closure is where the bus appears, and it is built here — in resolution —
// rather than by any machine. The rules entry it hands the bus to is the one
// place the SavingThrowChain fires: advantage cancellation, natural 1s and
// 20s, the totals, and the fold itself all stay where they live rather than
// being reimplemented on this side of the seam.
func gatherSavingThrow(in *SaveInput, cast *Participants, modifier int) Gather {
	return Gather{
		name: "saving throw",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			contributions, err := describeRollContributions(cast, in.SaverID, dnd5eEvents.RollKindSavingThrow)
			if err != nil {
				return nil, fmt.Errorf("describe saving throw contributions: %w", err)
			}
			result, err := saves.MakeSavingThrow(ctx, &saves.SavingThrowInput{
				Roller:    in.Roller,
				EventBus:  bus,
				SaverID:   in.SaverID,
				Cause:     in.Cause,
				Ability:   in.Ability,
				DC:        in.DC,
				Modifier:  modifier,
				D20Source: in.D20Source,
				ModifierSource: dnd5eEvents.RollSource{
					Ref: attackAbilityRef(in.Ability), Name: in.Ability.Display(),
				},
				Contributions:   contributions,
				HasAdvantage:    in.HasAdvantage,
				HasDisadvantage: in.HasDisadvantage,
			})
			if err != nil {
				return nil, fmt.Errorf("roll saving throw: %w", err)
			}

			return Done{Outcome: SaveOutcome{Result: result}}, nil
		},
	}
}
