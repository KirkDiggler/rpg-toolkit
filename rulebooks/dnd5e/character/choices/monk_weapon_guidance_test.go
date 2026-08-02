package choices

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MonkWeaponGuidanceTestSuite guards rpg-toolkit#873: the Monk starting
// equipment weapon choice must offer "simple melee weapon" (not the bare
// "simple weapon" superset) and must not resolve to
// weapons.CategorySimpleRanged - simple ranged weapons (dart, sling, light
// crossbow) are not part of a Monk's weapon set in this game. Shortsword
// stays the explicit, separate alternative option.
type MonkWeaponGuidanceTestSuite struct {
	suite.Suite
}

func TestMonkWeaponGuidanceSuite(t *testing.T) {
	suite.Run(t, new(MonkWeaponGuidanceTestSuite))
}

// findMonkWeaponOption locates the MonkWeaponSimple option inside the
// MonkWeaponsPrimary requirement, failing the test loudly if the shape has
// moved instead of silently skipping assertions.
func (s *MonkWeaponGuidanceTestSuite) findMonkWeaponOption() (req *EquipmentRequirement, opt *EquipmentOption) {
	reqs := GetClassRequirements(classes.Monk)
	s.Require().NotNil(reqs, "Monk should have requirements")

	for _, r := range reqs.Equipment {
		if r.ID != MonkWeaponsPrimary {
			continue
		}
		req = r
		for i := range r.Options {
			if r.Options[i].ID == MonkWeaponSimple {
				opt = &r.Options[i]
			}
		}
	}

	s.Require().NotNil(req, "MonkWeaponsPrimary requirement should exist")
	s.Require().NotNil(opt, "MonkWeaponSimple option should exist under MonkWeaponsPrimary")
	return req, opt
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_LabelSaysSimpleMeleeWeapon() {
	_, opt := s.findMonkWeaponOption()

	lower := strings.ToLower(opt.Label)
	s.Contains(lower, "simple melee weapon",
		"MonkWeaponSimple option label should say 'simple melee weapon', got %q", opt.Label)
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_CategoryChoiceLabelSaysSimpleMelee() {
	_, opt := s.findMonkWeaponOption()

	s.Require().Len(opt.CategoryChoices, 1, "MonkWeaponSimple should have exactly one category choice")
	lower := strings.ToLower(opt.CategoryChoices[0].Label)
	s.Contains(lower, "simple melee weapon",
		"MonkWeaponSimple category choice label should say 'simple melee weapon', got %q",
		opt.CategoryChoices[0].Label)
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_ExcludesSimpleRangedCategory() {
	_, opt := s.findMonkWeaponOption()

	s.Require().Len(opt.CategoryChoices, 1)
	categories := opt.CategoryChoices[0].Categories

	s.NotContains(categories, weapons.CategorySimpleRanged,
		"MonkWeaponSimple must NOT include CategorySimpleRanged - simple ranged weapons "+
			"(dart, sling, light crossbow) are not part of a Monk's weapon set")
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_IncludesSimpleMeleeCategory() {
	_, opt := s.findMonkWeaponOption()

	s.Require().Len(opt.CategoryChoices, 1)
	categories := opt.CategoryChoices[0].Categories

	s.Contains(categories, weapons.CategorySimpleMelee,
		"MonkWeaponSimple must include CategorySimpleMelee")
	s.Len(categories, 1,
		"MonkWeaponSimple should resolve to exactly one category (simple melee), got %v", categories)
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponShortsword_RemainsExplicitAlternative() {
	req, _ := s.findMonkWeaponOption()

	var shortswordOpt *EquipmentOption
	for i := range req.Options {
		if req.Options[i].ID == MonkWeaponShortsword {
			shortswordOpt = &req.Options[i]
		}
	}

	s.Require().NotNil(shortswordOpt, "MonkWeaponShortsword option should remain present")
	s.Require().Len(shortswordOpt.Items, 1)
	s.Equal(weapons.Shortsword, shortswordOpt.Items[0].ID,
		"MonkWeaponShortsword should grant a concrete shortsword, not a category choice")
	s.Empty(shortswordOpt.CategoryChoices,
		"MonkWeaponShortsword is a fixed item, not a category choice")

	// Choose exactly 1 between the two options.
	s.Equal(1, req.Choose, "Monk should choose exactly one weapon option")
	s.Len(req.Options, 2, "Monk weapon choice should offer exactly shortsword + simple melee weapon")
}
