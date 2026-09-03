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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type MartialArtsTestSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	bus         events.EventBus
	ctx         context.Context
	mockRoller  *mock_dice.MockRoller
	characterID string
	scores      shared.AbilityScores

	// strongID / strongScores are the mirror fixture: a monk whose STR beats
	// its DEX, so the swap must NOT fire.
	strongID     string
	strongScores shared.AbilityScores
}

func TestMartialArtsTestSuite(t *testing.T) {
	suite.Run(t, new(MartialArtsTestSuite))
}

func (s *MartialArtsTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.characterID = "monk-1"
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)

	// TWO monks, because the whole feature is a comparison and one of them
	// cannot test it. The default monk's DEX beats its STR, so the swap fires;
	// the strong monk's STR beats its DEX, so it must not. A rule hardcoded to
	// either ability passes half these tests and fails the other half, which is
	// the only way to tell a working comparison from a constant.
	s.scores = shared.AbilityScores{
		abilities.STR: 10, // +0 modifier
		abilities.DEX: 16, // +3 modifier
		abilities.CON: 14,
		abilities.INT: 10,
		abilities.WIS: 15,
		abilities.CHA: 8,
	}
	s.strongID = "monk-str"
	s.strongScores = shared.AbilityScores{
		abilities.STR: 16, // +3 modifier — higher than DEX
		abilities.DEX: 14, // +2 modifier
		abilities.CON: 14,
		abilities.INT: 10,
		abilities.WIS: 15,
		abilities.CHA: 8,
	}

	// The cast, installed the way resolution's one door installs it. Both monks
	// are in it: a condition reads ITSELF out of the cast by its own ID, so the
	// member it finds is the member whose scores it compares.
	s.ctx = castOf(context.Background(),
		&fakeConditionOwner{id: s.characterID, scores: s.scores},
		&fakeConditionOwner{id: s.strongID, scores: s.strongScores},
	)
}

func (s *MartialArtsTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestApplyAndRemove verifies basic apply/remove functionality
func (s *MartialArtsTestSuite) TestApplyAndRemove() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 1,
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
		name             string
		monkLevel        int
		expectedDice     string
		expectedNotation string
		expectedRolls    []int
	}{
		{
			name:             "Level 1-4: 1d4",
			monkLevel:        1,
			expectedDice:     "1d4",
			expectedNotation: "d4",
			expectedRolls:    []int{3},
		},
		{
			name:             "Level 5-10: 1d6",
			monkLevel:        5,
			expectedDice:     "1d6",
			expectedNotation: "d6",
			expectedRolls:    []int{5},
		},
		{
			name:             "Level 11-16: 1d8",
			monkLevel:        11,
			expectedDice:     "1d8",
			expectedNotation: "d8",
			expectedRolls:    []int{7},
		},
		{
			name:             "Level 17+: 1d10",
			monkLevel:        17,
			expectedDice:     "1d10",
			expectedNotation: "d10",
			expectedRolls:    []int{9},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			condition := NewMartialArtsCondition(MartialArtsInput{
				MemberID:  s.characterID,
				MonkLevel: tc.monkLevel,
				Roller:    s.mockRoller,
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
						Source: dnd5eEvents.DamageSourceWeapon,
						Roll: dnd5eEvents.RollComponent{
							Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
							Dice:   testDiceTrace(6, 1),
						},
						DamageType: "bludgeoning",
						Properties: []damage.Property{damage.AddsAttackAbilityModifier},
						IsCritical: false,
					},
					{
						Source: dnd5eEvents.DamageSourceAbility,
						Roll: dnd5eEvents.RollComponent{
							Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
							Modifier: intPtr(0), // Will be replaced with DEX modifier
						},
						DamageType: "bludgeoning",
					},
				},
				WeaponDamageDice: "1d1",
				IsCritical:       false,
				AbilityUsed:      abilities.STR,
				WeaponRef:        refs.Weapons.UnarmedStrike(),
			}

			// Publish through damage chain
			damageChain := dnd5eEvents.DamageChain.On(s.bus)
			chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

			modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
			s.Require().NoError(err)

			finalEvent, err := modifiedChain.Execute(s.ctx, event)
			s.Require().NoError(err)

			// Verify weapon damage dice were updated
			s.Equal(tc.expectedNotation, finalEvent.Components[0].Roll.Dice.Notation,
				"the trace records the physical pool in canonical notation")
			s.Equal(tc.expectedDice, finalEvent.WeaponDamageDice,
				"marked weapon metadata must follow the exact component replacement")

			// Verify weapon component has new rolls
			weaponComponent := &finalEvent.Components[0]
			s.Equal(tc.expectedRolls, weaponComponent.Roll.Dice.FinalRolls)

			// Verify ability modifier was replaced with DEX
			abilityComponent := &finalEvent.Components[1]
			s.Equal(3, abilityComponent.Total()) // DEX modifier (+3)
			s.Equal(abilities.DEX, finalEvent.AbilityUsed)
		})
	}
}

