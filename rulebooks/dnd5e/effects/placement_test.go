// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

type placementSuite struct{ suite.Suite }

func TestPlacementSuite(t *testing.T) { suite.Run(t, new(placementSuite)) }
func (s *placementSuite) compile(def dndfx.Definition) *dndfx.CompiledDefinition {
	out, err := dndfx.Compile(&dndfx.CompileInput{Definition: def})
	s.Require().NoError(err)
	return out.Definition
}
func (s *placementSuite) tree() *dndfx.CompiledDefinition {
	return s.compile(dndfx.Composition{Members: []dndfx.Member{
		{Key: "left", Definition: dndfx.Composition{Members: []dndfx.Member{{Key: "ward", Definition: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}}}}},
		{Key: "right", Definition: dndfx.Composition{Members: []dndfx.Member{{Key: "rest", Definition: dndfx.EndOn{Beat: dndfx.BeatRest, Reason: "rest"}}}}},
	}})
}
func (s *placementSuite) TestRootLeafAndTwoGroups() {
	leaf := s.compile(dndfx.Resistance{Types: []damage.Type{damage.Fire}})
	out, err := dndfx.Place(&dndfx.PlaceInput{Definition: leaf, Mounts: []dndfx.Mount{{Key: "ward", OnTerminal: generic.EndPart}}})
	s.Require().NoError(err)
	s.Require().Len(out.Parts, 1)
	s.Equal("ward", out.Parts[0].Declaration.Key)
	s.Equal("ward", out.Parts[0].MountKey)
	out, err = dndfx.Place(&dndfx.PlaceInput{Definition: s.tree(), Mounts: []dndfx.Mount{
		{Key: "maintenance", Path: []string{"right"}, OnTerminal: generic.EndAttachment},
		{Key: "benefits", Path: []string{"left"}, OnTerminal: generic.EndPart},
	}})
	s.Require().NoError(err)
	s.Require().Len(out.Parts, 2)
	s.Equal("benefits/ward", out.Parts[0].Declaration.Key, "mount ordering must not reorder authored leaves")
	s.Equal("maintenance/rest", out.Parts[1].Declaration.Key)
	s.Equal(generic.EndAttachment, out.Parts[1].Declaration.OnTerminal)
}
func (s *placementSuite) TestMissingOverlappingAndInvalidMountsRefuse() {
	for _, mounts := range [][]dndfx.Mount{
		nil,
		{{Key: "only-left", Path: []string{"left"}, OnTerminal: generic.EndPart}},
		{{Key: "root", OnTerminal: generic.EndPart}, {Key: "left", Path: []string{"left"}, OnTerminal: generic.EndPart}},
		{{Key: "same", Path: []string{"left"}, OnTerminal: generic.EndPart}, {Key: "same", Path: []string{"right"}, OnTerminal: generic.EndAttachment}},
		{{Key: "missing", Path: []string{"unknown"}, OnTerminal: generic.EndPart}},
		{{Key: "left", Path: []string{"left"}, OnTerminal: generic.EndApplication}, {Key: "right", Path: []string{"right"}, OnTerminal: generic.EndAttachment}},
		{{Key: "bad/key", OnTerminal: generic.EndPart}},
		{{Key: "bad", OnTerminal: "unknown"}},
	} {
		out, err := dndfx.Place(&dndfx.PlaceInput{Definition: s.tree(), Mounts: mounts})
		s.Error(err)
		s.Nil(out)
	}
	out, err := dndfx.Place(nil)
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.Place(&dndfx.PlaceInput{})
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.Place(&dndfx.PlaceInput{Definition: &dndfx.CompiledDefinition{}})
	s.Error(err, "an uninitialized handle is not a compiled empty composition")
	s.Nil(out)
}
func (s *placementSuite) TestGroupNameHasNoLifetimeMeaning() {
	def := s.compile(dndfx.Composition{Members: []dndfx.Member{{Key: "maintenance", Definition: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}}}})
	out, err := dndfx.Place(&dndfx.PlaceInput{Definition: def, Mounts: []dndfx.Mount{{Key: "local", Path: []string{"maintenance"}, OnTerminal: generic.EndPart}}})
	s.Require().NoError(err)
	s.Equal(generic.EndPart, out.Parts[0].Declaration.OnTerminal)
	s.Equal(dndfx.KindResistance, out.Parts[0].Declaration.Kind)
}
func (s *placementSuite) TestEmptyAndOwnedPlacement() {
	empty := s.compile(dndfx.DefinitionData{Composition: &dndfx.CompositionData{}})
	out, err := dndfx.Place(&dndfx.PlaceInput{Definition: empty})
	s.Require().NoError(err)
	s.Empty(out.Parts)
	def := s.compile(dndfx.Resistance{Types: []damage.Type{damage.Slashing}})
	input := &dndfx.PlaceInput{Definition: def, Mounts: []dndfx.Mount{{Key: "ward", OnTerminal: generic.EndPart}}}
	first, err := dndfx.Place(input)
	s.Require().NoError(err)
	first.Parts[0].Declaration.Params[0] = '!'
	first.Parts[0].Declaration.Key = "changed"
	second, err := dndfx.Place(input)
	s.Require().NoError(err)
	s.Equal("ward", second.Parts[0].Declaration.Key)
	s.JSONEq(`{"types":["slashing"]}`, string(second.Parts[0].Declaration.Params))
}
