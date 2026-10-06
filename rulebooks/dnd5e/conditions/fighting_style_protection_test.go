// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

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

// protectionFrame is the attack-roll frame resolution hands an attack by
// attacker on target: melee or not, with the protector→target distance given.
func protectionFrame(attacker, target string, melee bool, distance contributions.Fact[float64]) contributions.Frame {
	frame := testAttackFrame(attacker, target)
	frame.Action.Melee = contributions.Known(melee)
	if target != "fighter-1" {
		frame.Pairs = []contributions.PairFacts{{From: "fighter-1", To: target, DistanceCells: distance}}
	}
	return frame
}

// publishProtected runs one attack through the chain and returns the folded
// event.
func (s *FightingStyleProtectionTestSuite) publishProtected(
	ctx context.Context, event dnd5eEvents.AttackChainEvent,
) (dnd5eEvents.AttackChainEvent, error) {
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedChain, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(ctx, event, attackChain)
	if err != nil {
		return event, err
	}
	return modifiedChain.Execute(ctx, event)
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

// TestImposesDisadvantageOnNearbyAlly: shield and reaction eligibility come
// from the CAST, where the protector looks itself up by its own ID
// (rpg-toolkit#1178); melee and the protector→target distance come from the
// attack-roll frame resolution builds. Three participants: protector, ally and
// monster, and the ally is attacked.
func (s *FightingStyleProtectionTestSuite) TestImposesDisadvantageOnNearbyAlly() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, keeper := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", AttackBonus: 5, TargetAC: 15, CriticalThreshold: 20,
		Frame: protectionFrame("goblin-1", "ally-1", true, contributions.Known(1.0)),
	})
	s.Require().NoError(err)

	s.Require().Len(finalEvent.DisadvantageSources, 1)
	s.Equal(refs.Conditions.FightingStyleProtection(), finalEvent.DisadvantageSources[0].SourceRef)

	// The reaction is actually spent: the condition asked, and the keeper
	// that owns the sheet applied it, by the time Execute returns.
	s.Equal(0, keeper.sheet.reactions, "the reaction was actually debited")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, keeper.spent)
}

// TestAllyBeyondFiveFeetIsNotProtected: the frame's protector→target distance
// decides reach.
func (s *FightingStyleProtectionTestSuite) TestAllyBeyondFiveFeetIsNotProtected() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, keeper := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1",
		Frame: protectionFrame("goblin-1", "ally-1", true, contributions.Known(2.0)),
	})
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources)
	s.Equal(1, keeper.sheet.reactions, "an untaken reaction must not be debited")
}

// TestUnknownDistanceFailsTheAttack is R13: an eligible protector whose frame
// does not say how far it stands from the target fails the attack, never
// silently withholds the reaction.
func (s *FightingStyleProtectionTestSuite) TestUnknownDistanceFailsTheAttack() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, _ := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	_, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1",
		Frame: protectionFrame("goblin-1", "ally-1", true, contributions.Unknown[float64]()),
	})
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
}

// TestUnplacedProtectorIsNotEligible: a complete frame pairs every placed
// member, so one with no protector→target pair says the protector is not
// placed — not within 5 feet. The attack proceeds untouched and no reaction
// is spent; it never fails as unanswerable.
func (s *FightingStyleProtectionTestSuite) TestUnplacedProtectorIsNotEligible() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, keeper := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	frame := testAttackFrame("goblin-1", "ally-1")
	frame.Action.Melee = contributions.Known(true)
	frame.Pairs = []contributions.PairFacts{{
		From: "goblin-1", To: "ally-1", DistanceCells: contributions.Known(1.0),
		Stance: contributions.Known(contributions.StanceHostile), Sees: contributions.Known(true),
	}}
	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", AttackBonus: 5, Frame: frame,
	})

	s.Require().NoError(err)
	s.Empty(finalEvent.DisadvantageSources)
	s.Equal(5, finalEvent.AttackBonus, "the rest of the attack is untouched")
	s.Empty(keeper.spent, "no reaction is spent")

	s.Run("an incomplete frame cannot say the protector is absent", func() {
		frame.Complete = false
		_, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
			AttackerID: "goblin-1", TargetID: "ally-1", Frame: frame,
		})
		s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	})
}

