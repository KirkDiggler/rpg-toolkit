// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type definitionReferenceSuite struct{ suite.Suite }

func TestDefinitionReferenceSuite(t *testing.T) { suite.Run(t, new(definitionReferenceSuite)) }
func (s *definitionReferenceSuite) catalogue() *dndfx.Catalogue {
	out, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{{Ref: definitionRef("group"), Definition: dndfx.AuthoredDefinition{Composition: &dndfx.AuthoredComposition{Members: []dndfx.AuthoredMember{{Key: "benefits", Definition: dndfx.AuthoredDefinition{Composition: &dndfx.AuthoredComposition{Members: []dndfx.AuthoredMember{{Key: "ward", Definition: authoredResistance()}}}}}}}}}}})
	s.Require().NoError(err)
	return out.Catalogue
}
func (s *definitionReferenceSuite) TestRootGroupAndMember() {
	cat := s.catalogue()
	for _, tc := range []struct {
		path []string
		leaf bool
	}{{nil, false}, {[]string{"benefits"}, false}, {[]string{"benefits", "ward"}, true}} {
		out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("group"), tc.path...), Catalogue: cat})
		s.Require().NoError(err)
		s.Equal(tc.leaf, out.Definition.Effect != nil)
	}
	for _, path := range [][]string{{"missing"}, {"benefits", "ward", "through-leaf"}, {"bad/path"}} {
		out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("group"), path...), Catalogue: cat})
		s.Error(err)
		s.Nil(out)
	}
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("unknown")), Catalogue: cat})
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.Resolve(&dndfx.ResolveInput{Definition: authoredResistance()})
	s.Require().NoError(err)
	s.Equal(dndfx.KindResistance, out.Definition.Effect.Kind, "inline content needs no fake catalogue")
	out, err = dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("group"))})
	s.Error(err)
	s.Nil(out)
}
func (s *definitionReferenceSuite) TestStrictWireReference() {
	var definition dndfx.AuthoredDefinition
	s.Require().NoError(json.Unmarshal([]byte(`{"use":{"definition":"example:definitions:group","path":["benefits","ward"]}}`), &definition))
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definition, Catalogue: s.catalogue()})
	s.Require().NoError(err)
	s.Equal(dndfx.KindResistance, out.Definition.Effect.Kind)
	for _, bad := range []string{
		`{"use":{"definition":{"module":"example","type":"definitions","id":"group"},"path":[]}}`,
		`{"use":{"definition":"example:definitions:group","path":"benefits/ward"}}`,
		`{"use":{"definition":"example:definitions:group","unknown":true}}`,
		`{"use":{"definition":"example:definitions:group","definition":"example:definitions:other"}}`,
		`{"unknown":true,"effect":{"kind":"resistance","params":{"types":["fire"]}}}`,
	} {
		s.Error(json.Unmarshal([]byte(bad), &definition), bad)
	}
	var catalogue dndfx.CatalogueData
	s.Error(json.Unmarshal([]byte(`{"entries":[{"ref":{"module":"example","type":"definitions","id":"x"},"definition":{"effect":{"kind":"concentration"}}}]}`), &catalogue))
	s.Error(json.Unmarshal([]byte(`{"entries":[],"unknown":true}`), &catalogue))
}
func (s *definitionReferenceSuite) TestSelfAndCrossCyclesRefuse() {
	for _, entries := range [][]dndfx.CatalogueEntry{
		{{Ref: definitionRef("a"), Definition: definitionUse(definitionRef("a"))}},
		{{Ref: definitionRef("a"), Definition: definitionUse(definitionRef("b"))}, {Ref: definitionRef("b"), Definition: definitionUse(definitionRef("a"))}},
	} {
		out, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: entries})
		s.Error(err)
		s.Nil(out)
	}
	cycle := &dndfx.AuthoredComposition{}
	cycle.Members = []dndfx.AuthoredMember{{Key: "self", Definition: dndfx.AuthoredDefinition{Composition: cycle}}}
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: dndfx.AuthoredDefinition{Composition: cycle}})
	s.Error(err)
	s.Nil(out)
}
func (s *definitionReferenceSuite) TestAcyclicSameEntryMemberReuse() {
	value := dndfx.AuthoredDefinition{Composition: &dndfx.AuthoredComposition{Members: []dndfx.AuthoredMember{
		{Key: "alias", Definition: definitionUse(definitionRef("a"), "original")},
		{Key: "original", Definition: authoredResistance()},
	}}}
	cat, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{{Ref: definitionRef("a"), Definition: value}}})
	s.Require().NoError(err)
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("a")), Catalogue: cat.Catalogue})
	s.Require().NoError(err)
	s.Equal("alias", out.Definition.Composition.Members[0].Key)
	s.Equal(dndfx.KindResistance, out.Definition.Composition.Members[0].Definition.Effect.Kind)
}
func (s *definitionReferenceSuite) TestReferenceContractFixtures() {
	benefits := benefitDefinition()
	benefits.Composition.Members[1].Key = "physical-resistance"
	entry, err := dndfx.DefineEntry(&dndfx.DefineEntryInput{Ref: *refs.Definitions.Raging(), Definition: dndfx.DefinitionData{Composition: &dndfx.CompositionData{Members: []dndfx.MemberData{{Key: "benefits", Definition: benefits}}}}})
	s.Require().NoError(err)
	catalogue, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{entry.Entry}})
	s.Require().NoError(err)
	for _, file := range []string{"resistance-use.json", "fixed-rage-use.json"} {
		raw, err := os.ReadFile("testdata/definitions/" + file)
		s.Require().NoError(err)
		var input dndfx.AuthoredDefinition
		s.Require().NoError(json.Unmarshal(raw, &input))
		out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: input, Catalogue: catalogue.Catalogue})
		s.Require().NoError(err)
		if file == "resistance-use.json" {
			s.Equal(dndfx.KindResistance, out.Definition.Effect.Kind)
		} else {
			damage := out.Definition.Composition.Members[0].Definition.Effect
			s.Equal(2, *damage.Amount.Fixed)
			s.Nil(damage.Amount.ByClassLevel)
			s.Require().Len(damage.When.All, 3)
			s.Equal("str", damage.When.All[2].Fact.Value)
		}
	}
	original, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(*refs.Definitions.Raging(), "benefits", "damage-bonus"), Catalogue: catalogue.Catalogue})
	s.Require().NoError(err)
	s.NotNil(original.Definition.Effect.Amount.ByClassLevel)
}

