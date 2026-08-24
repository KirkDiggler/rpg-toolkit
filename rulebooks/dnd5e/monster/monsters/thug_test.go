// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type ThugTestSuite struct {
	suite.Suite
}

func TestThugSuite(t *testing.T) {
	suite.Run(t, new(ThugTestSuite))
}

func (s *ThugTestSuite) TestNewThug() {
	thug := NewThug("thug-1")

	s.Require().NotNil(thug)
	s.Assert().Equal("thug-1", thug.GetID())
	s.Assert().Equal("Thug", thug.Name())

	// Check stats (CR 1 boss)
	s.Assert().Equal(32, thug.HP())
	s.Assert().Equal(32, thug.MaxHP())
	s.Assert().Equal(11, thug.AC())

	// Check ability scores
	scores := thug.AbilityScores()
	s.Assert().Equal(15, scores[abilities.STR])
	s.Assert().Equal(11, scores[abilities.DEX])
	s.Assert().Equal(14, scores[abilities.CON])
	s.Assert().Equal(10, scores[abilities.INT])
	s.Assert().Equal(10, scores[abilities.WIS])
	s.Assert().Equal(11, scores[abilities.CHA])

	// Check speed
	speed := thug.Speed()
	s.Assert().Equal(30, speed.Walk)

	// The component attack remains; multiattack waits for a sequence profile.
	actions := thug.Actions()
	s.Require().Len(actions, 1)
	s.Equal(refs.MonsterActions.ThugMace(), &actions[0].Ref)
}

func (s *ThugTestSuite) TestThugTraits() {
	// Thugs have Pack Tactics; their mace remains available while sequence attacks are deferred.
	thug := NewThug("thug-1")
	s.Require().NotNil(thug)

	// Thugs are strong and tough
	scores := thug.AbilityScores()
	s.Assert().Equal(15, scores[abilities.STR], "thugs are strong")
	s.Assert().Equal(14, scores[abilities.CON], "thugs are tough")
	s.Assert().Equal(32, thug.HP(), "thugs have high HP")
}
