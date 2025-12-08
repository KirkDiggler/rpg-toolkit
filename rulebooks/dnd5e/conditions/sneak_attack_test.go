// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// SneakAttackTestSuite tests the SneakAttackCondition behavior
type SneakAttackTestSuite struct {
	suite.Suite
	ctrl   *gomock.Controller
	ctx    context.Context
	bus    events.EventBus
	roller *mock_dice.MockRoller
}

func (s *SneakAttackTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.roller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *SneakAttackTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestSneakAttackTestSuite(t *testing.T) {
	suite.Run(t, new(SneakAttackTestSuite))
}

// executeDamageChain creates a damage chain event and executes it.
func (s *SneakAttackTestSuite) executeDamageChain(
	attackerID string,
	abilityUsed abilities.Ability,
) (*dnd5eEvents.DamageChainEvent, error) {
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{5},
		FinalDiceRolls:    []int{5},
		DamageType:        damage.Piercing,
		IsCritical:        false,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   attackerID,
		TargetID:     "goblin-1",
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		WeaponDamage: "1d6",
		AbilityUsed:  abilityUsed,
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *SneakAttackTestSuite) TestSneakAttackAddsDiceLevel1() {
	// Level 1 rogue gets 1d6 sneak attack
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect 1d6 to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	// Execute damage chain with DEX (finesse weapon)
	finalEvent, err := s.executeDamageChain("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	// Should have weapon + sneak attack components
	s.Require().Len(finalEvent.Components, 2, "Should have weapon and sneak attack components")

	// Verify sneak attack component uses DamageSourceFeature
	sneakComp := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, sneakComp.Source)
	s.Equal([]int{4}, sneakComp.FinalDiceRolls, "Should have rolled 1d6")
	s.Equal(4, sneakComp.Total(), "Sneak attack should add 4 damage")
}

func (s *SneakAttackTestSuite) TestSneakAttackAddsDiceLevel5() {
	// Level 5 rogue gets 3d6 sneak attack
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       5,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect 3d6 to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 3, 6).
		Return([]int{3, 5, 6}, nil)

	finalEvent, err := s.executeDamageChain("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 2)

	sneakComp := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, sneakComp.Source)
	s.Equal([]int{3, 5, 6}, sneakComp.FinalDiceRolls, "Should have rolled 3d6")
	s.Equal(14, sneakComp.Total(), "Sneak attack should add 14 damage (3+5+6)")
}

func (s *SneakAttackTestSuite) TestSneakAttackOnlyOncePerTurn() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// First attack - expect sneak attack
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	finalEvent, err := s.executeDamageChain("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "First attack should have sneak attack")

	// Second attack - NO sneak attack (already used this turn)
	// No roller expectation - RollN should NOT be called

	finalEvent2, err := s.executeDamageChain("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent2.Components, 1, "Second attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestSneakAttackResetsOnTurnEnd() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// First attack
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	_, err = s.executeDamageChain("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	// End turn
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: "rogue-1",
		Round:       1,
	})
	s.Require().NoError(err)

	// Next turn - sneak attack should work again
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{6}, nil)

	finalEvent, err := s.executeDamageChain("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack after turn reset")
}

func (s *SneakAttackTestSuite) TestSneakAttackRequiresFinesseWeapon() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - attack with STR should not trigger sneak attack

	// Attack with STR (non-finesse weapon)
	finalEvent, err := s.executeDamageChain("rogue-1", abilities.STR)
	s.Require().NoError(err)

	// Should only have weapon component (no sneak attack)
	s.Require().Len(finalEvent.Components, 1, "STR attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestSneakAttackOnlyAffectsOwnAttacks() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		CharacterID: "rogue-1",
		Level:       1,
		Roller:      s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - different attacker should not trigger

	// Different character attacks
	finalEvent, err := s.executeDamageChain("rogue-2", abilities.DEX)
	s.Require().NoError(err)

	// Should only have weapon component
	s.Require().Len(finalEvent.Components, 1, "Other character's attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestCalculateSneakAttackDice() {
	testCases := []struct {
		level      int
		damageDice int
	}{
		{level: 1, damageDice: 1},
		{level: 2, damageDice: 1},
		{level: 3, damageDice: 2},
		{level: 4, damageDice: 2},
		{level: 5, damageDice: 3},
		{level: 6, damageDice: 3},
		{level: 7, damageDice: 4},
		{level: 9, damageDice: 5},
		{level: 11, damageDice: 6},
		{level: 13, damageDice: 7},
		{level: 15, damageDice: 8},
		{level: 17, damageDice: 9},
		{level: 19, damageDice: 10},
		{level: 20, damageDice: 10},
	}

	for _, tc := range testCases {
		sneak := NewSneakAttackCondition(SneakAttackInput{
			CharacterID: "rogue-1",
			Level:       tc.level,
			Roller:      s.roller,
		})
		s.Equal(tc.damageDice, sneak.DamageDice, "Level %d should have %dd6", tc.level, tc.damageDice)
	}
}

// Suppress unused import warning
var _ dice.Roller = (*mock_dice.MockRoller)(nil)
