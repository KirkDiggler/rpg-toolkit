// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/stretchr/testify/suite"
)

type ZombieTestSuite struct {
	suite.Suite
}

func TestZombieSuite(t *testing.T) {
	suite.Run(t, new(ZombieTestSuite))
}

func (s *ZombieTestSuite) TestNewZombie() {
	zombie := NewZombie("zombie-1")

	s.Require().NotNil(zombie)
	s.Assert().Equal("zombie-1", zombie.GetID())
	s.Assert().Equal("Zombie", zombie.Name())

	// Check stats
	s.Assert().Equal(22, zombie.HP())
	s.Assert().Equal(22, zombie.MaxHP())
	s.Assert().Equal(8, zombie.AC())

	// Check ability scores
	scores := zombie.AbilityScores()
	s.Assert().Equal(13, scores[abilities.STR])
	s.Assert().Equal(6, scores[abilities.DEX])
	s.Assert().Equal(16, scores[abilities.CON])
	s.Assert().Equal(3, scores[abilities.INT])
	s.Assert().Equal(6, scores[abilities.WIS])
	s.Assert().Equal(5, scores[abilities.CHA])

	// Check speed
	speed := zombie.Speed()
	s.Assert().Equal(20, speed.Walk)

	// Check actions - should have slam attack
	actions := zombie.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal("slam", actions[0].GetID())
}

func (s *ZombieTestSuite) TestZombieTraits() {
	// Test that zombie can be created
	// Note: Undead Fortitude trait (CON save to stay at 1 HP) is applied
	// when the monster is loaded into combat with an event bus.
	zombie := NewZombie("zombie-1")
	s.Require().NotNil(zombie)

	// Zombies have high CON (+3) for Undead Fortitude
	scores := zombie.AbilityScores()
	s.Assert().Equal(16, scores[abilities.CON], "zombies have high CON for undead fortitude")
	s.Assert().Equal(6, scores[abilities.DEX], "zombies are very slow")
}
