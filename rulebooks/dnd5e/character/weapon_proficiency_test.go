// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type WeaponProficiencyTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *WeaponProficiencyTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestWeaponProficiencySuite(t *testing.T) {
	suite.Run(t, new(WeaponProficiencyTestSuite))
}

// sheet loads a character carrying exactly the weapon proficiencies given —
// through the real loader, so the grants travel the same path a persisted
// character's do.
func (s *WeaponProficiencyTestSuite) sheet(profs ...proficiencies.Weapon) *Character {
	c, err := Load(s.ctx, &Data{
		ID:       "hero-1",
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:           12,
		MaxHitPoints:        12,
		ArmorClass:          16,
		ProficiencyBonus:    2,
		WeaponProficiencies: profs,
	})
	s.Require().NoError(err)

	return c
}

func (s *WeaponProficiencyTestSuite) weapon(id weapons.WeaponID) *weapons.Weapon {
	w, err := weapons.GetByID(id)
	s.Require().NoError(err)

	return &w
}

// THE HEADLINE. A fighter's real class grant — simple and martial, exactly
// what classes.Fighter hands out — covers both halves of the catalog.
func (s *WeaponProficiencyTestSuite) TestAFightersCategoryGrantsCoverSimpleAndMartial() {
	fighter := s.sheet(proficiencies.WeaponSimple, proficiencies.WeaponMartial)

	s.Assert().True(fighter.IsProficientWith(s.weapon(weapons.Longsword)), "martial")
	s.Assert().True(fighter.IsProficientWith(s.weapon(weapons.Club)), "simple")
	s.Assert().True(fighter.IsProficientWith(s.weapon(weapons.Greatsword)), "martial")
	s.Assert().True(fighter.IsProficientWith(s.weapon(weapons.Dagger)), "simple")
}

// A wizard's real grant list is specific, not categorical: it reaches the
// weapons it names and stops. This is the case that makes the compiler's
// proficiency branch worth having at all.
func (s *WeaponProficiencyTestSuite) TestAWizardsSpecificGrantsReachOnlyWhatTheyName() {
	wizard := s.sheet(
		proficiencies.WeaponDagger,
		proficiencies.WeaponDart,
		proficiencies.WeaponSling,
		proficiencies.WeaponQuarterstaff,
		proficiencies.WeaponLightCrossbow,
	)

	s.Assert().True(wizard.IsProficientWith(s.weapon(weapons.Dagger)))
	s.Assert().True(wizard.IsProficientWith(s.weapon(weapons.Quarterstaff)))

	s.Assert().False(wizard.IsProficientWith(s.weapon(weapons.Longsword)))
	s.Assert().False(wizard.IsProficientWith(s.weapon(weapons.Greatsword)))
	// A simple weapon it was not granted: proximity in category is not a grant.
	s.Assert().False(wizard.IsProficientWith(s.weapon(weapons.Club)))
	s.Assert().False(wizard.IsProficientWith(s.weapon(weapons.Handaxe)))
}

// One grant among several is enough — the scan is an any, not an all.
func (s *WeaponProficiencyTestSuite) TestOneMatchingGrantAmongManyIsEnough() {
	rogue := s.sheet(
		proficiencies.WeaponSimple,
		proficiencies.WeaponHandCrossbow,
		proficiencies.WeaponLongsword,
		proficiencies.WeaponRapier,
		proficiencies.WeaponShortsword,
	)

	// Reached by the last specific grant in the list, not the first.
	s.Assert().True(rogue.IsProficientWith(s.weapon(weapons.Shortsword)))
	// Reached by the category grant that leads the list.
	s.Assert().True(rogue.IsProficientWith(s.weapon(weapons.Mace)))
	// Reached by nothing.
	s.Assert().False(rogue.IsProficientWith(s.weapon(weapons.Greataxe)))
}

// A character with no weapon proficiencies is proficient with nothing in the
// catalog — the whole catalog, so a default-true implementation cannot hide.
func (s *WeaponProficiencyTestSuite) TestNoGrantsMeansProficientWithNothing() {
	commoner := s.sheet()

	for id, weapon := range weapons.All {
		s.Assert().False(commoner.IsProficientWith(&weapon), "%s", id)
	}
}

// And with both category grants, proficient with everything in the catalog.
func (s *WeaponProficiencyTestSuite) TestBothCategoryGrantsCoverTheWholeCatalog() {
	fighter := s.sheet(proficiencies.WeaponSimple, proficiencies.WeaponMartial)

	for id, weapon := range weapons.All {
		s.Assert().True(fighter.IsProficientWith(&weapon), "%s", id)
	}
}

// A nil weapon is not proficient rather than a panic: an empty equipment slot
// resolves to nil on the caller's path.
func (s *WeaponProficiencyTestSuite) TestANilWeaponIsNotProficient() {
	fighter := s.sheet(proficiencies.WeaponSimple, proficiencies.WeaponMartial)

	s.Assert().NotPanics(func() {
		s.Assert().False(fighter.IsProficientWith(nil))
	})
}