// TestUnknownMeleeFailsTheAttack is R13 for the other frame fact.
func (s *FightingStyleProtectionTestSuite) TestUnknownMeleeFailsTheAttack() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, _ := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	frame := protectionFrame("goblin-1", "ally-1", true, contributions.Known(1.0))
	frame.Action.Melee = contributions.Unknown[bool]()
	_, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1", Frame: frame,
	})
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
}

// TestNoShieldMeansNoProtection: a missing shield refuses eligibility before
// distance is read, so even a frame that cannot say the distance does not
// fail an attack the protector could never have reacted to.
func (s *FightingStyleProtectionTestSuite) TestNoShieldMeansNoProtection() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, _ := s.protector(false, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1",
		Frame: protectionFrame("goblin-1", "ally-1", true, contributions.Unknown[float64]()),
	})
	s.Require().NoError(err)
	s.Empty(finalEvent.DisadvantageSources)
}

// TestNoReactionMeansNoProtection mirrors the shield case for the other
// half of eligibility.
func (s *FightingStyleProtectionTestSuite) TestNoReactionMeansNoProtection() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, _ := s.protector(true, 0)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "ally-1",
		Frame: protectionFrame("goblin-1", "ally-1", true, contributions.Known(1.0)),
	})
	s.Require().NoError(err)
	s.Empty(finalEvent.DisadvantageSources)
}

func (s *FightingStyleProtectionTestSuite) TestDoesNotProtectSelf() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, keeper := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1", TargetID: "fighter-1",
		Frame: protectionFrame("goblin-1", "fighter-1", true, contributions.Known(1.0)),
	})
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources, "can't protect self")
	s.Empty(keeper.spent)
}

// TestDoesNotTriggerOnOwnAttack pins rpg-toolkit#1178's Protection half: the
// condition used to exclude only "target is me" and never "attacker is me",
// so it fired on the protector's OWN melee attacks.
//
// THE PROTECTOR IS FULLY ELIGIBLE HERE: shield, reaction and cast are
// installed, and the frame puts fighter-1 adjacent to the creature it attacks
// — TestImposesDisadvantageOnNearbyAlly with exactly one thing changed, the
// identity of the attacker. Removing the attacker-is-me guard fails this.
func (s *FightingStyleProtectionTestSuite) TestDoesNotTriggerOnOwnAttack() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, keeper := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "fighter-1", TargetID: "goblin-1", AttackBonus: 5, TargetAC: 15, CriticalThreshold: 20,
		Frame: protectionFrame("fighter-1", "goblin-1", true, contributions.Known(1.0)),
	})
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources, "Protection is a reaction to someone ELSE's attack, never my own")
	s.Equal(1, keeper.sheet.reactions, "an untaken reaction must not be debited")
	s.Empty(keeper.spent)
}

// TestDoesNotProtectAgainstRangedAttacks: the frame's Melee decides, with the
// protector otherwise eligible and adjacent.
func (s *FightingStyleProtectionTestSuite) TestDoesNotProtectAgainstRangedAttacks() {
	protection := NewFightingStyleProtectionCondition("fighter-1")
	castCtx, keeper := s.protector(true, 1)
	s.Require().NoError(protection.Apply(s.ctx, s.bus))
	defer func() { _ = protection.Remove(s.ctx, s.bus) }()

	finalEvent, err := s.publishProtected(castCtx, dnd5eEvents.AttackChainEvent{
		AttackerID: "archer-1", TargetID: "ally-1", AttackBonus: 5, TargetAC: 15, CriticalThreshold: 20,
		Frame: protectionFrame("archer-1", "ally-1", false, contributions.Known(1.0)),
	})
	s.Require().NoError(err)

	s.Empty(finalEvent.DisadvantageSources)
	s.Empty(keeper.spent)
}

func (s *FightingStyleProtectionTestSuite) TestToJSON() {
	protection := NewFightingStyleProtectionCondition("fighter-1")

	jsonData, err := protection.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.FightingStyleProtection().ID)
	s.Contains(string(jsonData), "fighter-1")
}
