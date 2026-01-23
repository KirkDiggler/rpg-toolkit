// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

type MultiattackActionTestSuite struct {
	suite.Suite
	bus    events.EventBus
	roller dice.Roller
}

func TestMultiattackActionSuite(t *testing.T) {
	suite.Run(t, new(MultiattackActionTestSuite))
}

func (s *MultiattackActionTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.roller = dice.NewRoller()
}

func (s *MultiattackActionTestSuite) TestNewMultiattackAction() {
	// Arrange
	config := MultiattackConfig{
		Attacks: []string{"bite", "claw", "claw"},
	}

	// Act
	action := NewMultiattackAction(config)

	// Assert
	s.Assert().NotNil(action)
	s.Assert().Equal("multiattack", action.GetID())
	s.Assert().Equal("monster-action", string(action.GetType()))
	s.Assert().Equal(monster.CostAction, action.Cost())
}

func (s *MultiattackActionTestSuite) TestCanActivate_NoMonster() {
	// Arrange
	action := NewMultiattackAction(MultiattackConfig{
		Attacks: []string{"bite", "claw"},
	})

	owner := &mockEntity{id: "not-a-monster"}
	input := monster.MonsterActionInput{}

	// Act
	err := action.CanActivate(context.Background(), owner, input)

	// Assert
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "owner must be a monster")
}

func (s *MultiattackActionTestSuite) TestCanActivate_NoTarget() {
	// Arrange
	action := NewMultiattackAction(MultiattackConfig{
		Attacks: []string{"bite", "claw"},
	})

	m := monster.New(monster.Config{
		ID:   "boss-1",
		Name: "Boss",
		HP:   50,
		AC:   16,
	})

	input := monster.MonsterActionInput{
		Target: nil,
	}

	// Act
	err := action.CanActivate(context.Background(), m, input)

	// Assert
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "no target")
}

func (s *MultiattackActionTestSuite) TestCanActivate_Valid() {
	// Arrange
	action := NewMultiattackAction(MultiattackConfig{
		Attacks: []string{"bite", "claw"},
	})

	m := monster.New(monster.Config{
		ID:   "boss-1",
		Name: "Boss",
		HP:   50,
		AC:   16,
	})
	// Add the sub-actions that multiattack will use
	m.AddAction(NewMeleeAction(MeleeConfig{
		Name:        "bite",
		AttackBonus: 5,
		DamageDice:  "1d8+3",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	}))
	m.AddAction(NewMeleeAction(MeleeConfig{
		Name:        "claw",
		AttackBonus: 5,
		DamageDice:  "1d6+3",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Slashing,
	}))

	target := &mockEntity{id: "hero-1"}
	perception := &monster.PerceptionData{
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(1), // 1 hex = adjacent
				Distance: 1,
				Adjacent: true,
			},
		},
	}

	input := monster.MonsterActionInput{
		Target:     target,
		Perception: perception,
	}

	// Act
	err := action.CanActivate(context.Background(), m, input)

	// Assert
	s.Assert().NoError(err)
}

func (s *MultiattackActionTestSuite) TestActivate_ExecutesMultipleAttacks() {
	// Arrange
	action := NewMultiattackAction(MultiattackConfig{
		Attacks: []string{"bite", "claw", "claw"},
	})

	m := monster.New(monster.Config{
		ID:   "boss-1",
		Name: "Boss",
		HP:   50,
		AC:   16,
	})
	// Add the sub-actions
	m.AddAction(NewMeleeAction(MeleeConfig{
		Name:        "bite",
		AttackBonus: 5,
		DamageDice:  "1d8+3",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	}))
	m.AddAction(NewMeleeAction(MeleeConfig{
		Name:        "claw",
		AttackBonus: 5,
		DamageDice:  "1d6+3",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Slashing,
	}))

	target := &mockEntity{id: "hero-1"}
	perception := &monster.PerceptionData{
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(1),
				Distance: 1,
				Adjacent: true,
			},
		},
	}

	// Subscribe to attack events
	attackCount := 0
	var receivedEvents []dnd5eEvents.AttackEvent
	topic := dnd5eEvents.AttackTopic.On(s.bus)
	_, err := topic.Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		attackCount++
		receivedEvents = append(receivedEvents, event)
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
	err = action.Activate(context.Background(), m, input)

	// Assert
	s.Assert().NoError(err)
	s.Assert().Equal(3, attackCount, "should have made 3 attacks")
	s.Assert().Equal("bite", receivedEvents[0].WeaponRef)
	s.Assert().Equal("claw", receivedEvents[1].WeaponRef)
	s.Assert().Equal("claw", receivedEvents[2].WeaponRef)
}

func (s *MultiattackActionTestSuite) TestScore_HigherForBosses() {
	// Arrange
	action := NewMultiattackAction(MultiattackConfig{
		Attacks: []string{"bite", "claw"},
	})

	m := monster.New(monster.Config{
		ID:   "boss-1",
		Name: "Boss",
		HP:   50,
		AC:   16,
	})
	perception := &monster.PerceptionData{
		Enemies: []monster.PerceivedEntity{
			{Adjacent: true},
		},
	}

	// Act
	score := action.Score(m, perception)

	// Assert - multiattack should score high for bosses
	s.Assert().Greater(score, 70)
}

func (s *MultiattackActionTestSuite) TestToData() {
	// Arrange
	config := MultiattackConfig{
		Attacks: []string{"bite", "claw", "claw"},
	}
	action := NewMultiattackAction(config)

	// Act
	data := action.ToData()

	// Assert
	s.Assert().Equal("multiattack", data.Ref.ID)
	s.Assert().NotNil(data.Config)
	s.Assert().Contains(string(data.Config), "bite")
	s.Assert().Contains(string(data.Config), "claw")
}
