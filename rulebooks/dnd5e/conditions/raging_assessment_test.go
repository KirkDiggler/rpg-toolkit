// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type ragingRuleSuite struct{ suite.Suite }

func TestRagingRuleSuite(t *testing.T)    { suite.Run(t, new(ragingRuleSuite)) }
func barbarianFrame() contributions.Frame { return barbarianFrameAt(1) }
func barbarianFrameAt(level int) contributions.Frame {
	return contributions.Frame{Actor: "barb", ActorClassLevels: contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Barbarian, Levels: level}), Target: contributions.Known("goblin"), Complete: true, Action: contributions.ActionFacts{
		Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR), Melee: contributions.Known(true), WeaponPool: contributions.Known(true), Advantage: contributions.Known(false),
	}}
}
func (s *ragingRuleSuite) assess(frame contributions.Frame) contributions.Effect {
	c := composedRageForTest("barb")
	out, err := c.AssessActionEffects(&contributions.AssessActionInput{Frame: frame})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Effects)
	return out.Effects[0]
}
func (s *ragingRuleSuite) TestRagingRuleAppliesToMeleeStrengthWeapon() {
	answer := s.assess(barbarianFrame())
	s.Equal(contributions.StateApplies, answer.State)
	s.NotEmpty(answer.Reason)
	s.Equal("+2 damage", answer.Benefit)
	s.Equal(refs.Conditions.Raging().String(), answer.Source.Ref.String())
	s.Equal("barb", answer.Source.SourceID)
}

// Preserve the class thresholds and multiclass distinction through the shared
// assessment, not through the removed private rule's concrete result type.
func (s *ragingRuleSuite) TestRagingRuleBonusScalesWithBarbarianLevel() {
	for level, want := range map[int]string{1: "+2 damage", 8: "+2 damage", 9: "+3 damage", 15: "+3 damage", 16: "+4 damage", 20: "+4 damage"} {
		s.Equal(want, s.assess(barbarianFrameAt(level)).Benefit, "barbarian %d", level)
	}
	frame := barbarianFrameAt(1)
	frame.ActorClassLevels = contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Barbarian, Levels: 1}, contributions.ClassLevel{Class: classes.Fighter, Levels: 10})
	s.Equal("+2 damage", s.assess(frame).Benefit, "the class level, not total level")
}
func (s *ragingRuleSuite) TestRagingRuleCannotInventMissingBarbarianLevels() {
	for _, levels := range []contributions.ClassLevels{contributions.KnownClassLevels(), contributions.UnknownClassLevels()} {
		frame := barbarianFrame()
		frame.ActorClassLevels = levels
		answer := s.assess(frame)
		s.Equal(contributions.StateDepends, answer.State)
		s.Empty(answer.Benefit, "unknown/zero class levels must not default to a bonus")
	}
}
func (s *ragingRuleSuite) TestRagingRuleDoesNotApplyToDexterity() {
	frame := barbarianFrame()
	frame.Action.Ability = contributions.Known(abilities.DEX)
	answer := s.assess(frame)
	s.Equal(contributions.StateDoesNotApply, answer.State)
	s.NotEmpty(answer.Reason)
	s.Empty(answer.Benefit)
}
func (s *ragingRuleSuite) TestRagingRuleKnownNegatives() {
	for name, mutate := range map[string]func(*contributions.Frame){
		"ranged":      func(f *contributions.Frame) { f.Action.Melee = contributions.Known(false) },
		"no weapon":   func(f *contributions.Frame) { f.Action.WeaponPool = contributions.Known(false) },
		"other actor": func(f *contributions.Frame) { f.Actor = "rogue" },
	} {
		frame := barbarianFrame()
		mutate(&frame)
		answer := s.assess(frame)
		s.Equal(contributions.StateDoesNotApply, answer.State, name)
		s.NotEmpty(answer.Reason, name)
		s.Empty(answer.Benefit, name)
	}
}
func (s *ragingRuleSuite) TestRagingRuleDependsWhenAbilityUnknown() {
	frame := barbarianFrame()
	frame.Action.Ability = contributions.Unknown[abilities.Ability]()
	answer := s.assess(frame)
	s.Equal(contributions.StateDepends, answer.State)
	s.Empty(answer.Benefit)
	frame.Action.Melee = contributions.Known(false)
	s.Equal(contributions.StateDoesNotApply, s.assess(frame).State, "known negative wins over unknown ability")
}

type ragingHandlerSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	condition *ComposedCondition
}

func TestRagingHandlerSuite(t *testing.T) { suite.Run(t, new(ragingHandlerSuite)) }
func (s *ragingHandlerSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.condition = composedRageForTest("barb")
	s.Require().NoError(s.condition.Apply(s.ctx, s.bus))
}
func (s *ragingHandlerSuite) fold(event *dndevents.DamageChainEvent) (*dndevents.DamageChainEvent, error) {
	chain := events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages)
	modified, err := dndevents.DamageChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	if err != nil {
		return nil, err
	}
	return modified.Execute(s.ctx, event)
}
func (s *ragingHandlerSuite) event(frame contributions.Frame) *dndevents.DamageChainEvent {
	return &dndevents.DamageChainEvent{AttackerID: "barb", TargetID: "goblin", WeaponDamageType: damage.Slashing, Components: []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, Properties: []damage.Property{damage.AddsAttackAbilityModifier}, DamageType: damage.Slashing}}, Frame: frame}
}
func (s *ragingHandlerSuite) TestRagingHandlerAddsWhenFrameSaysStrengthMelee() {
	result, err := s.fold(s.event(barbarianFrame()))
	s.Require().NoError(err)
	s.Require().Len(result.Components, 2)
	s.Equal("Raging", result.Components[1].Roll.Source.Name)
	s.Equal(2, *result.Components[1].Roll.Modifier)
	s.False(ragingState(s.condition).DidAttackThisTurn, "damage fold does not sustain Rage")
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
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	s.Nil(result)
	s.Len(event.Components, 1, "no component appended")
}
func (s *ragingHandlerSuite) TestRagingHandlerRefusesMissingClassLevels() {
	for _, levels := range []contributions.ClassLevels{contributions.KnownClassLevels(), contributions.UnknownClassLevels()} {
		frame := barbarianFrame()
		frame.ActorClassLevels = levels
		event := s.event(frame)
		result, err := s.fold(event)
		s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
		s.Nil(result)
		s.Len(event.Components, 1)
	}
}
func (s *ragingHandlerSuite) TestRagingHandlerRejectsZeroFrame() {
	event := s.event(contributions.Frame{})
	result, err := s.fold(event)
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	s.Nil(result)
	s.Len(event.Components, 1)
}
