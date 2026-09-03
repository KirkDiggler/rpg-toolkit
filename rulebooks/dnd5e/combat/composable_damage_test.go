// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type ComposableDamageTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestLifedrinkerComposableDamageSuite(t *testing.T) {
	suite.Run(t, new(ComposableDamageTestSuite))
}

func (s *ComposableDamageTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// TestFlatNecroticFeatureDoesNotDoubleOnCritical protects the extension
// contract: a feature can append its own typed flat component during the
// damage-chain fold without borrowing the critical weapon component's dice or
// primary metadata. Removing the StageFeatures subscriber changes the final
// instances; marking the flat feature as critical corrupts its typed evidence.
func (s *ComposableDamageTestSuite) TestFlatNecroticFeatureDoesNotDoubleOnCritical() {
	got := s.foldCriticalPactLongsword(3, 5)

	feature := componentBySourceAndType(got.components, dnd5eEvents.DamageSourceFeature, damage.Necrotic)
	s.Require().NotNil(feature, "the synthetic feature must append typed necrotic damage")
	s.Equal(5, feature.Total(), "a +5 Charisma modifier contributes five, not a doubled ten")
	s.False(feature.IsCritical, "a flat-only feature component does not double on a critical")
	s.True(got.featurePresentAtConditions,
		"StageFeatures must append the feature component before StageConditions applies defenses")

	weapon := componentBySourceAndType(got.components, dnd5eEvents.DamageSourceWeapon, damage.Slashing)
	s.Require().NotNil(weapon)
	s.True(weapon.IsCritical, "the weapon's two dice are the critical contribution")
	s.Equal([]int{8, 8}, weapon.Roll.Dice.FinalRolls)

	// Slashing vulnerability doubles only the longsword's 2d8+3, while
	// necrotic resistance halves only the flat feature contribution.
	s.Equal([]combat.DamageInstanceInput{
		{Amount: 2, Type: damage.Necrotic},
		{Amount: 38, Type: damage.Slashing},
	}, got.instances)
	s.Equal(40, got.total)
}

// A Lifedrinker-shaped feature decides its own floor; the chain accepts the
// resulting typed flat component without any production feature type.
func (s *ComposableDamageTestSuite) TestFlatNecroticFeatureCanExpressMinimumOne() {
	got := s.foldCriticalPactLongsword(3, -2)

	feature := componentBySourceAndType(got.components, dnd5eEvents.DamageSourceFeature, damage.Necrotic)
	s.Require().NotNil(feature)
	s.Equal(1, feature.Total())
	s.False(feature.IsCritical)
}

type foldedPactLongsword struct {
	components                 []dnd5eEvents.DamageComponent
	instances                  []combat.DamageInstanceInput
	total                      int
	featurePresentAtConditions bool
}

func (s *ComposableDamageTestSuite) foldCriticalPactLongsword(strengthModifier, charismaModifier int) foldedPactLongsword {
	s.installFlatNecroticFeature(charismaModifier)
	featurePresentAtConditions := false
	s.installTypeSpecificDefenses(&featurePresentAtConditions)

	event := dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		IsCritical: true,
		Components: []dnd5eEvents.DamageComponent{
			{
				Source: dnd5eEvents.DamageSourceWeapon,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
					Dice:   testDiceTrace(8, 8, 8),
				},
				DamageType: damage.Slashing,
				IsCritical: true,
			},
			{
				Source: dnd5eEvents.DamageSourceAbility, Roll: dnd5eEvents.RollComponent{
					Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
					Modifier: intPtr(strengthModifier),
				},
				DamageType: damage.Slashing,
				IsCritical: false,
			},
		},
	})

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modifiedChain, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	instances, total := combat.FinalDamage(folded.Components)
	return foldedPactLongsword{
		components:                 folded.Components,
		instances:                  instances,
		total:                      total,
		featurePresentAtConditions: featurePresentAtConditions,
	}
}

func (s *ComposableDamageTestSuite) installFlatNecroticFeature(charismaModifier int) {
	_, err := dnd5eEvents.DamageChain.On(s.bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.DamageChainEvent, c chain.Chain[*dnd5eEvents.DamageChainEvent]) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			err := c.Add(combat.StageFeatures, "test_lifedrinker_flat_necrotic",
				func(_ context.Context, event *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
					event.Components = append(event.Components, dnd5eEvents.DamageComponent{
						Source: dnd5eEvents.DamageSourceFeature,
						Roll: dnd5eEvents.RollComponent{
							Source:   dnd5eEvents.RollSource{Ref: refs.Features.SneakAttack(), Name: "Sneak Attack"},
							Modifier: intPtr(max(1, charismaModifier)),
						},
						DamageType: damage.Necrotic,
						IsCritical: false,
					})
					return event, nil
				})
			return c, err
		})
	s.Require().NoError(err)
}

func (s *ComposableDamageTestSuite) installTypeSpecificDefenses(featurePresentAtConditions *bool) {
	_, err := dnd5eEvents.DamageChain.On(s.bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.DamageChainEvent, c chain.Chain[*dnd5eEvents.DamageChainEvent]) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			err := c.Add(combat.StageConditions, "test_lifedrinker_type_defenses",
				func(_ context.Context, event *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
					*featurePresentAtConditions = componentBySourceAndType(event.Components, dnd5eEvents.DamageSourceFeature, damage.Necrotic) != nil
					event.Components = append(event.Components,
						dnd5eEvents.DamageComponent{
							Source:     dnd5eEvents.DamageSourceCondition,
							Multiplier: dnd5eEvents.Multiply(2),
							DamageType: damage.Slashing,
						},
						dnd5eEvents.DamageComponent{
							Source:     dnd5eEvents.DamageSourceCondition,
							Multiplier: dnd5eEvents.Multiply(0.5),
							DamageType: damage.Necrotic,
						},
					)
					return event, nil
				})
			return c, err
		})
	s.Require().NoError(err)
}

// testDiceTrace builds a self-consistent dice trace for one pool of faces.
func testDiceTrace(dieSize int, faces ...int) *dnd5eEvents.DiceTrace {
	subtotal := 0
	for _, face := range faces {
		subtotal += face
	}
	return &dnd5eEvents.DiceTrace{
		Notation:      dice.SimplePool(len(faces), dieSize, 0).Notation(),
		DieSize:       dieSize,
		OriginalRolls: faces,
		FinalRolls:    slices.Clone(faces),
		Subtotal:      subtotal,
	}
}

func componentBySourceAndType(components []dnd5eEvents.DamageComponent, source dnd5eEvents.DamageSourceType, typ damage.Type) *dnd5eEvents.DamageComponent {
	for i := range components {
		if components[i].Source == source && components[i].DamageType == typ {
			return &components[i]
		}
	}
	return nil
}