// TestUnarmedStrikeCriticalDamage tests that crits double the martial arts dice
func (s *MartialArtsTestSuite) TestUnarmedStrikeCriticalDamage() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 5, // 1d6 martial arts die
		Roller:    s.mockRoller,
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
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(6, 1),
				},
				DamageType: "bludgeoning",
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				IsCritical: true,
			},
			{
				Source: dnd5eEvents.DamageSourceAbility,
				Roll: dnd5eEvents.RollComponent{
					Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
					Modifier: intPtr(0),
				},
				DamageType: "bludgeoning",
				IsCritical: true,
			},
		},
		IsCritical:  true,
		AbilityUsed: abilities.STR,
		WeaponRef:   refs.Weapons.UnarmedStrike(),
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
	s.Equal([]int{4, 4}, weaponComponent.Roll.Dice.FinalRolls)
}

func (s *MartialArtsTestSuite) TestUnarmedStrikeReplacesMarkedPrimaryWhenPoolsShareType() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 1,
		Roller:    s.mockRoller,
	})
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	defer func() { _ = condition.Remove(s.ctx, s.bus) }()

	s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{3}, nil)

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID:  s.characterID,
		TargetID:    "target-1",
		AbilityUsed: abilities.STR,
		WeaponRef:   refs.Weapons.UnarmedStrike(),
		Components: []dnd5eEvents.DamageComponent{
			{
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(10, 9),
				}, DamageType: damage.Bludgeoning,
			},
			{
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(6, 1),
				}, DamageType: damage.Bludgeoning,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			},
			{
				Source:     dnd5eEvents.DamageSourceAbility,
				DamageType: "bludgeoning",
			},
		},
	}

	damageChain := dnd5eEvents.DamageChain.On(s.bus)
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Equal([]int{9}, finalEvent.Components[0].Roll.Dice.FinalRolls)
	s.Equal([]int{3}, finalEvent.Components[1].Roll.Dice.FinalRolls)
	s.Equal("d4", finalEvent.Components[1].Roll.Dice.Notation)
	s.Equal("d10", finalEvent.Components[0].Roll.Dice.Notation)
}

func (s *MartialArtsTestSuite) TestUnarmedStrikeDoesNotDoubleMarkedDoesNotCritPool() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 1,
		Roller:    s.mockRoller,
	})
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	defer func() { _ = condition.Remove(s.ctx, s.bus) }()

	s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{3}, nil).Times(1)

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID:  s.characterID,
		TargetID:    "target-1",
		IsCritical:  true,
		AbilityUsed: abilities.STR,
		WeaponRef:   refs.Weapons.UnarmedStrike(),
		Components: []dnd5eEvents.DamageComponent{
			{
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(6, 1),
				}, DamageType: damage.Bludgeoning,
				Properties: []damage.Property{
					damage.AddsAttackAbilityModifier,
					damage.DoesNotCrit,
				},
			},
		},
	}

	damageChain := dnd5eEvents.DamageChain.On(s.bus)
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Equal([]int{3}, finalEvent.Components[0].Roll.Dice.FinalRolls)
	s.False(finalEvent.Components[0].IsCritical)
}

