// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

func (s *BiteActionTestSuite) TestLoadBiteActionRejectsMalformedDamage() {
	for _, test := range []struct {
		name   string
		config BiteConfig
	}{
		{
			name: "invalid legacy dice",
			config: BiteConfig{
				DamageDice: "1d6++1", DamageType: damage.Piercing,
			},
		},
		{
			name: "invalid structured spec",
			config: BiteConfig{
				DamageSpec: &damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6++1", Type: damage.Piercing}}},
			},
		},
	} {
		s.Run(test.name, func() {
			configJSON, err := json.Marshal(test.config)
			s.Require().NoError(err)

			_, err = LoadAction(monster.ActionData{Ref: *refs.MonsterActions.Bite(), Config: configJSON})

			s.Require().Error(err)
		})
	}
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
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(2), // 2 hexes away
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
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(1), // 1 hex away = adjacent
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

func (s *BiteActionTestSuite) TestActivate_PublishesDamageOnlyAttackDefinition() {
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
	var receivedAttackEvent *dnd5eEvents.AttackEvent
	attackTopic := dnd5eEvents.AttackTopic.On(s.bus)
	_, err := attackTopic.Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		receivedAttackEvent = &event
		return nil
	})
	s.Require().NoError(err)

	conditionApplied := false
	_, err = dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(context.Background(), func(_ context.Context, _ dnd5eEvents.ConditionAppliedEvent) error {
		conditionApplied = true
		return nil
	})
	s.Require().NoError(err)

	saveRequested := false
	_, err = dnd5eEvents.SavingThrowChain.On(s.bus).SubscribeWithChain(context.Background(), func(_ context.Context, _ *dnd5eEvents.SavingThrowChainEvent, current chain.Chain[*dnd5eEvents.SavingThrowChainEvent]) (chain.Chain[*dnd5eEvents.SavingThrowChainEvent], error) {
		saveRequested = true
		return current, nil
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
	s.Assert().Equal(attack.Definition{
		ActionID:    "bite",
		DisplayName: "bite",
		Category:    attack.CategoryNatural,
		Bonus:       attack.FixedBonus(4),
		Targeting:   attack.MeleeReach(1),
		Damage: damage.DamageSpec{Pools: []damage.Damage{{
			Dice:       "2d4+2",
			Terms:      []damage.DiceTerm{{Dice: "2d4", Sign: 1}},
			Type:       damage.Piercing,
			FlatBonus:  2,
			Properties: []damage.Property{damage.PropertyCritEligible},
		}}},
	}, receivedAttackEvent.Definition)
	s.Assert().False(saveRequested, "bite should not request a saving throw")
	s.Assert().False(conditionApplied, "bite should not apply a condition")
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
			{Adjacent: false, Distance: 6}, // 6 hexes away
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

	var persisted BiteConfig
	s.Require().NoError(json.Unmarshal(data.Config, &persisted))
	s.Assert().Equal("2d4+2", persisted.DamageDice)
	s.Assert().Equal(11, persisted.KnockdownDC)
}
