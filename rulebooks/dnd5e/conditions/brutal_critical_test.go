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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// BrutalCriticalTestSuite tests the BrutalCriticalCondition behavior
type BrutalCriticalTestSuite struct {
	suite.Suite
	ctrl   *gomock.Controller
	ctx    context.Context
	bus    events.EventBus
	roller *mock_dice.MockRoller
}

func (s *BrutalCriticalTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.roller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *BrutalCriticalTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestBrutalCriticalTestSuite(t *testing.T) {
	suite.Run(t, new(BrutalCriticalTestSuite))
}

// executeCriticalDamageChain creates a critical damage chain event and executes it.
// Returns the final event after all chain modifications have been applied.
//
//nolint:unparam // Parameters kept for consistency with other test helpers in this package
func (s *BrutalCriticalTestSuite) executeCriticalDamageChain(
	attackerID string,
	weaponDamage string,
	isCritical bool,
) (*combat.DamageChainEvent, error) {
	// Create weapon component with base damage (already doubled for crit in real flow)
	weaponComp := combat.DamageComponent{
		Source:            combat.DamageSourceWeapon,
		OriginalDiceRolls: []int{6, 4}, // 2d8 rolled (crit doubles dice)
		FinalDiceRolls:    []int{6, 4},
		FlatBonus:         0,
		DamageType:        "slashing",
		IsCritical:        isCritical,
	}

	// Create ability component
	abilityComp := combat.DamageComponent{
		Source:            combat.DamageSourceAbility,
		OriginalDiceRolls: nil,
		FinalDiceRolls:    nil,
		FlatBonus:         4, // STR modifier
		DamageType:        "slashing",
		IsCritical:        isCritical,
	}

	damageEvent := &combat.DamageChainEvent{
		AttackerID:   attackerID,
		TargetID:     "goblin-1",
		Components:   []combat.DamageComponent{weaponComp, abilityComp},
		DamageType:   "slashing",
		IsCritical:   isCritical,
		WeaponDamage: weaponDamage,
		AbilityUsed:  "str",
	}

	chain := events.NewStagedChain[*combat.DamageChainEvent](dnd5e.ModifierStages)
	damageTopic := combat.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalAddsExtraDieLevel9() {
	// Level 9 barbarian gets 1 extra weapon damage die on crits
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       9,
		Roller:      s.roller,
	})

	// Apply condition to subscribe to damage chain
	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Mock the extra die roll: RollN(ctx, 1, 8) returns [5]
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 8).
		Return([]int{5}, nil)

	// Execute critical damage chain
	finalEvent, err := s.executeCriticalDamageChain("barbarian-1", "1d8", true)
	s.Require().NoError(err)

	// Should have weapon, ability, and brutal critical components
	s.Require().Len(finalEvent.Components, 3, "Should have weapon, ability, and brutal critical components")

	// Verify brutal critical component
	brutalComp := finalEvent.Components[2]
	s.Equal(combat.DamageSourceBrutalCritical, brutalComp.Source)
	s.Equal([]int{5}, brutalComp.FinalDiceRolls, "Should have rolled 1 extra d8")
	s.Equal(5, brutalComp.Total(), "Brutal critical should add 5 damage")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalAddsExtraDiceLevel13() {
	// Level 13 barbarian gets 2 extra weapon damage dice on crits
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       13,
		Roller:      s.roller,
	})

	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Mock the extra dice rolls: RollN(ctx, 2, 8) returns [5, 7]
	s.roller.EXPECT().
		RollN(gomock.Any(), 2, 8).
		Return([]int{5, 7}, nil)

	finalEvent, err := s.executeCriticalDamageChain("barbarian-1", "1d8", true)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 3)

	brutalComp := finalEvent.Components[2]
	s.Equal(combat.DamageSourceBrutalCritical, brutalComp.Source)
	s.Equal([]int{5, 7}, brutalComp.FinalDiceRolls, "Should have rolled 2 extra d8s")
	s.Equal(12, brutalComp.Total(), "Brutal critical should add 12 damage (5+7)")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalAddsExtraDiceLevel17() {
	// Level 17 barbarian gets 3 extra weapon damage dice on crits
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       17,
		Roller:      s.roller,
	})

	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Mock the extra dice rolls: RollN(ctx, 3, 8) returns [3, 6, 8]
	s.roller.EXPECT().
		RollN(gomock.Any(), 3, 8).
		Return([]int{3, 6, 8}, nil)

	finalEvent, err := s.executeCriticalDamageChain("barbarian-1", "1d8", true)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 3)

	brutalComp := finalEvent.Components[2]
	s.Equal(combat.DamageSourceBrutalCritical, brutalComp.Source)
	s.Equal([]int{3, 6, 8}, brutalComp.FinalDiceRolls, "Should have rolled 3 extra d8s")
	s.Equal(17, brutalComp.Total(), "Brutal critical should add 17 damage (3+6+8)")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalIgnoresNonCriticalHits() {
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       9,
		Roller:      s.roller,
	})

	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No mock expectation - RollN should NOT be called for non-crits

	// Execute NON-critical damage chain
	finalEvent, err := s.executeCriticalDamageChain("barbarian-1", "1d8", false)
	s.Require().NoError(err)

	// Should only have weapon and ability components (no brutal critical)
	s.Require().Len(finalEvent.Components, 2, "Should NOT have brutal critical on non-crit")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalOnlyAffectsOwnAttacks() {
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       9,
		Roller:      s.roller,
	})

	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No mock expectation - RollN should NOT be called for other characters

	// Create critical damage chain for a DIFFERENT attacker
	weaponComp := combat.DamageComponent{
		Source:            combat.DamageSourceWeapon,
		OriginalDiceRolls: []int{6, 4},
		FinalDiceRolls:    []int{6, 4},
		DamageType:        "slashing",
		IsCritical:        true,
	}

	damageEvent := &combat.DamageChainEvent{
		AttackerID:   "barbarian-2", // Different character
		TargetID:     "goblin-1",
		Components:   []combat.DamageComponent{weaponComp},
		DamageType:   "slashing",
		IsCritical:   true,
		WeaponDamage: "1d8",
		AbilityUsed:  "str",
	}

	chain := events.NewStagedChain[*combat.DamageChainEvent](dnd5e.ModifierStages)
	damageTopic := combat.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Should NOT have brutal critical component (different character)
	s.Require().Len(finalEvent.Components, 1, "Should NOT have brutal critical for other character's crit")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalWorksWithDifferentWeaponDice() {
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       9,
		Roller:      s.roller,
	})

	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Mock roll for d12 (greataxe): RollN(ctx, 1, 12) returns [10]
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 12).
		Return([]int{10}, nil)

	// Use a greataxe (1d12)
	weaponComp := combat.DamageComponent{
		Source:            combat.DamageSourceWeapon,
		OriginalDiceRolls: []int{8, 11}, // 2d12 for crit
		FinalDiceRolls:    []int{8, 11},
		DamageType:        "slashing",
		IsCritical:        true,
	}

	damageEvent := &combat.DamageChainEvent{
		AttackerID:   "barbarian-1",
		TargetID:     "goblin-1",
		Components:   []combat.DamageComponent{weaponComp},
		DamageType:   "slashing",
		IsCritical:   true,
		WeaponDamage: "1d12", // Greataxe
		AbilityUsed:  "str",
	}

	chain := events.NewStagedChain[*combat.DamageChainEvent](dnd5e.ModifierStages)
	damageTopic := combat.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 2)

	brutalComp := finalEvent.Components[1]
	s.Equal(combat.DamageSourceBrutalCritical, brutalComp.Source)
	s.Equal([]int{10}, brutalComp.FinalDiceRolls, "Should roll extra d12 for greataxe")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalRemoveUnsubscribes() {
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       9,
		Roller:      s.roller,
	})

	err := brutal.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Remove the condition
	err = brutal.Remove(s.ctx, s.bus)
	s.Require().NoError(err)

	// No mock expectation - RollN should NOT be called after Remove

	// Now critical hits should not get brutal critical bonus
	finalEvent, err := s.executeCriticalDamageChain("barbarian-1", "1d8", true)
	s.Require().NoError(err)

	// Should only have weapon and ability (brutal critical unsubscribed)
	s.Require().Len(finalEvent.Components, 2, "Brutal critical should not apply after Remove()")
}

