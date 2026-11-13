// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/stretchr/testify/suite"
)

type configuredEffectFactsSuite struct{ suite.Suite }

func TestConfiguredEffectFactsSuite(t *testing.T) { suite.Run(t, new(configuredEffectFactsSuite)) }

func configuredFrame() contributions.Frame {
	f := validFrame()
	held := contributions.HeldCondition{Ref: "dnd5e:conditions:linked_effects", SourceID: "recipient-use"}
	f.Held = []contributions.MemberHeld{{Member: "goblin", Conditions: []contributions.HeldCondition{held}}}
	f.HeldEffects = []contributions.HeldEffectDetail{{MemberID: "goblin", Held: held, ApplicationID: "use", DisplayName: "Reckless bite", OriginRef: core.Ref{Module: "example", Type: "items", ID: "sausage"}, OriginID: "sausage-1", Outputs: []contributions.ConfiguredOutput{}}}
	return f
}

func (s *configuredEffectFactsSuite) TestUnknownVersusKnownEmpty() {
	f := configuredFrame()
	s.Require().NoError(f.Validate())
	clone := f.Clone()
	s.Require().Len(clone.HeldEffects, 1)
	s.NotNil(clone.HeldEffects[0].Outputs)
	f.HeldEffects = nil
	s.Require().NoError(f.Validate())
	s.Nil(f.Clone().HeldEffects)
	f = configuredFrame()
	f.HeldEffects[0].Outputs = nil
	s.Error(f.Validate(), "missing detail and an explicit detail without its output list are different")
}

func (s *configuredEffectFactsSuite) TestRejectsMissingHoldingDuplicateDetailAndOutputIDs() {
	for name, change := range map[string]func(*contributions.Frame){
		"unknown holdings":     func(f *contributions.Frame) { f.Held = nil },
		"known absent holding": func(f *contributions.Frame) { f.Held[0].Conditions = nil },
		"foreign holder":       func(f *contributions.Frame) { f.HeldEffects[0].MemberID = "other" },
		"wrong address":        func(f *contributions.Frame) { f.HeldEffects[0].Held.SourceID = "foreign" },
		"duplicate detail":     func(f *contributions.Frame) { f.HeldEffects = append(f.HeldEffects, f.HeldEffects[0]) },
		"no generation":        func(f *contributions.Frame) { f.HeldEffects[0].ApplicationID = "" },
		"no display":           func(f *contributions.Frame) { f.HeldEffects[0].DisplayName = " " },
		"no origin":            func(f *contributions.Frame) { f.HeldEffects[0].OriginID = "" },
		"bad origin ref":       func(f *contributions.Frame) { f.HeldEffects[0].OriginRef = core.Ref{} },
		"empty output id": func(f *contributions.Frame) {
			f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{{Definition: generic.Output{Kind: "advantage"}}}
		},
		"empty kind": func(f *contributions.Frame) { f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{{ID: "one"}} },
		"invalid params": func(f *contributions.Frame) {
			f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{{ID: "one", Definition: generic.Output{Kind: "advantage", Params: []byte(`{`)}}}
		},
		"duplicate output": func(f *contributions.Frame) {
			out := contributions.ConfiguredOutput{ID: "one", Definition: generic.Output{Kind: "advantage"}}
			f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{out, out}
		},
	} {
		s.Run(name, func() { f := configuredFrame(); change(&f); s.Error(f.Validate()) })
	}
	// This layer checks values, not the rulebook registry: future semantic owners
	// must reject unsupported kinds, rather than contributions importing them.
	f := configuredFrame()
	f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{{ID: "future", Definition: generic.Output{Kind: "future-kind"}}}
	s.NoError(f.Validate())
}

func (s *configuredEffectFactsSuite) TestCloneOwnsNestedOutputData() {
	f := configuredFrame()
	amount := 2
	f.HeldEffects[0].Outputs = []contributions.ConfiguredOutput{{ID: "out", Definition: generic.Output{
		Kind: "damage_bonus", Params: []byte(`{"label":"original"}`),
		When:   &machine.Guard{All: []machine.Guard{{Not: &machine.Guard{Fact: &machine.FactLeaf{Name: "action.ability", Op: machine.OpEq, Value: "dex"}}}}},
		Amount: &generic.Amount{Fixed: &amount, ByClassLevel: &generic.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3}}},
	}}}
	// Structural copying does not choose a kind's legal amount strategy.
	s.Require().NoError(f.Validate())
	clone := f.Clone()
	out := &clone.HeldEffects[0].Outputs[0]
	out.Definition.Params[0] = '['
	out.Definition.When.All[0].Not.Fact.Value = "str"
	*out.Definition.Amount.Fixed = 99
	out.Definition.Amount.ByClassLevel.Steps[1] = 99
	clone.HeldEffects[0].OriginRef.ID = "changed"
	clone.Held[0].Conditions[0].SourceID = "changed"
	original := f.HeldEffects[0].Outputs[0].Definition
	s.Equal(`{"label":"original"}`, string(original.Params))
	s.Equal("dex", original.When.All[0].Not.Fact.Value)
	s.Equal(2, *original.Amount.Fixed)
	s.Equal(2, original.Amount.ByClassLevel.Steps[1])
	s.Equal("sausage", f.HeldEffects[0].OriginRef.ID)
	s.Equal("recipient-use", f.Held[0].Conditions[0].SourceID)
	direct := f.HeldEffects[0].Outputs[0].Clone()
	direct.Definition.When.All[0].Not.Fact.Name = "changed"
	s.Equal(machine.FactName("action.ability"), original.When.All[0].Not.Fact.Name)
}

func (s *configuredEffectFactsSuite) TestLegacyHeldIdentityRemainsComparable() {
	f := configuredFrame()
	key := f.Held[0].Conditions[0]
	seen := map[contributions.HeldCondition]bool{key: true}
	s.True(seen[f.HeldEffects[0].Held])
	f.HeldEffects[0].DisplayName = "another observed name"
	s.True(seen[f.HeldEffects[0].Held], "metadata is not membership identity")
}
