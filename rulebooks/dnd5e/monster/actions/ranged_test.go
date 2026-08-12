// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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
		RangeNormal: 16, // 16 hexes (80 feet / 5)
		RangeLong:   64, // 64 hexes (320 feet / 5)
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

func (s *RangedActionTestSuite) TestLoadRangedActionRejectsMalformedDamage() {
	for _, test := range []struct {
		name   string
		config RangedConfig
	}{
		{
			name: "invalid legacy dice",
			config: RangedConfig{
				Name: "crossbow", DamageDice: "1d6++1", DamageType: damage.Piercing,
			},
		},
		{
			name: "invalid structured spec",
			config: RangedConfig{
				Name: "crossbow", DamageSpec: &damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6++1", Type: damage.Piercing}}},
			},
		},
	} {
		s.Run(test.name, func() {
			configJSON, err := json.Marshal(test.config)
			s.Require().NoError(err)

			_, err = LoadAction(monster.ActionData{Ref: *refs.MonsterActions.Ranged(), Config: configJSON})

			s.Require().Error(err)
		})
	}
}

func (s *RangedActionTestSuite) TestCanActivate_NoTarget() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 16,
		RangeLong:   64,
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
		RangeNormal: 16,
		RangeLong:   64,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(80), // 80 hexes away (way beyond long range)
				Distance: 80,
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
		RangeNormal: 16,
		RangeLong:   64,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(12), // 12 hexes away (within normal range)
				Distance: 12,
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
		RangeNormal: 16,
		RangeLong:   64,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "monster-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(40), // 40 hexes away (in long range)
				Distance: 40,
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
		RangeNormal: 16,
		RangeLong:   64,
		DamageType:  damage.Piercing,
	})

	owner := &mockEntity{id: "bandit-1"}
	target := &mockEntity{id: "hero-1"}

	perception := &monster.PerceptionData{
		MyPosition: hexAt(0),
		Enemies: []monster.PerceivedEntity{
			{
				Entity:   target,
				Position: hexAt(12),
				Distance: 12,
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
	s.Assert().Equal(attack.Definition{
		ActionID:    "shortbow",
		DisplayName: "shortbow",
		Category:    attack.CategoryNatural,
		Bonus:       attack.FixedBonus(4),
		Targeting:   attack.Ranged(16, 64),
		Damage: damage.DamageSpec{Pools: []damage.Damage{{
			Dice:       "1d6+2",
			Terms:      []damage.DiceTerm{{Dice: "1d6", Sign: 1}},
			Type:       damage.Piercing,
			FlatBonus:  2,
			Properties: []damage.Property{damage.PropertyCritEligible},
		}}},
	}, receivedEvent.Definition)
}

func (s *RangedActionTestSuite) TestActivate_MalformedLegacyDamagePreventsEventPublication() {
	action := NewRangedAction(RangedConfig{
		Name: "shortbow", AttackBonus: 4, DamageDice: "1d6++2",
		RangeNormal: 16, RangeLong: 64, DamageType: damage.Piercing,
	})
	owner := &mockEntity{id: "bandit-1"}
	target := &mockEntity{id: "hero-1"}
	perception := &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 12}}}
	published := false
	_, err := dnd5eEvents.AttackTopic.On(s.bus).Subscribe(context.Background(), func(_ context.Context, _ dnd5eEvents.AttackEvent) error {
		published = true
		return nil
	})
	s.Require().NoError(err)

	err = action.Activate(context.Background(), owner, monster.MonsterActionInput{
		Bus: s.bus, Target: target, Perception: perception,
	})

	s.Require().Error(err)
	s.False(published)
}

func (s *RangedActionTestSuite) TestLegacyRangedRoundTripPreservesTextAndStructuredDamage() {
	action := NewRangedAction(RangedConfig{
		Name: "light crossbow", AttackBonus: 3, DamageDice: "1d8+1",
		RangeNormal: 80, RangeLong: 320, DamageType: damage.Piercing,
	})

	data := action.ToData()
	var persisted RangedConfig
	s.Require().NoError(json.Unmarshal(data.Config, &persisted))
	s.Equal("1d8+1", persisted.DamageDice)
	s.Equal(damage.Piercing, persisted.DamageType)
	s.Equal([]damage.Damage{{
		Dice: "1d8+1", Terms: []damage.DiceTerm{{Dice: "1d8", Sign: 1}}, Type: damage.Piercing,
		FlatBonus: 1, Properties: []damage.Property{damage.PropertyCritEligible},
	}}, persisted.DamageSpec.Pools)

	loaded, err := LoadAction(data)
	s.Require().NoError(err)
	loadedRanged, ok := loaded.(*RangedAction)
	s.Require().True(ok)
	s.Equal("1d8+1", loadedRanged.damageDice)
	s.Equal(persisted.DamageSpec, &loadedRanged.damageSpec)
}

func (s *RangedActionTestSuite) TestScore_NoAdjacentEnemy() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 16,
		RangeLong:   64,
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

	// Assert - should have base score + ranged bonus
	s.Assert().Greater(score, 50)
}

func (s *RangedActionTestSuite) TestScore_AdjacentEnemy() {
	// Arrange
	action := NewRangedAction(RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		RangeNormal: 16,
		RangeLong:   64,
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
		RangeNormal: 16,
		RangeLong:   64,
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
