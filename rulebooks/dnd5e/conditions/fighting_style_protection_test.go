// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// protectionTestEntity is a simple entity for testing
type protectionTestEntity struct {
	id   string
	kind string
}

func (e *protectionTestEntity) GetID() string            { return e.id }
func (e *protectionTestEntity) GetType() core.EntityType { return core.EntityType(e.kind) }

type FightingStyleProtectionTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *FightingStyleProtectionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestFightingStyleProtectionSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleProtectionTestSuite))
}

func (s *FightingStyleProtectionTestSuite) TestNewFightingStyleProtectionCondition() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	s.NotNil(protection)
	s.False(protection.IsApplied())
}

func (s *FightingStyleProtectionTestSuite) TestApplyAndRemove() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(protection.IsApplied())

	err = protection.Apply(s.ctx, s.bus)
	s.Error(err)

	err = protection.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(protection.IsApplied())
}

func (s *FightingStyleProtectionTestSuite) TestImposesDisadvantageOnNearbyAlly() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	// Set up room with fighter and ally adjacent
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	fighter := &protectionTestEntity{id: "fighter-1", kind: "character"}
	ally := &protectionTestEntity{id: "ally-1", kind: "character"}

	err = room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(ally, spatial.Position{X: 6, Y: 5}) // Adjacent
	s.Require().NoError(err)

	// Set up character registry with shield and reaction
	registry := gamectx.NewBasicCharacterRegistry()
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Name:     "Shield",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{shield})
	registry.Add("fighter-1", weapons)

	// Add action economy with reaction available
	actionEconomy := combat.NewActionEconomy()
	registry.AddActionEconomy("fighter-1", actionEconomy)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)
	ctx = gamectx.WithRoom(ctx, room)

	// Create attack chain event - melee attack on ally
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "goblin-1",
		TargetID:          "ally-1", // Attacking ally, not fighter
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	// Execute through attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	// Should have disadvantage imposed
	s.Len(finalEvent.DisadvantageSources, 1)
	s.Len(finalEvent.ReactionsConsumed, 1)
}

func (s *FightingStyleProtectionTestSuite) TestDoesNotProtectSelf() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	// Create attack targeting self
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "goblin-1",
		TargetID:          "fighter-1", // Attacking self
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// No disadvantage - can't protect self
	s.Empty(finalEvent.DisadvantageSources)
}

func (s *FightingStyleProtectionTestSuite) TestDoesNotProtectAgainstRangedAttacks() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	// Create ranged attack
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "archer-1",
		TargetID:          "ally-1",
		IsMelee:           false, // Ranged attack
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// No disadvantage for ranged attacks
	s.Empty(finalEvent.DisadvantageSources)
}

func (s *FightingStyleProtectionTestSuite) TestToJSON() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	jsonData, err := protection.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleProtection().ID)
	s.Contains(string(jsonData), "fighter-1")
}
