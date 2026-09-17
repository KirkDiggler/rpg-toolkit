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
// source's role within its calculation ("Fighter level"). SourceID optionally
// names the entity responsible for the fact and is the sole contributor-ID
// field in the calculation graph.
type RollSource struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	SourceID string `json:"source_id,omitempty"`
}

// DiceReroll records one ordered replacement of a die face and its source.
// Before must equal the die's current face under the rerolls that precede it.
type DiceReroll struct {
	DieIndex int        `json:"die_index"`
	Before   int        `json:"before"`
	After    int        `json:"after"`
	Source   RollSource `json:"source"`
}

// KeepRule names the rule that decided which faces of a dice pool count. The
// mirror of the rulebook's own spelling; these values are the wire words.
type KeepRule string

const (
	// KeepAdvantage keeps the highest face of a pool rolled with advantage.
	KeepAdvantage KeepRule = "advantage"

	// KeepDisadvantage keeps the lowest face of a pool rolled with disadvantage.
	KeepDisadvantage KeepRule = "disadvantage"

	// KeepCancelled records granted and imposed sources meeting: the pool was
	// rolled straight and every face counts, and the record says why.
	KeepCancelled KeepRule = "cancelled"
)

// DiceKeep records the rule that decided KeptIndices, and who brought it. Nil
// when nothing touched the pool: a straight roll keeps every face.
//
// A CANCELLATION IS THE CASE THIS RECORD EXISTS FOR. Its trace is identical to
// a straight roll's — one die, no kept indices — so without the record a
// persisted beat cannot tell a player that two rules met over their die
// (rpg-project#462 R2).
//
// Its sources name entities, unlike the general RollSource contract above: a
// keep rule was BROUGHT by somebody, and a record that cannot say who is a
// record of nothing. That is a shape requirement of this type, not a D&D rule
// this module has learned.
type DiceKeep struct {
	Rule    KeepRule     `json:"rule"`
	Granted []RollSource `json:"granted,omitempty"`
	Imposed []RollSource `json:"imposed,omitempty"`
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

	// Keep records the rule that decided KeptIndices. Nil when nothing
	// touched the pool.
	Keep *DiceKeep `json:"keep,omitempty"`
}

