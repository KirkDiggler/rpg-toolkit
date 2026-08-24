// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/stretchr/testify/suite"
)

type WolfTestSuite struct {
	suite.Suite
}

func TestWolfSuite(t *testing.T) {
	suite.Run(t, new(WolfTestSuite))
}

func (s *WolfTestSuite) TestNewWolf() {
	wolf := NewWolf("wolf-1")

	s.Require().NotNil(wolf)
	s.Assert().Equal("wolf-1", wolf.GetID())
	s.Assert().Equal("Wolf", wolf.Name())

	// Check stats (CR 1/4)
	s.Assert().Equal(11, wolf.HP())
	s.Assert().Equal(11, wolf.MaxHP())
	s.Assert().Equal(13, wolf.AC())

	// Check ability scores
	scores := wolf.AbilityScores()
	s.Assert().Equal(12, scores[abilities.STR])
	s.Assert().Equal(15, scores[abilities.DEX])
	s.Assert().Equal(12, scores[abilities.CON])
	s.Assert().Equal(3, scores[abilities.INT])
	s.Assert().Equal(12, scores[abilities.WIS])
	s.Assert().Equal(6, scores[abilities.CHA])

	// Check speed
	speed := wolf.Speed()
	s.Assert().Equal(40, speed.Walk)

	// Check actions - should have bite (with knockdown)
	actions := wolf.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal(refs.MonsterActions.WolfBite(), &actions[0].Ref)

	// Check targeting strategy
	s.Assert().Equal(monster.TargetLowestHP, wolf.Targeting())
}

func (s *WolfTestSuite) TestBiteIsACompleteSharedDefinition() {
	data := NewWolf("wolf-1").ToData()
	s.Require().Len(data.Actions, 1)

	bite := data.Actions[0]
	s.Equal(refs.MonsterActions.WolfBite(), &bite.Ref)
	s.Equal("Bite", bite.Name)
	s.Require().NotNil(bite.Attack)
	s.Equal(combatActions.AttackCategoryWeapon, bite.Attack.Category)
	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 5}, bite.Attack.Delivery.Melee)
	s.Equal(4, bite.Attack.AttackBonus)
	s.Equal([]damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}}, bite.Attack.Damage)
	s.Require().Len(bite.Attack.OnHit, 1)
	s.Equal(refs.Conditions.Prone(), &bite.Attack.OnHit[0].Ref)
	s.Equal(saves.NewSaveGate(abilities.STR, 11), bite.Attack.OnHit[0].Save)
}

func (s *WolfTestSuite) TestWolfTraits() {
	// Wolves have Pack Tactics and target lowest HP (focus wounded prey)
	wolf := NewWolf("wolf-1")
	s.Require().NotNil(wolf)

	// Wolves are fast and perceptive
	scores := wolf.AbilityScores()
	s.Assert().Equal(15, scores[abilities.DEX], "wolves are agile")
	s.Assert().Equal(12, scores[abilities.WIS], "wolves have keen senses")

	// Wolves are fast
	speed := wolf.Speed()
	s.Assert().Equal(40, speed.Walk, "wolves are faster than most creatures")
}
