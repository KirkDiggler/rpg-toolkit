// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

func (s *observationsSuite) TestDefinitionIsOwnedAndDoesNotChangeExecution() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	bound := rg.c.CurrentOutputs()[0]
	def := bound.Definition()
	s.Equal(kindDamageBonus, def.Kind)
	s.Require().NotNil(def.When)
	s.Require().NotNil(def.Amount.ByClassLevel)
	def.Params[0] = '!'
	def.When.All[0].Fact.Value = machine.BoolScalar(false)
	def.Amount.ByClassLevel.Steps[9] = 99
	s.Equal(13, rg.swing(), "changing captured definition must not change the live output")
	next := bound.Definition()
	s.Equal(machine.BoolScalar(true), next.When.All[0].Fact.Value)
	s.Equal(3, next.Amount.ByClassLevel.Steps[9])
	s.NotEqual(byte('!'), next.Params[0])
}
func (s *observationsSuite) TestDefinitionSurvivesEndObservation() {
	vocabulary := outputVocabulary()
	var rg *rig
	var captured effects.Output
	observed := false
	vocabulary.End = func(_ context.Context, _ events.EventBus, _ effects.Host, _ string) error {
		outs := rg.c.CurrentOutputs()
		s.Require().NotEmpty(outs)
		captured = outs[0].Definition()
		s.Equal(13, rg.swing())
		observed = true
		return nil
	}
	rg = newRig(&s.Suite, rageRecipe(), vocabulary)
	s.Require().NoError(restTopic.On(rg.bus).Publish(rg.ctx, rest{CharacterID: holder}))
	s.True(observed)
	s.Empty(rg.c.CurrentOutputs())
	s.Equal(kindDamageBonus, captured.Kind)
	s.Equal(3, captured.Amount.ByClassLevel.Steps[9])
	captured.Amount.ByClassLevel.Steps[9] = 99
	s.Equal(10, rg.swing())
}
func (s *observationsSuite) TestOutputCloneOwnsEveryGuardAndAmountBranch() {
	n := 2
	original := effects.Output{Kind: "test", Params: []byte(`{"v":1}`), Amount: &effects.Amount{Fixed: &n, ByClassLevel: &effects.ClassSteps{Class: "test", Steps: map[int]int{1: 2}}}, When: &machine.Guard{Any: []machine.Guard{
		{Not: &machine.Guard{Fact: &machine.FactLeaf{Name: "fact", Op: machine.OpEq, Value: machine.StringScalar("before")}}},
		{Bag: &machine.BagLeaf{Key: "bag", Value: machine.BoolScalar(true)}},
		{Rounds: &machine.RoundsLeaf{SinceAnchor: &machine.IntOp{Op: machine.OpGte, N: 3}}},
		{ClassLevel: &machine.ClassLevelLeaf{Class: "class", N: 4}},
	}}}
	clone := original.Clone()
	clone.Params[0] = '!'
	*clone.Amount.Fixed = 99
	clone.Amount.ByClassLevel.Steps[1] = 99
	clone.When.Any[0].Not.Fact.Value = machine.StringScalar("after")
	clone.When.Any[1].Bag.Key = "changed"
	clone.When.Any[2].Rounds.SinceAnchor.N = 99
	clone.When.Any[3].ClassLevel.N = 99
	s.Equal(byte('{'), original.Params[0])
	s.Equal(2, *original.Amount.Fixed)
	s.Equal(2, original.Amount.ByClassLevel.Steps[1])
	s.Equal(machine.StringScalar("before"), original.When.Any[0].Not.Fact.Value)
	s.Equal("bag", original.When.Any[1].Bag.Key)
	s.Equal(3, original.When.Any[2].Rounds.SinceAnchor.N)
	s.Equal(4, original.When.Any[3].ClassLevel.N)
	s.Nil((effects.Output{}).Clone().When)
	s.Nil((effects.Output{}).Clone().Amount)
}
