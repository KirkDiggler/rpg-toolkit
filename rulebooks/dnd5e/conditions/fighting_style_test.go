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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
)

type FightingStyleTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
}

func (s *FightingStyleTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *FightingStyleTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestFightingStyleSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleTestSuite))
}

// TestArcheryBonusApplication verifies Archery adds +2 to ranged attacks only
func (s *FightingStyleTestSuite) TestArcheryBonusApplication() {
	testCases := []struct {
		name               string
		weaponRef          string
		isMelee            bool
		baseBonus          int
		expectedFinalBonus int
		description        string
	}{
		{
			name:               "AddsToRangedAttacks",
			weaponRef:          "longbow",
			isMelee:            false,
			baseBonus:          5, // DEX(3) + Prof(2)
			expectedFinalBonus: 7, // 5 + 2 from Archery
			description:        "Archery should add +2 to ranged attack bonus",
		},
		{
			name:               "DoesNotAddToMeleeAttacks",
			weaponRef:          "longsword",
			isMelee:            true,
			baseBonus:          5, // STR(3) + Prof(2)
			expectedFinalBonus: 5, // No bonus from Archery
			description:        "Archery should NOT add to melee attacks",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Create Archery fighting style condition
			fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
				CharacterID: "fighter-1",
				Style:       fightingstyles.Archery,
				Roller:      s.mockRoller,
			})

			// Apply the condition
			err := fs.Apply(s.ctx, s.bus)
			s.Require().NoError(err)
			defer func() {
				_ = fs.Remove(s.ctx, s.bus)
			}()

			// Publish an AttackEvent
			attackTopic := dnd5eEvents.AttackTopic.On(s.bus)
			err = attackTopic.Publish(s.ctx, dnd5eEvents.AttackEvent{
				AttackerID: "fighter-1",
				TargetID:   "goblin-1",
				WeaponRef:  tc.weaponRef,
				IsMelee:    tc.isMelee,
			})
			s.Require().NoError(err)

			// Create attack chain event
			attackEvent := combat.AttackChainEvent{
				AttackerID:      "fighter-1",
				TargetID:        "goblin-1",
				AttackRoll:      15,
				AttackBonus:     tc.baseBonus,
				TargetAC:        13,
				IsNaturalTwenty: false,
				IsNaturalOne:    false,
			}

			// Publish through attack chain
			attackChain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
			attacks := combat.AttackChain.On(s.bus)
			modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
			s.Require().NoError(err)

			// Execute chain
			finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
			s.Require().NoError(err)

			// Verify expected bonus
			s.Equal(tc.expectedFinalBonus, finalEvent.AttackBonus, tc.description)
		})
	}
}

// TestGreatWeaponFightingRerolls verifies GWF rerolls 1s and 2s on weapon damage
func (s *FightingStyleTestSuite) TestGreatWeaponFightingRerolls() {
	// Create GWF fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.GreatWeaponFighting,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(s.ctx, s.bus)
	}()

	// Set up mock roller expectations for rerolls
	// First die is 1, reroll to 5
	// Second die is 2, reroll to 4
	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(5, nil).Times(1) // Reroll first die
	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(4, nil).Times(1) // Reroll second die

	// Create damage chain event with weapon damage containing 1s and 2s
	damageEvent := &combat.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []combat.DamageComponent{
			{
				Source:            combat.DamageSourceWeapon,
				OriginalDiceRolls: []int{1, 2, 6}, // Two dice need rerolling
				FinalDiceRolls:    []int{1, 2, 6},
				Rerolls:           nil,
				FlatBonus:         0,
				DamageType:        "slashing",
				IsCritical:        false,
			},
		},
		DamageType:   "slashing",
		IsCritical:   false,
		WeaponDamage: "2d6", // Greatsword
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*combat.DamageChainEvent](combat.ModifierStages)
	damages := combat.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Verify rerolls
	weaponComponent := finalEvent.Components[0]
	s.Require().Len(weaponComponent.Rerolls, 2, "Should have 2 rerolls")

	// Check first reroll (1 -> 5)
	s.Equal(0, weaponComponent.Rerolls[0].DieIndex)
	s.Equal(1, weaponComponent.Rerolls[0].Before)
	s.Equal(5, weaponComponent.Rerolls[0].After)
	s.Equal("great_weapon_fighting", weaponComponent.Rerolls[0].Reason)

	// Check second reroll (2 -> 4)
	s.Equal(1, weaponComponent.Rerolls[1].DieIndex)
	s.Equal(2, weaponComponent.Rerolls[1].Before)
	s.Equal(4, weaponComponent.Rerolls[1].After)
	s.Equal("great_weapon_fighting", weaponComponent.Rerolls[1].Reason)

	// Check final dice rolls
	s.Equal([]int{5, 4, 6}, weaponComponent.FinalDiceRolls)
}

