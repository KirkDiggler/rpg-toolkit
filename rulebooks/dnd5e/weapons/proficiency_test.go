// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type ProficiencyCoverageTestSuite struct {
	suite.Suite
}

func TestProficiencyCoverageSuite(t *testing.T) {
	suite.Run(t, new(ProficiencyCoverageTestSuite))
}

// specificGrants is this test's own copy of the grant-to-weapon mapping,
// deliberately not read from the package. A table and its test that share one
// source agree by construction and prove nothing; two independent lists
// disagree the moment either drifts.
var specificGrants = map[proficiencies.Weapon]weapons.WeaponID{
	proficiencies.WeaponClub:          weapons.Club,
	proficiencies.WeaponDagger:        weapons.Dagger,
	proficiencies.WeaponDart:          weapons.Dart,
	proficiencies.WeaponJavelin:       weapons.Javelin,
	proficiencies.WeaponLightCrossbow: weapons.LightCrossbow,
	proficiencies.WeaponMace:          weapons.Mace,
	proficiencies.WeaponQuarterstaff:  weapons.Quarterstaff,
	proficiencies.WeaponShortbow:      weapons.Shortbow,
	proficiencies.WeaponSickle:        weapons.Sickle,
	proficiencies.WeaponSling:         weapons.Sling,
	proficiencies.WeaponSpear:         weapons.Spear,
	proficiencies.WeaponHandCrossbow:  weapons.HandCrossbow,
	proficiencies.WeaponLongbow:       weapons.Longbow,
	proficiencies.WeaponLongsword:     weapons.Longsword,
	proficiencies.WeaponRapier:        weapons.Rapier,
	proficiencies.WeaponScimitar:      weapons.Scimitar,
	proficiencies.WeaponShortsword:    weapons.Shortsword,
}

// THE HEADLINE. A category grant covers exactly its own category, for every
// weapon in the catalog — so adding a weapon cannot silently land outside the
// rule. Both directions: the matching grant covers, the other one does not.
func (s *ProficiencyCoverageTestSuite) TestEveryCatalogWeaponIsCoveredByItsOwnCategoryGrant() {
	s.Require().NotEmpty(weapons.All)

	for id, weapon := range weapons.All {
		s.Assert().Equal(weapon.IsSimple(), weapon.CoveredBy(proficiencies.WeaponSimple),
			"%s (%s) vs the simple grant", id, weapon.Category)
		s.Assert().Equal(weapon.IsMartial(), weapon.CoveredBy(proficiencies.WeaponMartial),
			"%s (%s) vs the martial grant", id, weapon.Category)

		// Exactly one of the two, never neither and never both: a weapon
		// whose category is neither simple nor martial would be coverable by
		// nothing, and would silently lose its wielder the proficiency bonus.
		s.Assert().NotEqual(weapon.IsSimple(), weapon.IsMartial(),
			"%s belongs to exactly one category (%s)", id, weapon.Category)
	}
}

// Every specific grant names a weapon the catalog actually holds. A grant
// naming a weapon that does not exist reads as "not proficient" at runtime —
// no error, just a quietly missing bonus.
func (s *ProficiencyCoverageTestSuite) TestSpecificGrantsNameRealWeapons() {
	for grant, id := range specificGrants {
		weapon, err := weapons.GetByID(id)
		s.Require().NoError(err, "grant %q names weapon %q", grant, id)
		s.Assert().True(weapon.CoveredBy(grant), "grant %q covers %q", grant, id)
	}
}

// A specific grant covers the one weapon it names and nothing else in the
// catalog — the exclusivity a "does the grant mention this weapon at all"
// implementation loses.
func (s *ProficiencyCoverageTestSuite) TestASpecificGrantCoversExactlyTheWeaponItNames() {
	for grant, named := range specificGrants {
		for id, weapon := range weapons.All {
			covered := weapon.CoveredBy(grant)
			if id == named {
				s.Assert().True(covered, "grant %q covers the %q it names", grant, id)
				continue
			}
			s.Assert().False(covered, "grant %q does not cover %q", grant, id)
		}
	}
}

// Worked case, spelled out rather than derived from a loop: the longswords
// grant is a martial-weapon grant's opposite number.
func (s *ProficiencyCoverageTestSuite) TestTheLongswordsGrantCoversOnlyTheLongsword() {
	longsword, err := weapons.GetByID(weapons.Longsword)
	s.Require().NoError(err)
	shortsword, err := weapons.GetByID(weapons.Shortsword)
	s.Require().NoError(err)

	s.Assert().True(longsword.CoveredBy(proficiencies.WeaponLongsword))
	s.Assert().False(shortsword.CoveredBy(proficiencies.WeaponLongsword))

	// And the category grant it belongs to, from the other side.
	s.Assert().True(longsword.CoveredBy(proficiencies.WeaponMartial))
	s.Assert().False(longsword.CoveredBy(proficiencies.WeaponSimple))
}

// A simple weapon is not covered by the martial grant, which is the branch a
// "category grant covers everything" mutation erases.
func (s *ProficiencyCoverageTestSuite) TestACategoryGrantDoesNotReachTheOtherCategory() {
	club, err := weapons.GetByID(weapons.Club)
	s.Require().NoError(err)

	s.Assert().True(club.CoveredBy(proficiencies.WeaponSimple))
	s.Assert().False(club.CoveredBy(proficiencies.WeaponMartial))
}

// A grant nobody has defined covers nothing rather than everything.
func (s *ProficiencyCoverageTestSuite) TestAnUnknownGrantCoversNothing() {
	for id, weapon := range weapons.All {
		s.Assert().False(weapon.CoveredBy(proficiencies.Weapon("greatswords")), "%s", id)
		s.Assert().False(weapon.CoveredBy(proficiencies.Weapon("")), "%s", id)
	}
}

// The singular weapon ID is not itself a grant: grants are plural nouns, and
// a lookup that fell back to matching the raw ID would make this pass.
func (s *ProficiencyCoverageTestSuite) TestAWeaponIDIsNotAGrant() {
	longsword, err := weapons.GetByID(weapons.Longsword)
	s.Require().NoError(err)

	s.Assert().False(longsword.CoveredBy(proficiencies.Weapon(weapons.Longsword)))
}
