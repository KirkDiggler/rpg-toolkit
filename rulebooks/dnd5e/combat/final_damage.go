// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DamageInstanceInput represents a single damage amount with its type.
// Multiple instances allow mixed-type damage (e.g., flametongue: slashing + fire).
// It is what [SettleDamageOutput.FinalDamage] returns and what resolution
// folds into an [ApplyDamageInput].
type DamageInstanceInput struct {
	// Amount is the damage that lands.
	Amount int

	// Type is the damage type (slashing, fire, etc.)
	Type damage.Type
}

// SettleDamageInput is what the target step hands the settlement: the damage
// dealt and the target's answers to it.
type SettleDamageInput struct {
	// Dealt is the dealt fold's components after the save's halving: damage
	// only. Use the components the step sent, never the folded event's copy.
	Dealt []dnd5eEvents.DamageComponent
	// Reductions are the target's fixed reductions, from the incoming fold.
	Reductions []dnd5eEvents.DamageReduction
	// Multipliers are the target's immunities, resistances and
	// vulnerabilities, from the incoming fold, in fold order.
	Multipliers []dnd5eEvents.DamageMultiplier
}

// TypeSettlement is what happened to one damage type. A trace that explains
// the type sums to Taken: Dealt + Reduced + Floor + Change.
type TypeSettlement struct {
	// Type is the damage type.
	Type damage.Type
	// Dealt is the sum of the dealt components of this type.
	Dealt int
	// Reduced is the sum of the target's reductions on this type, zero or
	// negative.
	Reduced int
	// Floor is what brings the type back to zero when its reductions sank it
	// below zero; zero otherwise. A type cannot heal its target.
	Floor int
	// Factor is the effective multiplier the stacking rules chose: 0 for
	// immunity, 0.5 for resistance, 2 for vulnerability, 1 for none or for
	// resistance and vulnerability cancelling. It is decided by which factors
	// are present, never read back from an amount.
	Factor float64
	// DecidedBy is the first multiplier in fold order whose own factor is the
	// effective one, the rule a trace line names. Nil when Factor is 1.
	DecidedBy *dnd5eEvents.DamageMultiplier
	// Change is what the effective factor did after reductions and the floor:
	// Taken minus (Dealt + Reduced + Floor). Zero when Factor is 1.
	Change int
	// Taken is what the target takes of this type, zero included: an immune
	// type is settled at zero and reported, never omitted.
	Taken int
}

// SettleDamageOutput is the settlement: one entry per damage type dealt,
// sorted by damage type.
type SettleDamageOutput struct {
	Types []TypeSettlement
}

// SettleDamage is combat's settlement of received damage, per damage type:
// what was dealt, the target's reductions, the effective factor its stacking
// rule chose, and what is taken. Bus-free: the target step folds on its own
// bus and hands in what the folds settled on.
//
// The order is the 5e order. Reductions apply first, and a type they sink
// below zero is floored at zero; then the multipliers, after every other
// modifier, rounding down. Immunity wins over resistance and vulnerability;
// resistance and vulnerability cancel; neither stacks.
//
// Errors: a dealt component with no damage type or carrying a multiplier (a
// target answer on the dealt side); a malformed reduction or multiplier; or
// an answer on a damage type nothing dealt.
func SettleDamage(input *SettleDamageInput) (*SettleDamageOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "settle damage requires an input")
	}

	settled := make(map[damage.Type]*TypeSettlement)
	var order []damage.Type
	for i, component := range input.Dealt {
		if component.DamageType == "" {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"dealt component %d names no damage type", i)
		}
		if component.Multiplier != nil {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"dealt component %d carries a multiplier; a target's answer is settled from the incoming fold", i)
		}
		entry, ok := settled[component.DamageType]
		if !ok {
			entry = &TypeSettlement{Type: component.DamageType, Factor: 1}
			settled[component.DamageType] = entry
			order = append(order, component.DamageType)
		}
		entry.Dealt += component.Total()
	}

	for _, reduction := range input.Reductions {
		if err := reduction.Validate(); err != nil {
			return nil, rpgerr.Wrap(err, "settle damage")
		}
		entry, ok := settled[reduction.DamageType]
		if !ok {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"reduction from %s names %s damage, and none was dealt", reduction.Source.Ref, reduction.DamageType)
		}
		entry.Reduced += reduction.Modifier
	}

	byType := make(map[damage.Type][]int)
	for i, multiplier := range input.Multipliers {
		if err := multiplier.Validate(); err != nil {
			return nil, rpgerr.Wrap(err, "settle damage")
		}
		if _, ok := settled[multiplier.DamageType]; !ok {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"multiplier from %s names %s damage, and none was dealt", multiplier.Source.Ref, multiplier.DamageType)
		}
		byType[multiplier.DamageType] = append(byType[multiplier.DamageType], i)
	}

	output := &SettleDamageOutput{Types: make([]TypeSettlement, 0, len(order))}
	for _, damageType := range order {
		entry := settled[damageType]
		base := entry.Dealt + entry.Reduced
		if base < 0 {
			entry.Floor = -base
			base = 0
		}

		factors := make([]float64, 0, len(byType[damageType]))
		for _, index := range byType[damageType] {
			factors = append(factors, input.Multipliers[index].Factor)
		}
		entry.Factor = effectiveFactor(factors)
		if entry.Factor != 1 {
			for _, index := range byType[damageType] {
				if input.Multipliers[index].Factor == entry.Factor {
					decided := input.Multipliers[index]
					decided.Source = dnd5eEvents.CloneRollSource(decided.Source)
					entry.DecidedBy = &decided
					break
				}
			}
		}

		entry.Taken = int(float64(base) * entry.Factor)
		entry.Change = entry.Taken - base
		output.Types = append(output.Types, *entry)
	}

	slices.SortFunc(output.Types, func(a, b TypeSettlement) int {
		switch {
		case a.Type < b.Type:
			return -1
		case a.Type > b.Type:
			return 1
		default:
			return 0
		}
	})

	return output, nil
}

// FinalDamage is the settlement's landing instances: every type taken above
// zero, sorted by damage type, and their total. A type taken as nothing stays
// in [SettleDamageOutput.Types] and is not an instance.
func (s *SettleDamageOutput) FinalDamage() (instances []DamageInstanceInput, total int) {
	instances = make([]DamageInstanceInput, 0, len(s.Types))
	for _, settled := range s.Types {
		if settled.Taken <= 0 {
			continue
		}
		instances = append(instances, DamageInstanceInput{Amount: settled.Taken, Type: settled.Type})
		total += settled.Taken
	}

	return instances, total
}

// effectiveFactor applies 5e's stacking rules to the factors present on one
// damage type. Every factor is one of the three a [dnd5eEvents.DamageMultiplier]
// may carry; the caller validated them.
//   - Immunity always wins.
//   - Resistance and vulnerability cancel when both are present.
//   - Neither stacks: two resistances halve once.
func effectiveFactor(factors []float64) float64 {
	immune := slices.Contains(factors, dnd5eEvents.DamageFactorImmunity)
	resistant := slices.Contains(factors, dnd5eEvents.DamageFactorResistance)
	vulnerable := slices.Contains(factors, dnd5eEvents.DamageFactorVulnerability)

	switch {
	case immune:
		return dnd5eEvents.DamageFactorImmunity
	case resistant && vulnerable:
		return 1
	case resistant:
		return dnd5eEvents.DamageFactorResistance
	case vulnerable:
		return dnd5eEvents.DamageFactorVulnerability
	default:
		return 1
	}
}
