// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type RecklessAttackTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	condition *RecklessAttackCondition
}

func TestRecklessAttackSuite(t *testing.T) {
	suite.Run(t, new(RecklessAttackTestSuite))
}

func (s *RecklessAttackTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.condition = NewRecklessAttackCondition("barbarian-1")
}

func (s *RecklessAttackTestSuite) TestNewRecklessAttackCondition() {
	s.Equal("barbarian-1", s.condition.CharacterID)
	s.False(s.condition.IsApplied())
}

func (s *RecklessAttackTestSuite) TestApply() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(s.condition.IsApplied())
}

func (s *RecklessAttackTestSuite) TestApplyTwiceFails() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	err = s.condition.Apply(s.ctx, s.bus)
	s.Error(err, "should not be able to apply twice")
}

func (s *RecklessAttackTestSuite) TestRemove() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	err = s.condition.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(s.condition.IsApplied())
}

func (s *RecklessAttackTestSuite) TestRemoveWhenNotApplied() {
	err := s.condition.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
}

func (s *RecklessAttackTestSuite) TestGrantsAdvantageOnOwnMeleeAttacks() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack event: barbarian attacks with melee
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		IsMelee:    true,
	}

	// Execute through attack chain
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Verify advantage was granted
	s.Require().Len(finalEvent.AdvantageSources, 1, "Should have 1 advantage source")
	s.Equal(refs.Conditions.RecklessAttack(), finalEvent.AdvantageSources[0].SourceRef)
	s.Equal("Reckless Attack", finalEvent.AdvantageSources[0].Reason)
}

func (s *RecklessAttackTestSuite) TestNoAdvantageOnRangedAttacks() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack event: barbarian attacks with ranged
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		IsMelee:    false, // Ranged!
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// No advantage on ranged attacks
	s.Empty(finalEvent.AdvantageSources, "Should NOT have advantage on ranged attacks")
}

func (s *RecklessAttackTestSuite) TestEnemiesGetAdvantageAgainstBarbarian() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack event: enemy attacks the barbarian
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1",
		TargetID:   "barbarian-1", // Barbarian is the target
		IsMelee:    true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Enemy should have advantage
	s.Require().Len(finalEvent.AdvantageSources, 1, "Enemy should have advantage against reckless barbarian")
	s.Equal("Target is reckless", finalEvent.AdvantageSources[0].Reason)
}

func (s *RecklessAttackTestSuite) TestEnemyRangedAlsoGetsAdvantage() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Ranged enemy attacks the barbarian — still gets advantage
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "archer-1",
		TargetID:   "barbarian-1",
		IsMelee:    false, // Ranged attack
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Enemy ranged attacks also get advantage against reckless barbarian
	s.Require().Len(finalEvent.AdvantageSources, 1, "Ranged enemies also get advantage")
}

func (s *RecklessAttackTestSuite) TestBothAdvantageWhenBarbarianAttacksSelf() {
	// Edge case: when barbarian attacks themselves (shouldn't happen, but verify both trigger)
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Barbarian attacks a different target — only attacker advantage applies
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		IsMelee:    true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	// Only attacker advantage (not vulnerability, since barbarian isn't the target)
	s.Len(finalEvent.AdvantageSources, 1)
}

func (s *RecklessAttackTestSuite) TestNoEffectOnUnrelatedAttacks() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Attack between two other entities
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1",
		TargetID:   "fighter-1",
		IsMelee:    true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.AdvantageSources, "Should not affect unrelated attacks")
}

func (s *RecklessAttackTestSuite) TestRemovedOnTurnStart() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(s.condition.IsApplied())

	// Publish turn start for the barbarian
	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "barbarian-1",
		Round:       2,
	})
	s.Require().NoError(err)

	// Condition should be removed
	s.False(s.condition.IsApplied(), "Condition should be removed on turn start")
}

func (s *RecklessAttackTestSuite) TestNotRemovedOnOtherTurnStart() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Publish turn start for a different character
	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "goblin-1",
		Round:       2,
	})
	s.Require().NoError(err)

	// Condition should still be applied
	s.True(s.condition.IsApplied(), "Condition should NOT be removed on another character's turn")
}

