// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Monster factory tests intentionally follow same structure with different values
package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/stretchr/testify/suite"
)

type GiantRatTestSuite struct {
	suite.Suite
}

func TestGiantRatSuite(t *testing.T) {
	suite.Run(t, new(GiantRatTestSuite))
}

func (s *GiantRatTestSuite) TestNewGiantRat() {
	rat := NewGiantRat("rat-1")

	s.Require().NotNil(rat)
	s.Assert().Equal("rat-1", rat.GetID())
	s.Assert().Equal("Giant Rat", rat.Name())

	// Check stats (CR 1/8)
	s.Assert().Equal(7, rat.HP())
	s.Assert().Equal(7, rat.MaxHP())
	s.Assert().Equal(12, rat.AC())

	// Check ability scores
	scores := rat.AbilityScores()
	s.Assert().Equal(7, scores[abilities.STR])
	s.Assert().Equal(15, scores[abilities.DEX])
	s.Assert().Equal(11, scores[abilities.CON])
	s.Assert().Equal(2, scores[abilities.INT])
	s.Assert().Equal(10, scores[abilities.WIS])
	s.Assert().Equal(4, scores[abilities.CHA])

	// Check speed
	speed := rat.Speed()
	s.Assert().Equal(30, speed.Walk)

	// Check actions - should have bite
	actions := rat.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal("bite", actions[0].GetID())
}

func (s *GiantRatTestSuite) TestGiantRatTraits() {
	// Giant rats have Pack Tactics
	// Note: Pack Tactics trait is applied when the monster is loaded into combat
	rat := NewGiantRat("rat-1")
	s.Require().NotNil(rat)

	// Rats are dexterous but weak
	scores := rat.AbilityScores()
	s.Assert().Equal(15, scores[abilities.DEX], "rats are dexterous")
	s.Assert().Equal(7, scores[abilities.STR], "rats are weak")
	s.Assert().Equal(2, scores[abilities.INT], "rats have animal intelligence")
}
