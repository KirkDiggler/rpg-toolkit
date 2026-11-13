// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func framePresentation() *encounter.DoorFramePresentation {
	return &encounter.DoorFramePresentation{
		Span: &encounter.WallFrameSpan{Before: 3, After: 3},
		FaceFinishes: []encounter.DoorFrameFaceFinish{
			{Side: encounter.WallFacePositiveZ, MaterialRef: "material:gray", PhaseU: 0.125},
			{Side: encounter.WallFaceNegativeZ, MaterialRef: "material:gray", PhaseU: 0.875},
		},
	}
}

func (s *StructuralWallSuite) TestFrameSiteDataRequiresExplicitPhase() {
	field := structuralField()
	frame := framePresentation()
	for i := range frame.FaceFinishes {
		frame.FaceFinishes[i].PhaseU = 0
	}
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = frame
	original := s.play(field).Site()
	raw, err := json.Marshal(original.Data())
	s.Require().NoError(err)
	s.Require().Contains(string(raw), `"phase_u":0`)
	cases := []struct {
		name    string
		payload []byte
	}{
		{"explicit", raw},
		{"missing", bytes.ReplaceAll(raw, []byte(`,"phase_u":0`), nil)},
		{"null", bytes.ReplaceAll(raw, []byte(`"phase_u":0`), []byte(`"phase_u":null`))},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			var data encounter.SiteData
			s.Require().NoError(json.Unmarshal(tc.payload, &data))
			out, err := encounter.NewMemorySites(nil).Site(context.Background(), &encounter.SiteInput{Hash: original.Hash(), Data: &data})
			if tc.name != "explicit" {
				s.Require().ErrorIs(err, encounter.ErrNoField, "refuse presence before canonical hashing can turn it into zero")
				s.Contains(err.Error(), "phase")
				s.Nil(out)
				return
			}
			s.Require().NoError(err)
			s.Equal(original.Hash(), out.Site.Hash())
			encoded, err := json.Marshal(out.Site.Data())
			s.Require().NoError(err)
			s.Equal(raw, encoded, "valid stored bytes and their identity remain unchanged")
			s.T().Logf("valid zero-phase site hash: %s", original.Hash())
			*data.Field.StructuralWalls[0].Openings[0].Door.FramePresentation.FaceFinishes[0].PhaseU = 0.25
			returned := out.Site.Data()
			s.Equal(float64(0), *returned.Field.StructuralWalls[0].Openings[0].Door.FramePresentation.FaceFinishes[0].PhaseU)
			*returned.Field.StructuralWalls[0].Openings[0].Door.FramePresentation.FaceFinishes[0].PhaseU = 0.5
			s.Equal(float64(0), *out.Site.Data().Field.StructuralWalls[0].Openings[0].Door.FramePresentation.FaceFinishes[0].PhaseU)
		})
	}
	legacy := s.play(structuralField()).Site()
	data := legacy.Data()
	loaded, err := encounter.NewMemorySites(nil).Site(context.Background(), &encounter.SiteInput{Hash: legacy.Hash(), Data: &data})
	s.Require().NoError(err)
	s.Nil(loaded.Site.Data().Field.StructuralWalls[0].Openings[0].Door.FramePresentation)
}

func (s *StructuralWallSuite) TestFrameCaptureIsDetachedAcrossBothReadersAndReload() {
	field := structuralField()
	frame := framePresentation()
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = frame
	enc := s.play(field)
	frame.Span.Before = 99
	frame.FaceFinishes[0].MaterialRef = "later-input"
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().NotNil(full.StructuralDoors[0].FramePresentation)
	s.Equal(framePresentation(), full.StructuralDoors[0].FramePresentation)
	s.Equal(&encounter.WallFrameSpan{Before: 3, After: 3}, full.StructuralWalls[0].Openings[0].FrameSpan)
	s.Equal(2.0, full.StructuralWalls[0].Openings[0].Width, "frame width never replaces passage width")
	full.StructuralDoors[0].FramePresentation.FaceFinishes[0].PhaseU = 0.9
	full.StructuralDoors[0].FramePresentation.Span.After = 9
	full.StructuralWalls[0].Openings[0].FrameSpan.Before = 9
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(framePresentation(), view.StructuralDoors[0].FramePresentation)
	s.Equal(&encounter.WallFrameSpan{Before: 3, After: 3}, view.StructuralWalls[0].Openings[0].FrameSpan)
	view.StructuralDoors[0].FramePresentation.FaceFinishes[0].MaterialRef = "mutated-reader"
	loaded := s.reload(enc)
	restored, err := loaded.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(framePresentation(), restored.StructuralDoors[0].FramePresentation)
	s.Equal(2.0, restored.StructuralWalls[0].Openings[0].Width)
}

func (s *StructuralWallSuite) TestFrameDisclosureNeedsNoParentAndHiddenDoorLeavesNoCut() {
	field := concealedStructuralField([]encounter.PropID{"wall-presence"}, nil, nil)
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
	view, err := s.play(field).AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(view.StructuralWalls)
	s.Require().Len(view.StructuralDoors, 1)
	s.Equal(framePresentation(), view.StructuralDoors[0].FramePresentation)
	field = concealedStructuralField(nil, []encounter.DoorID{"vault/gate"}, nil)
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
	view, err = s.play(field).AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(view.StructuralDoors)
	s.Require().Len(view.StructuralWalls, 1)
	s.Empty(view.StructuralWalls[0].Openings)
}

func (s *StructuralWallSuite) TestFrameMalformedPresentationRefusesConstruction() {
	for name, change := range map[string]func(*encounter.DoorFramePresentation){
		"missing span":          func(f *encounter.DoorFramePresentation) { f.Span = nil },
		"nonpositive extent":    func(f *encounter.DoorFramePresentation) { f.Span.Before = 0 },
		"outside line":          func(f *encounter.DoorFramePresentation) { f.Span.After = 6 },
		"smaller than aperture": func(f *encounter.DoorFramePresentation) { f.Span.Before = 0.5 },
		"unknown side":          func(f *encounter.DoorFramePresentation) { f.FaceFinishes[0].Side = "left" },
		"blank ref":             func(f *encounter.DoorFramePresentation) { f.FaceFinishes[0].MaterialRef = " " },
		"duplicate side":        func(f *encounter.DoorFramePresentation) { f.FaceFinishes[1].Side = f.FaceFinishes[0].Side },
		"unwrapped phase":       func(f *encounter.DoorFramePresentation) { f.FaceFinishes[0].PhaseU = 1 },
		"negative phase":        func(f *encounter.DoorFramePresentation) { f.FaceFinishes[0].PhaseU = -0.1 },
		"NaN phase":             func(f *encounter.DoorFramePresentation) { f.FaceFinishes[0].PhaseU = math.NaN() },
	} {
		s.Run(name, func() {
			field := structuralField()
			f := framePresentation()
			change(f)
			field.StructuralWalls[0].Openings[0].Door.FramePresentation = f
			enc, err := s.build(field)
			s.ErrorIs(err, encounter.ErrNoField)
			s.Nil(enc)
		})
	}
	field := structuralField()
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
	field.StructuralWalls[0].Openings = append(field.StructuralWalls[0].Openings, encounter.StructuralOpeningInput{ID: "other", Position: 8.4, Width: 1})
	enc, err := s.build(field)
	s.ErrorIs(err, encounter.ErrNoField)
	s.Nil(enc)
}
