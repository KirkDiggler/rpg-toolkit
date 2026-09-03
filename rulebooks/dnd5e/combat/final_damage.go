// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DamageInstanceInput represents a single damage amount with its type.
// Multiple instances allow mixed-type damage (e.g., flametongue: slashing + fire).
//
// It lived in damage.go until rpg-project#319 Phase 6 deleted the second
// damage flow around it, and moves here rather than dying with it: this is
// what [FinalDamage] returns and what resolution folds into an
// [ApplyDamageInput].
type DamageInstanceInput struct {
	// Amount is the base damage before modifiers
	Amount int

	// Type is the damage type (slashing, fire, etc.)
	Type damage.Type
}

// FinalDamage turns folded damage components into the instances that land,
// applying 5e's resistance, vulnerability, and immunity stacking, and reports
// the total alongside them.
//
// Bus-free on purpose. This is the arithmetic half of [ResolveDamage], split
// out so a caller that folds the damage chain on its own bus can still get the
// multipliers right instead of reimplementing them (rpg-toolkit#965 slice 2 —
// the resolution module owns its own folds and had no way to reach this).
// The shape mirrors [github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves.MakeSavingThrow]:
// fold outside, hand in what the fold settled on, take back the arithmetic.
//
// ResolveDamage calls this too, so there is one implementation rather than a
// copy per stack.
//
// Components carrying a Multiplier are modifiers (resistance 0.5,
// vulnerability 2.0, immunity 0.0); every other component contributes its
// Total() — the authoritative dice subtotal plus any present modifier.
// Both are grouped by damage type before the multipliers apply, because 5e
// resists a TYPE rather than a source.
//
// Modifier-or-damage is decided by the Multiplier's PRESENCE, never its value:
// immunity's factor is zero, so a value test cannot tell it from an absent
// modifier. It could not, and immunity silently did nothing (rpg-toolkit#1012).
//
// Component totals come from the roll trace's authoritative subtotal and the
// modifier POINTER — never from resumming recorded faces, which would
// reconstruct reroll history the provider already settled. A kept-dice trace
// (advantage, a keep-high rule) intentionally has faces that sum to more than
// its subtotal; the subtotal is the number the provider asserted.
//
// Instances come back sorted by damage type, and a zero or negative instance
// is dropped rather than reported as landing.
func FinalDamage(components []dnd5eEvents.DamageComponent) (instances []DamageInstanceInput, total int) {
	instances = calculateFinalDamage(components)
	for _, instance := range instances {
		total += instance.Amount
	}

	return instances, total
}

// calculateFinalDamage processes damage components and applies multipliers.
// In D&D 5e:
// - Resistance (0.5) halves damage, Vulnerability (2.0) doubles it, Immunity (0.0) negates
// - Multiple resistances don't stack (apply most beneficial once)
// - If both resistance and vulnerability exist for a type, they cancel out
func calculateFinalDamage(components []dnd5eEvents.DamageComponent) []DamageInstanceInput {
	// Group damage and multipliers by type
	type damageGroup struct {
		baseDamage  int
		multipliers []float64
	}
	byType := make(map[damage.Type]*damageGroup)

	for _, component := range components {
		dmgType := component.DamageType
		if byType[dmgType] == nil {
			byType[dmgType] = &damageGroup{}
		}

		// Presence, not value: a component either IS a modifier or is damage,
		// and immunity's factor is zero. Testing `!= 0` conflated the two and
		// dropped immunity entirely (rpg-toolkit#1012).
		if component.Multiplier != nil {
			byType[dmgType].multipliers = append(byType[dmgType].multipliers, *component.Multiplier)
			continue
		}

		// Total() reads the dice trace's authoritative subtotal plus any
		// present modifier pointer — the one arithmetic a damage component
		// already owns.
		byType[dmgType].baseDamage += component.Total()
	}

	// Apply multipliers to each damage type
	result := make([]DamageInstanceInput, 0, len(byType))
	for dmgType, group := range byType {
		finalDamage := group.baseDamage

		if len(group.multipliers) > 0 {
			// Apply D&D 5e stacking rules
			effectiveMultiplier := resolveMultipliers(group.multipliers)
			finalDamage = int(float64(finalDamage) * effectiveMultiplier)
		}

		if finalDamage > 0 {
			result = append(result, DamageInstanceInput{
				Amount: finalDamage,
				Type:   dmgType,
			})
		}
	}

	// Sorted, because the grouping above is a map and a map's iteration order
	// is random per run. Nothing can correctly depend on that order — anything
	// that did was already flaky — so ordering by damage type is the one
	// direction of change that cannot break a correct consumer, and it makes a
	// mixed-type hit (a flame tongue's slashing plus fire) report the same way
	// twice. Prior art: ToData's member sort exists because map iteration made
	// round-trips intermittently fail.
	sort.Slice(result, func(i, j int) bool { return result[i].Type < result[j].Type })

	return result
}

// resolveMultipliers applies D&D 5e stacking rules for resistance,
// vulnerability, and immunity.
// - Immunity (0.0) always wins
// - Resistance (0.5) and vulnerability (2.0) cancel out if both present
// - Multiple resistances don't stack (use 0.5 once)
// - Multiple vulnerabilities don't stack (use 2.0 once)
func resolveMultipliers(multipliers []float64) float64 {
	hasImmunity := false
	hasResistance := false
	hasVulnerability := false

	for _, m := range multipliers {
		switch {
		case m == 0.0:
			hasImmunity = true
		case m < 1.0:
			hasResistance = true
		case m > 1.0:
			hasVulnerability = true
		}
	}

	// Immunity trumps everything
	if hasImmunity {
		return 0.0
	}

	// Resistance and vulnerability cancel out
	if hasResistance && hasVulnerability {
		return 1.0
	}

	// Apply resistance (0.5) or vulnerability (2.0)
	if hasResistance {
		return 0.5
	}
	if hasVulnerability {
		return 2.0
	}

	return 1.0
}