// TestGreatWeaponFightingDoesNotRerollHighNumbers verifies GWF only rerolls 1s and 2s
func (s *FightingStyleTestSuite) TestGreatWeaponFightingDoesNotRerollHighNumbers() {
	// Create GWF fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.GreatWeaponFighting,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(s.ctx, s.bus)
	}()

	// No mock expectations because nothing should be rerolled

	// Create damage chain event with all high rolls
	damageEvent := &combat.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []combat.DamageComponent{
			{
				Source:            combat.DamageSourceWeapon,
				OriginalDiceRolls: []int{5, 6}, // No rerolls needed
				FinalDiceRolls:    []int{5, 6},
				Rerolls:           nil,
				FlatBonus:         0,
				DamageType:        "slashing",
				IsCritical:        false,
			},
		},
		DamageType:   "slashing",
		IsCritical:   false,
		WeaponDamage: "2d6",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*combat.DamageChainEvent](combat.ModifierStages)
	damages := combat.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	// Verify no rerolls
	weaponComponent := finalEvent.Components[0]
	s.Empty(weaponComponent.Rerolls, "Should have no rerolls for high rolls")
	s.Equal([]int{5, 6}, weaponComponent.FinalDiceRolls)
}

// TestJSONRoundTrip verifies fighting style can be serialized and deserialized
func (s *FightingStyleTestSuite) TestJSONRoundTrip() {
	testCases := []struct {
		name  string
		style fightingstyles.FightingStyle
	}{
		{"Archery", fightingstyles.Archery},
		{"GreatWeaponFighting", fightingstyles.GreatWeaponFighting},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Create original condition
			original := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
				CharacterID: "fighter-1",
				Style:       tc.style,
				Roller:      s.mockRoller,
			})

			// Serialize to JSON
			jsonData, err := original.ToJSON()
			s.Require().NoError(err)

			// Deserialize from JSON
			loaded, err := conditions.LoadJSON(jsonData)
			s.Require().NoError(err)

			// Verify it's the right type
			loadedFS, ok := loaded.(*conditions.FightingStyleCondition)
			s.Require().True(ok, "Loaded condition should be FightingStyleCondition")

			// Verify fields match
			s.Equal(original.CharacterID, loadedFS.CharacterID)
			s.Equal(original.Style, loadedFS.Style)
		})
	}
}

// TestApplyAndRemove verifies the condition can be applied and removed
func (s *FightingStyleTestSuite) TestApplyAndRemove() {
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Archery,
		Roller:      s.mockRoller,
	})

	// Apply should succeed
	err := fs.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Remove should succeed
	err = fs.Remove(s.ctx, s.bus)
	s.Require().NoError(err)

	// Second remove should be safe (no-op)
	err = fs.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
}

// TestUnimplementedStyleReturnsError verifies unsupported styles return error
func (s *FightingStyleTestSuite) TestUnimplementedStyleReturnsError() {
	// Try to apply Defense (not yet implemented)
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Defense,
		Roller:      s.mockRoller,
	})

	// Apply should return error
	err := fs.Apply(s.ctx, s.bus)
	s.Require().Error(err)
	s.Contains(err.Error(), "not yet implemented")
}

// TestRejectsDoubleApply verifies that applying the same condition twice returns error
func (s *FightingStyleTestSuite) TestRejectsDoubleApply() {
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Archery,
		Roller:      s.mockRoller,
	})

	// First apply should succeed
	err := fs.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Second apply should fail
	err = fs.Apply(s.ctx, s.bus)
	s.Require().Error(err)
	s.Contains(err.Error(), "already applied")
}

// TestDuelingBonusWithOneHandedWeapon verifies Dueling adds +2 to damage with one-handed melee weapon
func (s *FightingStyleTestSuite) TestDuelingBonusWithOneHandedWeapon() {
	// Import gamectx
	ctx := s.ctx

	// Create character registry with one-handed melee weapon, no off-hand
	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand})
	registry.Add("fighter-1", weapons)

	// Wrap context with character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)

	// Create Dueling fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Dueling,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create damage chain event with weapon damage
	damageEvent := &combat.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []combat.DamageComponent{
			{
				Source:            combat.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        "slashing",
				IsCritical:        false,
			},
		},
		DamageType:   "slashing",
		IsCritical:   false,
		WeaponDamage: "1d8",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*combat.DamageChainEvent](combat.ModifierStages)
	damages := combat.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Verify weapon component is unchanged
	weaponComponent := finalEvent.Components[0]
	s.Equal(3, weaponComponent.FlatBonus, "Weapon component should have original STR modifier")

	// Verify Dueling added a separate component
	s.Require().Len(finalEvent.Components, 2, "Should have weapon + dueling components")
	duelingComponent := finalEvent.Components[1]
	s.Equal(combat.DamageSourceDueling, duelingComponent.Source, "Second component should be from Dueling")
	s.Equal(2, duelingComponent.FlatBonus, "Dueling should add +2 damage")
}