// TestDEXModifierReplacement tests that DEX replaces STR when DEX > STR
//
//nolint:dupl // Test cases require similar setup code
func (s *MartialArtsTestSuite) TestDEXModifierReplacement() {
	s.Run("DEX higher than STR - use DEX", func() {
		// Registry already has DEX=16 (+3), STR=10 (+0)
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
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
					Source: dnd5eEvents.DamageSourceWeapon,
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 3),
					},
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(0), // STR modifier
					},
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
		s.Equal(3, finalEvent.Components[1].Total())
		s.Equal(abilities.DEX, finalEvent.AbilityUsed)
	})

	s.Run("STR higher than DEX - use STR", func() {
		// Create a monk with higher STR than DEX
		strongMonk := s.strongID
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  strongMonk,
			MonkLevel: 1,
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
					Source: dnd5eEvents.DamageSourceWeapon,
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 3),
					},
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(3), // STR modifier
					},
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
		s.Equal(3, finalEvent.Components[1].Total())
		s.Equal(abilities.STR, finalEvent.AbilityUsed)
	})
}

// TestDEXModifierLabel tests that the SourceRef label is updated to DEX when Martial Arts uses DEX
// Regression test for https://github.com/KirkDiggler/rpg-toolkit/issues/605:
// combat log shows "STR" label even when DEX modifier value is correctly applied.
func (s *MartialArtsTestSuite) TestDEXModifierLabel() {
	s.Run("SourceRef label is DEX when DEX replaces STR for unarmed strike", func() {
		// Registry already has DEX=16 (+3), STR=10 (+0)
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
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
					Source: dnd5eEvents.DamageSourceWeapon,
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 3),
					},
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(0), // STR modifier (+0)
					},
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

		// Verify value is DEX
		s.Equal(3, finalEvent.Components[1].Total())
		s.Equal(abilities.DEX, finalEvent.AbilityUsed)

		// Verify label (SourceRef) is also updated to DEX — this is the bug (#605)
		s.Equal(refs.Abilities.Dexterity(), finalEvent.Components[1].Roll.Source.Ref,
			"SourceRef label must be DEX when Martial Arts replaces STR with DEX modifier")
	})

	s.Run("SourceRef label stays STR when STR is higher than DEX", func() {
		// Create a monk with higher STR than DEX
		strongMonk := s.strongID
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  strongMonk,
			MonkLevel: 1,
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
					Source: dnd5eEvents.DamageSourceWeapon,
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 3),
					},
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(3), // STR modifier (+3)
					},
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

		// STR is higher, so STR label and value are retained
		s.Equal(3, finalEvent.Components[1].Total())
		s.Equal(abilities.STR, finalEvent.AbilityUsed)
		s.Equal(refs.Abilities.Strength(), finalEvent.Components[1].Roll.Source.Ref,
			"SourceRef label must stay STR when STR modifier is retained")
	})

	s.Run("SourceRef label is DEX when DEX replaces STR for monk weapon", func() {
		// Registry already has DEX=16 (+3), STR=10 (+0)
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
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
					Source: dnd5eEvents.DamageSourceWeapon,
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 5),
					},
				},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(0), // STR modifier (+0)
					},
				},
			},
			AbilityUsed: abilities.STR,
			WeaponRef:   refs.Weapons.Shortsword(),
		}

		damageChain := dnd5eEvents.DamageChain.On(s.bus)
		chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

		modifiedChain, err := damageChain.PublishWithChain(s.ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, event)
		s.Require().NoError(err)

		// Verify value is DEX
		s.Equal(3, finalEvent.Components[1].Total())
		s.Equal(abilities.DEX, finalEvent.AbilityUsed)

		// Verify label (SourceRef) is also updated to DEX
		s.Equal(refs.Abilities.Dexterity(), finalEvent.Components[1].Roll.Source.Ref,
			"SourceRef label must be DEX when Martial Arts replaces STR with DEX modifier for monk weapon")
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
		MemberID:  s.characterID,
		MonkLevel: 1,
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
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(6, 6),
				},
			},
			{
				Source: dnd5eEvents.DamageSourceAbility,
				Roll: dnd5eEvents.RollComponent{
					Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
					Modifier: intPtr(0), // Will be replaced with DEX
				},
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
	s.Equal(3, finalEvent.Components[1].Total()) // DEX modifier (+3)
	s.Equal(abilities.DEX, finalEvent.AbilityUsed)
}

