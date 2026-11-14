// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type definitionCatalogueSuite struct{ suite.Suite }

func TestDefinitionCatalogueSuite(t *testing.T) { suite.Run(t, new(definitionCatalogueSuite)) }
func definitionRef(id string) core.Ref {
	return core.Ref{Module: "example", Type: "definitions", ID: id}
}
func authoredResistance() dndfx.AuthoredDefinition {
	return dndfx.AuthoredDefinition{Effect: resistanceDefinitionData().Effect}
}
func definitionUse(ref core.Ref, path ...string) dndfx.AuthoredDefinition {
	return dndfx.AuthoredDefinition{Use: &dndfx.DefinitionUse{Reference: dndfx.DefinitionReference{Definition: ref, Path: path}}}
}

func (s *definitionCatalogueSuite) TestTypedAndDataEntriesShareValues() {
	entry, err := dndfx.DefineEntry(&dndfx.DefineEntryInput{Ref: definitionRef("ward"), Name: "Ward", Definition: dndfx.Composition{Members: []dndfx.Member{{Key: "resist", Definition: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}}}}})
	s.Require().NoError(err)
	typed, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{entry.Entry}})
	s.Require().NoError(err)
	var wire dndfx.CatalogueData
	s.Require().NoError(json.Unmarshal([]byte(`{"entries":[{"ref":"example:definitions:ward","name":"Ward","definition":{"composition":{"members":[{"key":"resist","definition":{"effect":{"kind":"resistance","params":{"types":["slashing"]}}}}]}}}]}`), &wire))
	data, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: wire.Entries})
	s.Require().NoError(err)
	a, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("ward")), Catalogue: typed.Catalogue})
	s.Require().NoError(err)
	b, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("ward")), Catalogue: data.Catalogue})
	s.Require().NoError(err)
	s.Equal(a.Definition, b.Definition)
	s.Equal("dnd5e:definitions:raging", refs.Definitions.Raging().String())
	s.NotEqual(refs.Conditions.Raging().String(), refs.Definitions.Raging().String())
}
func (s *definitionCatalogueSuite) TestEnumerationIsOwnedAndOrdered() {
	entry := dndfx.CatalogueEntry{Ref: definitionRef("ward"), Name: "Ward", Definition: dndfx.AuthoredDefinition{Composition: &dndfx.AuthoredComposition{Members: []dndfx.AuthoredMember{
		{Key: "z", Name: "First", Definition: authoredResistance()}, {Key: "a", Name: "Second", Definition: authoredResistance()},
	}}}}
	cat, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{entry}})
	s.Require().NoError(err)
	entry.Definition.Composition.Members[0].Name = "changed-input"
	out, err := dndfx.DescribeCatalogue(&dndfx.DescribeCatalogueInput{Catalogue: cat.Catalogue})
	s.Require().NoError(err)
	s.Require().Len(out.Entries, 3)
	s.Empty(out.Entries[0].Path)
	s.Equal("Ward", out.Entries[0].Name)
	s.Equal([]string{"z"}, out.Entries[1].Path)
	s.Equal("First", out.Entries[1].Name)
	s.Equal(dndfx.KindResistance, out.Entries[1].Kind)
	s.Equal([]string{"a"}, out.Entries[2].Path)
	out.Entries[1].Path[0] = "changed-output"
	out.Entries[0].Ref.ID = "changed-ref"
	again, err := dndfx.DescribeCatalogue(&dndfx.DescribeCatalogueInput{Catalogue: cat.Catalogue})
	s.Require().NoError(err)
	s.Equal("z", again.Entries[1].Path[0])
	s.Equal("ward", again.Entries[0].Ref.ID)
}
func (s *definitionCatalogueSuite) TestDuplicateAndUnusedInvalidEntriesRefuse() {
	good := dndfx.CatalogueEntry{Ref: definitionRef("good"), Definition: authoredResistance()}
	for _, bad := range []dndfx.CatalogueEntry{
		good,
		{Ref: definitionRef("unused"), Definition: dndfx.AuthoredDefinition{Effect: &dndfx.EffectSpec{Kind: "unknown"}}},
		{Ref: definitionRef("missing"), Definition: definitionUse(definitionRef("absent"))},
		{Ref: *refs.Conditions.Raging(), Definition: authoredResistance()},
	} {
		out, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{good, bad}})
		s.Error(err)
		s.Nil(out)
	}
	out, err := dndfx.BuildCatalogue(nil)
	s.Error(err)
	s.Nil(out)
}
func (s *definitionCatalogueSuite) TestForwardReferenceAndEmptyCatalogue() {
	cat, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{Entries: []dndfx.CatalogueEntry{{Ref: definitionRef("first"), Definition: definitionUse(definitionRef("later"))}, {Ref: definitionRef("later"), Definition: authoredResistance()}}})
	s.Require().NoError(err)
	out, err := dndfx.Resolve(&dndfx.ResolveInput{Definition: definitionUse(definitionRef("first")), Catalogue: cat.Catalogue})
	s.Require().NoError(err)
	s.Equal(dndfx.KindResistance, out.Definition.Effect.Kind)
	empty, err := dndfx.BuildCatalogue(&dndfx.BuildCatalogueInput{})
	s.Require().NoError(err)
	listed, err := dndfx.DescribeCatalogue(&dndfx.DescribeCatalogueInput{Catalogue: empty.Catalogue})
	s.Require().NoError(err)
	s.Empty(listed.Entries)
}
