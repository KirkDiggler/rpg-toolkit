// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *PlacedPropsSuite) TestPropPresentationRejectsMalformedDefinitionsAndPoseDrift() {
	cases := []struct {
		name   string
		change func(*encounter.PropPresentation)
	}{
		{"id", func(p *encounter.PropPresentation) { p.ID = "" }},
		{"ref", func(p *encounter.PropPresentation) { p.Ref = "" }},
		{"origin", func(p *encounter.PropPresentation) { p.Origin.X = math.NaN() }},
		{"elevation", func(p *encounter.PropPresentation) { p.Elevation = math.Inf(1) }},
		{"facing", func(p *encounter.PropPresentation) { p.FacingDegrees = math.Inf(-1) }},
		{"scale", func(p *encounter.PropPresentation) { p.HeightScale = 0 }},
		{"color", func(p *encounter.PropPresentation) { p.PointLight.Color = "puce" }},
		{"range", func(p *encounter.PropPresentation) { p.PointLight.Range = 0 }},
		{"intensity", func(p *encounter.PropPresentation) { p.PointLight.Intensity = -1 }},
		{"disabled-invalid", func(p *encounter.PropPresentation) { p.PointLight.Enabled = false; p.PointLight.Offset.X = math.NaN() }},
		{"door", func(p *encounter.PropPresentation) { p.DoorID = "missing" }},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			p := encounter.PropPresentation{ID: "decor", Ref: "test:props:vase", HeightScale: 1, PointLight: &encounter.PropPointLight{Enabled: true, Color: "#ffffff", Range: 4}}
			tc.change(&p)
			field := placedField()
			field.PropPresentations = []encounter.PropPresentation{p}
			_, err := s.setup(field)
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
	field := placedField(placed("prop", coveredBox(1, spatial.Point{}), false, false))
	field.PropPresentations = []encounter.PropPresentation{{ID: "prop", Ref: "test:props:vase", HeightScale: 1, Origin: spatial.Point{X: 1}}}
	_, err := s.setup(field)
	s.ErrorIs(err, encounter.ErrNoField, "appearance cannot carry a conflicting bound pose")
	field.PropPresentations[0].Origin = spatial.Point{}
	field.PropPresentations = append(field.PropPresentations, field.PropPresentations[0])
	_, err = s.setup(field)
	s.ErrorIs(err, encounter.ErrNoField, "one render record per identity")
}

func (s *PlacedPropsSuite) TestPropPresentationCopiesDefinitionsAndSurvivesReload() {
	field := placedField()
	p := encounter.PropPresentation{ID: "decor", Ref: "test:props:vase", HeightScale: 1, PointLight: &encounter.PropPointLight{Enabled: true, Color: "#ffffff", Range: 4}}
	field.PropPresentations = []encounter.PropPresentation{p}
	enc, err := s.setup(field, encounter.MemberInput{ID: "a", Kind: encounter.KindPlayer, Position: spatial.Position{}})
	s.Require().NoError(err)
	p.PointLight.Range = 999
	field.PropPresentations[0].Ref = "changed"
	atlas, err := enc.AtlasFor("a")
	s.Require().NoError(err)
	s.Require().Len(atlas.PropPresentations, 1)
	s.Equal("test:props:vase", atlas.PropPresentations[0].Ref)
	s.Equal(4.0, atlas.PropPresentations[0].PointLight.Range)
	data := enc.ToData()
	data.Field.PropPresentations[0].PointLight.Color = "#000000"
	next, err := enc.AtlasFor("a")
	s.Require().NoError(err)
	s.Equal("#ffffff", next.PropPresentations[0].PointLight.Color)
	bad := enc.ToData()
	bad.Field.PropPresentations[0].HeightScale = 0
	_, badErr := encounter.LoadEncounter(encounter.CompileOnlyLoad(bad))
	s.Error(badErr, "malformed persisted definitions are refused on load")
	loaded := s.loadFrom(enc.ToData())
	after, err := loaded.AtlasFor("a")
	s.Require().NoError(err)
	s.Equal(next.PropPresentations, after.PropPresentations)
}
