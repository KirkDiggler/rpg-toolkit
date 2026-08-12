// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

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

func (s *MeleeActionTestSuite) TestActivatePublishesEveryDamageComponent() {
	action := NewMeleeAction(MeleeConfig{
		Name: "pseudopod", AttackBonus: 3, Reach: 1,
		DamageComponents: []dnd5eEvents.AttackDamageComponent{
			{Dice: "1d6-1", DamageType: damage.Bludgeoning},
			{Dice: "2d6", DamageType: damage.Acid},
		},
	})
	owner := &mockEntity{id: "ooze"}
	target := &mockEntity{id: "target"}
	perception := &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1}}}
	var received dnd5eEvents.AttackEvent
	_, err := dnd5eEvents.AttackTopic.On(s.bus).Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		received = event
		return nil
	})
	s.Require().NoError(err)
	s.Require().NoError(action.Activate(context.Background(), owner, monster.MonsterActionInput{Target: target, Perception: perception, Bus: s.bus}))
	s.Require().Equal([]dnd5eEvents.AttackDamageComponent{
		{Dice: "1d6-1", DamageType: damage.Bludgeoning},
		{Dice: "2d6", DamageType: damage.Acid},
	}, received.DamageComponents)
}

func (s *MeleeActionTestSuite) TestLegacyPseudopodComponentsConvertToTypedPools() {
	action := NewMeleeAction(MeleeConfig{
		Name: "pseudopod", AttackBonus: 3, DamageDice: "1d4", DamageType: damage.Piercing, Reach: 1,
		DamageComponents: []dnd5eEvents.AttackDamageComponent{
			{Dice: "1d6-1", DamageType: damage.Bludgeoning},
			{Dice: "2d6", DamageType: damage.Acid},
		},
	})

	event := s.activateAndReceiveEvent(action)

	s.Require().Equal([]damage.Damage{
		{Dice: "1d6-1", Terms: []damage.DiceTerm{{Dice: "1d6", Sign: 1}}, Type: damage.Bludgeoning, FlatBonus: -1, Properties: []damage.Property{damage.PropertyCritEligible}},
		{Dice: "2d6", Terms: []damage.DiceTerm{{Dice: "2d6", Sign: 1}}, Type: damage.Acid, Properties: []damage.Property{damage.PropertyCritEligible}},
	}, event.Definition.Damage.Pools)
}

func (s *MeleeActionTestSuite) TestLegacyMeleeRoundTripPreservesTextAndStructuredDamage() {
	action := NewMeleeAction(MeleeConfig{
		Name: "pseudopod", AttackBonus: 3, Reach: 1,
		DamageComponents: []dnd5eEvents.AttackDamageComponent{
			{Dice: "1d6-1", DamageType: damage.Bludgeoning},
			{Dice: "2d6", DamageType: damage.Acid},
		},
	})

	data := action.ToData()
	var persisted MeleeConfig
	s.Require().NoError(json.Unmarshal(data.Config, &persisted))
	s.Equal([]dnd5eEvents.AttackDamageComponent{
		{Dice: "1d6-1", DamageType: damage.Bludgeoning},
		{Dice: "2d6", DamageType: damage.Acid},
	}, persisted.DamageComponents)
	s.Equal([]damage.Damage{
		{Dice: "1d6-1", Terms: []damage.DiceTerm{{Dice: "1d6", Sign: 1}}, Type: damage.Bludgeoning, FlatBonus: -1, Properties: []damage.Property{damage.PropertyCritEligible}},
		{Dice: "2d6", Terms: []damage.DiceTerm{{Dice: "2d6", Sign: 1}}, Type: damage.Acid, Properties: []damage.Property{damage.PropertyCritEligible}},
	}, persisted.DamageSpec.Pools)

	loaded, err := LoadAction(data)
	s.Require().NoError(err)
	loadedMelee, ok := loaded.(*MeleeAction)
	s.Require().True(ok)
	s.Equal([]dnd5eEvents.AttackDamageComponent{
		{Dice: "1d6-1", DamageType: damage.Bludgeoning},
		{Dice: "2d6", DamageType: damage.Acid},
	}, loadedMelee.damageComponents)
	s.Equal(persisted.DamageSpec, &loadedMelee.damageSpec)
}

func (s *MeleeActionTestSuite) TestLegacyMeleeInvalidDamageIsReportedByConstructorAndRejectedByLoader() {
	config := MeleeConfig{Name: "club", AttackBonus: 2, DamageDice: "1d6++1", DamageType: damage.Bludgeoning, Reach: 1}
	action := NewMeleeAction(config)
	target := &mockEntity{id: "target"}
	err := action.CanActivate(context.Background(), &mockEntity{id: "monster"}, monster.MonsterActionInput{
		Target: target, Perception: &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1}}},
	})
	s.Require().Error(err)
	err = action.Activate(context.Background(), &mockEntity{id: "monster"}, monster.MonsterActionInput{
		Target: target, Perception: &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1}}},
	})
	s.Require().Error(err)

	configJSON, marshalErr := json.Marshal(config)
	s.Require().NoError(marshalErr)
	_, err = LoadAction(monster.ActionData{Ref: *refs.MonsterActions.Melee(), Config: configJSON})
	s.Require().Error(err)
}

