// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	mock_combat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/mock"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// AttackPhasesTestSuite tests the discrete two-phase attack resolution
// (ResolveAttackHit + ApplyAttackOutcome) introduced in Wave 2.11c slice 2.
type AttackPhasesTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	eventBus   events.EventBus
	lookup     *mock_combat.MockCombatantLookup
	attacker   *mock_combat.MockCombatant
	defender   *mock_combat.MockCombatant
	longsword  *weapons.Weapon
	mockRoller *mock_dice.MockRoller
}

func TestAttackPhasesSuite(t *testing.T) {
	suite.Run(t, new(AttackPhasesTestSuite))
}

func (s *AttackPhasesTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.eventBus = events.NewEventBus()
	s.lookup = mock_combat.NewMockCombatantLookup(s.ctrl)
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)

	// Standard attacker: STR 16 (+3), proficiency +2
	s.attacker = mock_combat.NewMockCombatant(s.ctrl)
	s.attacker.EXPECT().GetID().Return("fighter-1").AnyTimes()
	s.attacker.EXPECT().AbilityScores().Return(shared.AbilityScores{
		abilities.STR: 16, // +3 modifier
		abilities.DEX: 10,
	}).AnyTimes()
	s.attacker.EXPECT().ProficiencyBonus().Return(2).AnyTimes()

	// Standard defender: AC 15
	s.defender = mock_combat.NewMockCombatant(s.ctrl)
	s.defender.EXPECT().GetID().Return("goblin-1").AnyTimes()
	s.defender.EXPECT().AC().Return(15).AnyTimes()

	s.lookup.EXPECT().Get("fighter-1").Return(s.attacker, nil).AnyTimes()
	s.lookup.EXPECT().Get("goblin-1").Return(s.defender, nil).AnyTimes()

	s.longsword = &weapons.Weapon{
		ID:         weapons.Longsword,
		Name:       "Longsword",
		Category:   weapons.CategoryMartialMelee,
		Damage:     "1d8",
		DamageType: damage.Slashing,
	}

	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *AttackPhasesTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// =============================================================================
// ResolveAttackHit tests
// =============================================================================

// TestResolveAttackHit_BasicHit verifies phase 1 returns correct roll, AC, and wouldHit.
func (s *AttackPhasesTestSuite) TestResolveAttackHit_BasicHit() {
	// Roll 15 → total 20 (15 + 3 STR + 2 prof) vs AC 15 → hit
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)

	result, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})

	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(15, result.AttackRoll)
	s.Equal(5, result.AttackBonus, "STR(+3) + proficiency(+2) = 5")
	s.Equal(20, result.TotalAttack)
	s.Equal(15, result.OriginalAC, "defender's AC before any reactions")
	s.True(result.WouldHit, "20 vs AC 15 hits")
	s.False(result.IsNaturalTwenty)
	s.False(result.IsNaturalOne)
}

// TestResolveAttackHit_BasicMiss verifies phase 1 returns WouldHit=false when roll is low.
func (s *AttackPhasesTestSuite) TestResolveAttackHit_BasicMiss() {
	// Roll 5 → total 10 (5 + 3 STR + 2 prof) vs AC 15 → miss
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	result, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})

	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(5, result.AttackRoll)
	s.Equal(10, result.TotalAttack, "5 + 3 + 2 = 10")
	s.Equal(15, result.OriginalAC)
	s.False(result.WouldHit, "10 < AC 15 misses")
}

// TestResolveAttackHit_NaturalTwenty verifies natural 20 always hits regardless of AC.
func (s *AttackPhasesTestSuite) TestResolveAttackHit_NaturalTwenty() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil)

	result, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})

	s.Require().NoError(err)
	s.True(result.IsNaturalTwenty)
	s.True(result.WouldHit, "natural 20 always hits")
	// Critical threshold defaults to 20, and attackRoll (20) >= 20
	s.Equal(20, result.CriticalThreshold)
}

