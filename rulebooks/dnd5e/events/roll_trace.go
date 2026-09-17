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
// Label optionally describes the source's role within its calculation. SourceID
// optionally identifies the entity that contributed the fact; it is the sole
// calculation home for contributor entity identity.
type RollSource struct {
	Ref      *core.Ref
	Name     string
	Label    string
	SourceID string
}

// DiceContribution describes an unresolved homogeneous dice modification.
// Its SourceID names the responsible entity uniformly for every contribution;
// base dice and fixed components retain RollSource's broader optional provenance.
// Dice is unsigned notation; Subtract records the operator without encoding a
// sign into the pool or producing a face before the owning roll path evaluates it.
type DiceContribution struct {
	Source   RollSource
	Dice     string
	Subtract bool
}

// RollKind identifies the operation whose conditions are being consulted.
type RollKind string

const (
	// RollKindAttack identifies an attack roll.
	RollKindAttack RollKind = "attack"
	// RollKindSavingThrow identifies a saving throw.
	RollKindSavingThrow RollKind = "saving_throw"
)

// DescribeRollContributionsInput asks for condition contributions to one roll kind.
type DescribeRollContributionsInput struct {
	Kind RollKind
}

// DescribeRollContributionsOutput contains unresolved selected contributions.
type DescribeRollContributionsOutput struct {
	Contributions []DiceContribution
}

// RollConditionOwner exposes the selected contributions from a recipient's
// current ordered condition collection.
type RollConditionOwner interface {
	DescribeRollContributions(
		input *DescribeRollContributionsInput,
	) (*DescribeRollContributionsOutput, error)
}

// RollContributionMetadataOutput declares whether a provider applies to a roll
// kind and the stacking group in which only the oldest provider contributes.
type RollContributionMetadataOutput struct {
	Applicable bool
	Group      string
}

// RollContributionProvider is the optional capability implemented by a
// condition that can describe a dice contribution. The condition owns both its
// applicability/stacking rule and the resulting description.
type RollContributionProvider interface {
	RollContributionMetadata(input *DescribeRollContributionsInput) RollContributionMetadataOutput
	DescribeRollContributions(
		input *DescribeRollContributionsInput,
	) (*DescribeRollContributionsOutput, error)
}

// DiceReroll records one ordered replacement of a die face and its source.
type DiceReroll struct {
	DieIndex int
	Before   int
	After    int
	Source   RollSource
}

// KeepRule names the rule that decided which faces of a dice pool count.
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

// DiceKeep records the rule that decided KeptIndices, and who brought it.
// Nil when nothing touched the pool: a straight roll keeps every face.
//
// It is the sibling of Rerolls. Rerolls explains why FinalRolls differ from
// OriginalRolls; Keep explains why KeptIndices is what it is. Cancellation is
// a recorded rule rather than an absence: RAW rolls one die, and so do we, but
// the record names the two rules that met (rpg-project#462 R2).
type DiceKeep struct {
	// Rule names the keep rule that was applied.
	Rule KeepRule

	// Granted are the sources that granted advantage on this pool.
	Granted []RollSource

	// Imposed are the sources that imposed disadvantage on this pool.
	Imposed []RollSource
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

	// Keep records the rule that decided KeptIndices. Nil when nothing
	// touched the pool.
	Keep *DiceKeep
}

// RollComponent records dice, a modifier, or both from one source.
// A non-nil Modifier participates even when its value is zero.
type RollComponent struct {
	Source   RollSource
	Dice     *DiceTrace
	Modifier *int

	// SubtractDice subtracts Dice.Subtotal from the calculation. It never
	// negates physical die faces or a fixed Modifier on the same component.
	SubtractDice bool
}

// RollCalculation records the sourced components and authoritative total of a roll.
type RollCalculation struct {
	Components []RollComponent
	Total      int
}

// NewRollCalculation assembles a calculation from its components and sums the
// authoritative total off the components themselves, so no machine hand-writes
// its own arithmetic and then disagrees with the record it publishes.
// Subtractive dice subtract their subtotal; a non-nil Modifier participates
// even when it is zero. Validate the result before publishing it.
func NewRollCalculation(components []RollComponent) *RollCalculation {
	calculation := &RollCalculation{Components: components}
	for _, component := range components {
		if component.Dice != nil {
			if component.SubtractDice {
				calculation.Total -= component.Dice.Subtotal
			} else {
				calculation.Total += component.Dice.Subtotal
			}
		}
		if component.Modifier != nil {
			calculation.Total += *component.Modifier
		}
	}
	return calculation
}

// CloneRollSource returns a copy of source whose ref is its own, so a published
// source cannot be mutated through the one it was copied from.
func CloneRollSource(source RollSource) RollSource {
	return cloneRollSource(source)
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
			Source:       cloneRollSource(component.Source),
			Dice:         cloneDiceTrace(component.Dice),
			Modifier:     cloneInt(component.Modifier),
			SubtractDice: component.SubtractDice,
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
		Keep:          cloneDiceKeep(trace.Keep),
	}
}

func cloneDiceKeep(keep *DiceKeep) *DiceKeep {
	if keep == nil {
		return nil
	}

	return &DiceKeep{
		Rule:    keep.Rule,
		Granted: cloneRollSources(keep.Granted),
		Imposed: cloneRollSources(keep.Imposed),
	}
}

func cloneRollSources(sources []RollSource) []RollSource {
	if sources == nil {
		return nil
	}

	clones := make([]RollSource, len(sources))
	for i, source := range sources {
		clones[i] = cloneRollSource(source)
	}
	return clones
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
		Ref:      cloneRef(source.Ref),
		Name:     source.Name,
		Label:    source.Label,
		SourceID: source.SourceID,
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

func validateRollComponent(component RollComponent) error {
	if err := validateRollSource(component.Source); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if component.Dice == nil && component.Modifier == nil {
		return fmt.Errorf("must contain dice, a modifier, or both")
	}
	if component.SubtractDice && component.Dice == nil {
		return fmt.Errorf("cannot subtract dice without dice")
	}
	if component.Dice != nil && strings.TrimSpace(component.Source.SourceID) == "" {
		// R7: every dice pool names the entity whose rule threw it. The d20 is
		// the roller's, a contributed die its granter's, a damage die the
		// wielder's. A roll with no entity behind it does not exist in this
		// game, so it is refused here rather than rendered anonymous
		// (rpg-project#462).
		return fmt.Errorf("dice source id is required")
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
	if err := validateSubtotal(trace); err != nil {
		return err
	}

	return validateDiceKeep(trace)
}

// validateDiceKeep refuses any keep record that does not describe the pool it
// sits on. Fail closed: a builder that fills the record by hand and gets it
// wrong is refused at the seam, not rendered wrong (rpg-project#462).
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
