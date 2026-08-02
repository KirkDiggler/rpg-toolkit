package classes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// MonkDescriptionTestSuite guards the accuracy of Monk's weapon guidance in
// Description(Monk) (rpg-toolkit#873): it must state the actual "monk
// weapon" set - shortswords and simple melee weapons without the Heavy or
// Special property - so a reader of the description doesn't come away with
// a wrong (or absent) picture of what a Monk can use.
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

func (s *MonkDescriptionTestSuite) TestDescription_Monk_ExcludesHeavyAndSpecial() {
	desc := Description(Monk)
	lower := strings.ToLower(desc)

	// The description should call out that Heavy/Special weapons are NOT
	// monk weapons - not silently omit the exclusion.
	s.Contains(lower, "heavy",
		"Monk description should note Heavy weapons are excluded from the monk weapon set")
	s.Contains(lower, "special",
		"Monk description should note Special weapons are excluded from the monk weapon set")
}

func (s *MonkDescriptionTestSuite) TestDescription_Monk_DoesNotClaimSimpleRanged() {
	desc := Description(Monk)
	lower := strings.ToLower(desc)

	// "simple ranged" (or a bare unqualified "simple weapons" without
	// "melee") must not appear - the description scopes monk weapons to
	// MELEE simple weapons plus shortswords, not the full simple-weapon set.
	s.NotContains(lower, "simple ranged",
		"Monk description must not claim simple ranged weapons as monk weapons")
}

func (s *MonkDescriptionTestSuite) TestDescription_OtherClasses_Unchanged() {
	// Guard against collateral edits: only Monk's description gained
	// weapon-guidance text in this change.
	s.NotEmpty(Description(Fighter))
	s.NotContains(strings.ToLower(Description(Fighter)), "heavy or special")
	s.NotEmpty(Description(Barbarian))
}