func (s *BrutalCriticalTestSuite) TestBrutalCriticalToJSON() {
	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       13,
		Roller:      s.roller,
	})

	jsonData, err := brutal.ToJSON()
	s.Require().NoError(err)

	// Verify JSON contains expected fields
	s.Contains(string(jsonData), `"character_id":"barbarian-1"`)
	s.Contains(string(jsonData), `"level":13`)
	s.Contains(string(jsonData), `"extra_dice":2`)
	s.Contains(string(jsonData), `"id":"brutal_critical"`)
	s.Contains(string(jsonData), `"module":"dnd5e"`)
	s.Contains(string(jsonData), `"type":"conditions"`)
}

func (s *BrutalCriticalTestSuite) TestCalculateExtraDice() {
	testCases := []struct {
		level     int
		extraDice int
	}{
		{level: 1, extraDice: 0},  // Below level 9
		{level: 8, extraDice: 0},  // Just below level 9
		{level: 9, extraDice: 1},  // Level 9
		{level: 12, extraDice: 1}, // Between 9 and 13
		{level: 13, extraDice: 2}, // Level 13
		{level: 16, extraDice: 2}, // Between 13 and 17
		{level: 17, extraDice: 3}, // Level 17
		{level: 20, extraDice: 3}, // Max level
	}

	for _, tc := range testCases {
		brutal := NewBrutalCriticalCondition(BrutalCriticalInput{
			CharacterID: "barbarian-1",
			Level:       tc.level,
			Roller:      s.roller,
		})
		s.Equal(tc.extraDice, brutal.ExtraDice, "Level %d should have %d extra dice", tc.level, tc.extraDice)
	}
}

// Ensure we have the unused import warning suppressed
var _ dice.Roller = (*mock_dice.MockRoller)(nil)
