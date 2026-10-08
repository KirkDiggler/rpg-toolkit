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

type SettleDamageTestSuite struct {
	suite.Suite
}

func TestSettleDamageSuite(t *testing.T) {
	suite.Run(t, new(SettleDamageTestSuite))
}

// intPtr returns a pointer to v, so a present zero modifier stays present.
func intPtr(v int) *int { return &v }

// flat builds a plain modifier-only dealt component of a type.
func flat(amount int, t damage.Type) dnd5eEvents.DamageComponent {
	return dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceWeapon,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
			Modifier: intPtr(amount),
		},
		DamageType: t,
	}
}

// resist, vulnerable and immune build one target answer each, from a named
// rule, on one damage type.
func resist(t damage.Type) dnd5eEvents.DamageMultiplier {
	return dnd5eEvents.DamageMultiplier{
		Category:   dnd5eEvents.DamageSourceCondition,
		Source:     dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
		DamageType: t,
		Factor:     dnd5eEvents.DamageFactorResistance,
	}
}

func vulnerable(t damage.Type) dnd5eEvents.DamageMultiplier {
	return dnd5eEvents.DamageMultiplier{
		Category:   dnd5eEvents.DamageSourceMonsterTrait,
		Source:     dnd5eEvents.RollSource{Ref: refs.MonsterTraits.Vulnerability(), Name: "Vulnerability"},
		DamageType: t,
		Factor:     dnd5eEvents.DamageFactorVulnerability,
	}
}

func immune(t damage.Type) dnd5eEvents.DamageMultiplier {
	return dnd5eEvents.DamageMultiplier{
		Category:   dnd5eEvents.DamageSourceMonsterTrait,
		Source:     dnd5eEvents.RollSource{Ref: refs.MonsterTraits.Immunity(), Name: "Immunity"},
		DamageType: t,
		Factor:     dnd5eEvents.DamageFactorImmunity,
	}
}

func reduce(amount int, t damage.Type) dnd5eEvents.DamageReduction {
	return dnd5eEvents.DamageReduction{
		Category:   dnd5eEvents.DamageSourceFeature,
		Source:     dnd5eEvents.RollSource{Ref: refs.Features.DeflectMissiles(), Name: "Deflect Missiles"},
		DamageType: t,
		Modifier:   -amount,
	}
}

func (s *SettleDamageTestSuite) settle(in *combat.SettleDamageInput) *combat.SettleDamageOutput {
	out, err := combat.SettleDamage(in)
	s.Require().NoError(err)
	return out
}

// only returns the one settled type, failing when it is absent.
func (s *SettleDamageTestSuite) only(out *combat.SettleDamageOutput, t damage.Type) combat.TypeSettlement {
	for _, settled := range out.Types {
		if settled.Type == t {
			return settled
		}
	}
	s.FailNowf("type not settled", "%s", t)
	return combat.TypeSettlement{}
}

// An immune type is settled at factor 0 and taken 0, reported in the
// settlement, and dropped from FinalDamage's landing instances.
func (s *SettleDamageTestSuite) TestAnImmuneTypeIsSettledAtZeroAndDoesNotLand() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt:       []dnd5eEvents.DamageComponent{flat(12, damage.Fire), flat(5, damage.Slashing)},
		Multipliers: []dnd5eEvents.DamageMultiplier{immune(damage.Fire)},
	})

	fire := s.only(out, damage.Fire)
	s.Equal(12, fire.Dealt)
	s.Equal(0.0, fire.Factor)
	s.Equal(0, fire.Taken)
	s.Equal(-12, fire.Change)
	s.Require().NotNil(fire.DecidedBy)
	s.Equal(refs.MonsterTraits.Immunity(), fire.DecidedBy.Source.Ref)

	instances, total := out.FinalDamage()
	s.Equal([]combat.DamageInstanceInput{{Amount: 5, Type: damage.Slashing}}, instances)
	s.Equal(5, total)
}

// Resistance and vulnerability together settle at factor 1: they cancel, and
// no multiplier names the type, because none carries the effective factor.
func (s *SettleDamageTestSuite) TestResistanceAndVulnerabilityTogetherSettleAtOne() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt:       []dnd5eEvents.DamageComponent{flat(9, damage.Cold)},
		Multipliers: []dnd5eEvents.DamageMultiplier{resist(damage.Cold), vulnerable(damage.Cold)},
	})

	cold := s.only(out, damage.Cold)
	s.Equal(1.0, cold.Factor)
	s.Equal(9, cold.Taken)
	s.Zero(cold.Change)
	s.Nil(cold.DecidedBy)
}

