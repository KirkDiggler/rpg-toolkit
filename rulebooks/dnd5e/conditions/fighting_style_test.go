// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

			// Create attack chain event with IsMelee directly set
			attackEvent := dnd5eEvents.AttackChainEvent{
				AttackerID:        "fighter-1",
				TargetID:          "goblin-1",
				IsMelee:           tc.isMelee,
				AttackBonus:       tc.baseBonus,
				TargetAC:          13,
				CriticalThreshold: 20,
			}

			// Publish through attack chain
			attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
			attacks := dnd5eEvents.AttackChain.On(s.bus)
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
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{1, 2, 6}, // Two dice need rerolling
				FinalDiceRolls:    []int{1, 2, 6},
				Rerolls:           nil,
				FlatBonus:         0,
				DamageType:        damage.Slashing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Slashing,
		IsCritical:   false,
		WeaponDamage: "2d6", // Greatsword
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
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
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{5, 6}, // No rerolls needed
				FinalDiceRolls:    []int{5, 6},
				Rerolls:           nil,
				FlatBonus:         0,
				DamageType:        damage.Slashing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Slashing,
		IsCritical:   false,
		WeaponDamage: "2d6",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
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
	// Try to apply an unspecified style
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Unspecified,
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
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        damage.Slashing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Slashing,
		IsCritical:   false,
		WeaponDamage: "1d8",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
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
	s.Equal(dnd5eEvents.DamageSourceFeature, duelingComponent.Source, "Second component should be from Dueling")
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
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        damage.Slashing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Slashing,
		IsCritical:   false,
		WeaponDamage: "1d8",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
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
	s.Equal(dnd5eEvents.DamageSourceFeature, duelingComponent.Source, "Second component should be from Dueling")
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
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6, 5},
				FinalDiceRolls:    []int{6, 5},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        damage.Slashing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Slashing,
		IsCritical:   false,
		WeaponDamage: "2d6",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
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
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{5},
				FinalDiceRolls:    []int{5},
				Rerolls:           nil,
				FlatBonus:         3, // STR modifier
				DamageType:        damage.Piercing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		WeaponDamage: "1d6",
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
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

// TestTwoWeaponFightingOffHandBonus verifies Two-Weapon Fighting adds ability modifier to off-hand attacks
func (s *FightingStyleTestSuite) TestTwoWeaponFightingOffHandBonus() {
	// Import gamectx
	ctx := s.ctx

	// Create character registry with dual-wielding weapons
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

	// Add ability scores (DEX 16 = +3 modifier)
	registry.AddAbilityScores("fighter-1", &gamectx.AbilityScores{
		Strength:     10,
		Dexterity:    16, // +3 modifier
		Constitution: 10,
		Intelligence: 10,
		Wisdom:       10,
		Charisma:     10,
	})

	// Wrap context with character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)

	// Create Two-Weapon Fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.TwoWeaponFighting,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create damage chain event for off-hand attack (using DEX 16 = +3 modifier)
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{5},
				FinalDiceRolls:    []int{5},
				Rerolls:           nil,
				FlatBonus:         0, // Normally off-hand doesn't get ability modifier
				DamageType:        damage.Piercing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		WeaponDamage: "1d6",
		AbilityUsed:  abilities.DEX, // DEX is used for finesse weapons
		WeaponRef:    &core.Ref{ID: core.ID("shortsword-2")},
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Verify weapon component is unchanged
	weaponComponent := finalEvent.Components[0]
	s.Equal(0, weaponComponent.FlatBonus, "Weapon component should have no ability modifier initially")

	// Verify Two-Weapon Fighting added a separate component with ability modifier
	s.Require().Len(finalEvent.Components, 2, "Should have weapon + two-weapon fighting components")
	twfComponent := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, twfComponent.Source,
		"Second component should be from Two-Weapon Fighting")
	s.Equal(3, twfComponent.FlatBonus, "Two-Weapon Fighting should add +3 (DEX modifier)")
}

