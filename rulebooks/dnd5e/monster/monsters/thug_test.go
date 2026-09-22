// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
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

	// Multiattack first, then the components. Melee before ranged among
	// those, so a thug standing over you swings rather than shoots.
	actions := thug.Actions()
	s.Require().GreaterOrEqual(len(actions), 3)
	s.Equal(refs.MonsterActions.ThugMultiattack(), &actions[0].Ref)
	s.Require().NotNil(actions[0].Sequence)
	s.Equal([]combatActions.SequenceStep{
		{Action: *refs.Weapons.Mace()},
		{Action: *refs.Weapons.Mace()},
	}, actions[0].Sequence.Steps, "two melee attacks, and the mace is the only melee weapon a thug carries")
	s.Equal(refs.Weapons.Mace(), &actions[1].Ref)
	s.Equal(refs.Weapons.HeavyCrossbow(), &actions[2].Ref)
}

func (s *ThugTestSuite) TestThugTraits() {
	// Thugs have Pack Tactics, and their mace is what their Multiattack swings.
	thug := NewThug("thug-1")
	s.Require().NotNil(thug)

	// Thugs are strong and tough
	scores := thug.AbilityScores()
	s.Assert().Equal(15, scores[abilities.STR], "thugs are strong")
	s.Assert().Equal(14, scores[abilities.CON], "thugs are tough")
	s.Assert().Equal(32, thug.HP(), "thugs have high HP")
}