// TestDuelingBonusWithShield verifies Dueling adds +2 when wielding shield
//
//nolint:dupl // Test duplication acceptable for clarity
func (s *FightingStyleTestSuite) TestDuelingBonusWithShield() {
	// Import gamectx
	ctx := s.ctx

	// Create character registry with one-handed melee weapon and shield
	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:          "shield-1",
		Name:        "Shield",
		Slot:        "off_hand",
		IsShield:    true,
		IsTwoHanded: false,
		IsMelee:     false,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, shield})
	registry.Add("fighter-1", weapons)

	// Wrap context with character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)

	// Create Dueling fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Dueling,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create damage chain event with weapon damage
	damageEvent := &combat.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []combat.DamageComponent{
			{
				Source:            combat.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        "slashing",
				IsCritical:        false,
			},
		},
		DamageType:   "slashing",
		IsCritical:   false,
		WeaponDamage: "1d8",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*combat.DamageChainEvent](combat.ModifierStages)
	damages := combat.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Verify weapon component is unchanged
	weaponComponent := finalEvent.Components[0]
	s.Equal(3, weaponComponent.FlatBonus, "Weapon component should have original STR modifier")

	// Verify Dueling added a separate component (shields don't count as weapons)
	s.Require().Len(finalEvent.Components, 2, "Should have weapon + dueling components")
	duelingComponent := finalEvent.Components[1]
	s.Equal(combat.DamageSourceDueling, duelingComponent.Source, "Second component should be from Dueling")
	s.Equal(2, duelingComponent.FlatBonus, "Dueling should add +2 damage even with shield")
}

// TestDuelingNoBonusWithTwoHandedWeapon verifies Dueling does NOT add bonus with two-handed weapon
func (s *FightingStyleTestSuite) TestDuelingNoBonusWithTwoHandedWeapon() {
	// Import gamectx
	ctx := s.ctx

	// Create character registry with two-handed weapon
	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:          "greatsword-1",
		Name:        "Greatsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: true,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand})
	registry.Add("fighter-1", weapons)

	// Wrap context with character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)

	// Create Dueling fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Dueling,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create damage chain event with weapon damage
	damageEvent := &combat.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []combat.DamageComponent{
			{
				Source:            combat.DamageSourceWeapon,
				OriginalDiceRolls: []int{6, 5},
				FinalDiceRolls:    []int{6, 5},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        "slashing",
				IsCritical:        false,
			},
		},
		DamageType:   "slashing",
		IsCritical:   false,
		WeaponDamage: "2d6",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*combat.DamageChainEvent](combat.ModifierStages)
	damages := combat.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Verify NO Dueling component was added (only weapon component)
	s.Len(finalEvent.Components, 1, "Should only have weapon component, no Dueling")
	weaponComponent := finalEvent.Components[0]
	s.Equal(3, weaponComponent.FlatBonus, "Should NOT add Dueling bonus with two-handed weapon")
}

// TestDuelingNoBonusWithDualWield verifies Dueling does NOT add bonus when dual-wielding
//
//nolint:dupl // Test duplication acceptable for clarity
func (s *FightingStyleTestSuite) TestDuelingNoBonusWithDualWield() {
	// Import gamectx
	ctx := s.ctx

	// Create character registry with weapon in each hand
	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:          "shortsword-1",
		Name:        "Shortsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	offHand := &gamectx.EquippedWeapon{
		ID:          "shortsword-2",
		Name:        "Shortsword",
		Slot:        "off_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, offHand})
	registry.Add("fighter-1", weapons)

	// Wrap context with character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)

	// Create Dueling fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Dueling,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create damage chain event with weapon damage
	damageEvent := &combat.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []combat.DamageComponent{
			{
				Source:            combat.DamageSourceWeapon,
				OriginalDiceRolls: []int{5},
				FinalDiceRolls:    []int{5},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        "piercing",
				IsCritical:        false,
			},
		},
		DamageType:   "piercing",
		IsCritical:   false,
		WeaponDamage: "1d6",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*combat.DamageChainEvent](combat.ModifierStages)
	damages := combat.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Verify NO Dueling component was added (only weapon component)
	s.Len(finalEvent.Components, 1, "Should only have weapon component, no Dueling")
	weaponComponent := finalEvent.Components[0]
	s.Equal(3, weaponComponent.FlatBonus, "Should NOT add Dueling bonus when dual-wielding")
}
