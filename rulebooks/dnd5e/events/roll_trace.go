// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// RollSource identifies and describes the rulebook-owned source of a roll fact.
// Label optionally describes the source's role within its calculation.
type RollSource struct {
	Ref   *core.Ref
	Name  string
	Label string
}

// DiceReroll records one ordered replacement of a die face and its source.
type DiceReroll struct {
	DieIndex int
	Before   int
	After    int
	Source   RollSource
}

// DiceTrace records the original and final faces of one homogeneous dice pool.
// An empty KeptIndices means every final face contributes to Subtotal.
type DiceTrace struct {
	Notation      string
	DieSize       int
	OriginalRolls []int
	Rerolls       []DiceReroll
	FinalRolls    []int
	KeptIndices   []int
	Subtotal      int
}

// RollComponent records dice, a modifier, or both from one source.
// A non-nil Modifier participates even when its value is zero.
type RollComponent struct {
	Source   RollSource
	Dice     *DiceTrace
	Modifier *int
}

// RollCalculation records the sourced components and authoritative total of a roll.
type RollCalculation struct {
	Components []RollComponent
	Total      int
}

// CloneRollCalculation returns a deep clone of calculation, or nil when calculation is nil.
func CloneRollCalculation(calculation *RollCalculation) *RollCalculation {
	if calculation == nil {
		return nil
	}

	clone := &RollCalculation{
		Components: cloneRollComponents(calculation.Components),
		Total:      calculation.Total,
	}
	return clone
}

func cloneRollComponents(components []RollComponent) []RollComponent {
	if components == nil {
		return nil
	}

	clones := make([]RollComponent, len(components))
	for i, component := range components {
		clones[i] = RollComponent{
			Source:   cloneRollSource(component.Source),
			Dice:     cloneDiceTrace(component.Dice),
			Modifier: cloneInt(component.Modifier),
		}
	}
	return clones
}

func cloneDiceTrace(trace *DiceTrace) *DiceTrace {
	if trace == nil {
		return nil
	}

	return &DiceTrace{
		Notation:      trace.Notation,
		DieSize:       trace.DieSize,
		OriginalRolls: cloneInts(trace.OriginalRolls),
		Rerolls:       cloneDiceRerolls(trace.Rerolls),
		FinalRolls:    cloneInts(trace.FinalRolls),
		KeptIndices:   cloneInts(trace.KeptIndices),
		Subtotal:      trace.Subtotal,
	}
}

func cloneInts(values []int) []int {
	if values == nil {
		return nil
	}

	clones := make([]int, len(values))
	copy(clones, values)
	return clones
}

func cloneDiceRerolls(rerolls []DiceReroll) []DiceReroll {
	if rerolls == nil {
		return nil
	}

	clones := make([]DiceReroll, len(rerolls))
	for i, reroll := range rerolls {
		clones[i] = reroll
		clones[i].Source = cloneRollSource(reroll.Source)
	}
	return clones
}

func cloneRollSource(source RollSource) RollSource {
	return RollSource{
		Ref:   cloneRef(source.Ref),
		Name:  source.Name,
		Label: source.Label,
	}
}

func cloneRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}

	clone := *ref
	return &clone
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}

	clone := *value
	return &clone
}

// ValidateRollCalculation verifies source presence and the structural and arithmetic
// consistency of a calculation. It replays recorded rerolls but does not decide whether
// any D&D rule was eligible to cause them.
func ValidateRollCalculation(calculation *RollCalculation) error {
	if calculation == nil {
		return fmt.Errorf("roll calculation is required")
	}
	if len(calculation.Components) == 0 {
		return fmt.Errorf("roll calculation requires at least one component")
	}

	total := 0
	for i, component := range calculation.Components {
		if err := validateRollComponent(component); err != nil {
			return fmt.Errorf("roll component %d: %w", i, err)
		}
		if component.Dice != nil {
			total += component.Dice.Subtotal
		}
		if component.Modifier != nil {
			total += *component.Modifier
		}
	}

	if total != calculation.Total {
		return fmt.Errorf("roll calculation total is %d, want %d", calculation.Total, total)
	}
	return nil
}

func validateRollComponent(component RollComponent) error {
	if err := validateRollSource(component.Source); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if component.Dice == nil && component.Modifier == nil {
		return fmt.Errorf("must contain dice, a modifier, or both")
	}
	if component.Dice == nil {
		return nil
	}
	return validateDiceTrace(component.Dice)
}

