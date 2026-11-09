// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *StructuralWallSuite) TestMaterialOverridesAreCapturedAndDetachedAcrossHostReload() {
	want := []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "dnd5e:material:fantasy-kingdom:atlas_04_c"}}
	field := structuralField()
	field.StructuralWalls[0].MaterialOverrides = append([]encounter.MaterialOverride(nil), want...)
	field.PropPresentations = []encounter.PropPresentation{{ID: "decor", Ref: "test:prop", HeightScale: 1, MaterialOverrides: append([]encounter.MaterialOverride(nil), want...)}}
	enc := s.play(field)
	field.StructuralWalls[0].MaterialOverrides[0].MaterialRef = "edited-source"
	field.PropPresentations[0].MaterialOverrides[0].MaterialRef = "edited-source"
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(atlas.StructuralWalls, 1)
	s.Require().Len(atlas.PropPresentations, 1)
	s.Equal(want, atlas.StructuralWalls[0].MaterialOverrides)
	s.Equal(want, atlas.PropPresentations[0].MaterialOverrides)
	atlas.StructuralWalls[0].MaterialOverrides[0].MaterialRef = "edited-read"
	atlas.PropPresentations[0].MaterialOverrides[0].MaterialRef = "edited-read"
	data := enc.ToData()
	data.Field.StructuralWalls[0].MaterialOverrides[0].MaterialRef = "edited-data"
	data.Field.PropPresentations[0].MaterialOverrides[0].MaterialRef = "edited-data"
	next, err := s.reload(enc).AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(want, next.StructuralWalls[0].MaterialOverrides)
	s.Equal(want, next.PropPresentations[0].MaterialOverrides)
	s.Equal(atlas.Placed, next.Placed, "appearance selections never change blockers")
	s.Equal(atlas.StructuralWalls[0].Openings, next.StructuralWalls[0].Openings)
}

func (s *StructuralWallSuite) TestMalformedMaterialOverridesRefuseConstructionAndLoad() {
	for _, tc := range []struct {
		name   string
		values []encounter.MaterialOverride
	}{
		{"empty-surface", []encounter.MaterialOverride{{SurfaceID: "", MaterialRef: "look"}}},
		{"empty-material", []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: ""}}},
		{"blank", []encounter.MaterialOverride{{SurfaceID: " ", MaterialRef: "look"}}},
		{"blank-material", []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "\t"}}},
		{"duplicate", []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "one"}, {SurfaceID: "atlas", MaterialRef: "two"}}},
	} {
		s.Run(tc.name, func() {
			wall := structuralField()
			wall.StructuralWalls[0].MaterialOverrides = tc.values
			_, err := s.build(wall)
			s.ErrorIs(err, encounter.ErrNoField)
			prop := structuralField()
			prop.PropPresentations = []encounter.PropPresentation{{ID: "decor", Ref: "test:prop", HeightScale: 1, MaterialOverrides: tc.values}}
			_, err = s.build(prop)
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
	enc := s.play(structuralField())
	data := enc.ToData()
	data.Field.StructuralWalls[0].MaterialOverrides = []encounter.MaterialOverrideData{{SurfaceID: "atlas", MaterialRef: ""}}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: encounter.RefusingCapabilities()})
	s.Error(err, "malformed persisted appearance must not load")
}

func (s *StructuralWallSuite) TestConcealedWallDoesNotDiscloseMaterialSelections() {
	field := concealedStructuralField([]encounter.PropID{"wall-presence"}, nil, nil)
	field.StructuralWalls[0].MaterialOverrides = []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "private-look"}}
	enc := s.play(field)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(atlas.StructuralWalls)
	s.NotEmpty(atlas.StructuralDoors, "an unlisted door retains its independent disclosure")
}
