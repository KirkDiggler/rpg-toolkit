// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
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
	s.Assert().Equal("bite", actions[0].GetID())

	// Check targeting strategy
	s.Assert().Equal(monster.TargetLowestHP, wolf.Targeting())
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

func (s *WolfTestSuite) TestWolfBitePublishesDefinitionAndPreservesKnockdownDC() {
	wolf := NewWolf("wolf-1")
	target := NewWolf("target")
	bus := events.NewEventBus()
	var received dnd5eEvents.AttackEvent
	_, err := dnd5eEvents.AttackTopic.On(bus).Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		received = event
		return nil
	})
	s.Require().NoError(err)

	for _, action := range wolf.Actions() {
		if action.GetID() == "bite" {
			err = action.Activate(context.Background(), wolf, monster.MonsterActionInput{
				Bus: bus, Target: target, Perception: &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1, Adjacent: true}}},
			})
			break
		}
	}
	s.Require().NoError(err)
	s.Equal(attack.Definition{
		ActionID:    "bite",
		DisplayName: "bite",
		Category:    attack.CategoryNatural,
		Bonus:       attack.FixedBonus(4),
		Targeting:   attack.MeleeReach(1),
		Damage: damage.DamageSpec{Pools: []damage.Damage{{
			Dice: "2d4+2", Terms: []damage.DiceTerm{{Dice: "2d4", Sign: 1}}, Type: damage.Piercing,
			FlatBonus: 2, Properties: []damage.Property{damage.PropertyCritEligible},
		}}},
	}, received.Definition)

	data := wolf.ToData()
	s.Require().Len(data.Actions, 1)
	var persisted actions.BiteConfig
	s.Require().NoError(json.Unmarshal(data.Actions[0].Config, &persisted))
	s.Equal(11, persisted.KnockdownDC)
}