func validateRollSource(source RollSource) error {
	if source.Ref == nil {
		return fmt.Errorf("ref is required")
	}
	if err := source.Ref.IsValid(); err != nil {
		return fmt.Errorf("ref is invalid: %w", err)
	}
	if strings.TrimSpace(source.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

func validateDiceTrace(trace *DiceTrace) error {
	if trace.DieSize <= 0 {
		return fmt.Errorf("die size must be positive")
	}

	// dice.ParseNotation normalizes signed notation (e.g. "-d6" parses as one
	// positive d6 and composite terms drop negative parts), but a DiceTrace
	// records one unsigned homogeneous pool; modifiers live on the component.
	if strings.ContainsAny(trace.Notation, "+-") {
		return fmt.Errorf(
			"dice notation %q must be unsigned homogeneous dice without signed or composite terms",
			trace.Notation,
		)
	}

	pool, err := dice.ParseNotation(trace.Notation)
	if err != nil {
		return fmt.Errorf("invalid dice notation %q: %w", trace.Notation, err)
	}
	if len(trace.OriginalRolls) == 0 {
		return fmt.Errorf("original rolls must contain at least one face")
	}

	expectedNotation := dice.SimplePool(len(trace.OriginalRolls), trace.DieSize, 0).Notation()
	if pool.Notation() != expectedNotation {
		return fmt.Errorf(
			"dice notation %q does not describe %d dice with die size %d",
			trace.Notation,
			len(trace.OriginalRolls),
			trace.DieSize,
		)
	}
	if len(trace.FinalRolls) != len(trace.OriginalRolls) {
		return fmt.Errorf(
			"final rolls contain %d faces, want %d",
			len(trace.FinalRolls),
			len(trace.OriginalRolls),
		)
	}

	if err := validateFaces("original", trace.OriginalRolls, trace.DieSize); err != nil {
		return err
	}
	if err := validateFaces("final", trace.FinalRolls, trace.DieSize); err != nil {
		return err
	}
	if err := validateRerolls(trace); err != nil {
		return err
	}

	return validateSubtotal(trace)
}

func validateFaces(kind string, faces []int, dieSize int) error {
	for i, face := range faces {
		if face < 1 || face > dieSize {
			return fmt.Errorf("%s roll %d has face %d outside 1..%d", kind, i, face, dieSize)
		}
	}
	return nil
}

func validateRerolls(trace *DiceTrace) error {
	replayed := slices.Clone(trace.OriginalRolls)
	for i, reroll := range trace.Rerolls {
		if err := validateRollSource(reroll.Source); err != nil {
			return fmt.Errorf("reroll %d source: %w", i, err)
		}
		if reroll.DieIndex < 0 || reroll.DieIndex >= len(replayed) {
			return fmt.Errorf("reroll %d has die index %d outside rolls", i, reroll.DieIndex)
		}
		if replayed[reroll.DieIndex] != reroll.Before {
			return fmt.Errorf(
				"reroll %d before is %d, want current face %d",
				i,
				reroll.Before,
				replayed[reroll.DieIndex],
			)
		}
		if reroll.After < 1 || reroll.After > trace.DieSize {
			return fmt.Errorf("reroll %d after face %d is outside 1..%d", i, reroll.After, trace.DieSize)
		}
		replayed[reroll.DieIndex] = reroll.After
	}

	if !slices.Equal(replayed, trace.FinalRolls) {
		return fmt.Errorf("final rolls %v do not match replayed rolls %v", trace.FinalRolls, replayed)
	}
	return nil
}

func validateSubtotal(trace *DiceTrace) error {
	if len(trace.KeptIndices) == 0 {
		subtotal := 0
		for _, face := range trace.FinalRolls {
			subtotal += face
		}
		return compareSubtotal(trace.Subtotal, subtotal)
	}

	subtotal := 0
	seen := make(map[int]struct{}, len(trace.KeptIndices))
	for _, index := range trace.KeptIndices {
		if index < 0 || index >= len(trace.FinalRolls) {
			return fmt.Errorf("kept index %d is outside final rolls", index)
		}
		if _, exists := seen[index]; exists {
			return fmt.Errorf("kept index %d is duplicated", index)
		}
		seen[index] = struct{}{}
		subtotal += trace.FinalRolls[index]
	}
	return compareSubtotal(trace.Subtotal, subtotal)
}

func compareSubtotal(got, want int) error {
	if got != want {
		return fmt.Errorf("dice subtotal is %d, want %d", got, want)
	}
	return nil
}
