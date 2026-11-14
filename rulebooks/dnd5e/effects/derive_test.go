// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

type definitionDerivationSuite struct{ suite.Suite }

func TestDefinitionDerivationSuite(t *testing.T) { suite.Run(t, new(definitionDerivationSuite)) }
func liveDamageDefinition() dndfx.DefinitionData {
	return dndfx.DefinitionData{Effect: &dndfx.EffectSpec{Kind: dndfx.KindDamageBonus, Amount: &generic.Amount{ByClassLevel: &generic.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}, When: abilityGuard("str")}}
}
func benefitDefinition() dndfx.DefinitionData {
	return dndfx.DefinitionData{Composition: &dndfx.CompositionData{Members: []dndfx.MemberData{{Key: "damage-bonus", Name: "Damage", Definition: liveDamageDefinition()}, {Key: "ward", Definition: resistanceDefinitionData()}}}}
}
func fixedDamageDefinition(n int) dndfx.DefinitionData {
	return dndfx.DefinitionData{Effect: &dndfx.EffectSpec{Kind: dndfx.KindDamageBonus, Amount: &generic.Amount{Fixed: &n}, When: abilityGuard("str")}}
}

func (s *definitionDerivationSuite) TestFixedReplacementPreservesOriginal() {
	original := benefitDefinition()
	out, err := dndfx.Derive(&dndfx.DeriveInput{Definition: original, Replace: []dndfx.ResolvedMemberReplacement{{Path: []string{"damage-bonus"}, Definition: fixedDamageDefinition(2)}}})
	s.Require().NoError(err)
	member := out.Definition.Composition.Members[0]
	s.Equal("damage-bonus", member.Key)
	s.Equal("Damage", member.Name)
	s.Equal(2, *member.Definition.Effect.Amount.Fixed)
	s.Nil(member.Definition.Effect.Amount.ByClassLevel)
	s.Equal(4, original.Composition.Members[0].Definition.Effect.Amount.ByClassLevel.Steps[16])
	out.Definition.Composition.Members[1].Definition.Effect.Params[0] = '!'
	s.JSONEq(`{"types":["slashing"]}`, string(original.Composition.Members[1].Definition.Effect.Params))
}
func (s *definitionDerivationSuite) TestCompleteReplacementDoesNotMerge() {
	out, err := dndfx.Derive(&dndfx.DeriveInput{Definition: benefitDefinition(), Replace: []dndfx.ResolvedMemberReplacement{{Path: []string{"damage-bonus"}, Definition: resistanceDefinitionData()}}})
	s.Require().NoError(err)
	effect := out.Definition.Composition.Members[0].Definition.Effect
	s.Equal(dndfx.KindResistance, effect.Kind)
	s.Nil(effect.Amount)
	s.Nil(effect.When)
}
func (s *definitionDerivationSuite) TestMissingDuplicateOverlapAndLateInvalidRefuse() {
	nested := dndfx.DefinitionData{Composition: &dndfx.CompositionData{Members: []dndfx.MemberData{{Key: "benefits", Definition: benefitDefinition()}}}}
	for _, replacements := range [][]dndfx.ResolvedMemberReplacement{
		{{Definition: fixedDamageDefinition(2)}},
		{{Path: []string{"absent"}, Definition: fixedDamageDefinition(2)}},
		{{Path: []string{"benefits", "damage-bonus"}, Definition: fixedDamageDefinition(2)}, {Path: []string{"benefits", "damage-bonus"}, Definition: fixedDamageDefinition(3)}},
		{{Path: []string{"benefits"}, Definition: benefitDefinition()}, {Path: []string{"benefits", "ward"}, Definition: resistanceDefinitionData()}},
		{{Path: []string{"benefits", "damage-bonus"}, Definition: fixedDamageDefinition(2)}, {Path: []string{"benefits", "ward"}, Definition: dndfx.DefinitionData{Effect: &dndfx.EffectSpec{Kind: "unknown"}}}},
	} {
		out, err := dndfx.Derive(&dndfx.DeriveInput{Definition: nested, Replace: replacements})
		s.Error(err)
		s.Nil(out)
		s.NotNil(nested.Composition.Members[0].Definition.Composition.Members[0].Definition.Effect.Amount.ByClassLevel)
	}
	out, err := dndfx.Derive(nil)
	s.Error(err)
	s.Nil(out)
}
func (s *definitionDerivationSuite) TestReferenceReplacementUsesRelativeMemberPath() {
	typed, err := dndfx.DefineEntry(&dndfx.DefineEntryInput{Ref: definitionRef("raging"), Definition: dndfx.DefinitionData{Composition: &dndfx.CompositionData{Members: []dndfx.MemberData{{Key: "benefits", Definition: benefitDefinition()}}}}})
	s.Require().NoError(err)
	cat, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{typed.Entry}})
	s.Require().NoError(err)
	var request dndfx.AuthoredDefinition
	s.Require().NoError(json.Unmarshal([]byte(`{"use":{"definition":"example:definitions:raging","path":["benefits"],"replace":[{"path":["damage-bonus"],"definition":{"effect":{"kind":"damage_bonus","amount":{"fixed":2},"when":{"fact":{"name":"action.ability","op":"eq","value":"str"}}}}}]}}`), &request))
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: request, Catalogue: cat.Catalogue})
	s.Require().NoError(err)
	s.Equal(2, *out.Definition.Composition.Members[0].Definition.Effect.Amount.Fixed)
	original, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("raging"), "benefits", "damage-bonus"), Catalogue: cat.Catalogue})
	s.Require().NoError(err)
	s.NotNil(original.Definition.Effect.Amount.ByClassLevel)
}