// TestTwoWeaponFightingNoMainHandBonus verifies Two-Weapon Fighting doesn't add to main-hand attacks
func (s *FightingStyleTestSuite) TestTwoWeaponFightingNoMainHandBonus() {
	// Import gamectx
	ctx := s.ctx

	// Create character registry with dual-wielding weapons
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

	// Add ability scores (DEX 16 = +3 modifier)
	registry.AddAbilityScores("fighter-1", &gamectx.AbilityScores{
		Strength:     10,
		Dexterity:    16, // +3 modifier
		Constitution: 10,
		Intelligence: 10,
		Wisdom:       10,
		Charisma:     10,
	})

	// Wrap context with character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)

	// Create Two-Weapon Fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.TwoWeaponFighting,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create damage chain event for MAIN-hand attack
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{5},
				FinalDiceRolls:    []int{5},
				Rerolls:           nil,
				FlatBonus:         3, // Main hand already gets ability modifier
				DamageType:        damage.Piercing,
				IsCritical:        false,
			},
		},
		DamageType:   damage.Piercing,
		IsCritical:   false,
		WeaponDamage: "1d6",
		AbilityUsed:  abilities.DEX,
		WeaponRef:    &core.Ref{ID: core.ID("shortsword-1")}, // MAIN HAND weapon
	}

	// Publish through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Verify NO Two-Weapon Fighting component was added (only weapon component)
	s.Len(finalEvent.Components, 1, "Should only have weapon component, no Two-Weapon Fighting bonus")
	weaponComponent := finalEvent.Components[0]
	s.Equal(3, weaponComponent.FlatBonus, "Main hand should keep original ability modifier")
}

// TestTwoWeaponFightingAbilityModifier verifies Two-Weapon Fighting uses the correct ability modifier
// based on which ability was used for the attack (STR or DEX)
func (s *FightingStyleTestSuite) TestTwoWeaponFightingAbilityModifier() {
	testCases := []struct {
		name             string
		strength         int
		dexterity        int
		abilityUsed      abilities.Ability
		expectedModifier int
	}{
		{
			name:             "DEX16_PlusThree",
			strength:         10,
			dexterity:        16,
			abilityUsed:      abilities.DEX,
			expectedModifier: 3, // (16 - 10) / 2 = 3
		},
		{
			name:             "DEX14_PlusTwo",
			strength:         10,
			dexterity:        14,
			abilityUsed:      abilities.DEX,
			expectedModifier: 2, // (14 - 10) / 2 = 2
		},
		{
			name:             "DEX10_Zero",
			strength:         10,
			dexterity:        10,
			abilityUsed:      abilities.DEX,
			expectedModifier: 0, // (10 - 10) / 2 = 0
		},
		{
			name:             "DEX8_MinusOne",
			strength:         10,
			dexterity:        8,
			abilityUsed:      abilities.DEX,
			expectedModifier: -1, // (8 - 10) / 2 = -1 (floor division)
		},
		{
			name:             "STR16_PlusThree",
			strength:         16,
			dexterity:        10,
			abilityUsed:      abilities.STR,
			expectedModifier: 3, // (16 - 10) / 2 = 3
		},
		{
			name:             "STR14_PlusTwo",
			strength:         14,
			dexterity:        10,
			abilityUsed:      abilities.STR,
			expectedModifier: 2, // (14 - 10) / 2 = 2
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctx := s.ctx

			// Create character registry with dual-wielding weapons
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

			// Add ability scores with the test case's STR and DEX values
			registry.AddAbilityScores("fighter-1", &gamectx.AbilityScores{
				Strength:     tc.strength,
				Dexterity:    tc.dexterity,
				Constitution: 10,
				Intelligence: 10,
				Wisdom:       10,
				Charisma:     10,
			})

			// Wrap context with character registry
			gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
				CharacterRegistry: registry,
			})
			ctx = gamectx.WithGameContext(ctx, gameCtx)

			// Create Two-Weapon Fighting style condition
			fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
				CharacterID: "fighter-1",
				Style:       fightingstyles.TwoWeaponFighting,
				Roller:      s.mockRoller,
			})

			// Apply the condition
			err := fs.Apply(ctx, s.bus)
			s.Require().NoError(err)
			defer func() {
				_ = fs.Remove(ctx, s.bus)
			}()

			// Create damage chain event for off-hand attack using the test case's ability
			damageEvent := &dnd5eEvents.DamageChainEvent{
				AttackerID: "fighter-1",
				TargetID:   "goblin-1",
				Components: []dnd5eEvents.DamageComponent{
					{
						Source:            dnd5eEvents.DamageSourceWeapon,
						OriginalDiceRolls: []int{5},
						FinalDiceRolls:    []int{5},
						Rerolls:           nil,
						FlatBonus:         0, // Off-hand doesn't get ability modifier normally
						DamageType:        damage.Piercing,
						IsCritical:        false,
					},
				},
				DamageType:   damage.Piercing,
				IsCritical:   false,
				WeaponDamage: "1d6",
				AbilityUsed:  tc.abilityUsed,
				WeaponRef:    &core.Ref{ID: core.ID("shortsword-2")}, // Off-hand weapon
			}

			// Publish through damage chain
			damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
			damages := dnd5eEvents.DamageChain.On(s.bus)
			modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
			s.Require().NoError(err)

			// Execute chain
			finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
			s.Require().NoError(err)

			// Verify Two-Weapon Fighting added correct ability modifier
			s.Require().Len(finalEvent.Components, 2, "Should have weapon + TWF components")
			twfComponent := finalEvent.Components[1]
			s.Equal(dnd5eEvents.DamageSourceFeature, twfComponent.Source,
				"Second component should be from Two-Weapon Fighting")
			s.Equal(tc.expectedModifier, twfComponent.FlatBonus,
				"Two-Weapon Fighting should add %s modifier %d", tc.abilityUsed, tc.expectedModifier)
		})
	}
}

