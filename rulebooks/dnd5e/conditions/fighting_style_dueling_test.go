// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type FightingStyleDuelingTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *FightingStyleDuelingTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestFightingStyleDuelingSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleDuelingTestSuite))
}

func (s *FightingStyleDuelingTestSuite) TestNewFightingStyleDuelingCondition() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	s.NotNil(dueling)
	s.False(dueling.IsApplied())
}

func (s *FightingStyleDuelingTestSuite) TestApplyAndRemove() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(dueling.IsApplied())

	err = dueling.Apply(s.ctx, s.bus)
	s.Error(err)

	err = dueling.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(dueling.IsApplied())
}

func (s *FightingStyleDuelingTestSuite) TestAddsDamageWithOneHandedWeapon() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

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

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Create damage chain event
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				FlatBonus:         3,
				DamageType:        damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Should have 2 components: weapon + dueling bonus
	s.Len(finalEvent.Components, 2)
	s.Equal(2, finalEvent.Components[1].FlatBonus)
}

func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithTwoHandedWeapon() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	// Create character registry with two-handed weapon
	registry := gamectx.NewBasicCharacterRegistry()
	mainHand := &gamectx.EquippedWeapon{
		ID:          "greatsword-1",
		Name:        "Greatsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: true, // Two-handed
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand})
	registry.Add("fighter-1", weapons)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Create damage chain event
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6, 6},
				FinalDiceRolls:    []int{6, 6},
				FlatBonus:         4,
				DamageType:        damage.Slashing,
			},
		},
		DamageType: damage.Slashing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Should only have 1 component (no dueling bonus for two-handed)
	s.Len(finalEvent.Components, 1)
}

func (s *FightingStyleDuelingTestSuite) TestDoesNotAddWithOffHandWeapon() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	err := dueling.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

	// Create character registry with dual wielding
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

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Create damage chain event
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{
			{
				Source:            dnd5eEvents.DamageSourceWeapon,
				OriginalDiceRolls: []int{6},
				FinalDiceRolls:    []int{6},
				FlatBonus:         3,
				DamageType:        damage.Piercing,
			},
		},
		DamageType: damage.Piercing,
	}

	// Execute through damage chain
	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damages := dnd5eEvents.DamageChain.On(s.bus)
	modifiedChain, err := damages.PublishWithChain(ctx, damageEvent, damageChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
	s.Require().NoError(err)

	// Should only have 1 component (no dueling bonus when dual wielding)
	s.Len(finalEvent.Components, 1)
}

func (s *FightingStyleDuelingTestSuite) TestToJSON() {
	dueling := conditions.NewFightingStyleDuelingCondition("fighter-1")

	jsonData, err := dueling.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleDueling().ID)
	s.Contains(string(jsonData), "fighter-1")
}
