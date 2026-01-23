// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type MartialArtsTestSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	bus         events.EventBus
	ctx         context.Context
	mockRoller  *mock_dice.MockRoller
	characterID string
	registry    *gamectx.BasicCharacterRegistry
}

func TestMartialArtsTestSuite(t *testing.T) {
	suite.Run(t, new(MartialArtsTestSuite))
}

func (s *MartialArtsTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.characterID = "monk-1"
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)

	// Set up registry with ability scores
	s.registry = gamectx.NewBasicCharacterRegistry()
	scores := &gamectx.AbilityScores{
		Strength:     10, // +0 modifier
		Dexterity:    16, // +3 modifier
		Constitution: 14,
		Intelligence: 10,
		Wisdom:       15,
		Charisma:     8,
	}
	s.registry.AddAbilityScores(s.characterID, scores)

	// Set up context with game context
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: s.registry,
	})
	s.ctx = gamectx.WithGameContext(context.Background(), gameCtx)
}

func (s *MartialArtsTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestApplyAndRemove verifies basic apply/remove functionality
func (s *MartialArtsTestSuite) TestApplyAndRemove() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		CharacterID: s.characterID,
		MonkLevel:   1,
	})

	// Verify not applied initially
	s.False(condition.IsApplied())

	// Apply
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(condition.IsApplied())

	// Apply again should error
	err = condition.Apply(s.ctx, s.bus)
	s.Error(err)

	// Remove
	err = condition.Remove(s.ctx, s.bus)
	s.NoError(err)
	s.False(condition.IsApplied())

	// Remove again should be no-op
	err = condition.Remove(s.ctx, s.bus)
	s.NoError(err)
}

// TestUnarmedStrikeDamageScaling tests that unarmed damage scales with monk level
func (s *MartialArtsTestSuite) TestUnarmedStrikeDamageScaling() {
	testCases := []struct {
		name          string
		monkLevel     int
		expectedDice  string
		expectedRolls []int
	}{
		{
			name:          "Level 1-4: 1d4",
			monkLevel:     1,
			expectedDice:  "1d4",
			expectedRolls: []int{3},
		},
		{
			name:          "Level 5-10: 1d6",
			monkLevel:     5,
			expectedDice:  "1d6",
			expectedRolls: []int{5},
		},
		{
			name:          "Level 11-16: 1d8",
			monkLevel:     11,
			expectedDice:  "1d8",
			expectedRolls: []int{7},
		},
		{
			name:          "Level 17+: 1d10",
			monkLevel:     17,
			expectedDice:  "1d10",
			expectedRolls: []int{9},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			condition := NewMartialArtsCondition(MartialArtsInput{
				CharacterID: s.characterID,
				MonkLevel:   tc.monkLevel,
				Roller:      s.mockRoller,
			})

			err := condition.Apply(s.ctx, s.bus)
			s.Require().NoError(err)
			defer func() {
				_ = condition.Remove(s.ctx, s.bus)
			}()

			// Set up mock roller to return expected rolls
			// ParseNotation for 1d4, 1d6, 1d8, 1d10 will call RollN(ctx, 1, dieSize)
			var dieSize int
			switch tc.expectedDice {
			case "1d4":
				dieSize = 4
			case "1d6":
				dieSize = 6
			case "1d8":
				dieSize = 8
			case "1d10":
				dieSize = 10
			}
			s.mockRoller.EXPECT().
				RollN(gomock.Any(), 1, dieSize).
				Return(tc.expectedRolls, nil)

			// Create damage chain event for unarmed strike
			event := &dnd5eEvents.DamageChainEvent{
				AttackerID: s.characterID,
				TargetID:   "target-1",
				Components: []dnd5eEvents.DamageComponent{
					{
						Source:            dnd5eEvents.DamageSourceWeapon,
						OriginalDiceRolls: []int{1}, // Will be replaced
						FinalDiceRolls:    []int{1},
						FlatBonus:         0,
						DamageType:        "bludgeoning",
						IsCritical:        false,
					},
					{
						Source:     dnd5eEvents.DamageSourceAbility,
						FlatBonus:  0, // Will be replaced with DEX modifier
						DamageType: "bludgeoning",
					},
				},
				DamageType:   "bludgeoning",
				IsCritical:   false,
				WeaponDamage: "1", // Will be replaced with martial arts dice
				AbilityUsed:  abilities.STR,
				WeaponRef:    refs.Weapons.UnarmedStrike(),
			}

			// Publish through damage chain
			damageChain := dnd5eEvents.DamageChain.On(s.bus)
			chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

			modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
			s.Require().NoError(err)

			finalEvent, err := modifiedChain.Execute(s.ctx, event)
			s.Require().NoError(err)

			// Verify weapon damage dice were updated
			s.Equal(tc.expectedDice, finalEvent.WeaponDamage)

			// Verify weapon component has new rolls
			weaponComponent := &finalEvent.Components[0]
			s.Equal(tc.expectedRolls, weaponComponent.FinalDiceRolls)

			// Verify ability modifier was replaced with DEX
			abilityComponent := &finalEvent.Components[1]
			s.Equal(3, abilityComponent.FlatBonus) // DEX modifier (+3)
			s.Equal(abilities.DEX, finalEvent.AbilityUsed)
		})
	}
}

