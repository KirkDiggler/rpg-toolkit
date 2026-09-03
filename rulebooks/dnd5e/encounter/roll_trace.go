// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// The types in this file are the encounter's neutral mirror of the root D&D
// roll-trace primitives (rulebooks/dnd5e/events). This composition cannot
// import that package — C1 keeps the rulebook's rule facts OUT of a module
// whose go.mod has no rulebook dependency — so persistence carries the same
// fields, in the same order, with the same presence semantics, and every
// *core.Ref reduced to its canonical module:type:id string.
//
// Validation here is STRUCTURAL AND ARITHMETIC ONLY: canonical ref syntax,
// notation and cardinality, face ranges, ordered reroll replay against the
// current face, final rolls, kept indices, subtotal, and total. It never
// decides whether a D&D rule was eligible to produce any of it — a trace is
// refused when it cannot have happened as told, never when a rule the
// composition does not know would not have allowed it.

// RollSource identifies and describes the rulebook-owned source of a roll
// fact. Ref is the canonical module:type:id string of the content that
// produced the fact; Name is its display name. Label optionally describes the
// source's role within its calculation ("Fighter level").
type RollSource struct {
	Ref   string `json:"ref"`
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
}

// DiceReroll records one ordered replacement of a die face and its source.
// Before must equal the die's current face under the rerolls that precede it.
type DiceReroll struct {
	DieIndex int        `json:"die_index"`
	Before   int        `json:"before"`
	After    int        `json:"after"`
	Source   RollSource `json:"source"`
}

// DiceTrace records the original and final faces of one homogeneous dice
// pool. An empty KeptIndices means every final face contributes to Subtotal.
type DiceTrace struct {
	Notation      string       `json:"notation"`
	DieSize       int          `json:"die_size"`
	OriginalRolls []int        `json:"original_rolls"`
	Rerolls       []DiceReroll `json:"rerolls,omitempty"`
	FinalRolls    []int        `json:"final_rolls"`
	KeptIndices   []int        `json:"kept_indices,omitempty"`
	Subtotal      int          `json:"subtotal"`
}

// RollComponent records dice, a modifier, or both from one source.
// A non-nil Modifier participates even when its value is zero.
type RollComponent struct {
	Source   RollSource `json:"source"`
	Dice     *DiceTrace `json:"dice,omitempty"`
	Modifier *int       `json:"modifier,omitempty"`
}

// RollCalculation records the sourced components and authoritative total of
// a roll, in the order the rulebook produced them.
type RollCalculation struct {
	Components []RollComponent `json:"components"`
	Total      int             `json:"total"`
}

// ValidateRollCalculation verifies source presence and the structural and
// arithmetic consistency of a calculation. It replays recorded rerolls but
// does not decide whether any D&D rule was eligible to cause them.
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
	if source.Ref == "" {
		return fmt.Errorf("ref is required")
	}
	if _, err := core.ParseString(source.Ref); err != nil {
		return fmt.Errorf("ref %q is invalid: %w", source.Ref, err)
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
