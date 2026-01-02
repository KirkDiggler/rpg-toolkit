// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// mockTwoWeaponContext implements combat.TwoWeaponContext for testing
type mockTwoWeaponContext struct {
	mainHand      *combat.EquippedWeaponInfo
	offHand       *combat.EquippedWeaponInfo
	actionEconomy *combat.ActionEconomy
}

func (m *mockTwoWeaponContext) GetMainHandWeapon(_ string) *combat.EquippedWeaponInfo {
	return m.mainHand
}

func (m *mockTwoWeaponContext) GetOffHandWeapon(_ string) *combat.EquippedWeaponInfo {
	return m.offHand
}

func (m *mockTwoWeaponContext) GetActionEconomy(_ string) *combat.ActionEconomy {
	return m.actionEconomy
}

// TwoWeaponFightingTestSuite tests two-weapon fighting validation
type TwoWeaponFightingTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
}

func (s *TwoWeaponFightingTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *TwoWeaponFightingTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithLightWeapons() {
	// Setup: Fighter with shortsword (light) in main hand, dagger (light) in off hand
	twc := &mockTwoWeaponContext{
		mainHand:      &combat.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		offHand:       &combat.EquippedWeaponInfo{WeaponID: weapons.Dagger},
		actionEconomy: combat.NewActionEconomy(),
	}
	ctx := combat.WithTwoWeaponContext(s.ctx, twc)

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	// Get the dagger for the off-hand attack
	dagger, err := weapons.GetByID(weapons.Dagger)
	s.Require().NoError(err)

	// Mock rolls: 15 for attack, 4 for damage
	s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(15, nil)
	s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{4}, nil) // 1d4 dagger damage

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16, // +3
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &dagger,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	result, err := combat.ResolveAttack(ctx, input)

	s.Require().NoError(err)
	s.True(result.Hit, "Attack should hit with roll 15 + 5 = 20 vs AC 12")

	// Verify bonus action was consumed
	s.False(twc.actionEconomy.CanUseBonusAction(), "Bonus action should be consumed")
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithNonLightMainHand() {
	// Setup: Fighter with longsword (NOT light) in main hand, dagger (light) in off hand
	twc := &mockTwoWeaponContext{
		mainHand:      &combat.EquippedWeaponInfo{WeaponID: weapons.Longsword}, // NOT light
		offHand:       &combat.EquippedWeaponInfo{WeaponID: weapons.Dagger},
		actionEconomy: combat.NewActionEconomy(),
	}
	ctx := combat.WithTwoWeaponContext(s.ctx, twc)

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	dagger, err := weapons.GetByID(weapons.Dagger)
	s.Require().NoError(err)

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &dagger,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	_, err = combat.ResolveAttack(ctx, input)

	s.Require().Error(err)
	s.Contains(err.Error(), "main hand weapon must be light")
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithNonLightOffHand() {
	// Setup: Fighter with shortsword (light) in main hand, longsword (NOT light) in off hand
	twc := &mockTwoWeaponContext{
		mainHand:      &combat.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		offHand:       &combat.EquippedWeaponInfo{WeaponID: weapons.Longsword}, // NOT light
		actionEconomy: combat.NewActionEconomy(),
	}
	ctx := combat.WithTwoWeaponContext(s.ctx, twc)

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	longsword, err := weapons.GetByID(weapons.Longsword)
	s.Require().NoError(err)

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &longsword,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	_, err = combat.ResolveAttack(ctx, input)

	s.Require().Error(err)
	s.Contains(err.Error(), "off hand weapon must be light")
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithNoMainHand() {
	// Setup: No main hand weapon
	twc := &mockTwoWeaponContext{
		mainHand:      nil, // No main hand weapon
		offHand:       &combat.EquippedWeaponInfo{WeaponID: weapons.Dagger},
		actionEconomy: combat.NewActionEconomy(),
	}
	ctx := combat.WithTwoWeaponContext(s.ctx, twc)

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	dagger, err := weapons.GetByID(weapons.Dagger)
	s.Require().NoError(err)

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &dagger,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	_, err = combat.ResolveAttack(ctx, input)

	s.Require().Error(err)
	s.Contains(err.Error(), "no weapon in main hand")
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithNoOffHand() {
	// Setup: No off hand weapon
	twc := &mockTwoWeaponContext{
		mainHand:      &combat.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		offHand:       nil, // No off hand weapon
		actionEconomy: combat.NewActionEconomy(),
	}
	ctx := combat.WithTwoWeaponContext(s.ctx, twc)

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	shortsword, err := weapons.GetByID(weapons.Shortsword)
	s.Require().NoError(err)

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &shortsword,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	_, err = combat.ResolveAttack(ctx, input)

	s.Require().Error(err)
	s.Contains(err.Error(), "no weapon in off hand")
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithNoBonusAction() {
	// Setup: Valid weapons but no bonus action available
	ae := combat.NewActionEconomy()
	_ = ae.UseBonusAction() // Consume bonus action

	twc := &mockTwoWeaponContext{
		mainHand:      &combat.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		offHand:       &combat.EquippedWeaponInfo{WeaponID: weapons.Dagger},
		actionEconomy: ae,
	}
	ctx := combat.WithTwoWeaponContext(s.ctx, twc)

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	dagger, err := weapons.GetByID(weapons.Dagger)
	s.Require().NoError(err)

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &dagger,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	_, err = combat.ResolveAttack(ctx, input)

	s.Require().Error(err)
	s.Contains(err.Error(), "bonus action")
}

func (s *TwoWeaponFightingTestSuite) TestOffHandAttackWithNoContext() {
	// Setup: No TwoWeaponContext in context
	ctx := s.ctx // No TwoWeaponContext

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	dagger, err := weapons.GetByID(weapons.Dagger)
	s.Require().NoError(err)

	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &dagger,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandOff,
	}

	_, err = combat.ResolveAttack(ctx, input)

	s.Require().Error(err)
	s.Contains(err.Error(), "two-weapon context not available")
}

func (s *TwoWeaponFightingTestSuite) TestMainHandAttackDoesNotRequireContext() {
	// Main hand attacks should work without TwoWeaponContext
	ctx := s.ctx // No TwoWeaponContext

	attacker := &mockEntity{id: "fighter-1", name: "Fighter"}
	defender := &mockEntity{id: "goblin-1", name: "Goblin"}

	longsword, err := weapons.GetByID(weapons.Longsword)
	s.Require().NoError(err)

	// Mock rolls: 15 for attack, 6 for damage
	s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(15, nil)
	s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 8).Return([]int{6}, nil) // 1d8 longsword

	scores := shared.AbilityScores{
		abilities.STR: 16, // +3
		abilities.DEX: 10,
		abilities.CON: 10,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	input := &combat.AttackInput{
		Attacker:         attacker,
		Defender:         defender,
		Weapon:           &longsword,
		AttackerScores:   scores,
		DefenderAC:       12,
		ProficiencyBonus: 2,
		EventBus:         s.bus,
		Roller:           s.mockRoller,
		AttackHand:       combat.AttackHandMain, // Main hand (default)
	}

	result, err := combat.ResolveAttack(ctx, input)

	s.Require().NoError(err)
	s.True(result.Hit, "Main hand attack should work without TwoWeaponContext")
}

func TestTwoWeaponFightingSuite(t *testing.T) {
	suite.Run(t, new(TwoWeaponFightingTestSuite))
}