// TestNonMonkWeaponNotModified tests that non-monk weapons are not modified
//
//nolint:dupl // Test cases require similar setup code
func (s *MartialArtsTestSuite) TestNonMonkWeaponNotModified() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 1,
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
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(10, 10),
				},
			},
			{
				Source: dnd5eEvents.DamageSourceAbility,
				Roll: dnd5eEvents.RollComponent{
					Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
					Modifier: intPtr(0), // Should stay 0 (not modified)
				},
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
	s.Equal(0, finalEvent.Components[1].Total())
	s.Equal(abilities.STR, finalEvent.AbilityUsed)
}

// runAttackChain publishes an AttackChainEvent through the attack modifier
// chain and returns the executed (final) event.
func (s *MartialArtsTestSuite) runAttackChain(event dnd5eEvents.AttackChainEvent) dnd5eEvents.AttackChainEvent {
	s.T().Helper()
	attackChain := dnd5eEvents.AttackChain.On(s.bus)
	chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)

	modifiedChain, err := attackChain.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)
	return finalEvent
}

// TestAttackBonusUsesDEX is the regression test for
// https://github.com/KirkDiggler/rpg-toolkit/issues/709: the Martial Arts
// "use DEX when higher" swap applied to the damage chain but NOT the attack
// chain, so a DEX monk's unarmed attack rolled with STR + prof while its
// damage credited DEX — attack and damage disagreed on the governing ability.
func (s *MartialArtsTestSuite) TestAttackBonusUsesDEX() {
	s.Run("unarmed strike with DEX > STR - attack bonus swaps to DEX", func() {
		// Registry has DEX=16 (+3), STR=10 (+0); base bonus = STR 0 + prof 2.
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
		})
		s.Require().NoError(condition.Apply(s.ctx, s.bus))
		defer func() { _ = condition.Remove(s.ctx, s.bus) }()

		finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
			AttackerID:  s.characterID,
			TargetID:    "target-1",
			WeaponRef:   refs.Weapons.UnarmedStrike(),
			IsMelee:     true,
			AttackBonus: 2, // STR (+0) + proficiency (+2)
			TargetAC:    12,
		})

		// DEX (+3) replaces STR (+0): 2 - 0 + 3 = 5, matching the damage swap.
		s.Equal(5, finalEvent.AttackBonus,
			"attack bonus must use DEX + prof when DEX is higher (same governing ability as damage)")
	})

	s.Run("unarmed strike with STR >= DEX - attack bonus unchanged", func() {
		strongMonk := s.strongID
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  strongMonk,
			MonkLevel: 1,
		})
		s.Require().NoError(condition.Apply(s.ctx, s.bus))
		defer func() { _ = condition.Remove(s.ctx, s.bus) }()

		finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
			AttackerID:  strongMonk,
			TargetID:    "target-1",
			WeaponRef:   refs.Weapons.UnarmedStrike(),
			IsMelee:     true,
			AttackBonus: 5, // STR (+3) + proficiency (+2)
			TargetAC:    12,
		})

		s.Equal(5, finalEvent.AttackBonus, "STR stays when DEX is not higher")
	})

	s.Run("non-finesse monk weapon (quarterstaff) - attack bonus swaps to DEX", func() {
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
		})
		s.Require().NoError(condition.Apply(s.ctx, s.bus))
		defer func() { _ = condition.Remove(s.ctx, s.bus) }()

		finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
			AttackerID:  s.characterID,
			TargetID:    "target-1",
			WeaponRef:   refs.Weapons.Quarterstaff(),
			IsMelee:     true,
			AttackBonus: 2, // STR (+0) + proficiency (+2)
			TargetAC:    12,
		})

		s.Equal(5, finalEvent.AttackBonus, "monk weapons use DEX when higher")
	})

	s.Run("finesse monk weapon (shortsword) - not double-adjusted", func() {
		// The base attack path already picks the higher mod for finesse
		// weapons; Martial Arts must not add the delta a second time.
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
		})
		s.Require().NoError(condition.Apply(s.ctx, s.bus))
		defer func() { _ = condition.Remove(s.ctx, s.bus) }()

		finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
			AttackerID:  s.characterID,
			TargetID:    "target-1",
			WeaponRef:   refs.Weapons.Shortsword(),
			IsMelee:     true,
			AttackBonus: 5, // finesse base already used DEX (+3) + prof (+2)
			TargetAC:    12,
		})

		s.Equal(5, finalEvent.AttackBonus, "finesse weapons already use the higher mod")
	})

	s.Run("non-monk weapon (greatsword) - attack bonus unchanged", func() {
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
		})
		s.Require().NoError(condition.Apply(s.ctx, s.bus))
		defer func() { _ = condition.Remove(s.ctx, s.bus) }()

		finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
			AttackerID:  s.characterID,
			TargetID:    "target-1",
			WeaponRef:   refs.Weapons.Greatsword(),
			IsMelee:     true,
			AttackBonus: 2,
			TargetAC:    12,
		})

		s.Equal(2, finalEvent.AttackBonus, "Martial Arts does not apply to non-monk weapons")
	})

	s.Run("other character's attack - unchanged", func() {
		condition := NewMartialArtsCondition(MartialArtsInput{
			MemberID:  s.characterID,
			MonkLevel: 1,
		})
		s.Require().NoError(condition.Apply(s.ctx, s.bus))
		defer func() { _ = condition.Remove(s.ctx, s.bus) }()

		finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
			AttackerID:  "someone-else",
			TargetID:    "target-1",
			WeaponRef:   refs.Weapons.UnarmedStrike(),
			IsMelee:     true,
			AttackBonus: 2,
			TargetAC:    12,
		})

		s.Equal(2, finalEvent.AttackBonus, "only this monk's attacks are modified")
	})
}

