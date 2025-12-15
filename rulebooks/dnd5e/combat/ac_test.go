// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

type ACTestSuite struct {
	suite.Suite
}

// TestACBreakdownAddComponent tests that AddComponent correctly adds components and updates total
func (s *ACTestSuite) TestACBreakdownAddComponent() {
	breakdown := &ACBreakdown{
		Total:      0,
		Components: []ACComponent{},
	}

	// Add base AC
	breakdown.AddComponent(ACComponent{
		Type:   ACSourceBase,
		Source: nil,
		Value:  10,
	})
	s.Assert().Equal(10, breakdown.Total, "Total should be 10 after adding base")
	s.Assert().Len(breakdown.Components, 1, "Should have 1 component")

	// Add armor
	chainMailRef := &core.Ref{
		Module: "dnd5e",
		Type:   "armor",
		ID:     "chain_mail",
	}
	breakdown.AddComponent(ACComponent{
		Type:   ACSourceArmor,
		Source: chainMailRef,
		Value:  6,
	})
	s.Assert().Equal(16, breakdown.Total, "Total should be 16 after adding armor")
	s.Assert().Len(breakdown.Components, 2, "Should have 2 components")

	// Add shield
	shieldRef := &core.Ref{
		Module: "dnd5e",
		Type:   "armor",
		ID:     "shield",
	}
	breakdown.AddComponent(ACComponent{
		Type:   ACSourceShield,
		Source: shieldRef,
		Value:  2,
	})
	s.Assert().Equal(18, breakdown.Total, "Total should be 18 after adding shield")
	s.Assert().Len(breakdown.Components, 3, "Should have 3 components")
}

// TestACBreakdownNegativeModifiers tests that negative values work for debuffs
func (s *ACTestSuite) TestACBreakdownNegativeModifiers() {
	breakdown := &ACBreakdown{
		Total:      0,
		Components: []ACComponent{},
	}

	// Start with a base AC
	breakdown.AddComponent(ACComponent{
		Type:   ACSourceBase,
		Source: nil,
		Value:  15,
	})
	s.Assert().Equal(15, breakdown.Total, "Total should be 15")

	// Apply a debuff (negative modifier)
	debuffRef := &core.Ref{
		Module: "dnd5e",
		Type:   "condition",
		ID:     "frightened",
	}
	breakdown.AddComponent(ACComponent{
		Type:   ACSourceCondition,
		Source: debuffRef,
		Value:  -2,
	})
	s.Assert().Equal(13, breakdown.Total, "Total should be 13 after debuff")
	s.Assert().Len(breakdown.Components, 2, "Should have 2 components")

	// Verify the debuff component
	debuffComponent := breakdown.Components[1]
	s.Assert().Equal(ACSourceCondition, debuffComponent.Type)
	s.Assert().Equal(-2, debuffComponent.Value)
}

// TestACSourceTypes tests that all AC source types are defined
func (s *ACTestSuite) TestACSourceTypes() {
	// Verify all source types exist
	s.Assert().Equal(ACSourceType("base"), ACSourceBase)
	s.Assert().Equal(ACSourceType("armor"), ACSourceArmor)
	s.Assert().Equal(ACSourceType("shield"), ACSourceShield)
	s.Assert().Equal(ACSourceType("ability"), ACSourceAbility)
	s.Assert().Equal(ACSourceType("feature"), ACSourceFeature)
	s.Assert().Equal(ACSourceType("spell"), ACSourceSpell)
	s.Assert().Equal(ACSourceType("item"), ACSourceItem)
	s.Assert().Equal(ACSourceType("condition"), ACSourceCondition)
}

func TestACTestSuite(t *testing.T) {
	suite.Run(t, new(ACTestSuite))
}

type ACChainTestSuite struct {
	suite.Suite
	eventBus events.EventBus
	ctx      context.Context
}

func (s *ACChainTestSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
	s.ctx = context.Background()
}