// TestUnarmedStrikeCriticalDamage tests that crits double the martial arts dice
func (s *MartialArtsTestSuite) TestUnarmedStrikeCriticalDamage() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		CharacterID: s.characterID,
		MonkLevel:   5, // 1d6 martial arts die
		Roller:      s.mockRoller,
	})

	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = condition.Remove(s.ctx, s.bus)
	}()

	// Set up mock roller to return rolls for crit (doubles dice)
	// First roll: 1d6
	s.mockRoller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)
	// Second roll: 1d6 (for crit)
	s.mockRoller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	// Create critical damage chain event
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: s.characterID,
		TargetID:   "target-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{1}, // Will be replaced
				FinalDiceRolls:    []int{1},
				FlatBonus:         0,
				DamageType:        "bludgeoning",
				IsCritical:        true,
			},
			{
				Source:     dnd5eEvents.DamageSourceAbility,
				FlatBonus:  0,
				DamageType: "bludgeoning",
				IsCritical: true,
			},
		},
		DamageType:   "bludgeoning",
		IsCritical:   true,
		WeaponDamage: "1",
		AbilityUsed:  abilities.STR,
		WeaponRef:    refs.Weapons.UnarmedStrike(),
	}

	// Publish through damage chain
	damageChain := dnd5eEvents.DamageChain.On(s.bus)
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

	modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify weapon component has two dice (critical)
	weaponComponent := &finalEvent.Components[0]
	s.Equal([]int{4, 4}, weaponComponent.FinalDiceRolls)
}

// TestDEXModifierReplacement tests that DEX replaces STR when DEX > STR
//
//nolint:dupl // Test cases require similar setup code
func (s *MartialArtsTestSuite) TestDEXModifierReplacement() {
	s.Run("DEX higher than STR - use DEX", func() {
		// Registry already has DEX=16 (+3), STR=10 (+0)
		condition := NewMartialArtsCondition(MartialArtsInput{
			CharacterID: s.characterID,
			MonkLevel:   1,
		})

		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() {
			_ = condition.Remove(s.ctx, s.bus)
		}()

		event := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.characterID,
			TargetID:   "target-1",
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{3},
					FinalDiceRolls:    []int{3},
				},
				{
					Source:    dnd5eEvents.DamageSourceAbility,
					FlatBonus: 0, // STR modifier
				},
			},
			AbilityUsed: abilities.STR,
			WeaponRef:   refs.Weapons.UnarmedStrike(),
		}

		damageChain := dnd5eEvents.DamageChain.On(s.bus)
		chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

		modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, event)
		s.Require().NoError(err)

		// Verify DEX modifier is used
		s.Equal(3, finalEvent.Components[1].FlatBonus)
		s.Equal(abilities.DEX, finalEvent.AbilityUsed)
	})

	s.Run("STR higher than DEX - use STR", func() {
		// Create a monk with higher STR than DEX
		strongMonk := "monk-str"
		scores := &gamectx.AbilityScores{
			Strength:     16, // +3 modifier
			Dexterity:    14, // +2 modifier
			Constitution: 14,
			Intelligence: 10,
			Wisdom:       15,
			Charisma:     8,
		}
		s.registry.AddAbilityScores(strongMonk, scores)

		condition := NewMartialArtsCondition(MartialArtsInput{
			CharacterID: strongMonk,
			MonkLevel:   1,
		})

		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() {
			_ = condition.Remove(s.ctx, s.bus)
		}()

		event := &dnd5eEvents.DamageChainEvent{
			AttackerID: strongMonk,
			TargetID:   "target-1",
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{3},
					FinalDiceRolls:    []int{3},
				},
				{
					Source:    dnd5eEvents.DamageSourceAbility,
					FlatBonus: 3, // STR modifier
				},
			},
			AbilityUsed: abilities.STR,
			WeaponRef:   refs.Weapons.UnarmedStrike(),
		}

		damageChain := dnd5eEvents.DamageChain.On(s.bus)
		chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

		modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, event)
		s.Require().NoError(err)

		// Verify STR modifier is retained (DEX is not higher)
		s.Equal(3, finalEvent.Components[1].FlatBonus)
		s.Equal(abilities.STR, finalEvent.AbilityUsed)
	})
}

