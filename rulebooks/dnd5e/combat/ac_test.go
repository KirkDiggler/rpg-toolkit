// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

type ACChainTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	ctx      context.Context
}

func (s *ACChainTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	s.ctx = context.Background()
}

// TestACChainBasicModification tests that the AC chain can modify AC values
//
//nolint:dupl // Test structure similar to TestACChainNoArmorNoBonus but tests different scenarios
func (s *ACChainTestSuite) TestACChainBasicModification() {
	// Create initial AC event
	acEvent := ACChainEvent{
		CharacterID:    "fighter-1",
		BaseAC:         15, // Chain mail
		IsWearingArmor: true,
		ArmorType:      "medium",
		FinalAC:        15,
	}

	// Subscribe a modifier that adds +1 AC when wearing armor (like Defense style)
	acChainTopic := ACChain.On(s.eventBus)
	//nolint:lll // Function signature is unavoidably long due to generic types
	_, err := acChainTopic.SubscribeWithChain(
		s.ctx,
		func(_ context.Context, event ACChainEvent, c chain.Chain[ACChainEvent]) (chain.Chain[ACChainEvent], error) {
			if !event.IsWearingArmor {
				return c, nil
			}

			// Add modifier to chain
			err := c.Add(StageFeatures, "defense_test", func(_ context.Context, e ACChainEvent) (ACChainEvent, error) {
				e.FinalAC++
				return e, nil
			})
			if err != nil {
				return c, err
			}

			return c, nil
		},
	)
	s.Require().NoError(err)

	// Create chain and publish event
	acChain := events.NewStagedChain[ACChainEvent](ModifierStages)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify AC was modified
	s.Assert().Equal(15, finalEvent.BaseAC, "Base AC should remain unchanged")
	s.Assert().Equal(16, finalEvent.FinalAC, "Final AC should be increased by +1")
}

// TestACChainNoArmorNoBonus tests that modifier doesn't apply without armor
//
//nolint:dupl // Test structure similar to TestACChainBasicModification but tests different scenarios
func (s *ACChainTestSuite) TestACChainNoArmorNoBonus() {
	// Create initial AC event without armor
	acEvent := ACChainEvent{
		CharacterID:    "monk-1",
		BaseAC:         13, // Unarmored
		IsWearingArmor: false,
		ArmorType:      "",
		FinalAC:        13,
	}

	// Subscribe the same modifier (should not apply)
	acChainTopic := ACChain.On(s.eventBus)
	//nolint:lll // Function signature is unavoidably long due to generic types
	_, err := acChainTopic.SubscribeWithChain(
		s.ctx,
		func(_ context.Context, event ACChainEvent, c chain.Chain[ACChainEvent]) (chain.Chain[ACChainEvent], error) {
			if !event.IsWearingArmor {
				return c, nil
			}

			// Add modifier to chain
			err := c.Add(StageFeatures, "defense_test", func(_ context.Context, e ACChainEvent) (ACChainEvent, error) {
				e.FinalAC++
				return e, nil
			})
			if err != nil {
				return c, err
			}

			return c, nil
		},
	)
	s.Require().NoError(err)

	// Create chain and publish event
	acChain := events.NewStagedChain[ACChainEvent](ModifierStages)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify AC was NOT modified
	s.Assert().Equal(13, finalEvent.BaseAC, "Base AC should remain unchanged")
	s.Assert().Equal(13, finalEvent.FinalAC, "Final AC should remain unchanged without armor")
}

// TestACChainMultipleModifiers tests stacking multiple AC modifiers
func (s *ACChainTestSuite) TestACChainMultipleModifiers() {
	// Create initial AC event
	acEvent := ACChainEvent{
		CharacterID:    "fighter-1",
		BaseAC:         15,
		IsWearingArmor: true,
		ArmorType:      "heavy",
		FinalAC:        15,
	}

	acChainTopic := ACChain.On(s.eventBus)

	// Add Defense fighting style modifier (+1)
	//nolint:lll // Function signature is unavoidably long due to generic types
	_, err := acChainTopic.SubscribeWithChain(
		s.ctx,
		func(_ context.Context, event ACChainEvent, c chain.Chain[ACChainEvent]) (chain.Chain[ACChainEvent], error) {
			if !event.IsWearingArmor {
				return c, nil
			}

			err := c.Add(StageFeatures, "defense_test", func(_ context.Context, e ACChainEvent) (ACChainEvent, error) {
				e.FinalAC++
				return e, nil
			})
			if err != nil {
				return c, err
			}

			return c, nil
		},
	)
	s.Require().NoError(err)

	// Add Shield modifier (+2)
	//nolint:lll // Function signature is unavoidably long due to generic types
	_, err = acChainTopic.SubscribeWithChain(
		s.ctx,
		func(_ context.Context, _ ACChainEvent, c chain.Chain[ACChainEvent]) (chain.Chain[ACChainEvent], error) {
			// Simulating shield bonus
			err := c.Add(StageEquipment, "shield_test", func(_ context.Context, e ACChainEvent) (ACChainEvent, error) {
				e.FinalAC += 2
				return e, nil
			})
			if err != nil {
				return c, err
			}

			return c, nil
		},
	)
	s.Require().NoError(err)

	// Create chain and publish event
	acChain := events.NewStagedChain[ACChainEvent](ModifierStages)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify both modifiers applied in order
	s.Assert().Equal(15, finalEvent.BaseAC, "Base AC should remain unchanged")
	s.Assert().Equal(18, finalEvent.FinalAC, "Final AC should be 15 + 1 (Defense) + 2 (Shield) = 18")
}

func TestACChainTestSuite(t *testing.T) {
	suite.Run(t, new(ACChainTestSuite))
}