// RollComponent records dice, a modifier, or both from one source.
// A non-nil Modifier participates even when its value is zero.
type RollComponent struct {
	Source   RollSource `json:"source"`
	Dice     *DiceTrace `json:"dice,omitempty"`
	Modifier *int       `json:"modifier,omitempty"`

	// SubtractDice subtracts Dice.Subtotal while leaving every physical face
	// positive. A fixed Modifier on the same component remains additive.
	SubtractDice bool `json:"subtract_dice,omitempty"`
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
			if component.SubtractDice {
				total -= component.Dice.Subtotal
			} else {
				total += component.Dice.Subtotal
			}
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

// validateRecordedD20 checks a scalar d20 summary against the authoritative
// calculation that carries it. The first component is the operation-owned d20
// pool by the shared calculation contract; this validates shape and agreement,
// not success, failure, or any D&D natural-face policy.
func validateRecordedD20(calculation *RollCalculation, roll, total int) error {
	if err := validateRecordedTotal(calculation, total); err != nil {
		return err
	}
	if first := calculation.Components[0]; first.Dice.Subtotal != roll {
		return fmt.Errorf("roll summary is %d, want calculation d20 subtotal %d", roll, first.Dice.Subtotal)
	}
	return nil
}

// validateRecordedTotal is [validateRecordedD20] for a beat that carries the
// total but no separate roll summary — a social attempt, an unlock. The d20
// shape is still checked: the first component is the operation's own pool by
// the shared calculation contract, which is what makes its keep record the
// place a reader looks for advantage and disadvantage.
func validateRecordedTotal(calculation *RollCalculation, total int) error {
	if err := ValidateRollCalculation(calculation); err != nil {
		return err
	}
	first := calculation.Components[0]
	if first.Dice == nil || first.Dice.DieSize != 20 || first.SubtractDice {
		return fmt.Errorf("first component must be an additive d20 pool")
	}
	if calculation.Total != total {
		return fmt.Errorf("total summary is %d, want calculation total %d", total, calculation.Total)
	}
	return nil
}

func validateRollComponent(component RollComponent) error {
	if err := validateRollComponentData(component); err != nil {
		return err
	}
	if component.Dice == nil && component.Modifier == nil {
		return fmt.Errorf("must contain dice, a modifier, or both")
	}
	if component.Dice != nil && strings.TrimSpace(component.Source.SourceID) == "" {
		// EVERY DICE POOL NAMES THE ENTITY WHOSE RULE THREW IT
		// (rpg-project#462 R7). This is PROVENANCE, not a 5e eligibility
		// question: the same presence rule this file already enforces on
		// subtractive dice above and on every keep source below, applied to
		// the pools those two were carved out of.
		//
		// It lives HERE rather than in validateRollComponentData because the
		// damage container validates through that one and carries components
		// whose provenance rules are its own. This is the path
		// ValidateRollCalculation takes, which is the path a persisted
		// calculation is decoded through — so an anonymous pool cannot
		// survive a round trip and reach a client with nobody behind it.
		return fmt.Errorf("dice source id is required")
	}
	return nil
}

// validateRollComponentData is the one neutral source/operator/trace contract
// shared by every container that persists a RollComponent. Each container
// separately decides which absent roll facts it permits.
func validateRollComponentData(component RollComponent) error {
	if err := validateRollSource(component.Source); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if component.SubtractDice && component.Dice == nil {
		return fmt.Errorf("cannot subtract dice without dice")
	}
	if component.SubtractDice && strings.TrimSpace(component.Source.SourceID) == "" {
		return fmt.Errorf("subtractive dice source id is required")
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
	if err := validateSubtotal(trace); err != nil {
		return err
	}

	return validateDiceKeep(trace)
}

// validateDiceKeep refuses a keep record that does not describe the pool it
// sits on. Structural and arithmetic only, like everything else here: it
// checks that the record and the dice agree — advantage kept the highest of at
// least two faces, disadvantage the lowest, a cancellation kept none and names
// both sides — never whether a D&D rule was eligible to produce either.
func validateDiceKeep(trace *DiceTrace) error {
	keep := trace.Keep
	if keep == nil {
		return nil
	}

	for i, source := range keep.Granted {
		if err := validateKeepSource(source); err != nil {
			return fmt.Errorf("keep granted source %d: %w", i, err)
		}
	}
	for i, source := range keep.Imposed {
		if err := validateKeepSource(source); err != nil {
			return fmt.Errorf("keep imposed source %d: %w", i, err)
		}
	}

	switch keep.Rule {
	case KeepAdvantage:
		return validateKeptExtreme(trace, keep.Rule, len(keep.Granted), len(keep.Imposed),
			func(a, b int) int { return max(a, b) })
	case KeepDisadvantage:
		return validateKeptExtreme(trace, keep.Rule, len(keep.Imposed), len(keep.Granted),
			func(a, b int) int { return min(a, b) })
	case KeepCancelled:
		if len(keep.Granted) == 0 || len(keep.Imposed) == 0 {
			return fmt.Errorf(
				"keep rule %q requires both a granted and an imposed source, got %d and %d",
				keep.Rule, len(keep.Granted), len(keep.Imposed),
			)
		}
		if len(trace.FinalRolls) != 1 {
			// RAW ROLLS ONE DIE WHEN THE TWO RULES MEET, and so do we. A
			// cancellation recorded over a pair is not a cancellation: with no
			// kept indices every face counts toward the subtotal, so the
			// record would say "cancelled" over a pool that added both dice
			// together — an advantage or disadvantage whose keep decision went
			// unrecorded, wearing the one label that hides it.
			return fmt.Errorf(
				"keep rule %q rolls one die, got %d faces", keep.Rule, len(trace.FinalRolls))
		}
		if len(trace.KeptIndices) != 0 {
			return fmt.Errorf("keep rule %q keeps no face, got %d kept", keep.Rule, len(trace.KeptIndices))
		}
		return nil
	default:
		return fmt.Errorf("keep rule %q is not a keep rule", keep.Rule)
	}
}

// validateKeptExtreme checks the shape both one-sided keep rules share: the
// rule was brought by at least one source, nothing opposed it, the pool holds
// more than one face, exactly one was kept, and the kept face is the extreme
// the rule names.
func validateKeptExtreme(
	trace *DiceTrace, rule KeepRule, bringing, opposing int, extreme func(int, int) int,
) error {
	if bringing == 0 {
		return fmt.Errorf("keep rule %q records no source that brought it", rule)
	}
	if opposing != 0 {
		return fmt.Errorf("keep rule %q records an opposing source: that is a cancellation", rule)
	}
	if len(trace.FinalRolls) < 2 {
		return fmt.Errorf("keep rule %q requires at least 2 faces, got %d", rule, len(trace.FinalRolls))
	}
	if len(trace.KeptIndices) != 1 {
		return fmt.Errorf("keep rule %q keeps exactly one face, got %d", rule, len(trace.KeptIndices))
	}

	index := trace.KeptIndices[0]
	if index < 0 || index >= len(trace.FinalRolls) {
		return fmt.Errorf("kept index %d is outside final rolls", index)
	}
	want := trace.FinalRolls[0]
	for _, face := range trace.FinalRolls[1:] {
		want = extreme(want, face)
	}
	if trace.FinalRolls[index] != want {
		return fmt.Errorf("keep rule %q kept face %d, want %d", rule, trace.FinalRolls[index], want)
	}
	return nil
}

func validateKeepSource(source RollSource) error {
	if err := validateRollSource(source); err != nil {
		return err
	}
	if strings.TrimSpace(source.SourceID) == "" {
		return fmt.Errorf("source id is required: a keep rule is brought by an entity")
	}
	return nil
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
