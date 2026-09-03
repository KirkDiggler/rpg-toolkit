// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/dice"
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

// intPtr returns a pointer to v, so a present zero modifier stays present.
func intPtr(v int) *int { return &v }

// diceTrace builds a self-consistent dice trace for one pool of faces: the
// notation, die size, original/final rolls, and authoritative subtotal all
// describe the same physical pool.
func diceTrace(dieSize int, faces ...int) *dnd5eEvents.DiceTrace {
	subtotal := 0
	for _, face := range faces {
		subtotal += face
	}
	return &dnd5eEvents.DiceTrace{
		Notation:      dice.SimplePool(len(faces), dieSize, 0).Notation(),
		DieSize:       dieSize,
		OriginalRolls: faces,
		FinalRolls:    slices.Clone(faces),
		Subtotal:      subtotal,
	}
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

// TestRerolls1sAnd2s pins the new roll-trace contract: GWF mutates only the
// weapon component's Roll.Dice, appends ordered rerolls sourced to the
// canonical Great Weapon Fighting condition, keeps the original faces, and
// moves the authoritative subtotal from 9 to 15.
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
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:     dnd5eEvents.DamageSourceWeapon,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{
						Ref:  refs.Weapons.Greatsword(),
						Name: "Greatsword",
					},
					Dice:     diceTrace(6, 1, 2, 6),
					Modifier: intPtr(4),
				},
				DamageType: damage.Slashing,
			},
		},
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	trace := finalEvent.Components[0].Roll.Dice
	s.Require().NotNil(trace)

	// Check that 1 and 2 were rerolled to 5 and 4, and only the final faces
	// and subtotal moved.
	s.Equal([]int{5, 4, 6}, trace.FinalRolls)
	s.Equal([]int{1, 2, 6}, trace.OriginalRolls, "original faces are immutable")
	s.Equal(15, trace.Subtotal, "subtotal is authoritative: 9 rerolled to 15")

	s.Require().Len(trace.Rerolls, 2)
	for i, want := range []struct {
		dieIndex int
		before   int
		after    int
	}{{0, 1, 5}, {1, 2, 4}} {
		reroll := trace.Rerolls[i]
		s.Equal(want.dieIndex, reroll.DieIndex)
		s.Equal(want.before, reroll.Before)
		s.Equal(want.after, reroll.After)
		s.Require().NotNil(reroll.Source.Ref)
		s.Equal("dnd5e:conditions:fighting_style_great_weapon_fighting", reroll.Source.Ref.String())
		s.Equal("Great Weapon Fighting", reroll.Source.Name)
	}

	// The weapon's modifier pointer survives the reroll untouched.
	s.Require().NotNil(finalEvent.Components[0].Roll.Modifier)
	s.Equal(4, *finalEvent.Components[0].Roll.Modifier)
	s.Equal(19, finalEvent.Components[0].Total(), "subtotal 15 plus the present +4 modifier")
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestDoesNotRerollHigherValues() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	err := gwf.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = gwf.Remove(s.ctx, s.bus) }()

	// No rerolls expected - all dice are 3+
	// Create damage chain event with no 1s or 2s
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:     dnd5eEvents.DamageSourceWeapon,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{
						Ref:  refs.Weapons.Greatsword(),
						Name: "Greatsword",
					},
					Dice:     diceTrace(6, 3, 4, 6),
					Modifier: intPtr(4),
				},
				DamageType: damage.Slashing,
			},
		},
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	trace := finalEvent.Components[0].Roll.Dice
	s.Require().NotNil(trace)

	// Dice should be unchanged
	s.Equal([]int{3, 4, 6}, trace.FinalRolls)
	s.Empty(trace.Rerolls)
	s.Equal(13, trace.Subtotal, "subtotal is untouched when nothing rerolls")
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestRerollsMarkedPrimaryWhenWeaponPoolsShareType() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)
	s.Require().NoError(gwf.Apply(s.ctx, s.bus))
	defer func() { _ = gwf.Remove(s.ctx, s.bus) }()

	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(5, nil)

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Dagger(), Name: "Dagger"},
					Dice:   diceTrace(4, 1),
				},
				DamageType: damage.Slashing,
			},
			{
				Source:     dnd5eEvents.DamageSourceWeapon,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
					Dice:   diceTrace(6, 1),
				},
				DamageType: damage.Slashing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)
	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	s.Equal([]int{1}, finalEvent.Components[0].Roll.Dice.FinalRolls)
	s.Equal([]int{5}, finalEvent.Components[1].Roll.Dice.FinalRolls)
}

