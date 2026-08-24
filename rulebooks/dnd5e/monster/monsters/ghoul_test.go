// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type GhoulTestSuite struct {
	suite.Suite
}

func TestGhoulSuite(t *testing.T) {
	suite.Run(t, new(GhoulTestSuite))
}

func (s *GhoulTestSuite) TestNewGhoul() {
	ghoul := NewGhoul("ghoul-1")

	s.Require().NotNil(ghoul)
	s.Assert().Equal("ghoul-1", ghoul.GetID())
	s.Assert().Equal("Ghoul", ghoul.Name())

	// Check stats (CR 1 boss)
	s.Assert().Equal(22, ghoul.HP())
	s.Assert().Equal(22, ghoul.MaxHP())
	s.Assert().Equal(12, ghoul.AC())

	// Check ability scores
	scores := ghoul.AbilityScores()
	s.Assert().Equal(13, scores[abilities.STR])
	s.Assert().Equal(15, scores[abilities.DEX])
	s.Assert().Equal(10, scores[abilities.CON])
	s.Assert().Equal(7, scores[abilities.INT])
	s.Assert().Equal(10, scores[abilities.WIS])
	s.Assert().Equal(6, scores[abilities.CHA])

	// Check speed
	speed := ghoul.Speed()
	s.Assert().Equal(30, speed.Walk)

	// Component attacks remain available; multiattack waits for a sequence profile.
	actions := ghoul.Actions()
	s.Require().Len(actions, 2)
	s.Equal(refs.MonsterActions.GhoulBite(), &actions[0].Ref)
	s.Equal(refs.MonsterActions.GhoulClaw(), &actions[1].Ref)
	s.NotEqual(actions[0].Ref, actions[1].Ref)
}

func (s *GhoulTestSuite) TestGhoulTraits() {
	// Ghouls have paralyzing touch (on claw hit)
	// Note: The paralysis effect would be implemented in a full combat system
	// For now, we just verify the ghoul has the correct stats and actions
	ghoul := NewGhoul("ghoul-1")
	s.Require().NotNil(ghoul)

	// Ghouls are dexterous (DEX 15) and moderately strong
	scores := ghoul.AbilityScores()
	s.Assert().Equal(15, scores[abilities.DEX], "ghouls are dexterous")
	s.Assert().Equal(13, scores[abilities.STR], "ghouls are moderately strong")
}