func (s *definitionReferenceSuite) TestReplacementCanResolveAnotherEntryButCannotCycle() {
	catalogue, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{
		{Ref: definitionRef("group"), Definition: dndfx.AuthoredDefinition{Composition: &dndfx.AuthoredComposition{Members: []dndfx.AuthoredMember{{Key: "ward", Definition: authoredResistance()}}}}},
		{Ref: definitionRef("fire"), Definition: dndfx.AuthoredDefinition{Effect: &dndfx.EffectSpec{Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["fire"]}`)}}},
	}})
	s.Require().NoError(err)
	use := &dndfx.DefinitionUse{Reference: dndfx.DefinitionReference{Definition: definitionRef("group")}, Replace: []dndfx.MemberReplacement{{Path: []string{"ward"}, Definition: definitionUse(definitionRef("fire"))}}}
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: dndfx.AuthoredDefinition{Use: use}, Catalogue: catalogue.Catalogue})
	s.Require().NoError(err)
	s.JSONEq(`{"types":["fire"]}`, string(out.Definition.Composition.Members[0].Definition.Effect.Params))
	use.Replace[0].Definition = dndfx.AuthoredDefinition{Use: use}
	out, err = dndfx.Resolve(&dndfx.ResolveInput{Definition: dndfx.AuthoredDefinition{Use: use}, Catalogue: catalogue.Catalogue})
	s.Error(err)
	s.Nil(out)
}

func (s *definitionReferenceSuite) TestRepeatedSelectionOwnsData() {
	cat := s.catalogue()
	in := &dndfx.ResolveInput{Definition: definitionUse(definitionRef("group"), "benefits", "ward"), Catalogue: cat}
	first, err := dndfx.Resolve(in)
	s.Require().NoError(err)
	first.Definition.Effect.Params[0] = '!'
	second, err := dndfx.Resolve(in)
	s.Require().NoError(err)
	s.JSONEq(`{"types":["slashing"]}`, string(second.Definition.Effect.Params))
}
