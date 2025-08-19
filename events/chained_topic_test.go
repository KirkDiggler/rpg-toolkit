// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
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
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		// Add +5 damage modifier
		c.Add(TestStageModifiers, "bonus", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage += 5
			return event, nil
		})
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
	s.Assert().Equal(15, result.Damage) // 10 + 5
}

func (s *ChainedTopicTestSuite) TestMultipleSubscribersAddModifiers() {
	// First subscriber adds +2
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		c.Add(TestStageModifiers, "rage", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage += 2
			return event, nil
		})
		return c, nil
	})
	s.Require().NoError(err)

	// Second subscriber adds +3
	_, err = s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		c.Add(TestStageModifiers, "bless", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage += 3
			return event, nil
		})
		return c, nil
	})
	s.Require().NoError(err)

	// Third subscriber adds x2 at final stage
	_, err = s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		c.Add(TestStageFinal, "critical", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage *= 2
			return event, nil
		})
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
	s.Assert().Equal(30, result.Damage)
}

func (s *ChainedTopicTestSuite) TestConditionalModification() {
	// Only modify if attacker is "barbarian"
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		if e.AttackerID == "barbarian" {
			c.Add(TestStageConditions, "rage", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
				event.Damage += 10
				return event, nil
			})
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
	s.Assert().Equal(20, result1.Damage) // Modified

	// Test with wizard
	attack2 := TestAttackEvent{
		AttackerID: "wizard",
		Damage:     10,
	}
	chain2 := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain2, _ := s.attacks.PublishWithChain(s.ctx, attack2, chain2)
	result2, _ := modChain2.Execute(s.ctx, attack2)
	s.Assert().Equal(10, result2.Damage) // Not modified
}

func (s *ChainedTopicTestSuite) TestStageOrdering() {
	var executionOrder []string

	// Add modifiers at different stages
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		// Add in reverse order to test ordering
		c.Add(TestStageFinal, "final", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			executionOrder = append(executionOrder, "final")
			return event, nil
		})
		c.Add(TestStageBase, "base", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			executionOrder = append(executionOrder, "base")
			return event, nil
		})
		c.Add(TestStageConditions, "conditions", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			executionOrder = append(executionOrder, "conditions")
			return event, nil
		})
		c.Add(TestStageModifiers, "modifiers", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			executionOrder = append(executionOrder, "modifiers")
			return event, nil
		})
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
	s.Assert().Equal([]string{"base", "modifiers", "conditions", "final"}, executionOrder)
}

func (s *ChainedTopicTestSuite) TestChainErrorHandling() {
	// Subscribe handler that returns error
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		return c, errors.New("subscription error")
	})
	s.Require().NoError(err) // Subscription itself succeeds

	// Publish should return the error
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	_, err = s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "subscription error")
}

func (s *ChainedTopicTestSuite) TestModifierError() {
	// Add modifier that errors
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		c.Add(TestStageModifiers, "error", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			return event, errors.New("modifier error")
		})
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
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "modifier error")
}

func (s *ChainedTopicTestSuite) TestDuplicateModifierID() {
	// Subscribe handler that adds same ID twice
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		c.Add(TestStageModifiers, "duplicate", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage += 1
			return event, nil
		})
		// Try to add with same ID
		err := c.Add(TestStageModifiers, "duplicate", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage += 2
			return event, nil
		})
		s.Assert().Error(err) // Should fail
		return c, nil
	})
	s.Require().NoError(err)

	// Verify only first modifier is applied
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain, _ := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	result, _ := modChain.Execute(s.ctx, attack)
	s.Assert().Equal(11, result.Damage) // Only +1, not +2
}

func (s *ChainedTopicTestSuite) TestRemoveModifier() {
	// Subscribe handler that adds then removes
	_, err := s.attacks.SubscribeWithChain(s.ctx, func(ctx context.Context, e TestAttackEvent, c chain.Chain[TestAttackEvent]) (chain.Chain[TestAttackEvent], error) {
		// Add modifier
		c.Add(TestStageModifiers, "temp", func(ctx context.Context, event TestAttackEvent) (TestAttackEvent, error) {
			event.Damage += 100
			return event, nil
		})
		// Remove it
		c.Remove("temp")
		return c, nil
	})
	s.Require().NoError(err)

	// Verify modifier was removed
	attack := TestAttackEvent{Damage: 10}
	attackChain := events.NewStagedChain[TestAttackEvent](s.stages)
	modChain, _ := s.attacks.PublishWithChain(s.ctx, attack, attackChain)
	result, _ := modChain.Execute(s.ctx, attack)
	s.Assert().Equal(10, result.Damage) // No modification
}

func TestChainedTopicSuite(t *testing.T) {
	suite.Run(t, new(ChainedTopicTestSuite))
}