// TestResolveAttackHit_NaturalOne verifies natural 1 always misses regardless of AC.
func (s *AttackPhasesTestSuite) TestResolveAttackHit_NaturalOne() {
	// Use a low-AC defender so natural 1 would otherwise hit
	lowACDefender := mock_combat.NewMockCombatant(s.ctrl)
	lowACDefender.EXPECT().GetID().Return("minion-1").AnyTimes()
	lowACDefender.EXPECT().AC().Return(5).AnyTimes()
	s.lookup.EXPECT().Get("minion-1").Return(lowACDefender, nil).AnyTimes()

	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	result, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "minion-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})

	s.Require().NoError(err)
	s.True(result.IsNaturalOne)
	s.False(result.WouldHit, "natural 1 always misses")
}

// TestResolveAttackHit_CarriesOriginalAC verifies originalAC is captured before any reactions.
func (s *AttackPhasesTestSuite) TestResolveAttackHit_CarriesOriginalAC() {
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	// AC 16 → total 19 (14 + 5) → would hit
	highACDefender := mock_combat.NewMockCombatant(s.ctrl)
	highACDefender.EXPECT().GetID().Return("guardian-1").AnyTimes()
	highACDefender.EXPECT().AC().Return(16).AnyTimes()
	s.lookup.EXPECT().Get("guardian-1").Return(highACDefender, nil)

	result, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "guardian-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})

	s.Require().NoError(err)
	s.Equal(16, result.OriginalAC, "should capture defender AC before any reactions")
	s.Equal(19, result.TotalAttack, "14 + 3(STR) + 2(prof)")
	s.True(result.WouldHit, "19 >= 16")
}

// =============================================================================
// ApplyAttackOutcome tests
// =============================================================================

// TestApplyAttackOutcome_NoReactions_Hit verifies the hit path works with no reaction modifiers.
func (s *AttackPhasesTestSuite) TestApplyAttackOutcome_NoReactions_Hit() {
	// Phase 1 produces a hit with roll 15
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)
	// Phase 2 rolls damage: 1d8 → 6
	s.mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{6}, nil)

	hitResult, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})
	s.Require().NoError(err)

	result, err := combat.ApplyAttackOutcome(s.ctx, &combat.ApplyAttackOutcomeInput{
		HitResult: hitResult,
		Reactions: nil,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.True(result.Hit)
	s.Equal(15, result.TargetAC, "no reactions → effective AC = original AC 15")
	s.Equal([]int{6}, result.DamageRolls)
	//nolint:gocritic // math explanation: 6 (roll) + 3 (STR) = 9
	s.Equal(9, result.TotalDamage)
}

// TestApplyAttackOutcome_NoReactions_Miss verifies the miss path returns no damage.
func (s *AttackPhasesTestSuite) TestApplyAttackOutcome_NoReactions_Miss() {
	// Roll 5 → total 10 → miss vs AC 15
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	hitResult, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})
	s.Require().NoError(err)
	s.Require().False(hitResult.WouldHit)

	result, err := combat.ApplyAttackOutcome(s.ctx, &combat.ApplyAttackOutcomeInput{
		HitResult: hitResult,
		Reactions: nil,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.False(result.Hit)
	s.Equal(0, result.TotalDamage)
	s.Nil(result.Breakdown)
}

// TestApplyAttackOutcome_ACBonus_TurnsHitIntoMiss verifies that a +5 AC reaction
// correctly retroactively converts a hit into a miss (the Shield spell case).
// The attack roll was 14 → total 19 vs AC 16 → would hit.
// Shield adds +5 AC → effective AC 21 → 19 < 21 → miss.
func (s *AttackPhasesTestSuite) TestApplyAttackOutcome_ACBonus_TurnsHitIntoMiss() {
	// AC 16 defender; roll 14 → total 19 → would hit original AC 16
	highACDefender := mock_combat.NewMockCombatant(s.ctrl)
	highACDefender.EXPECT().GetID().Return("wizard-1").AnyTimes()
	highACDefender.EXPECT().AC().Return(16).AnyTimes()
	s.lookup.EXPECT().Get("wizard-1").Return(highACDefender, nil).AnyTimes()

	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil)

	hitResult, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "wizard-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})
	s.Require().NoError(err)
	s.Require().True(hitResult.WouldHit, "14+5=19 >= AC 16 should hit")
	s.Equal(16, hitResult.OriginalAC)

	// Player chooses to cast Shield: +5 AC
	shieldMod := combat.ReactionModifier{
		ConditionRef: "dnd5e:conditions:shield",
		ACBonus:      5,
	}

	result, err := combat.ApplyAttackOutcome(s.ctx, &combat.ApplyAttackOutcomeInput{
		HitResult: hitResult,
		Reactions: []combat.ReactionModifier{shieldMod},
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Equal(21, result.TargetAC, "16 + 5 = 21 effective AC")
	s.False(result.Hit, "19 < 21: Shield converts hit into miss")
	s.Equal(0, result.TotalDamage, "miss deals no damage")
}

// TestApplyAttackOutcome_ACBonus_HitStillHits verifies that a reaction AC bonus
// that is too small to change the outcome leaves Hit=true.
// Roll 18 → total 23 vs AC 15 (hits); Shield +5 → effective AC 20; 23 >= 20 still hits.
func (s *AttackPhasesTestSuite) TestApplyAttackOutcome_ACBonus_HitStillHits() {
	// Roll 18 → total 23 vs AC 15 → hits
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(18, nil)
	// damage roll
	s.mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{4}, nil)

	hitResult, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   s.eventBus,
		Roller:     s.mockRoller,
	})
	s.Require().NoError(err)
	s.Require().True(hitResult.WouldHit)

	result, err := combat.ApplyAttackOutcome(s.ctx, &combat.ApplyAttackOutcomeInput{
		HitResult: hitResult,
		Reactions: []combat.ReactionModifier{{ConditionRef: "dnd5e:conditions:shield", ACBonus: 5}},
	})
	s.Require().NoError(err)

	s.Equal(20, result.TargetAC, "15 + 5 = 20")
	s.True(result.Hit, "23 >= 20: attack still hits despite Shield")
	s.Greater(result.TotalDamage, 0)
}

