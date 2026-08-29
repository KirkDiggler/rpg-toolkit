// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
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

// protector builds this fighter's own sheet and the keeper that owns it, and
// installs the sheet in the cast the way resolution's one door does.
//
// Both halves, every time, because the condition now needs both to do
// anything: it reads its shield and its reaction off the cast, and it pays by
// asking the keeper. A test that installed only the cast would watch the rule
// decide correctly and then publish a bill to nobody.
func (s *FightingStyleProtectionTestSuite) protector(shield bool, reactions int) (context.Context, *fakeSheetKeeper) {
	sheet := &fakeConditionOwner{id: "fighter-1", shield: shield, hasEconomy: true, reactions: reactions}

	keeper, err := keeperFor(s.ctx, s.bus, sheet)
	s.Require().NoError(err)

	return castOf(s.ctx, sheet), keeper
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
	protection := NewFightingStyleProtectionCondition("fighter-1")

	s.NotNil(protection)
	s.False(protection.IsApplied())
}

func (s *FightingStyleProtectionTestSuite) TestApplyAndRemove() {
	protection := NewFightingStyleProtectionCondition("fighter-1")

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
// fix: shield and reaction eligibility come from the CAST, where the
// protector looks itself up by its own ID and gets the same read surface it
// would get for anybody else — not from a handle a loader had to remember to
// pass in. Positions come from gamectx (WithRoom), which resolution installs
// on every path that folds anything. Team-lead's exact reproduction shape:
// three participants (protector + ally + monster), the ally is attacked, not
// the protector.
func (s *FightingStyleProtectionTestSuite) TestImposesDisadvantageOnNearbyAlly() {
	protection := NewFightingStyleProtectionCondition("fighter-1")

	castCtx, keeper := s.protector(true, 1)

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
	ctx := gamectx.WithRoom(castCtx, room)

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

	// And the reaction is actually spent: the condition asked, and the keeper
	// that owns the sheet applied it. Debited by the time Execute returns,
	// because the bus is synchronous and the request goes out inside the
	// stage — the same instant the direct SpendSlots call used to land.
	//
	// This IS the reaction evidence now. The chain event used to carry a
	// ReactionsConsumed shelf alongside, written here and read by nobody;
	// rpg-project#319 Phase 6 deleted it, leaving the keeper's ledger as the
	// single answer to "was the reaction spent".
	s.Equal(0, keeper.sheet.reactions, "the reaction was actually debited")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, keeper.spent)
}

// TestNoShieldMeansNoProtection pins that a missing shield refuses
// eligibility before ever touching gamectx.RequireRoom — no room is
// installed in this test at all, and the condition must never reach for
// one when the shield check alone already disqualifies it.
func (s *FightingStyleProtectionTestSuite) TestNoShieldMeansNoProtection() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, _ := s.protector(false, 1)

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", IsMelee: true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(castCtx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(castCtx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources)
}

// TestNoReactionMeansNoProtection mirrors the shield case for the other
// half of eligibility.
func (s *FightingStyleProtectionTestSuite) TestNoReactionMeansNoProtection() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, _ := s.protector(true, 0)

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", IsMelee: true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(castCtx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(castCtx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources)
}

func (s *FightingStyleProtectionTestSuite) TestDoesNotProtectSelf() {
	protection := NewFightingStyleProtectionCondition("fighter-1")

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

// TestDoesNotTriggerOnOwnAttack pins rpg-toolkit#1178's Protection half: the
// condition used to exclude only "target is me" and never "attacker is me",
// so it fired on the protector's OWN melee attacks.
//
// THE PROTECTOR IS FULLY ELIGIBLE HERE, and that is the whole point. Shield,
// reaction, cast and room are all installed, and fighter-1 stands adjacent to
// the creature it is attacking — this is TestImposesDisadvantageOnNearbyAlly
// with exactly one thing changed, the identity of the attacker. So the
// attacker-is-me guard is the only thing between this attack and a
// disadvantage source, and removing that guard fails the assertion below.
//
// It did not used to be. This test installed no cast and no room, which meant
// a regression fell through to the fail-closed "a protector nobody can look
// up is NOT ELIGIBLE" branch and returned the same empty chain the exclusion
// returns. Verified by mutation, not by reading: with the guard deleted the
// old test still passed, and so did every other test in the module. The
// comment claimed a failure with ErrNoGameContext, from a gamectx symbol that
// no longer exists.
func (s *FightingStyleProtectionTestSuite) TestDoesNotTriggerOnOwnAttack() {
	protection := NewFightingStyleProtectionCondition("fighter-1")

	castCtx, keeper := s.protector(true, 1)

	err := protection.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	// Adjacent, so range is not what refuses this — the exclusion is.
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "room",
		Grid: grid,
	})

	fighter := &protectionTestEntity{id: "fighter-1", kind: "character"}
	goblin := &protectionTestEntity{id: "goblin-1", kind: "monster"}

	err = room.PlaceEntity(fighter, spatial.Position{X: 5, Y: 5})
	s.Require().NoError(err)
	err = room.PlaceEntity(goblin, spatial.Position{X: 6, Y: 5})
	s.Require().NoError(err)

	ctx := gamectx.WithRoom(castCtx, room)

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
	modifiedChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources, "Protection is a reaction to someone ELSE's attack, never my own")

	// And nothing was spent for the reaction it never took.
	s.Equal(1, keeper.sheet.reactions, "an untaken reaction must not be debited")
	s.Empty(keeper.spent)
}

func (s *FightingStyleProtectionTestSuite) TestDoesNotProtectAgainstRangedAttacks() {
	protection := NewFightingStyleProtectionCondition("fighter-1")

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
	protection := NewFightingStyleProtectionCondition("fighter-1")

	jsonData, err := protection.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleProtection().ID)
	s.Contains(string(jsonData), "fighter-1")
}
