// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FinalDamageTestSuite struct {
	suite.Suite
}

func TestFinalDamageSuite(t *testing.T) {
	suite.Run(t, new(FinalDamageTestSuite))
}

// intPtr returns a pointer to v, so a present zero modifier stays present.
func intPtr(v int) *int { return &v }

// flat builds a plain modifier-only damage component contributing Amount of a type.
func (s *FinalDamageTestSuite) flat(amount int, t damage.Type) dnd5eEvents.DamageComponent {
	return dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceWeapon,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
			Modifier: intPtr(amount),
		},
		DamageType: t,
	}
}

// multiplier builds a modifier component — resistance, vulnerability, immunity.
func (s *FinalDamageTestSuite) multiplier(m float64, t damage.Type) dnd5eEvents.DamageComponent {
	return dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceCondition,
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
		},
		Multiplier: dnd5eEvents.Multiply(m),
		DamageType: t,
	}
}

// THE ORDER PIN. A mixed-type hit reports its instances sorted by damage type,
// the same way every run.
//
// This is the one observable change in the split: the grouping is a map, and a
// map's iteration order is random per run, so before this the order was
// whatever Go felt like. Nothing can correctly depend on a random order, which
// is why sorting cannot break a correct consumer — and why this assertion is
// on the exact slice rather than on a set.
func (s *FinalDamageTestSuite) TestInstancesComeBackSortedByDamageType() {
	// Deliberately built out of alphabetical order, and not in one grouping
	// pass either — slashing, then fire, then more slashing.
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(7, damage.Slashing),
		s.flat(3, damage.Fire),
		s.flat(2, damage.Slashing),
		s.flat(1, damage.Cold),
	})

	s.Require().Equal([]combat.DamageInstanceInput{
		{Amount: 1, Type: damage.Cold},
		{Amount: 3, Type: damage.Fire},
		{Amount: 9, Type: damage.Slashing},
	}, instances)
	s.Require().Equal(1+3+9, total)
}

// Repeated calls agree exactly — the assertion a map-ordered implementation
// fails intermittently rather than never.
func (s *FinalDamageTestSuite) TestTheSameComponentsProduceTheSameOrderEveryTime() {
	components := []dnd5eEvents.DamageComponent{
		s.flat(4, damage.Thunder),
		s.flat(6, damage.Acid),
		s.flat(5, damage.Radiant),
		s.flat(2, damage.Necrotic),
		s.flat(8, damage.Piercing),
	}

	first, firstTotal := combat.FinalDamage(components)
	for i := 0; i < 50; i++ {
		again, againTotal := combat.FinalDamage(components)
		s.Require().Equal(first, again, "run %d disagreed with the first", i)
		s.Require().Equal(firstTotal, againTotal)
	}
}

// Resistance halves, and the total follows the instances.
func (s *FinalDamageTestSuite) TestResistanceHalvesItsOwnTypeOnly() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(10, damage.Slashing),
		s.flat(10, damage.Fire),
		s.multiplier(0.5, damage.Fire),
	})

	s.Require().Equal([]combat.DamageInstanceInput{
		{Amount: 5, Type: damage.Fire},
		{Amount: 10, Type: damage.Slashing},
	}, instances, "fire halved, slashing untouched")
	s.Require().Equal(15, total)
}

func (s *FinalDamageTestSuite) TestVulnerabilityDoubles() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(7, damage.Fire),
		s.multiplier(2.0, damage.Fire),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 14, Type: damage.Fire}}, instances)
	s.Require().Equal(14, total)
}

// Immunity negates, and an instance that lands for nothing is not reported as
// landing at all.
//
// This is rpg-toolkit#1012's fix-day assertion. Until the dispatch keyed on
// presence rather than the value zero, immunity's 0.0 was indistinguishable
// from "no multiplier" and the immune target took full damage — the shape of
// bug where the rule is written, the branch exists, and nothing ever reaches it.
func (s *FinalDamageTestSuite) TestImmunityDropsTheInstanceEntirely() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(12, damage.Poison),
		s.multiplier(0.0, damage.Poison),
		s.flat(4, damage.Slashing),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 4, Type: damage.Slashing}}, instances,
		"the poison instance is gone, not present at zero")
	s.Require().Equal(4, total)
}

// Immunity beats vulnerability and resistance both, whatever else is stacked —
// the branch that was unreachable until #1012, now reached.
func (s *FinalDamageTestSuite) TestImmunityTrumpsEverythingElse() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(20, damage.Fire),
		s.multiplier(2.0, damage.Fire),
		s.multiplier(0.5, damage.Fire),
		s.multiplier(0.0, damage.Fire),
	})

	s.Require().Empty(instances, "immune, so nothing lands — not the 1.0 cancel case")
	s.Require().Zero(total)
}

// A zero factor is a MODIFIER, not an absent one. The distinction the whole
// fix rests on, asserted directly rather than only through its consequences:
// a component carrying Multiply(0) must never be read as damage of zero.
func (s *FinalDamageTestSuite) TestAZeroFactorIsAModifierNotAbsentDamage() {
	// If Multiply(0) were read as a base contribution, its Total() of 0 would
	// add nothing and the fire would land in full.
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(15, damage.Fire),
		s.multiplier(0.0, damage.Fire),
	})

	s.Require().Empty(instances)
	s.Require().Zero(total)
}

// Resistance and vulnerability cancel exactly — not 0.5 * 2.0 applied in some
// order, but a flat 1.0.
func (s *FinalDamageTestSuite) TestResistanceAndVulnerabilityCancel() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(9, damage.Cold),
		s.multiplier(0.5, damage.Cold),
		s.multiplier(2.0, damage.Cold),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 9, Type: damage.Cold}}, instances)
	s.Require().Equal(9, total)
}