// =============================================================================
// ResolveAttack wrapper parity test
// =============================================================================

// TestResolveAttack_Wrapper_MatchesTwoPhase verifies that ResolveAttack (the
// backwards-compat wrapper) produces identical results to calling the two
// discrete phases with no reaction modifiers.
func (s *AttackPhasesTestSuite) TestResolveAttack_Wrapper_MatchesTwoPhase() {
	// Two separate event buses — one per call set — to avoid subscription interference.
	bus1 := events.NewEventBus()
	bus2 := events.NewEventBus()

	// Roller 1: for the single-call ResolveAttack
	roller1 := mock_dice.NewMockRoller(s.ctrl)
	roller1.EXPECT().Roll(s.ctx, 20).Return(12, nil)
	roller1.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil)

	// Roller 2: for the two-phase call
	roller2 := mock_dice.NewMockRoller(s.ctrl)
	roller2.EXPECT().Roll(s.ctx, 20).Return(12, nil)
	roller2.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil)

	// Single-call path
	singleResult, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   bus1,
		Roller:     roller1,
	})
	s.Require().NoError(err)
	s.Require().NotNil(singleResult)

	// Two-phase path
	hitResult, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
		AttackerID: "fighter-1",
		TargetID:   "goblin-1",
		Weapon:     s.longsword,
		EventBus:   bus2,
		Roller:     roller2,
	})
	s.Require().NoError(err)

	twoPhaseResult, err := combat.ApplyAttackOutcome(s.ctx, &combat.ApplyAttackOutcomeInput{
		HitResult: hitResult,
		Reactions: nil,
	})
	s.Require().NoError(err)
	s.Require().NotNil(twoPhaseResult)

	// Both paths must produce identical outcomes for the same roll + AC
	s.Equal(singleResult.AttackRoll, twoPhaseResult.AttackRoll, "roll must match")
	s.Equal(singleResult.AttackBonus, twoPhaseResult.AttackBonus, "bonus must match")
	s.Equal(singleResult.TotalAttack, twoPhaseResult.TotalAttack, "total attack must match")
	s.Equal(singleResult.Hit, twoPhaseResult.Hit, "hit result must match")
	s.Equal(singleResult.Critical, twoPhaseResult.Critical, "critical must match")
	s.Equal(singleResult.TotalDamage, twoPhaseResult.TotalDamage, "damage must match")
	s.Equal(singleResult.DamageRolls, twoPhaseResult.DamageRolls, "damage rolls must match")
}