// TestDefenseACBonus verifies Defense fighting style adds +1 to AC when wearing armor
//
//nolint:dupl // Test duplication acceptable for clarity
func (s *FightingStyleTestSuite) TestDefenseACBonus() {
	// Create Defense fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Defense,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(s.ctx, s.bus)
	}()

	// Create AC chain event for a character wearing armor
	breakdown := &combat.ACBreakdown{
		Total: 16,
		Components: []combat.ACComponent{
			{Type: combat.ACSourceBase, Source: nil, Value: 10},
			{Type: combat.ACSourceArmor, Source: &core.Ref{
				Module: "dnd5e",
				Type:   "armor",
				ID:     "chain_mail",
			}, Value: 6},
		},
	}
	acEvent := &combat.ACChainEvent{
		CharacterID: "fighter-1",
		Breakdown:   breakdown,
		HasArmor:    true,
		HasShield:   false,
	}

	// Publish through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acs := combat.ACChain.On(s.bus)
	modifiedChain, err := acs.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify Defense added +1 to AC
	s.Equal(17, finalEvent.Breakdown.Total, "Defense should add +1 to AC when wearing armor")
}

// TestDefenseNoArmorNoBonus verifies Defense fighting style doesn't add bonus when not wearing armor
//
//nolint:dupl // Test duplication acceptable for clarity
func (s *FightingStyleTestSuite) TestDefenseNoArmorNoBonus() {
	// Create Defense fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Defense,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err := fs.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(s.ctx, s.bus)
	}()

	// Create AC chain event for a character NOT wearing armor
	breakdown := &combat.ACBreakdown{
		Total: 13,
		Components: []combat.ACComponent{
			{Type: combat.ACSourceBase, Source: nil, Value: 10},
			{Type: combat.ACSourceAbility, Source: nil, Value: 3},
		},
	}
	acEvent := &combat.ACChainEvent{
		CharacterID: "fighter-1",
		Breakdown:   breakdown,
		HasArmor:    false,
		HasShield:   false,
	}

	// Publish through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acs := combat.ACChain.On(s.bus)
	modifiedChain, err := acs.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify Defense did NOT add any bonus
	s.Equal(13, finalEvent.Breakdown.Total, "Defense should NOT add bonus when not wearing armor")
}

