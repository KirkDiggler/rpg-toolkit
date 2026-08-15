// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type FinalDamageTestSuite struct {
	suite.Suite
}

func TestFinalDamageSuite(t *testing.T) {
	suite.Run(t, new(FinalDamageTestSuite))
}

// flat builds a plain damage component contributing Amount of a type.
func (s *FinalDamageTestSuite) flat(amount int, t damage.Type) dnd5eEvents.DamageComponent {
	return dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceWeapon,
		FlatBonus:  amount,
		DamageType: t,
	}
}

// multiplier builds a modifier component — resistance, vulnerability, immunity.
func (s *FinalDamageTestSuite) multiplier(m float64, t damage.Type) dnd5eEvents.DamageComponent {
	return dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceCondition,
		Multiplier: m,
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

// KNOWN GAP — damage immunity does not reduce damage, and never has.
//
// Pinned rather than fixed, in the style of TestKnownRoundTripGaps (#987):
// this PR moves the arithmetic without changing it, and correcting immunity is
// a live behavior change to the legacy stack. Filed separately.
//
// The mechanism: a component is read as a MULTIPLIER only when
// `component.Multiplier != 0`, so immunity — which monstertraits/immunity.go
// expresses as `Multiplier: 0` with the comment "Multiply by 0 = no damage" —
// fails that test and is read as a base-damage contribution of Total() == 0
// instead. The immunity component is silently discarded and full damage lands.
// resolveMultipliers' `m == 0.0 -> hasImmunity` branch is unreachable as a
// direct consequence.
//
// Nothing pins the correct behavior today, which is why it went unnoticed:
// monstertraits' immunity tests assert the condition applies, never that
// damage drops. WHEN THIS IS FIXED, THIS TEST SHOULD FAIL — update it to
// assert the poison instance is dropped entirely.
func (s *FinalDamageTestSuite) TestKnownGapImmunityIsIgnored() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(12, damage.Poison),
		s.multiplier(0.0, damage.Poison),
		s.flat(4, damage.Slashing),
	})

	s.Require().Equal([]combat.DamageInstanceInput{
		{Amount: 12, Type: damage.Poison},
		{Amount: 4, Type: damage.Slashing},
	}, instances, "CURRENT behavior: the immune target takes full poison damage")
	s.Require().Equal(16, total)
}

// The same gap from the stacking side: with immunity inert, a target that is
// vulnerable AND resistant AND immune resolves as the cancel case (1.0) rather
// than as immune (0.0).
func (s *FinalDamageTestSuite) TestKnownGapImmunityDoesNotTrumpTheOthers() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		s.flat(20, damage.Fire),
		s.multiplier(2.0, damage.Fire),
		s.multiplier(0.5, damage.Fire),
		s.multiplier(0.0, damage.Fire),
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 20, Type: damage.Fire}}, instances,
		"CURRENT behavior: resistance and vulnerability cancel, immunity never counted")
	s.Require().Equal(20, total)
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

// Dice ride through Total() alongside the flat bonus.
func (s *FinalDamageTestSuite) TestDiceAndFlatBonusBothCount() {
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		{
			Source:            dnd5eEvents.DamageSourceWeapon,
			OriginalDiceRolls: []int{4, 5},
			FinalDiceRolls:    []int{4, 5},
			FlatBonus:         3,
			DamageType:        damage.Slashing,
		},
	})

	s.Require().Equal([]combat.DamageInstanceInput{{Amount: 4 + 5 + 3, Type: damage.Slashing}}, instances)
	s.Require().Equal(12, total)
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
