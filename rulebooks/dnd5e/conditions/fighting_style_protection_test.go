// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
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

// fakeProtectionOwner is the minimal combat.Ledger + HasShieldEquipped
// implementation Protection's owner interface needs (rpg-toolkit#1178) —
// enough to prove eligibility and consumption without gamectx or a full
// *character.Character.
type fakeProtectionOwner struct {
	hasShield bool
	reactions int
	spent     []coreCombat.ActionType
}

func (f *fakeProtectionOwner) HasShieldEquipped() bool { return f.hasShield }
func (f *fakeProtectionOwner) InCombat() bool          { return true }

func (f *fakeProtectionOwner) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionReaction {
		return f.reactions
	}
	return 0
}

func (f *fakeProtectionOwner) CapacityLeft(_ combat.CapacityType) int       { return 0 }
func (f *fakeProtectionOwner) PoolLeft(_ coreResources.ResourceKey) int     { return 0 }
func (f *fakeProtectionOwner) SpendCapacity(_ combat.CapacityType, _ int)   {}
func (f *fakeProtectionOwner) SpendPool(_ coreResources.ResourceKey, _ int) {}
func (f *fakeProtectionOwner) BankCapacity(_ combat.CapacityType, _ int)    {}

func (f *fakeProtectionOwner) SpendSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionReaction {
		f.reactions -= n
	}
	f.spent = append(f.spent, slot)
}

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

// TestImposesDisadvantageOnNearbyAlly pins rpg-toolkit#1178's Protection
// fix: shield and reaction eligibility now come from the owner handed over
// at attach time (SetOwner), read the same way combat.Pay/CanPay already
// read a ledger — not a gamectx.CharacterRegistry. Only positions still
// come from gamectx (WithRoom), which resolution.Resolve genuinely does
// install. Team-lead's exact reproduction shape: three participants
// (protector + ally + monster), the ally is attacked, not the protector.
func (s *FightingStyleProtectionTestSuite) TestImposesDisadvantageOnNearbyAlly() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	owner := &fakeProtectionOwner{hasShield: true, reactions: 1}
	protection.SetOwner(owner)

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

	// Positions are the one thing this condition still reads from gamectx —
	// no CharacterRegistry, no GameContext installed at all.
	ctx := gamectx.WithRoom(s.ctx, room)

	// Create attack chain event - melee attack on ally, by a THIRD
	// combatant (neither the protector nor the target) — this is exactly
	// the shape Copilot flagged as reachable on the session stack: three
	// or more participants, someone other than the protector attacking
	// someone other than the protector.
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

	// And the reaction is actually spent on the owner's own ledger.
	s.Equal(0, owner.reactions, "the reaction was actually debited")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, owner.spent)
}

// TestNoShieldMeansNoProtection pins that a missing shield refuses
// eligibility before ever touching gamectx.RequireRoom — no room is
// installed in this test at all, and the condition must never reach for
// one when the shield check alone already disqualifies it.
func (s *FightingStyleProtectionTestSuite) TestNoShieldMeansNoProtection() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")
	protection.SetOwner(&fakeProtectionOwner{hasShield: false, reactions: 1})

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", IsMelee: true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources)
}

// TestNoReactionMeansNoProtection mirrors the shield case for the other
// half of eligibility.
func (s *FightingStyleProtectionTestSuite) TestNoReactionMeansNoProtection() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")
	protection.SetOwner(&fakeProtectionOwner{hasShield: true, reactions: 0})

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", IsMelee: true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources)
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

// TestDoesNotTriggerOnOwnAttack pins rpg-toolkit#1178's Protection half:
// the condition used to exclude only "target is me" and never "attacker is
// me", so it fired on the protector's OWN melee attacks — which reached
// gamectx.RequireCharacters below, a dependency the session stack never
// satisfies. No gamectx.WithGameContext is installed in this test at all;
// if the exclusion regresses, this test fails with ErrNoGameContext rather
// than a wrong disadvantage count, which is the honest failure for what
// broke live.
func (s *FightingStyleProtectionTestSuite) TestDoesNotTriggerOnOwnAttack() {
	protection := conditions.NewFightingStyleProtectionCondition("fighter-1")

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	// The protector attacking someone else — never a reaction to their own swing.
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:        "fighter-1",
		TargetID:          "goblin-1",
		IsMelee:           true,
		AttackBonus:       5,
		TargetAC:          15,
		CriticalThreshold: 20,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err, "the protector's own attack must never reach the gamectx-dependent branch")

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources, "Protection is a reaction to someone ELSE's attack, never my own")
	s.Empty(finalEvent.ReactionsConsumed)
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
