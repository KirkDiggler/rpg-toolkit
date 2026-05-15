// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ShieldSpellConditionSuite covers the Shield condition's predicate against
// PostAttackRollEvent: target match, would-hit, deflectability, and readiness.
type ShieldSpellConditionSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestShieldSpellConditionSuite(t *testing.T) {
	suite.Run(t, new(ShieldSpellConditionSuite))
}

func (s *ShieldSpellConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// subscribeTriggers buffers ReactionTriggerEvents during the test so the
// predicate's effect is observable.
func (s *ShieldSpellConditionSuite) subscribeTriggers() *[]dnd5eEvents.ReactionTriggerEvent {
	mu := &sync.Mutex{}
	collected := &[]dnd5eEvents.ReactionTriggerEvent{}
	topic := dnd5eEvents.ReactionTriggerTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
		mu.Lock()
		*collected = append(*collected, e)
		mu.Unlock()
		return nil
	})
	s.Require().NoError(err)
	return collected
}

// publishPostRoll publishes a PostAttackRollEvent through the post-roll chain
// with the supplied context. Returns nothing — the test inspects the trigger
// collector. Mirrors what combat.ResolveAttackHit does internally.
func (s *ShieldSpellConditionSuite) publishPostRoll(ctx context.Context, evt dnd5eEvents.PostAttackRollEvent) {
	topic := dnd5eEvents.PostAttackRollChain.On(s.bus)
	c := events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](combat.ModifierStages)
	_, err := topic.PublishWithChain(ctx, &evt, c)
	s.Require().NoError(err)
}

func (s *ShieldSpellConditionSuite) TestApplyAndRemove() {
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.False(sh.IsApplied())

	s.Require().NoError(sh.Apply(s.ctx, s.bus))
	s.True(sh.IsApplied())

	// Re-apply should error
	s.Error(sh.Apply(s.ctx, s.bus))

	s.Require().NoError(sh.Remove(s.ctx, s.bus))
	s.False(sh.IsApplied())
}

