// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
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