// Two resistances are still one: 0.5, not a quarter.
func (s *SettleDamageTestSuite) TestTwoResistancesSettleAtOneHalf() {
	second := resist(damage.Slashing)
	second.Source = dnd5eEvents.RollSource{Ref: refs.Conditions.BladeWard(), Name: "Blade Ward"}
	out := s.settle(&combat.SettleDamageInput{
		Dealt:       []dnd5eEvents.DamageComponent{flat(9, damage.Slashing)},
		Multipliers: []dnd5eEvents.DamageMultiplier{resist(damage.Slashing), second},
	})

	slashing := s.only(out, damage.Slashing)
	s.Equal(0.5, slashing.Factor)
	s.Equal(4, slashing.Taken)
	s.Equal(-5, slashing.Change)
	s.Require().NotNil(slashing.DecidedBy)
	s.Equal(refs.Conditions.Raging(), slashing.DecidedBy.Source.Ref,
		"the first multiplier in fold order carrying the effective factor names the line")
}

func (s *SettleDamageTestSuite) TestVulnerabilityDoubles() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt:       []dnd5eEvents.DamageComponent{flat(7, damage.Radiant)},
		Multipliers: []dnd5eEvents.DamageMultiplier{vulnerable(damage.Radiant), vulnerable(damage.Radiant)},
	})

	radiant := s.only(out, damage.Radiant)
	s.Equal(2.0, radiant.Factor)
	s.Equal(14, radiant.Taken)
}

// Immunity wins, and names the line, even listed after the others.
func (s *SettleDamageTestSuite) TestImmunityWinsWhereverItSits() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt: []dnd5eEvents.DamageComponent{flat(1, damage.Poison)},
		Multipliers: []dnd5eEvents.DamageMultiplier{
			resist(damage.Poison), vulnerable(damage.Poison), immune(damage.Poison),
		},
	})

	poison := s.only(out, damage.Poison)
	s.Equal(0.0, poison.Factor)
	s.Require().NotNil(poison.DecidedBy)
	s.Equal(refs.MonsterTraits.Immunity(), poison.DecidedBy.Source.Ref,
		"a total of 1 halves to 0 too; the factor, not the amount, names the rule")
}

// Components of one type sum before the multiplier: resistance halves the
// type's total, rounding down once.
func (s *SettleDamageTestSuite) TestComponentsGroupBeforeTheMultiplierApplies() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt:       []dnd5eEvents.DamageComponent{flat(3, damage.Slashing), flat(3, damage.Slashing)},
		Multipliers: []dnd5eEvents.DamageMultiplier{resist(damage.Slashing)},
	})

	s.Equal(3, s.only(out, damage.Slashing).Taken, "halving 6 is 3, halving each 3 would be 2")
}

// Reductions apply before the multiplier, as every other modifier does.
func (s *SettleDamageTestSuite) TestReductionsApplyBeforeTheMultiplier() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt:       []dnd5eEvents.DamageComponent{flat(10, damage.Piercing)},
		Reductions:  []dnd5eEvents.DamageReduction{reduce(4, damage.Piercing)},
		Multipliers: []dnd5eEvents.DamageMultiplier{resist(damage.Piercing)},
	})

	piercing := s.only(out, damage.Piercing)
	s.Equal(-4, piercing.Reduced)
	s.Equal(3, piercing.Taken, "(10 - 4) halved, not 10 halved minus 4")
	s.Equal(piercing.Taken, piercing.Dealt+piercing.Reduced+piercing.Floor+piercing.Change,
		"the parts sum to what is taken")
}

// A reduction larger than the type floors it at zero, and the floor is named
// so a trace still sums.
func (s *SettleDamageTestSuite) TestAReductionBelowZeroFloors() {
	out := s.settle(&combat.SettleDamageInput{
		Dealt:      []dnd5eEvents.DamageComponent{flat(3, damage.Piercing)},
		Reductions: []dnd5eEvents.DamageReduction{reduce(8, damage.Piercing)},
	})

	piercing := s.only(out, damage.Piercing)
	s.Equal(5, piercing.Floor)
	s.Equal(0, piercing.Taken)
	s.Equal(piercing.Taken, piercing.Dealt+piercing.Reduced+piercing.Floor+piercing.Change)
}