func (s *MeleeActionTestSuite) TestLoadMeleeActionRejectsMalformedStructuredDamageSpec() {
	for _, test := range []struct {
		name string
		spec damage.DamageSpec
	}{
		{name: "empty pools", spec: damage.DamageSpec{}},
		{name: "unknown type", spec: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6", Type: damage.Type("unknown")}}}},
		{name: "invalid dice", spec: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6++1", Type: damage.Acid}}}},
		{name: "leading negative term", spec: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6", Terms: []damage.DiceTerm{{Dice: "1d6", Sign: -1}}, Type: damage.Acid}}}},
	} {
		s.Run(test.name, func() {
			configJSON, err := json.Marshal(MeleeConfig{Name: "pseudopod", Reach: 1, DamageSpec: &test.spec})
			s.Require().NoError(err)

			_, err = LoadAction(monster.ActionData{Ref: *refs.MonsterActions.Melee(), Config: configJSON})

			s.Require().Error(err)
		})
	}
}

func (s *MeleeActionTestSuite) TestMeleeActionPrefersDamageSpec() {
	action := NewMeleeAction(MeleeConfig{
		Name: "Pseudopod", AttackBonus: 3, Reach: 1,
		DamageDice: "1d6", DamageType: damage.Bludgeoning,
		DamageSpec: &damage.DamageSpec{Pools: []damage.Damage{{Dice: "2d6", Type: damage.Acid}}},
	})

	event := s.activateAndReceiveEvent(action)

	s.Equal("2d6", event.Definition.Damage.Pools[0].Dice)
}

func (s *MeleeActionTestSuite) TestMeleeActionConvertsLegacySinglePool() {
	action := NewMeleeAction(MeleeConfig{
		Name: "Club", AttackBonus: 2, Reach: 1,
		DamageDice: "1d4", DamageType: damage.Bludgeoning,
	})

	event := s.activateAndReceiveEvent(action)

	s.Equal("1d4", event.Definition.Damage.Pools[0].Dice)
}

func (s *MeleeActionTestSuite) TestMeleeActionPublishesIsolatedDamageSpec() {
	spec := &damage.DamageSpec{Pools: []damage.Damage{{
		Dice: "2d6", Type: damage.Acid, Properties: []damage.Property{damage.PropertyCritEligible},
		Save: &damage.SaveSpec{Ability: abilities.DEX, DC: 12, Effect: damage.SaveEffectHalf},
	}}}
	action := NewMeleeAction(MeleeConfig{Name: "Pseudopod", AttackBonus: 3, Reach: 1, DamageSpec: spec})

	spec.Pools[0].Dice = "9d9"
	spec.Pools[0].Properties[0] = "changed"
	spec.Pools[0].Save.DC = 99
	first := s.activateAndReceiveEvent(action)
	s.Equal("2d6", first.Definition.Damage.Pools[0].Dice)
	s.Equal(damage.PropertyCritEligible, first.Definition.Damage.Pools[0].Properties[0])
	s.Equal(12, first.Definition.Damage.Pools[0].Save.DC)

	first.Definition.Damage.Pools[0].Dice = "8d8"
	first.Definition.Damage.Pools[0].Properties[0] = "changed again"
	first.Definition.Damage.Pools[0].Save.DC = 88
	second := s.activateAndReceiveEvent(action)
	s.Equal("2d6", second.Definition.Damage.Pools[0].Dice)
	s.Equal(damage.PropertyCritEligible, second.Definition.Damage.Pools[0].Properties[0])
	s.Equal(12, second.Definition.Damage.Pools[0].Save.DC)
}

func (s *MeleeActionTestSuite) TestMeleeActionDamageSpecOverridesLegacyDamageComponents() {
	action := NewMeleeAction(MeleeConfig{
		Name: "Pseudopod", AttackBonus: 3, Reach: 1,
		DamageSpec:       &damage.DamageSpec{Pools: []damage.Damage{{Dice: "2d6", Type: damage.Acid}}},
		DamageComponents: []dnd5eEvents.AttackDamageComponent{{Dice: "1d6", DamageType: damage.Bludgeoning}},
	})

	event := s.activateAndReceiveEvent(action)

	s.Equal("2d6", event.Definition.Damage.Pools[0].Dice)
	s.Equal([]dnd5eEvents.AttackDamageComponent{{Dice: "2d6", DamageType: damage.Acid}}, event.DamageComponents)
}

func (s *MeleeActionTestSuite) activateAndReceiveEvent(action *MeleeAction) dnd5eEvents.AttackEvent {
	s.T().Helper()
	owner := &mockEntity{id: "monster"}
	target := &mockEntity{id: "target"}
	perception := &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1}}}
	var received dnd5eEvents.AttackEvent
	_, err := dnd5eEvents.AttackTopic.On(s.bus).Subscribe(context.Background(), func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		received = event
		return nil
	})
	s.Require().NoError(err)
	s.Require().NoError(action.Activate(context.Background(), owner, monster.MonsterActionInput{
		Bus: s.bus, Target: target, Perception: perception,
	}))
	return received
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
