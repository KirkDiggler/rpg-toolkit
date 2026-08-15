// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

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

		s.Assert().Equal(want, weapon.VersatileDamage(), "%s two-handed", id)
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

// Notation that is not "NdM" passes through, and the catalog is where that
// case comes from rather than a hypothetical: a blowgun deals "1" and a net
// deals "0". Stepping either would invent a die the weapon has not got.
func (s *VersatileTestSuite) TestNonDiceNotationPassesThrough() {
	blowgun, err := weapons.GetByID(weapons.Blowgun)
	s.Require().NoError(err)
	s.Assert().Equal("1", blowgun.Damage)
	s.Assert().Equal("1", weapons.VersatileTwoHandedDamage(blowgun.Damage))

	net, err := weapons.GetByID(weapons.Net)
	s.Require().NoError(err)
	s.Assert().Equal("0", net.Damage)
	s.Assert().Equal("0", weapons.VersatileTwoHandedDamage(net.Damage))
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
	s.Assert().Equal("2d6", greatsword.VersatileDamage())

	// A rapier's die IS on the step-up table — the property, not the die, is
	// what decides. This is the case a check-the-table-not-the-property
	// implementation gets wrong.
	rapier, err := weapons.GetByID(weapons.Rapier)
	s.Require().NoError(err)
	s.Require().False(rapier.HasProperty(weapons.PropertyVersatile))
	s.Assert().Equal("1d8", rapier.VersatileDamage())
}

// Every non-versatile weapon in the catalog reports Damage verbatim.
func (s *VersatileTestSuite) TestVersatileDamageMatchesDamageForEveryNonVersatileWeapon() {
	checked := 0
	for id, weapon := range weapons.All {
		if weapon.HasProperty(weapons.PropertyVersatile) {
			continue
		}
		s.Assert().Equal(weapon.Damage, weapon.VersatileDamage(), "%s is not versatile", id)
		checked++
	}
	s.Require().Positive(checked, "the catalog holds non-versatile weapons")
}
