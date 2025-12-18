// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cubeCoord creates a CubeCoordinate from X (z defaults to 0), deriving Y = -X
func cubeCoord(x int) spatial.CubeCoordinate {
	return spatial.CubeCoordinate{X: x, Y: -x, Z: 0}
}

type MeleeActionTestSuite struct {
	suite.Suite
	bus    events.EventBus
	roller dice.Roller
}

func TestMeleeActionSuite(t *testing.T) {
	suite.Run(t, new(MeleeActionTestSuite))
}

func (s *MeleeActionTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.roller = dice.NewRoller()
}

func (s *MeleeActionTestSuite) TestNewMeleeAction() {
	// Arrange
	config := MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	}

	// Act
	action := NewMeleeAction(config)

	// Assert
	s.Assert().NotNil(action)
	s.Assert().Equal("shortsword", action.GetID())
	s.Assert().Equal("monster-action", string(action.GetType()))
	s.Assert().Equal(monster.CostAction, action.Cost())
	s.Assert().Equal(monster.TypeMeleeAttack, action.ActionType())
}

func (s *MeleeActionTestSuite) TestCanActivate_NoTarget() {
	// Arrange
	action := NewMeleeAction(MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
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

func (s *MeleeActionTestSuite) TestCanActivate_TargetOutOfReach() {
	// Arrange
	action := NewMeleeAction(MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: cubeCoord(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: cubeCoord(2), // 2 hexes away
				Distance: 2,
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

func (s *MeleeActionTestSuite) TestCanActivate_TargetInReach() {
	// Arrange
	action := NewMeleeAction(MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: cubeCoord(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: cubeCoord(1), // 1 hex = adjacent
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
	err := action.CanActivate(context.Background(), owner, input)

	// Assert
	s.Assert().NoError(err)
}

func (s *MeleeActionTestSuite) TestActivate_PublishesAttackEvent() {
	// Arrange
	action := NewMeleeAction(MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "skeleton-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: cubeCoord(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: cubeCoord(1),
				Distance: 1,
				Adjacent: true,
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
	s.Assert().Equal("skeleton-1", receivedEvent.AttackerID)
	s.Assert().Equal("hero-1", receivedEvent.TargetID)
	s.Assert().Equal("shortsword", receivedEvent.WeaponRef)
	s.Assert().True(receivedEvent.IsMelee)
}

func (s *MeleeActionTestSuite) TestScore_AdjacentEnemy() {
	// Arrange
	action := NewMeleeAction(MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
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

	// Assert - should have base score + adjacency bonus
	s.Assert().Greater(score, 50)
}

func (s *MeleeActionTestSuite) TestScore_NoAdjacentEnemy() {
	// Arrange
	action := NewMeleeAction(MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
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
			{Adjacent: false, Distance: 6}, // 6 hexes away
		},
	}

	// Act
	score := action.Score(m, perception)

	// Assert - should have only base score
	s.Assert().Equal(50, score)
}

func (s *MeleeActionTestSuite) TestToData() {
	// Arrange
	config := MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       1, // 1 hex = 5 feet
		DamageType:  damage.Piercing,
	}
	action := NewMeleeAction(config)

	// Act
	data := action.ToData()

	// Assert
	s.Assert().Equal("melee", data.Ref.ID)
	s.Assert().NotNil(data.Config)
	// Config should be valid JSON with our config
	s.Assert().Contains(string(data.Config), "shortsword")
}

// Mock types for testing

type mockEntity struct {
	id string
}

func (m *mockEntity) GetID() string {
	return m.id
}

func (m *mockEntity) GetType() core.EntityType {
	return "test-entity"
}