// =============================================================================
// ReactionTriggerEvent tests
// =============================================================================

// TestReactionTriggerEvent_PublishedWhenReady verifies that a condition handler
// publishes a ReactionTriggerEvent when predicate matches AND readiness is true.
func (s *AttackPhasesTestSuite) TestReactionTriggerEvent_PublishedWhenReady() {
	bus := events.NewEventBus()

	// Collect published ReactionTriggerEvents
	var captured []dnd5eEvents.ReactionTriggerEvent
	topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
	_, err := topic.Subscribe(context.Background(), func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
		captured = append(captured, e)
		return nil
	})
	s.Require().NoError(err)

	// Simulate a condition handler that checks readiness and publishes the trigger event.
	// This stands in for Wave 2.11d's Shield / OA condition; here we publish directly.
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		"wizard-1": {"dnd5e:conditions:shield": true},
	})

	reactorID := "wizard-1"
	conditionRef := "dnd5e:conditions:shield"

	// Predicate: attack would hit AND reactor is ready
	wouldHit := true
	if wouldHit && gamectx.IsReactionReady(ctx, reactorID, conditionRef) {
		pubErr := topic.Publish(ctx, dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    reactorID,
			ConditionRef: conditionRef,
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
			SourceEntity: "goblin-1",
			Payload:      "attack_context_placeholder",
		})
		s.Require().NoError(pubErr)
	}

	s.Len(captured, 1, "should publish exactly one ReactionTriggerEvent")
	s.Equal("wizard-1", captured[0].ReactorID)
	s.Equal("dnd5e:conditions:shield", captured[0].ConditionRef)
	s.Equal(dnd5eEvents.TriggerKindPostHit, captured[0].TriggerKind)
	s.Equal("goblin-1", captured[0].SourceEntity)
	_ = bus
}

// TestReactionTriggerEvent_NotPublishedWhenNotReady verifies that a condition
// handler does NOT publish a ReactionTriggerEvent when readiness is false,
// even if the predicate matches.
func (s *AttackPhasesTestSuite) TestReactionTriggerEvent_NotPublishedWhenNotReady() {
	bus := events.NewEventBus()

	var captured []dnd5eEvents.ReactionTriggerEvent
	topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
	_, err := topic.Subscribe(context.Background(), func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
		captured = append(captured, e)
		return nil
	})
	s.Require().NoError(err)

	// No readiness map at all → IsReactionReady returns false
	ctx := context.Background()

	reactorID := "wizard-1"
	conditionRef := "dnd5e:conditions:shield"

	// Predicate matches, but readiness gate blocks publication
	wouldHit := true
	if wouldHit && gamectx.IsReactionReady(ctx, reactorID, conditionRef) {
		// Should NOT reach here
		_ = topic.Publish(ctx, dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    reactorID,
			ConditionRef: conditionRef,
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
		})
	}

	s.Empty(captured, "readiness=false: no ReactionTriggerEvent should be published")
	_ = bus
}

