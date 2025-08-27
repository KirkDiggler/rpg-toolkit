// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Test events for chains
type TestAttackEvent struct {
	AttackerID string
	TargetID   string
	Damage     int
}

type TestDefenseEvent struct {
	DefenderID string
	Armor      int
	Shield     int
}

// Test stages
const (
	TestStageBase       chain.Stage = "base"
	TestStageModifiers  chain.Stage = "modifiers"
	TestStageConditions chain.Stage = "conditions"
	TestStageFinal      chain.Stage = "final"
)

// Test topics
const (
	TopicTestAttack  events.Topic = "test.attack"
	TopicTestDefense events.Topic = "test.defense"
)

// Test chained topics
var (
	TestAttackChain  = events.DefineChainedTopic[TestAttackEvent](TopicTestAttack)
	TestDefenseChain = events.DefineChainedTopic[TestDefenseEvent](TopicTestDefense)
)

// ChainedTopicTestSuite tests chained topics with modifiers
type ChainedTopicTestSuite struct {
	suite.Suite
	bus     events.EventBus
	ctx     context.Context
	attacks events.ChainedTopic[TestAttackEvent]
	stages  []chain.Stage
}

func (s *ChainedTopicTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.attacks = TestAttackChain.On(s.bus)
	s.stages = []chain.Stage{
		TestStageBase,
		TestStageModifiers,
		TestStageConditions,
		TestStageFinal,
	}
}

func (s *ChainedTopicTestSuite) TestBasicChainModification() {
	// Subscribe to add modifier
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			// Add +5 damage modifier
			err := c.Add(TestStageModifiers, "bonus", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				event.Damage += 5
				return event, nil
			})
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Create event and chain
	attack := TestAttackEvent{
		AttackerID: "hero",
		TargetID:   "goblin",
		Damage:     10,
	}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)

	// Publish with chain
	modifiedChain, err := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	s.Require().NoError(err)
	s.Require().NotNil(modifiedChain)

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, attack)
	s.Require().NoError(err)
	s.Equal(15, result.Damage) // 10 + 5
}

func (s *ChainedTopicTestSuite) TestMultipleSubscribersAddModifiers() {
	// First subscriber adds +2
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			err := c.Add(TestStageModifiers, "rage", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				event.Damage += 2
				return event, nil
			})
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Second subscriber adds +3
	_, err = s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			err := c.Add(TestStageModifiers, "bless", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				event.Damage += 3
				return event, nil
			})
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Third subscriber adds x2 at final stage
	_, err = s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			err := c.Add(TestStageFinal, "critical", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				event.Damage *= 2
				return event, nil
			})
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Create and process
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)

	modifiedChain, err := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	s.Require().NoError(err)

	result, err := modifiedChain.Execute(s.ctx, attack)
	s.Require().NoError(err)

	// Should be (10 + 2 + 3) * 2 = 30
	s.Equal(30, result.Damage)
}

func (s *ChainedTopicTestSuite) TestConditionalModification() {
	// Only modify if attacker is "barbarian"
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			if e.AttackerID == "barbarian" {
				err := c.Add(TestStageConditions, "rage", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
					event.Damage += 10
					return event, nil
				})
				s.NoError(err)
			}
			return c, nil
		})
	s.Require().NoError(err)

	// Test with barbarian
	attack1 := TestAttackEvent{
		AttackerID: "barbarian",
		Damage:     10,
	}
	chain1 := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain1, _ := s.attacks.PublishWithChain(s.ctx, attack1, chain1)
	result1, _ := modChain1.Execute(s.ctx, attack1)
	s.Equal(20, result1.Damage) // Modified

	// Test with wizard
	attack2 := TestAttackEvent{
		AttackerID: "wizard",
		Damage:     10,
	}
	chain2 := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain2, _ := s.attacks.PublishWithChain(s.ctx, attack2, chain2)
	result2, _ := modChain2.Execute(s.ctx, attack2)
	s.Equal(10, result2.Damage) // Not modified
}

func (s *ChainedTopicTestSuite) TestStageOrdering() {
	var executionOrder []string

	// Add modifiers at different stages
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			// Add in reverse order to test ordering
			err := c.Add(TestStageFinal, "final", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				executionOrder = append(executionOrder, "final")
				return event, nil
			})
			s.NoError(err)
			err = c.Add(TestStageBase, "base", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				executionOrder = append(executionOrder, "base")
				return event, nil
			})
			s.NoError(err)
			err = c.Add(TestStageConditions, "conditions",
				func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
					executionOrder = append(executionOrder, "conditions")
					return event, nil
				})
			s.NoError(err)
			err = c.Add(TestStageModifiers, "modifiers",
				func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
					executionOrder = append(executionOrder, "modifiers")
					return event, nil
				})
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Execute
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain, _ := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	_, err = modChain.Execute(s.ctx, attack)
	s.Require().NoError(err)

	// Verify execution order matches stage order
	s.Equal([]string{"base", "modifiers", "conditions", "final"}, executionOrder)
}

func (s *ChainedTopicTestSuite) TestChainErrorHandling() {
	// Subscribe handler that returns error
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			return c, errors.New("subscription error")
		})
	s.Require().NoError(err) // Subscription itself succeeds

	// Publish should return the error
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	_, err = s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	s.Error(err)
	s.Contains(err.Error(), "subscription error")
}

func (s *ChainedTopicTestSuite) TestModifierError() {
	// Add modifier that errors
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			err := c.Add(TestStageModifiers, "error", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				return event, errors.New("modifier error")
			})
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Publish succeeds
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain, err := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	s.Require().NoError(err)

	// Execute fails
	_, err = modChain.Execute(s.ctx, attack)
	s.Error(err)
	s.Contains(err.Error(), "modifier error")
}

func (s *ChainedTopicTestSuite) TestDuplicateModifierID() {
	// Subscribe handler that adds same ID twice
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			err := c.Add(TestStageModifiers, "duplicate",
				func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
					event.Damage++
					return event, nil
				})
			s.NoError(err)
			// Try to add with same ID
			err = c.Add(TestStageModifiers, "duplicate",
				func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
					event.Damage += 2
					return event, nil
				})
			s.Error(err) // Should fail
			return c, nil
		})
	s.Require().NoError(err)

	// Verify only first modifier is applied
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain, _ := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	result, _ := modChain.Execute(s.ctx, attack)
	s.Equal(11, result.Damage) // Only +1, not +2
}

func (s *ChainedTopicTestSuite) TestRemoveModifier() {
	// Subscribe handler that adds then removes
	_, err := s.attacks.SubscribeWithChain(s.ctx,
		func(_ context.Context, _ TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
			// Add modifier
			err := c.Add(TestStageModifiers, "temp", func(_ context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				event.Damage += 100
				return event, nil
			})
			s.NoError(err)
			// Remove it
			err = c.Remove("temp")
			s.NoError(err)
			return c, nil
		})
	s.Require().NoError(err)

	// Verify modifier was removed
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain, _ := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	result, _ := modChain.Execute(s.ctx, attack)
	s.Equal(10, result.Damage) // No modification
}

func TestChainedTopicSuite(t *testing.T) {
	suite.Run(t, new(ChainedTopicTestSuite))
}
