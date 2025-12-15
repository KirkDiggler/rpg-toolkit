// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type UnarmoredMovementTestSuite struct {
	suite.Suite
	condition *UnarmoredMovementCondition
	bus       events.EventBus
	ctx       context.Context
}

func TestUnarmoredMovementSuite(t *testing.T) {
	suite.Run(t, new(UnarmoredMovementTestSuite))
}

func (s *UnarmoredMovementTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.condition = NewUnarmoredMovementCondition(UnarmoredMovementInput{
		CharacterID: "monk-1",
		MonkLevel:   3,
	})
}

func (s *UnarmoredMovementTestSuite) TestNewUnarmoredMovementCondition() {
	s.Assert().Equal("monk-1", s.condition.CharacterID)
	s.Assert().Equal(3, s.condition.MonkLevel)
	s.Assert().False(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestApply() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestApplyTwice() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Applying twice should not error (unlike some other conditions)
	err = s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestRemove() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	err = s.condition.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestRemoveWhenNotApplied() {
	err := s.condition.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.condition.IsApplied())
}

func (s *UnarmoredMovementTestSuite) TestToJSON() {
	data, err := s.condition.ToJSON()
	s.Require().NoError(err)
	s.Require().NotNil(data)

	var umData UnarmoredMovementData
	err = json.Unmarshal(data, &umData)
	s.Require().NoError(err)

	s.Assert().Equal(refs.Conditions.UnarmoredMovement(), umData.Ref)
	s.Assert().Equal("monk-1", umData.CharacterID)
	s.Assert().Equal(3, umData.MonkLevel)
}

func (s *UnarmoredMovementTestSuite) TestLoadJSON() {
	// Create JSON data
	data := UnarmoredMovementData{
		Ref:         refs.Conditions.UnarmoredMovement(),
		CharacterID: "monk-2",
		MonkLevel:   10,
	}
	jsonData, err := json.Marshal(data)
	s.Require().NoError(err)

	// Load into condition
	condition := &UnarmoredMovementCondition{}
	err = condition.loadJSON(jsonData)
	s.Require().NoError(err)

	s.Assert().Equal("monk-2", condition.CharacterID)
	s.Assert().Equal(10, condition.MonkLevel)
}

func (s *UnarmoredMovementTestSuite) TestCalculateSpeedBonus() {
	testCases := []struct {
		name          string
		monkLevel     int
		expectedBonus int
	}{
		{"Level 2", 2, 10},
		{"Level 3", 3, 10},
		{"Level 5", 5, 10},
		{"Level 6", 6, 15},
		{"Level 9", 9, 15},
		{"Level 10", 10, 20},
		{"Level 13", 13, 20},
		{"Level 14", 14, 25},
		{"Level 17", 17, 25},
		{"Level 18", 18, 30},
		{"Level 20", 20, 30},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			condition := NewUnarmoredMovementCondition(UnarmoredMovementInput{
				CharacterID: "monk-test",
				MonkLevel:   tc.monkLevel,
			})
			bonus := condition.calculateSpeedBonus()
			s.Assert().Equal(tc.expectedBonus, bonus, "Monk level %d should grant +%d ft", tc.monkLevel, tc.expectedBonus)
		})
	}
}

func (s *UnarmoredMovementTestSuite) TestGetSpeedBonusWithoutContext() {
	// Without context, should return bonus (assumes unarmored)
	bonus := s.condition.GetSpeedBonus(s.ctx)
	s.Assert().Equal(10, bonus, "Level 3 monk should get +10 ft")
}

func (s *UnarmoredMovementTestSuite) TestGetSpeedBonusUnarmored() {
	// Create registry with no weapons
	registry := gamectx.NewBasicCharacterRegistry()
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// No weapons equipped (unarmored)
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{})
	registry.Add("monk-1", weapons)

	bonus := s.condition.GetSpeedBonus(ctx)
	s.Assert().Equal(10, bonus, "Unarmored monk should get speed bonus")
}

func (s *UnarmoredMovementTestSuite) TestGetSpeedBonusWithWeaponNoShield() {
	// Create registry with weapon but no shield
	registry := gamectx.NewBasicCharacterRegistry()
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Equip quarterstaff (not a shield)
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{
		{
			ID:       "quarterstaff-1",
			Name:     "Quarterstaff",
			Slot:     gamectx.SlotMainHand,
			IsShield: false,
			IsMelee:  true,
		},
	})
	registry.Add("monk-1", weapons)

	bonus := s.condition.GetSpeedBonus(ctx)
	s.Assert().Equal(10, bonus, "Monk with weapon but no shield should get speed bonus")
}

func (s *UnarmoredMovementTestSuite) TestGetSpeedBonusWithShieldInMainHand() {
	// Create registry with shield
	registry := gamectx.NewBasicCharacterRegistry()
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Equip shield in main hand
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{
		{
			ID:       "shield-1",
			Name:     "Shield",
			Slot:     gamectx.SlotMainHand,
			IsShield: true,
		},
	})
	registry.Add("monk-1", weapons)

	bonus := s.condition.GetSpeedBonus(ctx)
	s.Assert().Equal(0, bonus, "Monk with shield should not get speed bonus")
}

func (s *UnarmoredMovementTestSuite) TestGetSpeedBonusWithDifferentLevels() {
	// Create registry
	registry := gamectx.NewBasicCharacterRegistry()
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// No equipment (unarmored)
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{})
	registry.Add("monk-1", weapons)

	testCases := []struct {
		name          string
		monkLevel     int
		expectedBonus int
	}{
		{"Low level", 3, 10},
		{"Mid level", 10, 20},
		{"High level", 18, 30},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			condition := NewUnarmoredMovementCondition(UnarmoredMovementInput{
				CharacterID: "monk-1",
				MonkLevel:   tc.monkLevel,
			})
			bonus := condition.GetSpeedBonus(ctx)
			s.Assert().Equal(tc.expectedBonus, bonus)
		})
	}
}

func (s *UnarmoredMovementTestSuite) TestRoundTripSerialization() {
	// Apply condition
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Serialize
	jsonData, err := s.condition.ToJSON()
	s.Require().NoError(err)

	// Deserialize
	newCondition := &UnarmoredMovementCondition{}
	err = newCondition.loadJSON(jsonData)
	s.Require().NoError(err)

	// Verify fields match
	s.Assert().Equal(s.condition.CharacterID, newCondition.CharacterID)
	s.Assert().Equal(s.condition.MonkLevel, newCondition.MonkLevel)

	// Note: bus state is not serialized, so IsApplied will be false
	s.Assert().False(newCondition.IsApplied())
}