// testEntity is a simple entity for testing
type testEntity struct {
	id   string
	kind string
}

func (e *testEntity) GetID() string            { return e.id }
func (e *testEntity) GetType() core.EntityType { return core.EntityType(e.kind) }

// TestProtectionImposesDisadvantage verifies Protection imposes disadvantage on attacks against nearby allies
func (s *FightingStyleTestSuite) TestProtectionImposesDisadvantage() {
	ctx := s.ctx

	// Create a room with entities
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	// Place entities: fighter at (5,5), ally at (6,5) (adjacent), enemy at (4,5)
	fighter := &testEntity{id: "fighter-1", kind: "character"}
	ally := &testEntity{id: "ally-1", kind: "character"}
	enemy := &testEntity{id: "enemy-1", kind: "monster"}

	err := room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(ally, spatial.Position{X: 6, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(enemy, spatial.Position{X: 4, Y: 5})
	s.Require().NoError(err)

	// Create character registry with shield equipped and reaction available
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
		ID:       "shield-1",
		Name:     "Shield",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, shield})
	registry.Add("fighter-1", weapons)

	// Add action economy with reaction available
	actionEconomy := combat.NewActionEconomy()
	registry.AddActionEconomy("fighter-1", actionEconomy)

	// Wrap context with room and character registry
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	// Create Protection fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Protection,
		Roller:      s.mockRoller,
	})

	// Apply the condition
	err = fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create attack chain event: enemy attacks ally (melee)
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "enemy-1",
		TargetID:          "ally-1",
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	// Publish through attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Verify disadvantage was imposed
	s.Require().Len(finalEvent.DisadvantageSources, 1, "Protection should add disadvantage source")
	s.Equal("fighter-1", finalEvent.DisadvantageSources[0].SourceID)
	s.Contains(finalEvent.DisadvantageSources[0].Reason, "Protection")

	// Verify reaction was consumed
	s.Require().Len(finalEvent.ReactionsConsumed, 1, "Protection should consume reaction")
	s.Equal("fighter-1", finalEvent.ReactionsConsumed[0].CharacterID)
}

// TestProtectionRequiresShield verifies Protection doesn't trigger without a shield
func (s *FightingStyleTestSuite) TestProtectionRequiresShield() {
	ctx := s.ctx

	// Create a room with entities
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	// Place entities adjacent to each other
	fighter := &testEntity{id: "fighter-1", kind: "character"}
	ally := &testEntity{id: "ally-1", kind: "character"}

	err := room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(ally, spatial.Position{X: 6, Y: 5})
	s.Require().NoError(err)

	// Create character registry WITHOUT shield
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
	registry.AddActionEconomy("fighter-1", combat.NewActionEconomy())

	// Wrap context
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	// Create Protection fighting style condition
	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Protection,
		Roller:      s.mockRoller,
	})

	err = fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Create attack on ally
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "enemy-1",
		TargetID:          "ally-1",
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Verify NO disadvantage was imposed (no shield)
	s.Empty(finalEvent.DisadvantageSources, "Protection should NOT trigger without shield")
	s.Empty(finalEvent.ReactionsConsumed, "No reaction should be consumed without shield")
}

// TestProtectionRequiresReaction verifies Protection doesn't trigger without reaction available
func (s *FightingStyleTestSuite) TestProtectionRequiresReaction() {
	ctx := s.ctx

	// Create a room with entities
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	fighter := &testEntity{id: "fighter-1", kind: "character"}
	ally := &testEntity{id: "ally-1", kind: "character"}

	err := room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(ally, spatial.Position{X: 6, Y: 5})
	s.Require().NoError(err)

	// Create character registry with shield but reaction already used
	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:      "longsword-1",
		Name:    "Longsword",
		Slot:    "main_hand",
		IsMelee: true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Name:     "Shield",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, shield})
	registry.Add("fighter-1", weapons)

	// Action economy with reaction already used
	actionEconomy := combat.NewActionEconomy()
	_ = actionEconomy.UseReaction() // Consume the reaction
	registry.AddActionEconomy("fighter-1", actionEconomy)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Protection,
		Roller:      s.mockRoller,
	})

	err = fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "enemy-1",
		TargetID:          "ally-1",
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Verify NO disadvantage (no reaction available)
	s.Empty(finalEvent.DisadvantageSources, "Protection should NOT trigger without reaction")
}