// TestACChainBasicModification tests that the AC chain can modify AC breakdown
func (s *ACChainTestSuite) TestACChainBasicModification() {
	// Create initial breakdown
	breakdown := &ACBreakdown{
		Total: 15,
		Components: []ACComponent{
			{Type: ACSourceBase, Source: nil, Value: 10},
			{Type: ACSourceArmor, Source: &core.Ref{
				Module: "dnd5e",
				Type:   "armor",
				ID:     "chain_mail",
			}, Value: 5},
		},
	}

	acEvent := &ACChainEvent{
		CharacterID: "fighter-1",
		Breakdown:   breakdown,
		HasArmor:    true,
		HasShield:   false,
	}

	// Subscribe a modifier that adds +1 AC when wearing armor (like Defense style)
	acChainTopic := ACChain.On(s.eventBus)
	//nolint:lll // Function signature is unavoidably long due to generic types
	_, err := acChainTopic.SubscribeWithChain(
		s.ctx,
		func(_ context.Context, event *ACChainEvent, c chain.Chain[*ACChainEvent]) (chain.Chain[*ACChainEvent], error) {
			if !event.HasArmor {
				return c, nil
			}

			// Add modifier to chain
			err := c.Add(StageFeatures, "defense_test", func(_ context.Context, e *ACChainEvent) (*ACChainEvent, error) {
				e.Breakdown.AddComponent(ACComponent{
					Type: ACSourceFeature,
					Source: &core.Ref{
						Module: "dnd5e",
						Type:   "feature",
						ID:     "fighting_style_defense",
					},
					Value: 1,
				})
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
	acChain := events.NewStagedChain[*ACChainEvent](ModifierStages)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify AC was modified
	s.Assert().Equal(16, finalEvent.Breakdown.Total, "Final AC should be increased by +1")
	s.Assert().Len(finalEvent.Breakdown.Components, 3, "Should have 3 components")

	// Verify the added component
	defenseComponent := finalEvent.Breakdown.Components[2]
	s.Assert().Equal(ACSourceFeature, defenseComponent.Type)
	s.Assert().Equal(1, defenseComponent.Value)
}

// TestACChainNoArmorNoBonus tests that modifier doesn't apply without armor
func (s *ACChainTestSuite) TestACChainNoArmorNoBonus() {
	// Create initial breakdown without armor
	breakdown := &ACBreakdown{
		Total: 13,
		Components: []ACComponent{
			{Type: ACSourceBase, Source: nil, Value: 10},
			{Type: ACSourceAbility, Source: nil, Value: 3},
		},
	}

	acEvent := &ACChainEvent{
		CharacterID: "monk-1",
		Breakdown:   breakdown,
		HasArmor:    false,
		HasShield:   false,
	}

	// Subscribe the same modifier (should not apply)
	acChainTopic := ACChain.On(s.eventBus)
	//nolint:lll // Function signature is unavoidably long due to generic types
	_, err := acChainTopic.SubscribeWithChain(
		s.ctx,
		func(_ context.Context, event *ACChainEvent, c chain.Chain[*ACChainEvent]) (chain.Chain[*ACChainEvent], error) {
			if !event.HasArmor {
				return c, nil
			}

			err := c.Add(StageFeatures, "defense_test", func(_ context.Context, e *ACChainEvent) (*ACChainEvent, error) {
				e.Breakdown.AddComponent(ACComponent{
					Type: ACSourceFeature,
					Source: &core.Ref{
						Module: "dnd5e",
						Type:   "feature",
						ID:     "fighting_style_defense",
					},
					Value: 1,
				})
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
	acChain := events.NewStagedChain[*ACChainEvent](ModifierStages)
	modifiedChain, err := acChainTopic.PublishWithChain(s.ctx, acEvent, acChain)
	s.Require().NoError(err)

	// Execute chain
	finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
	s.Require().NoError(err)

	// Verify AC was NOT modified
	s.Assert().Equal(13, finalEvent.Breakdown.Total, "AC should remain unchanged without armor")
	s.Assert().Len(finalEvent.Breakdown.Components, 2, "Should still have 2 components")
}

func TestACChainTestSuite(t *testing.T) {
	suite.Run(t, new(ACChainTestSuite))
}