// Two resistances are still one resistance — 5e does not stack them into a
// quarter.
func (s *FinalDamageTestSuite) TestMultipleResistancesDoNotStack() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(20, damage.Bludgeoning),
		s.multiplier(0.5, damage.Bludgeoning),
		s.multiplier(0.5, damage.Bludgeoning),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 10, Type: damage.Bludgeoning}}, instances,
		"halved once, not twice")
	s.Require().Equal(10, total)
}

func (s *FinalDamageTestSuite) TestMultipleVulnerabilitiesDoNotStack() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(5, damage.Fire),
		s.multiplier(2.0, damage.Fire),
		s.multiplier(2.0, damage.Fire),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 10, Type: damage.Fire}}, instances,
		"doubled once, not quadrupled")
	s.Require().Equal(10, total)
}

// Components of one type sum before the multiplier applies — resistance halves
// the TYPE's total, not each contribution, which rounds differently.
func (s *FinalDamageTestSuite) TestComponentsGroupBeforeTheMultiplierApplies() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(3, damage.Fire),
		s.flat(3, damage.Fire),
		s.flat(3, damage.Fire),
		s.multiplier(0.5, damage.Fire),
	})

	// 9 halved is 4 (truncating). Halving each 3 first would give 1+1+1 = 3.
	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 4, Type: damage.Fire}}, instances)
	s.Require().Equal(4, total)
}

// Dice ride through the authoritative subtotal alongside the modifier pointer.
func (s *FinalDamageTestSuite) TestDiceAndModifierBothCount() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		{
			Source: dnd5eEvents.DamageSourceWeapon,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      "2d8",
					DieSize:       8,
					OriginalRolls: []int{4, 5},
					FinalRolls:    []int{4, 5},
					Subtotal:      9,
				},
				Modifier: intPtr(3),
			},
			DamageType: damage.Slashing,
		},
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 4 + 5 + 3, Type: damage.Slashing}}, instances)
	s.Require().Equal(12, total)
}

// Final damage consumes the AUTHORITATIVE subtotal and the modifier POINTER.
// A kept-dice trace whose faces sum to more than its subtotal pins both halves:
// summing the face array would report 22, and ignoring the present modifier
// would report 15 — the contract is subtotal 15 plus +3, which is 18.
func (s *FinalDamageTestSuite) TestFinalDamageConsumesSubtotalsAndModifierPointers() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		{
			Source: dnd5eEvents.DamageSourceWeapon,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      "3d8",
					DieSize:       8,
					OriginalRolls: []int{7, 8, 4},
					FinalRolls:    []int{7, 8, 4},
					KeptIndices:   []int{0, 1},
					Subtotal:      15,
				},
			},
			DamageType: damage.Slashing,
		},
		{
			Source: dnd5eEvents.DamageSourceAbility,
			Roll: dnd5eEvents.RollComponent{
				Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
				Modifier: intPtr(3),
			},
			DamageType: damage.Slashing,
		},
	})

	// The dropped third face keeps the face-array sum (19) away from the
	// authoritative subtotal (15); only a consumer reading the subtotal plus
	// the present modifier reports 18.
	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 18, Type: damage.Slashing}}, instances)
	s.Require().Equal(18, total)
}

// A present zero modifier participates: it is a real pointer carrying zero,
// not an absent modifier, and it adds nothing to the landed damage.
func (s *FinalDamageTestSuite) TestAPresentZeroModifierRemainsPresent() {
	component := dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceAbility,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
			Modifier: intPtr(0),
		},
		DamageType: damage.Slashing,
	}

	s.Require().NotNil(component.Roll.Modifier, "a present zero modifier stays present")
	s.Require().Zero(component.Total())

	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		component,
		s.flat(5, damage.Slashing),
	})
	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 5, Type: damage.Slashing}}, instances)
	s.Require().Equal(5, total)
}

// An instance that resolves to zero is not reported as landing. Resistance
// halving a single point of damage is the realistic way to get there — 1
// halved truncates to 0 — and a target that took no cold damage should not
// appear in the breakdown as having taken cold damage of zero.
func (s *FinalDamageTestSuite) TestAnInstanceThatResolvesToZeroIsDropped() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(1, damage.Cold),
		s.multiplier(0.5, damage.Cold),
		s.flat(6, damage.Slashing),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 6, Type: damage.Slashing}}, instances,
		"the cold instance is absent, not present at zero")
	s.Require().Equal(6, total)
}

// The same boundary from a component that simply carries no damage — the
// catalog has weapons whose damage is "0" (a net).
func (s *FinalDamageTestSuite) TestAComponentCarryingNoDamageProducesNoInstance() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(0, damage.Bludgeoning),
		s.flat(3, damage.Piercing),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 3, Type: damage.Piercing}}, instances)
	s.Require().Equal(3, total)
}

func (s *FinalDamageTestSuite) TestNoComponentsIsNoDamage() {
	instances, total := combat.FinalDamage(nil)

	s.Require().Empty(instances)
	s.Require().Zero(total)
}

// The reported total is the sum of the instances reported, not a separately
// accumulated number that could drift from them.
func (s *FinalDamageTestSuite) TestTheTotalIsTheSumOfTheInstances() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(10, damage.Fire),
		s.multiplier(0.5, damage.Fire),
		s.flat(7, damage.Slashing),
	})

	sum := 0
	for _, instance := range instances {
		sum += instance.Amount
	}
	s.Require().Equal(sum, total)
	s.Require().Equal(5+7, total, "fire halved to 5, slashing 7")
}