func (s *ShieldSpellConditionSuite) TestPublishesTriggerWhenReadyAndDeflectable() {
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.Require().NoError(sh.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithReactionReadiness(s.ctx, gamectx.ReactionReadinessMap{
		"wizard-1": {refs.Spells.Shield().String(): true},
	})

	// roll 19 vs AC 14 → would hit; +5 → 19 vs 19 → still hits actually.
	// So choose roll 16 vs AC 14 → would hit; +5 → 16 vs 19 → miss. Deflectable.
	s.publishPostRoll(ctx, dnd5eEvents.PostAttackRollEvent{
		AttackerID:  "goblin-1",
		TargetID:    "wizard-1",
		OriginalAC:  14,
		AttackRoll:  10,
		AttackBonus: 6,
		TotalAttack: 16,
		WouldHit:    true,
	})

	s.Require().Len(*collected, 1, "expected one Shield trigger event")
	got := (*collected)[0]
	s.Equal("wizard-1", got.ReactorID)
	s.Equal(refs.Spells.Shield().String(), got.ConditionRef)
	s.Equal(dnd5eEvents.TriggerKindPostHit, got.TriggerKind)
	s.Equal("goblin-1", got.SourceEntity)
}

func (s *ShieldSpellConditionSuite) TestNoTriggerWhenReadinessOff() {
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.Require().NoError(sh.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	// readiness map missing — IsReactionReady returns false.
	ctx := gamectx.WithReactionReadiness(s.ctx, gamectx.ReactionReadinessMap{
		"wizard-1": {refs.Spells.Shield().String(): false},
	})

	s.publishPostRoll(ctx, dnd5eEvents.PostAttackRollEvent{
		AttackerID:  "goblin-1",
		TargetID:    "wizard-1",
		OriginalAC:  14,
		AttackRoll:  10,
		AttackBonus: 6,
		TotalAttack: 16,
		WouldHit:    true,
	})

	s.Empty(*collected, "no trigger expected when readiness is off")
}

func (s *ShieldSpellConditionSuite) TestNoTriggerWhenAttackMisses() {
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.Require().NoError(sh.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithReactionReadiness(s.ctx, gamectx.ReactionReadinessMap{
		"wizard-1": {refs.Spells.Shield().String(): true},
	})

	s.publishPostRoll(ctx, dnd5eEvents.PostAttackRollEvent{
		AttackerID:  "goblin-1",
		TargetID:    "wizard-1",
		OriginalAC:  18,
		AttackRoll:  5,
		AttackBonus: 3,
		TotalAttack: 8,
		WouldHit:    false,
	})

	s.Empty(*collected, "no trigger expected when attack already misses")
}

func (s *ShieldSpellConditionSuite) TestNoTriggerWhenAttackStillHitsWithShield() {
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.Require().NoError(sh.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithReactionReadiness(s.ctx, gamectx.ReactionReadinessMap{
		"wizard-1": {refs.Spells.Shield().String(): true},
	})

	// roll 25 vs AC 14 → would hit; +5 → 25 vs 19 → still hits. Don't waste reaction.
	s.publishPostRoll(ctx, dnd5eEvents.PostAttackRollEvent{
		AttackerID:  "goblin-1",
		TargetID:    "wizard-1",
		OriginalAC:  14,
		AttackRoll:  19,
		AttackBonus: 6,
		TotalAttack: 25,
		WouldHit:    true,
	})

	s.Empty(*collected, "no trigger expected when Shield wouldn't deflect")
}

func (s *ShieldSpellConditionSuite) TestNoTriggerWhenTargetDifferent() {
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.Require().NoError(sh.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithReactionReadiness(s.ctx, gamectx.ReactionReadinessMap{
		"wizard-1": {refs.Spells.Shield().String(): true},
	})

	// Attack against a different character — Shield only protects self.
	s.publishPostRoll(ctx, dnd5eEvents.PostAttackRollEvent{
		AttackerID:  "goblin-1",
		TargetID:    "fighter-2",
		OriginalAC:  14,
		AttackRoll:  10,
		AttackBonus: 6,
		TotalAttack: 16,
		WouldHit:    true,
	})

	s.Empty(*collected, "no trigger expected when attack targets someone else")
}

func (s *ShieldSpellConditionSuite) TestNoTriggerOnNaturalTwenty() {
	// Natural 20 always hits — Shield cannot prevent crit hits.
	sh := conditions.NewShieldSpellCondition("wizard-1")
	s.Require().NoError(sh.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithReactionReadiness(s.ctx, gamectx.ReactionReadinessMap{
		"wizard-1": {refs.Spells.Shield().String(): true},
	})

	// Natural 20 with low totals — would hit on nat20 rule, but we shouldn't trigger.
	s.publishPostRoll(ctx, dnd5eEvents.PostAttackRollEvent{
		AttackerID:      "goblin-1",
		TargetID:        "wizard-1",
		OriginalAC:      18,
		AttackRoll:      20,
		AttackBonus:     0,
		TotalAttack:     20, // numerically would deflect with +5 (20 vs 23 = miss)
		WouldHit:        true,
		IsNaturalTwenty: true,
	})

	s.Empty(*collected, "no trigger expected on natural 20")
}

func (s *ShieldSpellConditionSuite) TestJSONRoundTrip() {
	sh := conditions.NewShieldSpellCondition("wizard-7")
	raw, err := sh.ToJSON()
	s.Require().NoError(err)

	loaded, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)

	roundTripped, ok := loaded.(*conditions.ShieldSpellCondition)
	s.Require().True(ok, "loader should return *ShieldSpellCondition")
	s.Equal("wizard-7", roundTripped.CharacterID)
}

func (s *ShieldSpellConditionSuite) TestJSONShapeContainsRef() {
	sh := conditions.NewShieldSpellCondition("c-1")
	raw, err := sh.ToJSON()
	s.Require().NoError(err)

	var data conditions.ShieldSpellConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.NotNil(data.Ref)
	s.Equal(refs.Spells.Shield().String(), data.Ref.String())
}
