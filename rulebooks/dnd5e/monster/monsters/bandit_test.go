// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Monster factory tests intentionally follow same structure with different values
package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/stretchr/testify/suite"
)

type BanditTestSuite struct {
	suite.Suite
}

func TestBanditSuite(t *testing.T) {
	suite.Run(t, new(BanditTestSuite))
}

func (s *BanditTestSuite) TestNewBanditMelee() {
	bandit := NewBanditMelee("bandit-1")

	s.Require().NotNil(bandit)
	s.Assert().Equal("bandit-1", bandit.GetID())
	s.Assert().Equal("Bandit", bandit.Name())

	// Check stats (CR 1/8)
	s.Assert().Equal(11, bandit.HP())
	s.Assert().Equal(11, bandit.MaxHP())
	s.Assert().Equal(12, bandit.AC())

	// Check ability scores
	scores := bandit.AbilityScores()
	s.Assert().Equal(11, scores[abilities.STR])
	s.Assert().Equal(12, scores[abilities.DEX])
	s.Assert().Equal(12, scores[abilities.CON])
	s.Assert().Equal(10, scores[abilities.INT])
	s.Assert().Equal(10, scores[abilities.WIS])
	s.Assert().Equal(10, scores[abilities.CHA])

	// Check speed
	speed := bandit.Speed()
	s.Assert().Equal(30, speed.Walk)

	// Check actions - should have scimitar
	actions := bandit.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal("scimitar", actions[0].GetID())
}

func (s *BanditTestSuite) TestNewBanditRanged() {
	bandit := NewBanditRanged("bandit-2")

	s.Require().NotNil(bandit)
	s.Assert().Equal("bandit-2", bandit.GetID())
	s.Assert().Equal("Bandit", bandit.Name())

	// Check stats (CR 1/8)
	s.Assert().Equal(11, bandit.HP())
	s.Assert().Equal(11, bandit.MaxHP())
	s.Assert().Equal(12, bandit.AC())

	// Check ability scores (same as melee)
	scores := bandit.AbilityScores()
	s.Assert().Equal(11, scores[abilities.STR])
	s.Assert().Equal(12, scores[abilities.DEX])
	s.Assert().Equal(12, scores[abilities.CON])

	// Check speed
	speed := bandit.Speed()
	s.Assert().Equal(30, speed.Walk)

	// Check actions - should have light crossbow
	actions := bandit.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal("light crossbow", actions[0].GetID())
}

func (s *BanditTestSuite) TestBanditTraits() {
	// Bandits are average humans with no special traits
	melee := NewBanditMelee("bandit-1")
	ranged := NewBanditRanged("bandit-2")

	s.Require().NotNil(melee)
	s.Require().NotNil(ranged)

	// Both have average ability scores (10-12 range)
	meleeScores := melee.AbilityScores()
	rangedScores := ranged.AbilityScores()

	for _, ability := range []abilities.Ability{
		abilities.STR, abilities.DEX, abilities.CON,
		abilities.INT, abilities.WIS, abilities.CHA,
	} {
		s.Assert().GreaterOrEqual(meleeScores[ability], 10, "bandits have average or above stats")
		s.Assert().LessOrEqual(meleeScores[ability], 12, "bandits are not exceptional")
		s.Assert().Equal(meleeScores[ability], rangedScores[ability], "melee and ranged have same stats")
	}
}
