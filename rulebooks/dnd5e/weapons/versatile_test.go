// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type VersatileTestSuite struct {
	suite.Suite
}

func TestVersatileSuite(t *testing.T) {
	suite.Run(t, new(VersatileTestSuite))
}

// THE HEADLINE. Every versatile weapon in the catalog, gripped two-handed,
// steps to the exact notation the ruleset prints for it — real content, not a
// synthetic "1d8".
func (s *VersatileTestSuite) TestEveryCatalogVersatileWeaponStepsToItsPrintedDie() {
	stepped := map[weapons.WeaponID]string{
		weapons.Battleaxe:    "1d10",
		weapons.Longsword:    "1d10",
		weapons.Warhammer:    "1d10",
		weapons.Quarterstaff: "1d8",
		weapons.Spear:        "1d8",
		weapons.Trident:      "1d8",
	}

	for id, want := range stepped {
		weapon, err := weapons.GetByID(id)
		s.Require().NoError(err)
		s.Require().True(weapon.HasProperty(weapons.PropertyVersatile), "%s is versatile", id)

		pools, err := weapon.DamageForGrip(true)
		s.Require().NoError(err)
		primary, ok := weapon.PrimaryDamage()
		s.Require().True(ok)
		for _, pool := range pools {
			if pool.Type == primary.Type && pool.HasProperty(damage.AddsAttackAbilityModifier) {
				s.Assert().Equal(want, pool.Dice, "%s two-handed", id)
				break
			}
		}
	}
}

// The catalog's versatile weapons only exercise two rows of the step-up table
// (1d6 and 1d8). All four rows are asserted directly so dropping or changing
// any single entry fails, not just the two content happens to reach.
func (s *VersatileTestSuite) TestTheStepUpTableIsExactAtEveryRow() {
	s.Assert().Equal("1d6", weapons.VersatileTwoHandedDamage("1d4"))
	s.Assert().Equal("1d8", weapons.VersatileTwoHandedDamage("1d6"))
	s.Assert().Equal("1d10", weapons.VersatileTwoHandedDamage("1d8"))
	s.Assert().Equal("1d12", weapons.VersatileTwoHandedDamage("1d10"))
}

// A d12 is the top of the progression: nothing steps past it.
func (s *VersatileTestSuite) TestADieAboveTheTableDoesNotStep() {
	s.Assert().Equal("1d12", weapons.VersatileTwoHandedDamage("1d12"))
	s.Assert().Equal("1d20", weapons.VersatileTwoHandedDamage("1d20"))
}

// The die count rides through untouched — only the size steps. Pins the
// count against a step that rebuilds the notation as "1d{next}".
func (s *VersatileTestSuite) TestTheDieCountIsPreserved() {
	s.Assert().Equal("2d8", weapons.VersatileTwoHandedDamage("2d6"))
	s.Assert().Equal("3d12", weapons.VersatileTwoHandedDamage("3d10"))
}

// TestTwoHandsReplaceOnlyMarkedPrimaryPool protects the compiler boundary:
// versatile grip changes the base weapon pool only, never a separately
// declared rider, and never the catalog declaration itself.
func (s *VersatileTestSuite) TestTwoHandsReplaceOnlyMarkedPrimaryPool() {
	w := weapons.Weapon{Properties: []weapons.WeaponProperty{weapons.PropertyVersatile}, Damage: []damage.Damage{
		{Dice: "1d8", Type: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
		{Dice: "1d6", Type: damage.Fire},
	}}

	got, err := w.DamageForGrip(true)
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Assert().Equal("1d10", got[0].Dice)
	s.Assert().Equal("1d6", got[1].Dice)
	s.Assert().Equal("1d8", w.Damage[0].Dice, "compiler helpers must not mutate catalog content")
}

func (s *VersatileTestSuite) TestVersatileWeaponRequiresExactlyOnePrimaryPool() {
	w := weapons.Weapon{Properties: []weapons.WeaponProperty{weapons.PropertyVersatile}, Damage: []damage.Damage{
		{Dice: "1d8", Type: damage.Slashing},
	}}

	_, err := w.DamageForGrip(true)
	s.Require().Error(err)
}

// Notation that is not "NdM" passes through. Weapon declarations themselves
// are canonical pure-NdM pools, but this helper remains total for callers that
// have not yet validated input.
func (s *VersatileTestSuite) TestNonDiceNotationPassesThrough() {
	s.Assert().Equal("1", weapons.VersatileTwoHandedDamage("1"))
	s.Assert().Equal("0", weapons.VersatileTwoHandedDamage("0"))
}

func (s *VersatileTestSuite) TestUnparseableNotationPassesThrough() {
	s.Assert().Equal("", weapons.VersatileTwoHandedDamage(""))
	s.Assert().Equal("d8", weapons.VersatileTwoHandedDamage("d8"))
	s.Assert().Equal("1dX", weapons.VersatileTwoHandedDamage("1dX"))
	s.Assert().Equal("greatsword", weapons.VersatileTwoHandedDamage("greatsword"))
}

// A weapon without the property reports its one-handed die whatever the grip:
// a greatsword is two-handed, not versatile, and 2d6 never becomes 2d8.
func (s *VersatileTestSuite) TestANonVersatileWeaponNeverSteps() {
	greatsword, err := weapons.GetByID(weapons.Greatsword)
	s.Require().NoError(err)
	s.Require().False(greatsword.HasProperty(weapons.PropertyVersatile))
	greatswordDamage, err := greatsword.DamageForGrip(true)
	s.Require().NoError(err)
	s.Assert().Equal("2d6", greatswordDamage[0].Dice)

	// A rapier's die IS on the step-up table — the property, not the die, is
	// what decides. This is the case a check-the-table-not-the-property
	// implementation gets wrong.
	rapier, err := weapons.GetByID(weapons.Rapier)
	s.Require().NoError(err)
	s.Require().False(rapier.HasProperty(weapons.PropertyVersatile))
	rapierDamage, err := rapier.DamageForGrip(true)
	s.Require().NoError(err)
	s.Assert().Equal("1d8", rapierDamage[0].Dice)
}

// Every non-versatile weapon returns its declared pools verbatim.
func (s *VersatileTestSuite) TestDamageForGripMatchesDamageForEveryNonVersatileWeapon() {
	checked := 0
	for id, weapon := range weapons.All {
		if weapon.HasProperty(weapons.PropertyVersatile) {
			continue
		}
		got, err := weapon.DamageForGrip(true)
		s.Require().NoError(err)
		s.Assert().Equal(weapon.Damage, got, "%s is not versatile", id)
		checked++
	}
	s.Require().Positive(checked, "the catalog holds non-versatile weapons")
}