// TestSerialization tests JSON serialization round-trip
func (s *MartialArtsTestSuite) TestSerialization() {
	original := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 5,
	})

	// Serialize
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Verify structure
	var data MartialArtsData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Equal(refs.Conditions.MartialArts(), data.Ref)
	s.Equal(s.characterID, data.MemberID)
	s.Equal(5, data.MonkLevel)

	// Deserialize
	loaded := &MartialArtsCondition{}
	err = loaded.loadJSON(jsonData)
	s.Require().NoError(err)

	// Verify fields
	s.Equal(original.MemberID, loaded.MemberID)
	s.Equal(original.MonkLevel, loaded.MonkLevel)
}

// TestOtherCharacterNotModified verifies that other characters' attacks are not modified
func (s *MartialArtsTestSuite) TestOtherCharacterNotModified() {
	condition := NewMartialArtsCondition(MartialArtsInput{
		MemberID:  s.characterID,
		MonkLevel: 5,
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
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
					Dice:   testDiceTrace(6, 1),
				},
			},
			{
				Source: dnd5eEvents.DamageSourceAbility,
				Roll: dnd5eEvents.RollComponent{
					Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
					Modifier: intPtr(2),
				},
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
	s.Equal(originalEvent.Components[0].Roll.Dice.FinalRolls, finalEvent.Components[0].Roll.Dice.FinalRolls)
	s.Equal(originalEvent.Components[1].Total(), finalEvent.Components[1].Total())
}
