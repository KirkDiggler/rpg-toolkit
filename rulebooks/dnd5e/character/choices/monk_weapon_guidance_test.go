// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MonkWeaponGuidanceTestSuite guards rpg-toolkit#873/#874: the Monk
// *starting equipment* weapon choice is the 2014 PHB "a shortsword or any
// simple weapon" option, and it must stay that broad - offering simple
// MELEE and simple RANGED weapons alike, plus the explicit shortsword
// alternative.
//
// This is deliberately broader than (and must not be confused with) the
// Martial Arts "monk weapon" set described in classes.Description(Monk)
// and enforced by conditions/actions.isMonkWeapon (shortsword + simple
// melee weapons without Heavy/Two-Handed). Narrowing the starting-equipment
// picker to melee-only would incorrectly block legitimate 2014 starting
// gear picks (dart, sling, light crossbow) that a Monk is entitled to at
// character creation even though they won't benefit from Martial Arts.
// See monk_description_test.go in classes for the Martial Arts contract.
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

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_LabelSaysAnySimpleWeapon() {
	_, opt := s.findMonkWeaponOption()

	lower := strings.ToLower(opt.Label)
	s.Contains(lower, "any simple weapon",
		"MonkWeaponSimple option label should say 'any simple weapon' (2014 PHB starting "+
			"equipment text), got %q", opt.Label)
	s.NotContains(lower, "melee",
		"MonkWeaponSimple label must not be narrowed to melee-only - starting equipment is "+
			"'any simple weapon', got %q", opt.Label)
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_CategoryChoiceLabelSaysSimpleWeapon() {
	_, opt := s.findMonkWeaponOption()

	s.Require().Len(opt.CategoryChoices, 1, "MonkWeaponSimple should have exactly one category choice")
	lower := strings.ToLower(opt.CategoryChoices[0].Label)
	s.Contains(lower, "simple weapon",
		"MonkWeaponSimple category choice label should say 'simple weapon', got %q",
		opt.CategoryChoices[0].Label)
	s.NotContains(lower, "melee",
		"MonkWeaponSimple category choice label must not be narrowed to melee-only, got %q",
		opt.CategoryChoices[0].Label)
}

func (s *MonkWeaponGuidanceTestSuite) TestMonkWeaponSimple_IncludesBothSimpleCategories() {
	_, opt := s.findMonkWeaponOption()

	s.Require().Len(opt.CategoryChoices, 1)
	categories := opt.CategoryChoices[0].Categories

	s.Contains(categories, weapons.CategorySimpleMelee,
		"MonkWeaponSimple must include CategorySimpleMelee")
	s.Contains(categories, weapons.CategorySimpleRanged,
		"MonkWeaponSimple must include CategorySimpleRanged - the 2014 Monk starting equipment "+
			"option is 'any simple weapon', which includes ranged simple weapons "+
			"(dart, sling, light crossbow), unlike the narrower Martial Arts 'monk weapon' set")
	s.Len(categories, 2,
		"MonkWeaponSimple should resolve to exactly both simple categories (melee + ranged), got %v",
		categories)
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
	s.Len(req.Options, 2, "Monk weapon choice should offer exactly shortsword + any simple weapon")
}