// Types come back sorted, the same way every run.
func (s *SettleDamageTestSuite) TestTypesComeBackSortedByDamageType() {
	dealt := []dnd5eEvents.DamageComponent{
		flat(5, damage.Slashing), flat(3, damage.Fire), flat(2, damage.Cold),
	}
	for range 20 {
		out := s.settle(&combat.SettleDamageInput{Dealt: dealt})
		instances, total := out.FinalDamage()
		s.Equal([]combat.DamageInstanceInput{
			{Amount: 2, Type: damage.Cold},
			{Amount: 3, Type: damage.Fire},
			{Amount: 5, Type: damage.Slashing},
		}, instances)
		s.Equal(10, total)
	}
}

// Dealt totals come from the trace's authoritative subtotal plus the modifier
// pointer: a kept-dice trace whose faces sum to more still settles at the
// subtotal.
func (s *SettleDamageTestSuite) TestDealtReadsSubtotalsAndModifierPointers() {
	out := s.settle(&combat.SettleDamageInput{Dealt: []dnd5eEvents.DamageComponent{
		{
			Source: dnd5eEvents.DamageSourceWeapon,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation: "3d8", DieSize: 8,
					OriginalRolls: []int{7, 8, 4}, FinalRolls: []int{7, 8, 4},
					KeptIndices: []int{0, 1}, Subtotal: 15,
				},
				Modifier: intPtr(3),
			},
			DamageType: damage.Slashing,
		},
	}})

	s.Equal(18, s.only(out, damage.Slashing).Dealt)
}

// A type dealt as nothing is settled and does not land.
func (s *SettleDamageTestSuite) TestATypeDealtAsNothingDoesNotLand() {
	out := s.settle(&combat.SettleDamageInput{Dealt: []dnd5eEvents.DamageComponent{flat(0, damage.Bludgeoning)}})

	s.Equal(0, s.only(out, damage.Bludgeoning).Taken)
	instances, total := out.FinalDamage()
	s.Empty(instances)
	s.Zero(total)
}

// The settlement refuses what it cannot settle truthfully.
func (s *SettleDamageTestSuite) TestRefusals() {
	withMultiplier := flat(5, damage.Fire)
	half := 0.5
	withMultiplier.Multiplier = &half
	quarter := resist(damage.Fire)
	quarter.Factor = 0.25
	unnamed := resist(damage.Fire)
	unnamed.Source = dnd5eEvents.RollSource{}
	adding := reduce(3, damage.Fire)
	adding.Modifier = 3

	cases := map[string]*combat.SettleDamageInput{
		"a dealt component carrying a multiplier": {Dealt: []dnd5eEvents.DamageComponent{withMultiplier}},
		"a dealt component with no damage type":   {Dealt: []dnd5eEvents.DamageComponent{flat(5, "")}},
		"a factor the stacking rules do not know": {
			Dealt: []dnd5eEvents.DamageComponent{flat(5, damage.Fire)}, Multipliers: []dnd5eEvents.DamageMultiplier{quarter},
		},
		"a multiplier naming no source": {
			Dealt: []dnd5eEvents.DamageComponent{flat(5, damage.Fire)}, Multipliers: []dnd5eEvents.DamageMultiplier{unnamed},
		},
		"a multiplier on a type nothing dealt": {
			Dealt: []dnd5eEvents.DamageComponent{flat(5, damage.Fire)}, Multipliers: []dnd5eEvents.DamageMultiplier{resist(damage.Cold)},
		},
		"a reduction that adds": {
			Dealt: []dnd5eEvents.DamageComponent{flat(5, damage.Fire)}, Reductions: []dnd5eEvents.DamageReduction{adding},
		},
		"a reduction on a type nothing dealt": {
			Dealt: []dnd5eEvents.DamageComponent{flat(5, damage.Fire)}, Reductions: []dnd5eEvents.DamageReduction{reduce(1, damage.Cold)},
		},
		"no input": nil,
	}
	for name, in := range cases {
		s.Run(name, func() {
			_, err := combat.SettleDamage(in)
			s.Error(err)
		})
	}
}
