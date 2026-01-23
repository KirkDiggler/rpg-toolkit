// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FightingStyleGreatWeaponFightingTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
}

func (s *FightingStyleGreatWeaponFightingTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestFightingStyleGreatWeaponFightingSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleGreatWeaponFightingTestSuite))
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestNewFightingStyleGreatWeaponFightingCondition() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	s.NotNil(gwf)
	s.False(gwf.IsApplied())
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestApplyAndRemove() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	err := gwf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(gwf.IsApplied())

	err = gwf.Apply(s.ctx, s.bus)
	s.Error(err)

	err = gwf.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(gwf.IsApplied())
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestRerolls1sAnd2s() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	err := gwf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = gwf.Remove(s.ctx, s.bus) }()

	// Expect rerolls for the 1 and 2
	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(5, nil).Times(1) // Reroll the 1
	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(4, nil).Times(1) // Reroll the 2

	// Create damage chain event with 1s and 2s
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   "fighter-1",
		TargetID:     "goblin-1",
		WeaponDamage: "2d6", // Die size for rerolls
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{1, 2, 6}, // 1 and 2 need rerolling, 6 stays
				FinalDiceRolls:    []int{1, 2, 6},
				FlatBonus:         4,
				DamageType:        damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Check that 1 and 2 were rerolled to 5 and 4
	s.Equal([]int{5, 4, 6}, finalEvent.Components[0].FinalDiceRolls)
	s.Len(finalEvent.Components[0].Rerolls, 2)
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestDoesNotRerollHigherValues() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	err := gwf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = gwf.Remove(s.ctx, s.bus) }()

	// No rerolls expected - all dice are 3+
	// Create damage chain event with no 1s or 2s
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   "fighter-1",
		TargetID:     "goblin-1",
		WeaponDamage: "2d6",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{3, 4, 6}, // All above 2
				FinalDiceRolls:    []int{3, 4, 6},
				FlatBonus:         4,
				DamageType:        damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Dice should be unchanged
	s.Equal([]int{3, 4, 6}, finalEvent.Components[0].FinalDiceRolls)
	s.Empty(finalEvent.Components[0].Rerolls)
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestToJSON() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	jsonData, err := gwf.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleGreatWeaponFighting().ID)
	s.Contains(string(jsonData), "fighter-1")
}
