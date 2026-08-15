// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// The external suite in proficiency_test.go keeps its own copy of the
// grant-to-weapon mapping so the two can disagree. That leaves one gap it
// cannot see by construction: a grant added to specificWeaponGrants and not
// to the test's list is simply never exercised. These two assertions close it
// from inside the package.
type ProficiencyTableTestSuite struct {
	suite.Suite
}

func TestProficiencyTableSuite(t *testing.T) {
	suite.Run(t, new(ProficiencyTableTestSuite))
}

// A count, deliberately. Adding a grant without adding it to the external
// suite's list fails here, which is the only place that can notice.
func (s *ProficiencyTableTestSuite) TestTheSpecificGrantTableHoldsEveryGrantTheTestsKnowAbout() {
	s.Assert().Len(specificWeaponGrants, 17,
		"a grant was added or removed — mirror it in proficiency_test.go's specificGrants")
}

// Every entry resolves against the catalog. A grant naming a weapon that does
// not exist reads as "not proficient" at runtime rather than erroring.
func (s *ProficiencyTableTestSuite) TestEveryTableEntryNamesACatalogWeapon() {
	for grant, id := range specificWeaponGrants {
		weapon, err := GetByID(id)
		s.Require().NoError(err, "grant %q names weapon %q", grant, id)
		s.Assert().Equal(id, weapon.ID)
	}
}