// TestProtectionDoesNotTriggerForSelf verifies Protection doesn't trigger when the fighter is attacked
func (s *FightingStyleTestSuite) TestProtectionDoesNotTriggerForSelf() {
	ctx := s.ctx

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	fighter := &testEntity{id: "fighter-1", kind: "character"}
	err := room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)

	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:      "longsword-1",
		Slot:    "main_hand",
		IsMelee: true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, shield})
	registry.Add("fighter-1", weapons)
	registry.AddActionEconomy("fighter-1", combat.NewActionEconomy())

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Protection,
		Roller:      s.mockRoller,
	})

	err = fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// Attack the FIGHTER themselves (not an ally)
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "enemy-1",
		TargetID:          "fighter-1", // Attacking self
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Verify NO disadvantage (can't protect self)
	s.Empty(finalEvent.DisadvantageSources, "Protection should NOT trigger for attacks on self")
}

// TestProtectionOnlyMeleeAttacks verifies Protection only triggers on melee attacks
//
//nolint:dupl // Test duplication acceptable for clarity
func (s *FightingStyleTestSuite) TestProtectionOnlyMeleeAttacks() {
	ctx := s.ctx

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	fighter := &testEntity{id: "fighter-1", kind: "character"}
	ally := &testEntity{id: "ally-1", kind: "character"}

	err := room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(ally, spatial.Position{X: 6, Y: 5})
	s.Require().NoError(err)

	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:      "longsword-1",
		Slot:    "main_hand",
		IsMelee: true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, shield})
	registry.Add("fighter-1", weapons)
	registry.AddActionEconomy("fighter-1", combat.NewActionEconomy())

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Protection,
		Roller:      s.mockRoller,
	})

	err = fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	// RANGED attack on ally
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "enemy-1",
		TargetID:          "ally-1",
		IsMelee:           false, // Ranged attack
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Verify NO disadvantage (ranged attack)
	s.Empty(finalEvent.DisadvantageSources, "Protection should NOT trigger for ranged attacks")
}

// TestProtectionRequiresTargetWithin5ft verifies Protection only triggers for nearby allies
//
//nolint:dupl // Test duplication acceptable for clarity
func (s *FightingStyleTestSuite) TestProtectionRequiresTargetWithin5ft() {
	ctx := s.ctx

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	// Place fighter at (5,5) and ally at (8,5) - 3 squares away (15ft)
	fighter := &testEntity{id: "fighter-1", kind: "character"}
	ally := &testEntity{id: "ally-1", kind: "character"}

	err := room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(ally, spatial.Position{X: 8, Y: 5}) // 3 squares = 15ft
	s.Require().NoError(err)

	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:      "longsword-1",
		Slot:    "main_hand",
		IsMelee: true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, shield})
	registry.Add("fighter-1", weapons)
	registry.AddActionEconomy("fighter-1", combat.NewActionEconomy())

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx = gamectx.WithGameContext(ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	fs := conditions.NewFightingStyleCondition(conditions.FightingStyleConditionConfig{
		CharacterID: "fighter-1",
		Style:       fightingstyles.Protection,
		Roller:      s.mockRoller,
	})

	err = fs.Apply(ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = fs.Remove(ctx, s.bus)
	}()

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "enemy-1",
		TargetID:          "ally-1",
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Verify NO disadvantage (ally too far away)
	s.Empty(finalEvent.DisadvantageSources, "Protection should NOT trigger for allies beyond 5ft")
}