// TestReactionTriggerEvent_NotPublishedWhenFalseInMap verifies the case where
// the readiness map is present but the entry is explicitly false.
func (s *AttackPhasesTestSuite) TestReactionTriggerEvent_NotPublishedWhenFalseInMap() {
	bus := events.NewEventBus()

	var captured []dnd5eEvents.ReactionTriggerEvent
	topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
	_, err := topic.Subscribe(context.Background(), func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
		captured = append(captured, e)
		return nil
	})
	s.Require().NoError(err)

	// Readiness map present but explicitly false for this reaction
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		"wizard-1": {"dnd5e:conditions:shield": false},
	})

	reactorID := "wizard-1"
	conditionRef := "dnd5e:conditions:shield"

	wouldHit := true
	if wouldHit && gamectx.IsReactionReady(ctx, reactorID, conditionRef) {
		_ = topic.Publish(ctx, dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    reactorID,
			ConditionRef: conditionRef,
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
		})
	}

	s.Empty(captured, "readiness explicitly false: no event should be published")
	_ = bus
}

// TestReactionTriggerEvent_NPCDoesNotPublish verifies the NPC inline-resolution
// path: NPC reactors apply modifiers directly to the chain and do NOT publish
// a ReactionTriggerEvent. This test simulates that by showing the distinction
// between the player-reactor path (publishes event) and NPC path (no event).
func (s *AttackPhasesTestSuite) TestReactionTriggerEvent_NPCDoesNotPublish() {
	bus := events.NewEventBus()

	var captured []dnd5eEvents.ReactionTriggerEvent
	topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
	_, err := topic.Subscribe(context.Background(), func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
		captured = append(captured, e)
		return nil
	})
	s.Require().NoError(err)

	// NPC context: readiness map has no entry for this NPC
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		// "npc-protector" is NOT in the map → IsReactionReady returns false
	})

	npcID := "npc-protector"
	conditionRef := "dnd5e:conditions:opportunity_attack"

	// NPC auto-resolve: condition checks readiness; if false, applies inline
	// (no event published). Here we just verify the guard prevents publication.
	predicateMatches := true
	isReady := gamectx.IsReactionReady(ctx, npcID, conditionRef)

	if predicateMatches && isReady {
		// player path — not taken for NPC
		_ = topic.Publish(ctx, dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    npcID,
			ConditionRef: conditionRef,
			TriggerKind:  dnd5eEvents.TriggerKindMovementOA,
		})
	} else {
		// NPC auto-resolve: apply inline, do nothing to the topic
		s.False(isReady, "NPC not in readiness map → not ready → no event")
	}

	s.Empty(captured, "NPC auto-resolve path must not publish ReactionTriggerEvent")
	_ = bus
}

// =============================================================================
// Input validation tests
// =============================================================================

func (s *AttackPhasesTestSuite) TestResolveAttackHit_Validation() {
	s.Run("nil input", func() {
		_, err := combat.ResolveAttackHit(s.ctx, nil)
		s.Error(err)
	})

	s.Run("missing attacker", func() {
		_, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
			TargetID: "goblin-1",
			Weapon:   s.longsword,
			EventBus: s.eventBus,
		})
		s.Error(err)
	})

	s.Run("missing target", func() {
		_, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
			AttackerID: "fighter-1",
			Weapon:     s.longsword,
			EventBus:   s.eventBus,
		})
		s.Error(err)
	})

	s.Run("nil weapon", func() {
		_, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
			AttackerID: "fighter-1",
			TargetID:   "goblin-1",
			EventBus:   s.eventBus,
		})
		s.Error(err)
	})

	s.Run("nil event bus", func() {
		_, err := combat.ResolveAttackHit(s.ctx, &combat.ResolveAttackHitInput{
			AttackerID: "fighter-1",
			TargetID:   "goblin-1",
			Weapon:     s.longsword,
		})
		s.Error(err)
	})
}

func (s *AttackPhasesTestSuite) TestApplyAttackOutcome_Validation() {
	s.Run("nil input", func() {
		_, err := combat.ApplyAttackOutcome(s.ctx, nil)
		s.Error(err)
	})

	s.Run("nil hit result", func() {
		_, err := combat.ApplyAttackOutcome(s.ctx, &combat.ApplyAttackOutcomeInput{
			HitResult: nil,
		})
		s.Error(err)
	})
}
