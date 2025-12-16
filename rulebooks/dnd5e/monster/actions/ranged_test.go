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

type RangedActionTestSuite struct {
	suite.Suite
	bus    events.EventBus
	roller dice.Roller
}

func TestRangedActionSuite(t *testing.T) {
	suite.Run(t, new(RangedActionTestSuite))
}

func (s *RangedActionTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.roller = dice.NewRoller()
}

func (s *RangedActionTestSuite) TestNewRangedAction() {
	// Arrange
	config := RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	}

	// Act
	action := NewRangedAction(config)

	// Assert
	s.Assert().NotNil(action)
	s.Assert().Equal("shortbow", action.GetID())
	s.Assert().Equal("monster-action", string(action.GetType()))
	s.Assert().Equal(monster.CostAction, action.Cost())
	s.Assert().Equal(monster.TypeRangedAttack, action.ActionType())
}

func (s *RangedActionTestSuite) TestCanActivate_NoTarget() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	input := monster.MonsterActionInput{
		Target: nil,
	}

	// Act
	err := action.CanActivate(context.Background(), owner, input)

	// Assert
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "no target")
}

func (s *RangedActionTestSuite) TestCanActivate_TargetOutOfRange() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 400, Y: 0},
				Distance: 400,
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
	s.Assert().Contains(err.Error(), "out of range")
}

func (s *RangedActionTestSuite) TestCanActivate_TargetInNormalRange() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 60, Y: 0},
				Distance: 60,
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
	s.Assert().NoError(err)
}

func (s *RangedActionTestSuite) TestCanActivate_TargetInLongRange() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 200, Y: 0},
				Distance: 200,
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
	s.Assert().NoError(err)
}

func (s *RangedActionTestSuite) TestActivate_PublishesAttackEvent() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "bandit-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: monster.Position{X: 0, Y: 0},
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: monster.Position{X: 60, Y: 0},
				Distance: 60,
				Adjacent: false,
			},
		},
	}

	// Subscribe to attack events
	var receivedEvent *dnd5eEvents.AttackEvent
	topic := dnd5eEvents.AttackTopic.On(s.bus)
	_, err := topic.Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		receivedEvent = &event
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
	s.Assert().NotNil(receivedEvent)
	s.Assert().Equal("bandit-1", receivedEvent.AttackerID)
	s.Assert().Equal("hero-1", receivedEvent.TargetID)
	s.Assert().Equal("shortbow", receivedEvent.WeaponRef)
	s.Assert().False(receivedEvent.IsMelee)
}

func (s *RangedActionTestSuite) TestScore_NoAdjacentEnemy() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	m := monster.New(monster.Config{
		ID:   "test-monster",
		Name: "Test",
		HP:   10,
		AC:   15,
	})
	perception := &monster.PerceptionData{
		Enemies: []monster.PerceivedEntity{
			{Adjacent: false, Distance: 30},
		},
	}

	// Act
	score := action.Score(m, perception)

	// Assert - should have base score + ranged bonus
	s.Assert().Greater(score, 50)
}

func (s *RangedActionTestSuite) TestScore_AdjacentEnemy() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	})

	m := monster.New(monster.Config{
		ID:   "test-monster",
		Name: "Test",
		HP:   10,
		AC:   15,
	})
	perception := &monster.PerceptionData{
		Enemies: []monster.PerceivedEntity{
			{Adjacent: true},
		},
	}

	// Act
	score := action.Score(m, perception)

	// Assert - should have only base score (penalty for adjacent enemy)
	s.Assert().Equal(50, score)
}

func (s *RangedActionTestSuite) TestToData() {
	// Arrange
	config := RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 80,
		RangeLong:   320,
		DamageType:  damage.Piercing,
	}
	action := NewRangedAction(config)

	// Act
	data := action.ToData()

	// Assert
	s.Assert().Equal("ranged", data.Ref.ID)
	s.Assert().NotNil(data.Config)
	// Config should be valid JSON with our config
	s.Assert().Contains(string(data.Config), "shortbow")
}