// TestRerollsCurrentFacesAfterAnExistingReroll pins the staged-face contract:
// GWF judges the CURRENT final face, not the original. An earlier reroll rule
// already replaced the original 1 with a 2; Great Weapon Fighting must reroll
// that 2 to 5, extending the ordered history [1→2, 2→5] while the original
// face stays 1 and the subtotal moves 2 → 5. Judging the original face would
// record Before 1 against a current face of 2 — a trace that fails ordered
// replay — and a subtotal delta measured from the wrong face.
func (s *FightingStyleGreatWeaponFightingTestSuite) TestRerollsCurrentFacesAfterAnExistingReroll() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)
	s.Require().NoError(gwf.Apply(s.ctx, s.bus))
	defer func() { _ = gwf.Remove(s.ctx, s.bus) }()

	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(5, nil).Times(1)

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:     dnd5eEvents.DamageSourceWeapon,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
					Dice: &dnd5eEvents.DiceTrace{
						Notation:      "d6",
						DieSize:       6,
						OriginalRolls: []int{1},
						Rerolls: []dnd5eEvents.DiceReroll{{
							DieIndex: 0,
							Before:   1,
							After:    2,
							// Any earlier reroll rule; its identity is not what
							// this test pins — only that it exists and is valid.
							Source: dnd5eEvents.RollSource{
								Ref:  refs.Conditions.RecklessAttack(),
								Name: "Reckless Attack",
							},
						}},
						FinalRolls: []int{2},
						Subtotal:   2,
					},
				},
				DamageType: damage.Slashing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)
	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	trace := finalEvent.Components[0].Roll.Dice
	s.Require().NotNil(trace)
	s.Equal([]int{1}, trace.OriginalRolls, "original faces remain untouched")
	s.Require().Len(trace.Rerolls, 2, "history extends, never restarts")
	s.Equal(1, trace.Rerolls[0].Before)
	s.Equal(2, trace.Rerolls[0].After)
	s.Equal(2, trace.Rerolls[1].Before, "Before is the current staged face, not the original")
	s.Equal(5, trace.Rerolls[1].After)
	s.Equal("Great Weapon Fighting", trace.Rerolls[1].Source.Name)
	s.Equal([]int{5}, trace.FinalRolls)
	s.Equal(5, trace.Subtotal, "subtotal 2 follows the 2→5 replacement, not the original 1")

	// The extended history must replay cleanly: this is exactly the ordered
	// validity the old original-face iteration destroyed.
	calc := &dnd5eEvents.RollCalculation{
		Components: []dnd5eEvents.RollComponent{finalEvent.Components[0].Roll},
		Total:      5,
	}
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(calc))
}

// TestRollerErrorLeavesTheCallerTraceUnmutated pins the all-or-nothing
// contract: the caller-owned DiceTrace is published back only after every
// required reroll succeeds. A failure on a LATER die must leave the complete
// trace deeply equal to its pre-call state — no partial reroll, no moved
// final face, no shifted subtotal.
func (s *FightingStyleGreatWeaponFightingTestSuite) TestRollerErrorLeavesTheCallerTraceUnmutated() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)
	s.Require().NoError(gwf.Apply(s.ctx, s.bus))
	defer func() { _ = gwf.Remove(s.ctx, s.bus) }()

	// Die 0 rerolls successfully; die 1's reroll fails.
	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(5, nil).Times(1)
	s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(0, errors.New("roller exploded")).Times(1)

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:     dnd5eEvents.DamageSourceWeapon,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
					Dice:   diceTrace(6, 1, 2), // original/final [1,2], subtotal 3
				},
				DamageType: damage.Slashing,
			},
		},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(s.ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	_, execErr := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().Error(execErr, "the roller failure must surface to the caller")

	// The complete caller-owned trace is deeply equal to its pre-call state:
	// no staged mutation leaked through the failed reroll.
	got := damageEvent.Components[0].Roll.Dice
	s.Require().NotNil(got)
	s.Equal(&dnd5eEvents.DiceTrace{
		Notation:      "2d6",
		DieSize:       6,
		OriginalRolls: []int{1, 2},
		FinalRolls:    []int{1, 2},
		Subtotal:      3,
	}, got, "a failed reroll leaves the caller-owned trace deeply unchanged")
	s.Empty(got.Rerolls, "no partial reroll history leaks")
}

func (s *FightingStyleGreatWeaponFightingTestSuite) TestToJSON() {
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-1", s.mockRoller)

	jsonData, err := gwf.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleGreatWeaponFighting().ID)
	s.Contains(string(jsonData), "fighter-1")
}
