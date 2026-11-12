// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type configuredHeldEffectsSuite struct{ suite.Suite }

func TestConfiguredHeldEffectsSuite(t *testing.T) { suite.Run(t, new(configuredHeldEffectsSuite)) }
func (s *configuredHeldEffectsSuite) fixture(id string) (dndevents.ConditionBehavior, events.EventBus, contributions.Frame) {
	raw, err := os.ReadFile("testdata/effects/library.json")
	s.Require().NoError(err)
	compiled, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: raw})
	s.Require().NoError(err)
	bound, err := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: compiled.Definitions, DefinitionID: id, UserID: "A", RecipientID: "B", ApplicationID: "use", Origin: definitionOrigin()})
	s.Require().NoError(err)
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "use", CoordinatorID: bound.CoordinatorID, Origin: definitionOrigin(), Spec: bound.Spec, Existing: []conditions.ParticipantConditions{{MemberID: "A"}, {MemberID: "B"}}})
	s.Require().NoError(err)
	made, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(made.Condition.Apply(context.Background(), bus))
	s.T().Cleanup(func() { s.NoError(made.Condition.Remove(context.Background(), bus)) })
	projected, err := conditions.ProjectHeldEffects(&conditions.ProjectHeldEffectsInput{HolderID: "B", Conditions: []dndevents.ConditionBehavior{made.Condition}})
	s.Require().NoError(err)
	s.Require().Len(projected.Details, 1)
	f := contributions.Frame{Actor: "enemy", Target: contributions.Known("B"), Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR), Melee: contributions.Known(true), WeaponPool: contributions.Known(true), Opportunity: contributions.Known(false)}, Held: []contributions.MemberHeld{{Member: "B", Conditions: []contributions.HeldCondition{projected.Details[0].Held}}}, HeldEffects: projected.Details}
	return made.Condition, bus, f
}
func (s *configuredHeldEffectsSuite) info(f contributions.Frame) []contributions.Effect {
	out, err := conditions.AssessTargetHeldEffects(&conditions.AssessTargetHeldEffectsInput{Frame: f})
	s.Require().NoError(err)
	return out.Effects
}
func (s *configuredHeldEffectsSuite) fold(bus events.EventBus, f contributions.Frame) (dndevents.AttackChainEvent, error) {
	target, _ := f.Target.Get()
	e := dndevents.AttackChainEvent{AttackerID: f.Actor, TargetID: target, Frame: f}
	ctx := context.Background()
	c, err := dndevents.AttackChain.On(bus).PublishWithChain(ctx, e, events.NewStagedChain[dndevents.AttackChainEvent](combat.ModifierStages))
	if err != nil {
		return e, err
	}
	return c.Execute(ctx, e)
}
func (s *configuredHeldEffectsSuite) TestObservedUnknownEmptyAndKnown() {
	_, bus, f := s.fixture("kitchen-sausage")
	rows := s.info(f)
	s.Require().Len(rows, 1)
	s.Equal(contributions.StateApplies, rows[0].State)
	s.Equal("item-1", rows[0].Source.SourceID)
	out, err := s.fold(bus, f)
	s.Require().NoError(err)
	s.Len(out.AdvantageSources, 1)
	unknown := f.Clone()
	unknown.HeldEffects = nil
	rows = s.info(unknown)
	s.Require().Len(rows, 1)
	s.Equal(contributions.StateDepends, rows[0].State)
	s.Empty(rows[0].Benefit)
	s.Empty(rows[0].Source.SourceID)
	empty := f.Clone()
	empty.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{}
	s.Empty(s.info(empty))
	_, _, ward := s.fixture("physical-ward")
	s.NotNil(ward.HeldEffects[0].Outputs)
	s.Empty(s.info(ward))
}
func (s *configuredHeldEffectsSuite) TestSnapshotDoesNotReadLiveTarget() {
	condition, bus, f := s.fixture("kitchen-sausage")
	captured := f.Clone()
	s.Require().NoError(condition.Remove(context.Background(), bus))
	rows := s.info(captured)
	s.Require().Len(rows, 1)
	s.Equal(contributions.StateApplies, rows[0].State)
	out, err := s.fold(bus, captured)
	s.Require().NoError(err)
	s.Empty(out.AdvantageSources, "there is no live attachment left to execute")
	f.HeldEffects[0].Outputs[0].Definition.Params[0] = '!'
	s.Equal(contributions.StateApplies, s.info(captured)[0].State)
}
func (s *configuredHeldEffectsSuite) TestLiveGenerationOrConfigurationMismatchRefusesExecution() {
	_, bus, f := s.fixture("kitchen-sausage")
	for name, mutate := range map[string]func(*contributions.Frame){
		"missing":        func(f *contributions.Frame) { f.HeldEffects = nil },
		"generation":     func(f *contributions.Frame) { f.HeldEffects[0].ApplicationID = "other" },
		"missing output": func(f *contributions.Frame) { f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{} },
		"configuration": func(f *contributions.Frame) {
			f.HeldEffects[0].Outputs[0].Definition.Params = []byte(`{"on":"attack","side":"by_holder"}`)
		},
		"origin": func(f *contributions.Frame) { f.HeldEffects[0].OriginID = "other" },
	} {
		s.Run(name, func() {
			bad := f.Clone()
			mutate(&bad)
			_, err := s.fold(bus, bad)
			s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
		})
	}
}
func (s *configuredHeldEffectsSuite) TestCanonicalRoundTripIsNotAForeignConfiguration() {
	_, bus, f := s.fixture("kitchen-sausage")
	f.HeldEffects[0].Outputs[0].Definition.Params = []byte("{ \"side\" : \"against_holder\", \"on\" : \"attack\" }")
	raw, err := json.Marshal(f.HeldEffects)
	s.Require().NoError(err)
	var details []contributions.HeldEffectDetail
	s.Require().NoError(json.Unmarshal(raw, &details))
	f.HeldEffects = details
	out, err := s.fold(bus, f)
	s.Require().NoError(err)
	s.Len(out.AdvantageSources, 1)
}
func (s *configuredHeldEffectsSuite) TestSelfAttackNamesTwoIndependentSources() {
	condition, bus, f := s.fixture("kitchen-sausage")
	f.Actor = "B"
	out, err := s.fold(bus, f)
	s.Require().NoError(err)
	s.Require().Len(out.AdvantageSources, 2)
	own, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: []dndevents.ConditionBehavior{condition}, Frame: f})
	s.Require().NoError(err)
	s.Require().Len(own.Effects, 1, "the actor rows must not duplicate its target-side drawback")
	s.Equal(contributions.StateApplies, own.Effects[0].State)
	s.Len(s.info(f), 1)
}
func (s *configuredHeldEffectsSuite) TestFixedNamedConditionsKeepTheirExistingPath() {
	f := contributions.Frame{Actor: "enemy", Target: contributions.Known("B"), Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)}, Held: []contributions.MemberHeld{{Member: "B", Conditions: []contributions.HeldCondition{{Ref: refs.Conditions.RecklessAttack().String()}}}}}
	rows := s.info(f)
	s.Require().Len(rows, 1)
	s.Equal(contributions.StateApplies, rows[0].State)
}
