// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

type definitionCompilerSuite struct{ suite.Suite }

func TestDefinitionCompilerSuite(t *testing.T) { suite.Run(t, new(definitionCompilerSuite)) }
func resistanceDefinitionData() dndfx.DefinitionData {
	return dndfx.DefinitionData{Effect: &dndfx.EffectSpec{Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["slashing"]}`)}}
}
func (s *definitionCompilerSuite) TestRootEffectWithoutCatalogueOrHolder() {
	for _, definition := range []dndfx.Definition{dndfx.Resistance{Types: []damage.Type{damage.Slashing}}, dndfx.Concentration{}, dndfx.Duration{TurnEnds: 3}} {
		out, err := dndfx.Compile(&dndfx.CompileInput{Definition: definition})
		s.Require().NoError(err)
		leaves := out.Definition.Leaves()
		s.Require().Len(leaves, 1)
		s.Empty(leaves[0].Path)
	}
}
func (s *definitionCompilerSuite) TestExplicitEmptyVersusAbsent() {
	out, err := dndfx.Compile(nil)
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.Compile(&dndfx.CompileInput{})
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.Compile(&dndfx.CompileInput{Definition: dndfx.DefinitionData{}})
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.Compile(&dndfx.CompileInput{Definition: dndfx.DefinitionData{Composition: &dndfx.CompositionData{}}})
	s.Require().NoError(err)
	s.NotNil(out.Definition)
	s.Empty(out.Definition.Leaves())
	both := resistanceDefinitionData()
	both.Composition = &dndfx.CompositionData{}
	out, err = dndfx.Compile(&dndfx.CompileInput{Definition: both})
	s.Error(err)
	s.Nil(out)
}
func (s *definitionCompilerSuite) TestOrderedQualifiedLeaves() {
	input := dndfx.Composition{Members: []dndfx.Member{
		{Key: "z-group", Name: "First group", Definition: dndfx.Composition{Members: []dndfx.Member{{Key: "b", Definition: dndfx.Resistance{Types: []damage.Type{damage.Fire}}}, {Key: "a", Definition: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}}}}},
		{Key: "a-group", Definition: dndfx.Concentration{}},
	}}
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: input})
	s.Require().NoError(err)
	var paths [][]string
	for _, leaf := range out.Definition.Leaves() {
		paths = append(paths, leaf.Path)
	}
	s.Equal([][]string{{"z-group", "b"}, {"z-group", "a"}, {"a-group"}}, paths, "neither keys nor map order may reorder authored members")
	s.Equal("First group", out.Definition.Data().Composition.Members[0].Name)
}
func (s *definitionCompilerSuite) TestDuplicateSiblingKeysRefuse() {
	for _, keys := range [][]string{{"x", "x"}, {"", "x"}, {" ", "x"}, {"x/y", "z"}} {
		out, err := dndfx.Compile(&dndfx.CompileInput{Definition: dndfx.Composition{Members: []dndfx.Member{{Key: keys[0], Definition: resistanceDefinitionData()}, {Key: keys[1], Definition: resistanceDefinitionData()}}}})
		s.Error(err)
		s.Nil(out)
	}
}
func (s *definitionCompilerSuite) TestRepeatedSubtreeIsIndependent() {
	shared := &dndfx.CompositionData{Members: []dndfx.MemberData{{Key: "resist", Definition: resistanceDefinitionData()}}}
	root := dndfx.DefinitionData{Composition: &dndfx.CompositionData{Members: []dndfx.MemberData{{Key: "left", Definition: dndfx.DefinitionData{Composition: shared}}, {Key: "right", Definition: dndfx.DefinitionData{Composition: shared}}}}}
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: root})
	s.Require().NoError(err)
	leaves := out.Definition.Leaves()
	s.Require().Len(leaves, 2)
	s.Equal([]string{"left", "resist"}, leaves[0].Path)
	s.Equal([]string{"right", "resist"}, leaves[1].Path)
	leaves[0].Effect.Params[0] = '!'
	leaves[0].Path[0] = "changed"
	s.JSONEq(`{"types":["slashing"]}`, string(leaves[1].Effect.Params))
	s.Equal("left", out.Definition.Leaves()[0].Path[0])
}
func (s *definitionCompilerSuite) TestCanonicalAndGoCyclesRefuse() {
	cycle := &dndfx.CompositionData{}
	cycle.Members = []dndfx.MemberData{{Key: "self", Definition: dndfx.DefinitionData{Composition: cycle}}}
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: dndfx.DefinitionData{Composition: cycle}})
	s.Error(err)
	s.Nil(out)
	goCycle := &dndfx.Composition{}
	goCycle.Members = []dndfx.Member{{Key: "self", Definition: goCycle}}
	out, err = dndfx.Compile(&dndfx.CompileInput{Definition: goCycle})
	s.Error(err)
	s.Nil(out)
}
func (s *definitionCompilerSuite) TestMalformedLaterLeafRefusesAll() {
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: dndfx.Composition{Members: []dndfx.Member{{Key: "first", Definition: resistanceDefinitionData()}, {Key: "bad", Definition: dndfx.DefinitionData{Effect: &dndfx.EffectSpec{Kind: "unknown"}}}}}})
	s.Error(err)
	s.Nil(out)
}
func (s *definitionCompilerSuite) TestCompilationOwnsNestedValues() {
	bonus := 2
	guard := abilityGuard("str")
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: dndfx.DamageBonus{Amount: generic.Amount{Fixed: &bonus}, When: guard}})
	s.Require().NoError(err)
	bonus = 99
	guard.Fact.Value = "dex"
	leaves := out.Definition.Leaves()
	s.Equal(2, *leaves[0].Effect.Amount.Fixed)
	s.Equal("str", leaves[0].Effect.When.Fact.Value)
	*leaves[0].Effect.Amount.Fixed = 88
	leaves[0].Effect.When.Fact.Value = "wis"
	saved := out.Definition.Data()
	*saved.Effect.Amount.Fixed = 77
	s.Equal(2, *out.Definition.Leaves()[0].Effect.Amount.Fixed)
}

func (s *definitionCompilerSuite) TestIntrinsicCapabilitiesAreOwned() {
	data := resistanceDefinitionData()
	first, err := dndfx.ValidateEffect(&dndfx.ValidateEffectInput{Effect: *data.Effect})
	s.Require().NoError(err)
	s.Equal([]generic.EndScope{generic.EndPart}, first.Capabilities.AllowedScopes)
	first.Capabilities.AllowedScopes[0] = generic.EndApplication
	second, err := dndfx.ValidateEffect(&dndfx.ValidateEffectInput{Effect: *data.Effect})
	s.Require().NoError(err)
	s.Equal([]generic.EndScope{generic.EndPart}, second.Capabilities.AllowedScopes)
	_, err = dndfx.ValidateEffect(nil)
	s.Error(err)
}

type refusedProjection struct{ err error }

func (d refusedProjection) DefinitionData() (dndfx.DefinitionData, error) {
	return dndfx.DefinitionData{}, d.err
}
func (s *definitionCompilerSuite) TestProjectionErrorsAndTypedNilRefuse() {
	cause := errors.New("definition construction refused")
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: refusedProjection{err: cause}})
	s.ErrorIs(err, cause)
	s.Nil(out)
	var nilLeaf *dndfx.Resistance
	s.NotPanics(func() { out, err = dndfx.Compile(&dndfx.CompileInput{Definition: nilLeaf}); s.Error(err); s.Nil(out) })
}
