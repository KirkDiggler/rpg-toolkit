// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package classes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// MonkDescriptionTestSuite guards the accuracy of Monk's weapon guidance in
// Description(Monk) (rpg-toolkit#873/#874): it must state the actual
// "monk weapon" set used by the Martial Arts rule engine
// (conditions.isMonkWeapon / actions.isMonkWeapon) - shortswords and simple
// melee weapons without the Heavy or Two-Handed property - so a reader of
// the description doesn't come away with a wrong picture of what a Monk's
// Martial Arts feature applies to.
//
// This is deliberately narrower than (and must not be confused with) the
// Monk's 2014 *starting equipment* choice, which offers "a shortsword or
// any simple weapon" (melee OR ranged) - see monk_weapon_guidance_test.go
// in character/choices for that contract.
type MonkDescriptionTestSuite struct {
	suite.Suite
}

func TestMonkDescriptionSuite(t *testing.T) {
	suite.Run(t, new(MonkDescriptionTestSuite))
}

func (s *MonkDescriptionTestSuite) TestDescription_Monk_MentionsShortswordAndSimpleMelee() {
	desc := Description(Monk)
	lower := strings.ToLower(desc)

	s.Contains(lower, "shortsword",
		"Monk description should mention shortswords as a monk weapon")
	s.Contains(lower, "simple melee weapon",
		"Monk description should state monk weapons are simple MELEE weapons")
}

func (s *MonkDescriptionTestSuite) TestDescription_Monk_ExcludesHeavyAndTwoHanded() {
	desc := Description(Monk)
	lower := strings.ToLower(desc)

	// Must match the actual rule engine's exclusions
	// (conditions.isMonkWeapon / actions.isMonkWeapon check Heavy and
	// Two-Handed - PropertySpecial is declared but never assigned to any
	// weapon and never checked anywhere in this codebase, so it must not
	// be claimed here).
	s.Contains(lower, "heavy",
		"Monk description should note Heavy weapons are excluded from the monk weapon set")
	s.Contains(lower, "two-handed",
		"Monk description should note Two-Handed weapons are excluded from the monk weapon set "+
			"(matches isMonkWeapon's actual check, not the unused PropertySpecial)")
	s.NotContains(lower, "special",
		"Monk description must not claim a 'Special' exclusion - PropertySpecial is dead in this "+
			"codebase and isMonkWeapon() does not check it")
}

func (s *MonkDescriptionTestSuite) TestDescription_Monk_DoesNotClaimSimpleRanged() {
	desc := Description(Monk)
	lower := strings.ToLower(desc)

	// "simple ranged" must not appear in the description - the Martial
	// Arts monk-weapon set is scoped to MELEE simple weapons plus
	// shortswords. (Simple ranged weapons remain a valid *starting
	// equipment* pick, per the 2014 "any simple weapon" text - that's a
	// separate, broader contract covered in the choices package.)
	s.NotContains(lower, "simple ranged",
		"Monk description must not claim simple ranged weapons as monk weapons")
}

func (s *MonkDescriptionTestSuite) TestDescription_OtherClasses_Unchanged() {
	// Guard against collateral edits: only Monk's description gained
	// weapon-guidance text in this change.
	s.NotEmpty(Description(Fighter))
	s.NotContains(strings.ToLower(Description(Fighter)), "heavy or two-handed")
	s.NotEmpty(Description(Barbarian))
}
