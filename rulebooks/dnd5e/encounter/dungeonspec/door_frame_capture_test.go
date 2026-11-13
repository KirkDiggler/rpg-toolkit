// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const nativeFrame = "{span: {before: 3, after: 3}, faceFinishes: [{side: positive-z, materialRef: 'material:gray', phaseU: 0}]}"
const nativeFramePath = "room.room.walls[0].openings[0].door.framePresentation"

func (s *SingleRoomWallDoorSuite) framedSource(frame string) []byte {
	raw, err := os.ReadFile(wallDoorsFixture)
	s.Require().NoError(err)
	old := "door: {id: long-wall-door, assetRef: 'dnd5e:env:test:door'}"
	s.Require().Contains(string(raw), old)
	return []byte(strings.Replace(string(raw), old, "door: {id: long-wall-door, assetRef: 'dnd5e:env:test:door', framePresentation: "+frame+"}", 1))
}

func (s *SingleRoomWallDoorSuite) TestFrameNativeCaptureConvertsOnlyLengthsAndNeverChangesMechanics() {
	raw := s.framedSource(nativeFrame)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	rewritten := s.encoded(decoded.Spec)
	s.Contains(string(rewritten), "phaseU: 0", "explicit zero is not omitted")
	compiled, err := dungeonspec.Load(rewritten)
	s.Require().NoError(err)
	frame := compiled.Field.StructuralWalls[0].Openings[0].Door.FramePresentation
	s.Require().NotNil(frame)
	s.InDelta(3*scenePerFoot(), frame.Span.Before, 1e-9)
	s.InDelta(3*scenePerFoot(), frame.Span.After, 1e-9)
	s.Require().Len(frame.FaceFinishes, 1)
	s.Equal(float64(0), frame.FaceFinishes[0].PhaseU)
	s.Equal("material:gray", frame.FaceFinishes[0].MaterialRef)
	original := s.load(s.baseSpec())
	s.Equal(original.Field.Placed, compiled.Field.Placed, "visual surround does not carve the mechanical blocker")
	s.Equal(original.Field.Doors, compiled.Field.Doors, "state and passage footprint remain unchanged")
	s.Equal(original.Field.StructuralWalls[0].Openings[0].Width, compiled.Field.StructuralWalls[0].Openings[0].Width)
}

func (s *SingleRoomWallDoorSuite) TestFrameNativeMalformedFactsAreNamedAtTheirSourcePath() {
	cases := []struct{ frame, path string }{
		{"null", nativeFramePath},
		{"{}", nativeFramePath + ".span"},
		{strings.Replace(nativeFrame, "before: 3", "before: .nan", 1), nativeFramePath + ".span.before"},
		{strings.Replace(nativeFrame, ", phaseU: 0", "", 1), nativeFramePath + ".faceFinishes[0].phaseU"},
		{strings.Replace(nativeFrame, "phaseU: 0", "phaseU: 1", 1), nativeFramePath + ".faceFinishes[0].phaseU"},
		{strings.Replace(nativeFrame, "positive-z", "left", 1), nativeFramePath + ".faceFinishes[0].side"},
		{strings.Replace(nativeFrame, "'material:gray'", "' '", 1), nativeFramePath + ".faceFinishes[0].materialRef"},
		{strings.Replace(nativeFrame, "before: 3", "before: 0.5", 1), nativeFramePath + ".span"},
		{strings.Replace(nativeFrame, "before: 3", "before: 11", 1), nativeFramePath + ".span"},
	}
	for _, tc := range cases {
		s.Run(tc.frame, func() { s.requireDefect(s.defects(s.framedSource(tc.frame)), tc.path, "") })
	}
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.framedSource(nativeFrame)})
	s.Require().NoError(err)
	decoded.Spec.Room.Gameplay.Walls[0].Openings = append(decoded.Spec.Room.Gameplay.Walls[0].Openings, dungeonspec.RoomWallOpening{ID: "another-gap", Position: 13.25, Width: 1})
	s.requireDefect(s.defects(s.encoded(decoded.Spec)), "room.room.walls[0].openings[1]", "overlap")
}
