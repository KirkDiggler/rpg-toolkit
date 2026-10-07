// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type ragingRuleSuite struct{ suite.Suite }

func TestRagingRuleSuite(t *testing.T) { suite.Run(t, new(ragingRuleSuite)) }

func barbarianFrame() contributions.Frame {
	return contributions.Frame{
		Actor:  "barb",
		Target: contributions.Known("goblin"),
		Action: contributions.ActionFacts{
			Roll:       contributions.Known(contributions.RollKindAttack),
			Ability:    contributions.Known(abilities.STR),
			Melee:      contributions.Known(true),
			WeaponPool: contributions.Known(true),
			Advantage:  contributions.Known(false),
		},
		Complete: true,
	}
}

func (s *ragingRuleSuite) assess(frame contributions.Frame) contributions.Answer {
	out, err := ragingDamageRule{owner: "barb", bonus: 2}.AssessAction(&contributions.AssessActionInput{Frame: frame})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NoError(out.Answer.Decision.Validate())
	s.Equal(contributions.ContributesNow, out.Answer.Participation)
	return out.Answer
}

func (s *ragingRuleSuite) TestRagingRuleAppliesToMeleeStrengthWeapon() {
	answer := s.assess(barbarianFrame())

	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("The melee weapon attack uses Strength", answer.Decision.Reason)
	s.Equal("+2 damage", answer.Benefit)
	s.Require().Len(answer.Damage, 1)
	s.Equal(contributions.PrimaryWeaponPool, answer.Damage[0].PoolID)
	s.Equal("weapon:primary", answer.Damage[0].PoolID)
	s.Require().NotNil(answer.Damage[0].Fixed)
	s.Equal(2, *answer.Damage[0].Fixed)
	s.Empty(answer.Damage[0].Dice)
	s.Equal(refs.Conditions.Raging().String(), answer.Damage[0].Source.Ref.String())
}

func (s *ragingRuleSuite) TestRagingRuleDoesNotApplyToDexterity() {
	frame := barbarianFrame()
	frame.Action.Ability = contributions.Known(abilities.DEX)

	answer := s.assess(frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Rage's damage bonus requires Strength", answer.Decision.Reason)
	s.Empty(answer.Benefit)
	s.Empty(answer.Damage)
}

func (s *ragingRuleSuite) TestRagingRuleKnownNegatives() {
	for name, tc := range map[string]struct {
		mutate func(*contributions.Frame)
		reason string
	}{
		"ranged":      {func(f *contributions.Frame) { f.Action.Melee = contributions.Known(false) }, "Rage's damage bonus requires a melee weapon attack"},
		"no weapon":   {func(f *contributions.Frame) { f.Action.WeaponPool = contributions.Known(false) }, "Rage requires a weapon damage pool"},
		"other actor": {func(f *contributions.Frame) { f.Actor = "rogue" }, "Rage modifies its recipient's attacks"},
	} {
		frame := barbarianFrame()
		tc.mutate(&frame)
		answer := s.assess(frame)
		s.Equal(contributions.DoesNotApply, answer.Decision.Applicability, name)
		s.Equal(tc.reason, answer.Decision.Reason, name)
		s.Empty(answer.Damage, name)
	}
}

func (s *ragingRuleSuite) TestRagingRuleDependsWhenAbilityUnknown() {
	frame := barbarianFrame()
	frame.Action.Ability = contributions.Unknown[abilities.Ability]()

	answer := s.assess(frame)

	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Empty(answer.Benefit)
	s.Empty(answer.Damage)

	frame.Action.Melee = contributions.Known(false)
	s.Equal(contributions.DoesNotApply, s.assess(frame).Decision.Applicability,
		"a decisive known negative wins over an unknown ability")
}

// ragingHandlerSuite drives the real damage chain through a live rage.
type ragingHandlerSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	condition *RagingCondition
}

func TestRagingHandlerSuite(t *testing.T) { suite.Run(t, new(ragingHandlerSuite)) }

func (s *ragingHandlerSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.condition = &RagingCondition{CharacterID: "barb", DamageBonus: 2}
	s.Require().NoError(s.condition.Apply(s.ctx, s.bus))
}

func (s *ragingHandlerSuite) fold(event *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	if err != nil {
		return nil, err
	}
	return modified.Execute(s.ctx, event)
}

func (s *ragingHandlerSuite) event(frame contributions.Frame) *dnd5eEvents.DamageChainEvent {
	return &dnd5eEvents.DamageChainEvent{
		AttackerID: "barb", TargetID: "goblin",
		WeaponDamageType: damage.Slashing,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			DamageType: damage.Slashing,
		}},
		Frame: frame,
	}
}

func (s *ragingHandlerSuite) TestRagingHandlerAddsWhenFrameSaysStrengthMelee() {
	result, err := s.fold(s.event(barbarianFrame()))

	s.Require().NoError(err)
	s.Require().Len(result.Components, 2, "the frame says Strength melee, so Rage adds")
	s.Equal("Raging", result.Components[1].Roll.Source.Name)
	s.Equal(2, *result.Components[1].Roll.Modifier)
	s.False(s.condition.DidAttackThisTurn, "a damage fold does not sustain Rage")
}

func (s *ragingHandlerSuite) TestRagingHandlerSkipsWhenFrameSaysDexterity() {
	frame := barbarianFrame()
	frame.Action.Ability = contributions.Known(abilities.DEX)

	result, err := s.fold(s.event(frame))

	s.Require().NoError(err)
	s.Len(result.Components, 1)
}

func (s *ragingHandlerSuite) TestRagingHandlerFailsWhenRuleDepends() {
	frame := barbarianFrame()
	frame.Action.Ability = contributions.Unknown[abilities.Ability]()
	event := s.event(frame)

	result, err := s.fold(event)

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
	s.Nil(result)
	s.Len(event.Components, 1, "no component appended")
}

func (s *ragingHandlerSuite) TestRagingHandlerRejectsZeroFrame() {
	event := s.event(contributions.Frame{})

	result, err := s.fold(event)

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
	s.Nil(result)
	s.Len(event.Components, 1)
}
