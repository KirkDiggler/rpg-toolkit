// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/stretchr/testify/suite"
)

const multiattackActionID = "multiattack"

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
	s.Assert().Equal(dnd5e.SizeLarge, bear.Size())

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
		case multiattackActionID:
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

func (s *BrownBearTestSuite) TestBrownBearBiteConvertsLegacyDamage() {
	bear := NewBrownBear("bear-1")
	bus := events.NewEventBus()
	var received dnd5eEvents.AttackEvent
	_, err := dnd5eEvents.AttackTopic.On(bus).Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		received = event
		return nil
	})
	s.Require().NoError(err)

	for _, action := range bear.Actions() {
		if action.GetID() != "bite" {
			continue
		}
		target := NewBrownBear("target")
		err = action.Activate(context.Background(), bear, monster.MonsterActionInput{
			Bus: bus, Target: target, Perception: &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1}}},
		})
		s.Require().NoError(err)
		break
	}

	s.Equal(attack.CategoryNatural, received.Definition.Category)
	s.Equal([]damage.Damage{{Dice: "1d8+4", Terms: []damage.DiceTerm{{Dice: "1d8", Sign: 1}}, Type: damage.Piercing, FlatBonus: 4, Properties: []damage.Property{damage.PropertyCritEligible}}}, received.Definition.Damage.Pools)
}
