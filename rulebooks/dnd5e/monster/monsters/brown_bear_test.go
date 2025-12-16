// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/stretchr/testify/suite"
)

type BrownBearTestSuite struct {
	suite.Suite
}

func TestBrownBearSuite(t *testing.T) {
	suite.Run(t, new(BrownBearTestSuite))
}

func (s *BrownBearTestSuite) TestNewBrownBear() {
	bear := NewBrownBear("bear-1")

	s.Require().NotNil(bear)
	s.Assert().Equal("bear-1", bear.GetID())
	s.Assert().Equal("Brown Bear", bear.Name())

	// Check stats (CR 1 boss)
	s.Assert().Equal(34, bear.HP())
	s.Assert().Equal(34, bear.MaxHP())
	s.Assert().Equal(11, bear.AC())

	// Check ability scores
	scores := bear.AbilityScores()
	s.Assert().Equal(19, scores[abilities.STR])
	s.Assert().Equal(10, scores[abilities.DEX])
	s.Assert().Equal(16, scores[abilities.CON])
	s.Assert().Equal(2, scores[abilities.INT])
	s.Assert().Equal(13, scores[abilities.WIS])
	s.Assert().Equal(7, scores[abilities.CHA])

	// Check speed
	speed := bear.Speed()
	s.Assert().Equal(40, speed.Walk)
	s.Assert().Equal(30, speed.Climb)

	// Check actions - should have multiattack, bite, and claw
	actions := bear.Actions()
	s.Require().Len(actions, 3)

	// Find multiattack, bite, and claw
	var hasMultiattack, hasBite, hasClaw bool
	for _, action := range actions {
		switch action.GetID() {
		case "multiattack":
			hasMultiattack = true
		case "bite":
			hasBite = true
		case "claw":
			hasClaw = true
		}
	}
	s.Assert().True(hasMultiattack, "should have multiattack action")
	s.Assert().True(hasBite, "should have bite action")
	s.Assert().True(hasClaw, "should have claw action")
}

func (s *BrownBearTestSuite) TestBrownBearTraits() {
	// Bears are strong and tough
	bear := NewBrownBear("bear-1")
	s.Require().NotNil(bear)

	scores := bear.AbilityScores()
	s.Assert().Equal(19, scores[abilities.STR], "bears are very strong")
	s.Assert().Equal(16, scores[abilities.CON], "bears are tough")
	s.Assert().Equal(2, scores[abilities.INT], "bears have animal intelligence")

	// Bears can climb
	speed := bear.Speed()
	s.Assert().Equal(30, speed.Climb, "bears can climb")
}
