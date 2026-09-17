// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package rolls

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// RollD20Input asks for one d20 pool: who rolls it, what rule it answers to,
// and every source that granted advantage or imposed disadvantage on it.
//
// Granted and Imposed are the sources themselves, not a pair of booleans. A
// keep record names the rules that met, and a boolean cannot (rpg-project#462
// R1/R2) — so whoever knows a rule applied is the one who names it here.
type RollD20Input struct {
	// Roller rolls the pool. Required; this package substitutes no hidden
	// randomness.
	Roller dice.Roller

	// Source identifies the d20 itself. Ref and Name say the RULE that caused
	// the roll — the approach, the ability, the weapon. SourceID says the
	// ENTITY that rolled it, and is required: whose die and whose rule are
	// different facts and both are kept (rpg-project#462 R7).
	Source dnd5eEvents.RollSource

	// Granted are the sources that granted advantage on this roll.
	Granted []dnd5eEvents.RollSource

	// Imposed are the sources that imposed disadvantage on this roll.
	Imposed []dnd5eEvents.RollSource
}

// RollD20Output is the settled face and the complete record of how it was got.
type RollD20Output struct {
	// Face is the face that counts: the kept one under advantage or
	// disadvantage, the only one otherwise. Natural 1 and natural 20 are read
	// off this.
	Face int

	// Trace records every face rolled, which was kept, and the keep rule that
	// decided it. The caller attaches it to a RollComponent under
	// RollD20Input.Source — the same source this entry validated.
	Trace *dnd5eEvents.DiceTrace
}

// RollD20 is the one place in this rulebook that knows advantage means two
// dice.
//
// It replaces the hand-written switch every d20 machine used to carry: attack,
// save and check each rolled their own pair, two kept both faces, one threw
// the second away, and none of them recorded WHY a face was kept. One roller,
// one trace shape, one keep record — so the fourth machine cannot invent a
// fourth way for the log to lie (rpg-project#462).
//
// # It is one pool, not the roll
//
// The roll is the [dnd5eEvents.RollCalculation], and this fills its first
// component and nothing else. Every other die keeps the door it has now:
// Bless's d4 is described before the roll and rolled beside the d20 by
// [ResolveContributions]; Bardic Inspiration and Guidance freeze the settled
// calculation, ask the player, and land as a new component on resume. Each is
// its own component with its own source. Advantage never touches those pools —
// Keep lives on the d20's trace only — so a later effect that adds a die adds
// a component through one of those doors and never changes this function.
//
// # The four shapes it can produce
//
//   - Neither list filled: 1d20, no kept indices, Keep nil. Zero value tells
//     the truth — nobody touched the pool.
//   - Granted only: 2d20, one kept index on the highest face, Keep advantage.
//   - Imposed only: 2d20, one kept index on the lowest face, Keep
//     disadvantage.
//   - Both filled: 1d20, no kept indices, Keep cancelled. The trace looks like
//     a straight roll except that the record says why — the case the log has
//     never been able to show.
//
// Every source is validated before any die is rolled: a roll this entry cannot
// describe is refused rather than made and recorded wrong.
func RollD20(ctx context.Context, input *RollD20Input) (*RollD20Output, error) {
	if input == nil {
		return nil, fmt.Errorf("roll d20 input is required")
	}
	if input.Roller == nil {
		return nil, fmt.Errorf("roll d20 roller is required")
	}
	if err := validateDiceSource(input.Source, "a d20 roll"); err != nil {
		return nil, err
	}
	for i, source := range input.Granted {
		if err := validateDiceSource(source, "an advantage source"); err != nil {
			return nil, fmt.Errorf("granted source %d: %w", i, err)
		}
	}
	for i, source := range input.Imposed {
		if err := validateDiceSource(source, "a disadvantage source"); err != nil {
			return nil, fmt.Errorf("imposed source %d: %w", i, err)
		}
	}

	granted := len(input.Granted) > 0
	imposed := len(input.Imposed) > 0

	// D&D 5e: advantage and disadvantage cancel, and one die is rolled. The
	// difference from a straight roll is the record, not the dice.
	if granted && imposed {
		face, err := rollOneD20(ctx, input.Roller)
		if err != nil {
			return nil, err
		}
		trace := straightD20Trace(face)
		trace.Keep = &dnd5eEvents.DiceKeep{
			Rule:    dnd5eEvents.KeepCancelled,
			Granted: cloneSources(input.Granted),
			Imposed: cloneSources(input.Imposed),
		}
		return &RollD20Output{Face: face, Trace: trace}, nil
	}

	if !granted && !imposed {
		face, err := rollOneD20(ctx, input.Roller)
		if err != nil {
			return nil, err
		}
		return &RollD20Output{Face: face, Trace: straightD20Trace(face)}, nil
	}

	faces, err := input.Roller.RollN(ctx, 2, 20)
	if err != nil {
		return nil, err
	}
	if len(faces) != 2 {
		return nil, fmt.Errorf("d20 roller returned %d faces, want 2", len(faces))
	}
	for i, face := range faces {
		if face < 1 || face > 20 {
			return nil, fmt.Errorf("d20 face %d is %d outside 1..20", i, face)
		}
	}

	keep := &dnd5eEvents.DiceKeep{Rule: dnd5eEvents.KeepAdvantage, Granted: cloneSources(input.Granted)}
	kept := 0
	if faces[1] > faces[0] {
		kept = 1
	}
	if imposed {
		keep = &dnd5eEvents.DiceKeep{Rule: dnd5eEvents.KeepDisadvantage, Imposed: cloneSources(input.Imposed)}
		kept = 0
		if faces[1] < faces[0] {
			kept = 1
		}
	}

	return &RollD20Output{
		Face: faces[kept],
		Trace: &dnd5eEvents.DiceTrace{
			Notation:      "2d20",
			DieSize:       20,
			OriginalRolls: append([]int(nil), faces...),
			FinalRolls:    append([]int(nil), faces...),
			KeptIndices:   []int{kept},
			Subtotal:      faces[kept],
			Keep:          keep,
		},
	}, nil
}

func rollOneD20(ctx context.Context, roller dice.Roller) (int, error) {
	face, err := roller.Roll(ctx, 20)
	if err != nil {
		return 0, err
	}
	if face < 1 || face > 20 {
		return 0, fmt.Errorf("d20 face is %d outside 1..20", face)
	}
	return face, nil
}

func straightD20Trace(face int) *dnd5eEvents.DiceTrace {
	return &dnd5eEvents.DiceTrace{
		Notation:      "1d20",
		DieSize:       20,
		OriginalRolls: []int{face},
		FinalRolls:    []int{face},
		Subtotal:      face,
	}
}

func cloneSources(sources []dnd5eEvents.RollSource) []dnd5eEvents.RollSource {
	if len(sources) == 0 {
		return nil
	}

	clones := make([]dnd5eEvents.RollSource, len(sources))
	for i, source := range sources {
		clones[i] = dnd5eEvents.CloneRollSource(source)
	}
	return clones
}