func (s *RecklessAttackTestSuite) TestToJSON() {
	data, err := s.condition.ToJSON()
	s.Require().NoError(err)
	s.Require().NotNil(data)

	var raData RecklessAttackData
	err = json.Unmarshal(data, &raData)
	s.Require().NoError(err)

	s.Equal(refs.Conditions.RecklessAttack(), raData.Ref)
	s.Equal("barbarian-1", raData.CharacterID)
}

func (s *RecklessAttackTestSuite) TestLoadJSON() {
	data := RecklessAttackData{
		Ref:         refs.Conditions.RecklessAttack(),
		CharacterID: "barbarian-2",
	}
	jsonData, err := json.Marshal(data)
	s.Require().NoError(err)

	condition := &RecklessAttackCondition{}
	err = condition.loadJSON(jsonData)
	s.Require().NoError(err)

	s.Equal("barbarian-2", condition.CharacterID)
}

func (s *RecklessAttackTestSuite) TestRoundTripSerialization() {
	// Serialize
	jsonData, err := s.condition.ToJSON()
	s.Require().NoError(err)

	// Deserialize
	newCondition := &RecklessAttackCondition{}
	err = newCondition.loadJSON(jsonData)
	s.Require().NoError(err)

	s.Equal(s.condition.CharacterID, newCondition.CharacterID)
}

func (s *RecklessAttackTestSuite) TestLoaderIntegration() {
	// Verify the condition can be loaded through the standard loader
	jsonData, err := s.condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	// Apply and verify it works
	err = loaded.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack to verify it's wired up
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		IsMelee:    true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Len(finalEvent.AdvantageSources, 1, "Loaded condition should grant advantage")

	_ = loaded.Remove(s.ctx, s.bus)
}

// Verify that the chain stages are correct:
// - Attacker advantage at StageFeatures (class feature)
// - Target vulnerability at StageConditions (like other conditions)
func (s *RecklessAttackTestSuite) TestChainStageOrdering() {
	err := s.condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track which stages fire
	var featuresFired, conditionsFired bool

	// Add a tracker at StageFeatures
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)
	_, err = attackTopic.SubscribeWithChain(s.ctx,
		func(_ context.Context, event dnd5eEvents.AttackChainEvent, c chain.Chain[dnd5eEvents.AttackChainEvent]) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			if event.AttackerID != "barbarian-1" {
				return c, nil
			}
			// Add a tracker at final stage to see what's been done
			_ = c.Add(combat.StageFinal, "tracker", func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
				for _, src := range e.AdvantageSources {
					if src.Reason == "Reckless Attack" {
						featuresFired = true
					}
				}
				return e, nil
			})
			return c, nil
		},
	)
	s.Require().NoError(err)

	// Test barbarian attacking (triggers feature stage)
	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "barbarian-1",
		TargetID:   "goblin-1",
		IsMelee:    true,
	}

	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)
	_, err = modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.True(featuresFired, "Reckless Attack advantage should fire at features stage")

	// Test enemy attacking barbarian (triggers conditions stage)
	enemyEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1",
		TargetID:   "barbarian-1",
		IsMelee:    true,
	}

	// Reset and add condition tracker
	_, err = attackTopic.SubscribeWithChain(s.ctx,
		func(_ context.Context, event dnd5eEvents.AttackChainEvent, c chain.Chain[dnd5eEvents.AttackChainEvent]) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			if event.TargetID != "barbarian-1" {
				return c, nil
			}
			_ = c.Add(combat.StageFinal, "tracker2", func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
				for _, src := range e.AdvantageSources {
					if src.Reason == "Target is reckless" {
						conditionsFired = true
					}
				}
				return e, nil
			})
			return c, nil
		},
	)
	s.Require().NoError(err)

	attackChain2 := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedChain2, err := attackTopic.PublishWithChain(s.ctx, enemyEvent, attackChain2)
	s.Require().NoError(err)
	_, err = modifiedChain2.Execute(s.ctx, enemyEvent)
	s.Require().NoError(err)

	s.True(conditionsFired, "Target vulnerability should fire at conditions stage")
}