// TestMonkWeaponDetection tests that monk weapons are correctly identified
func (s *MartialArtsTestSuite) TestMonkWeaponDetection() {
	testCases := []struct {
		name         string
		weaponID     weapons.WeaponID
		isMonkWeapon bool
	}{
		{
			name:         "Shortsword is monk weapon",
			weaponID:     weapons.Shortsword,
			isMonkWeapon: true,
		},
		{
			name:         "Club is monk weapon (simple melee, no Heavy/TwoHanded)",
			weaponID:     weapons.Club,
			isMonkWeapon: true,
		},
		{
			name:         "Quarterstaff is monk weapon",
			weaponID:     weapons.Quarterstaff,
			isMonkWeapon: true,
		},
		{
			name:         "Greatsword is NOT monk weapon (Heavy)",
			weaponID:     weapons.Greatsword,
			isMonkWeapon: false,
		},
		{
			name:         "Longbow is NOT monk weapon (ranged)",
			weaponID:     weapons.Longbow,
			isMonkWeapon: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			weapon, err := weapons.GetByID(tc.weaponID)
			s.Require().NoError(err)

			result := isMonkWeapon(&weapon)
			s.Equal(tc.isMonkWeapon, result)
		})
	}
}

// TestMonkWeaponDEXUsage tests that monk weapons can use DEX
//
//nolint:dupl // Test cases require similar setup code
func (s *MartialArtsTestSuite) TestMonkWeaponDEXUsage() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		CharacterID: s.characterID,
		MonkLevel:   1,
	})

	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = condition.Remove(s.ctx, s.bus)
	}()

	// Test with quarterstaff (monk weapon)
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: s.characterID,
		TargetID:   "target-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
			},
			{
				Source:    dnd5eEvents.DamageSourceAbility,
				FlatBonus: 0, // Will be replaced with DEX
			},
		},
		AbilityUsed: abilities.STR,
		WeaponRef:   refs.Weapons.Quarterstaff(),
	}

	damageChain := dnd5eEvents.DamageChain.On(s.bus)
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

	modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify DEX modifier is used for monk weapon
	s.Equal(3, finalEvent.Components[1].FlatBonus) // DEX modifier (+3)
	s.Equal(abilities.DEX, finalEvent.AbilityUsed)
}

// TestNonMonkWeaponNotModified tests that non-monk weapons are not modified
//
//nolint:dupl // Test cases require similar setup code
func (s *MartialArtsTestSuite) TestNonMonkWeaponNotModified() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		CharacterID: s.characterID,
		MonkLevel:   1,
	})

	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = condition.Remove(s.ctx, s.bus)
	}()

	// Test with greatsword (not a monk weapon)
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: s.characterID,
		TargetID:   "target-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{10},
				FinalDiceRolls:    []int{10},
			},
			{
				Source:    dnd5eEvents.DamageSourceAbility,
				FlatBonus: 0, // Should stay 0 (not modified)
			},
		},
		AbilityUsed: abilities.STR,
		WeaponRef:   refs.Weapons.Greatsword(),
	}

	damageChain := dnd5eEvents.DamageChain.On(s.bus)
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

	modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify event was not modified (greatsword is not a monk weapon)
	s.Equal(0, finalEvent.Components[1].FlatBonus)
	s.Equal(abilities.STR, finalEvent.AbilityUsed)
}

// TestSerialization tests JSON serialization round-trip
func (s *MartialArtsTestSuite) TestSerialization() {
	original := NewMartialArtsCondition(MartialArtsInput{
		CharacterID: s.characterID,
		MonkLevel:   5,
	})

	// Serialize
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Verify structure
	var data MartialArtsData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Equal(refs.Conditions.MartialArts(), data.Ref)
	s.Equal(s.characterID, data.CharacterID)
	s.Equal(5, data.MonkLevel)

	// Deserialize
	loaded := &MartialArtsCondition{}
	err = loaded.loadJSON(jsonData)
	s.Require().NoError(err)

	// Verify fields
	s.Equal(original.CharacterID, loaded.CharacterID)
	s.Equal(original.MonkLevel, loaded.MonkLevel)
}

// TestOtherCharacterNotModified verifies that other characters' attacks are not modified
func (s *MartialArtsTestSuite) TestOtherCharacterNotModified() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		CharacterID: s.characterID,
		MonkLevel:   5,
	})

	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() {
		_ = condition.Remove(s.ctx, s.bus)
	}()

	// Attack by different character
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "other-character",
		TargetID:   "target-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{1},
				FinalDiceRolls:    []int{1},
			},
			{
				Source:    dnd5eEvents.DamageSourceAbility,
				FlatBonus: 2,
			},
		},
		WeaponRef: refs.Weapons.UnarmedStrike(),
	}

	originalEvent := *event

	damageChain := dnd5eEvents.DamageChain.On(s.bus)
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

	modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Verify event was not modified
	s.Equal(originalEvent.Components[0].FinalDiceRolls, finalEvent.Components[0].FinalDiceRolls)
	s.Equal(originalEvent.Components[1].FlatBonus, finalEvent.Components[1].FlatBonus)
}
