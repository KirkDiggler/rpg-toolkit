// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/stretchr/testify/suite"
)

type BiteActionTestSuite struct {
	suite.Suite
	bus    events.EventBus
	roller dice.Roller
}

func TestBiteActionSuite(t *testing.T) {
	suite.Run(t, new(BiteActionTestSuite))
}

func (s *BiteActionTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.roller = dice.NewRoller()
}

func (s *BiteActionTestSuite) TestNewBiteAction() {
	// Arrange
	config := BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	}

	// Act
	action := NewBiteAction(config)

	// Assert
	s.Assert().NotNil(action)
	s.Assert().Equal("bite", action.GetID())
	s.Assert().Equal("monster-action", string(action.GetType()))
	s.Assert().Equal(monster.CostAction, action.Cost())
	s.Assert().Equal(monster.TypeMeleeAttack, action.ActionType())
}

func (s *BiteActionTestSuite) TestCanActivate_NoTarget() {
	// Arrange
	action := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "wolf-1"}
	input := monster.MonsterActionInput{
		Target: nil,
	}

	// Act
	err := action.CanActivate(context.Background(), owner, input)

	// Assert
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "no target")
}

func (s *BiteActionTestSuite) TestCanActivate_TargetOutOfReach() {
	// Arrange
	action := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "wolf-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 10, Y: 0},
				Distance: 10,
				Adjacent: false,
			},
		},
	}

	input := monster.MonsterActionInput{
		Target:     target,
		Perception: perception,
	}

	// Act
	err := action.CanActivate(context.Background(), owner, input)

	// Assert
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "not in melee range")
}

func (s *BiteActionTestSuite) TestCanActivate_TargetInReach() {
	// Arrange
	action := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "wolf-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 1, Y: 0},
				Distance: 5,
				Adjacent: true,
			},
		},
	}

	input := monster.MonsterActionInput{
		Target:     target,
		Perception: perception,
	}

	// Act
	err := action.CanActivate(context.Background(), owner, input)

	// Assert
	s.Assert().NoError(err)
}

func (s *BiteActionTestSuite) TestActivate_PublishesAttackEvent() {
	// Arrange
	action := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "wolf-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 1, Y: 0},
				Distance: 5,
				Adjacent: true,
			},
		},
	}

	// Subscribe to attack events
	var receivedAttackEvent *dnd5eEvents.AttackEvent
	attackTopic := dnd5eEvents.AttackTopic.On(s.bus)
	_, err := attackTopic.Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		receivedAttackEvent = &event
		return nil
	})
	s.Require().NoError(err)

	input := monster.MonsterActionInput{
		Bus:           s.bus,
		Target:        target,
		Perception:    perception,
		ActionEconomy: combat.NewActionEconomy(),
		Roller:        s.roller,
	}

	// Act
	err = action.Activate(context.Background(), owner, input)

	// Assert
	s.Assert().NoError(err)
	s.Assert().NotNil(receivedAttackEvent)
	s.Assert().Equal("wolf-1", receivedAttackEvent.AttackerID)
	s.Assert().Equal("hero-1", receivedAttackEvent.TargetID)
	s.Assert().Equal("bite", receivedAttackEvent.WeaponRef)
	s.Assert().True(receivedAttackEvent.IsMelee)
}

func (s *BiteActionTestSuite) TestScore_AdjacentEnemy() {
	// Arrange
	action := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	})

	m := monster.New(monster.Config{
		ID:   "wolf-1",
		Name: "Wolf",
		HP:   11,
		AC:   13,
	})
	perception := &monster.PerceptionData{
		Enemies: []monster.PerceivedEntity{
			{Adjacent: true},
		},
	}

	// Act
	score := action.Score(m, perception)

	// Assert - should have base score + adjacency bonus + knockdown bonus
	s.Assert().Greater(score, 60)
}

func (s *BiteActionTestSuite) TestScore_NoAdjacentEnemy() {
	// Arrange
	action := NewBiteAction(BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	})

	m := monster.New(monster.Config{
		ID:   "wolf-1",
		Name: "Wolf",
		HP:   11,
		AC:   13,
	})
	perception := &monster.PerceptionData{
		Enemies: []monster.PerceivedEntity{
			{Adjacent: false, Distance: 30},
		},
	}

	// Act
	score := action.Score(m, perception)

	// Assert - should have base score + knockdown bonus
	s.Assert().Greater(score, 50)
}

func (s *BiteActionTestSuite) TestToData() {
	// Arrange
	config := BiteConfig{
		AttackBonus: 4,
		DamageDice:  "2d4+2",
		KnockdownDC: 11,
		DamageType:  damage.Piercing,
	}
	action := NewBiteAction(config)

	// Act
	data := action.ToData()

	// Assert
	s.Assert().Equal("bite", data.Ref.ID)
	s.Assert().NotNil(data.Config)
	// Config should be valid JSON with our config
	s.Assert().Contains(string(data.Config), "2d4+2")
}
