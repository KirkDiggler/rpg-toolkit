// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"bytes"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

func (s *SingleRoomWallSuite) TestSectionsDecodeLowerOnceAndDoNotChangeMechanicalContributors() {
	spec := s.baseSpec()
	plain, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)
	spec.Room.Gameplay.Walls[0].Appearance.Sections = []dungeonspec.RoomWallSection{{ID: "window", AssetRef: "opaque:window", Position: 8, Width: 3.75}}
	raw := s.encoded(spec)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Require().Equal(spec.Room.Gameplay.Walls[0].Appearance.Sections, decoded.Spec.Room.Gameplay.Walls[0].Appearance.Sections)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	s.Equal(plain.Field.Placed, compiled.Field.Placed)
	s.Equal(plain.Field.Doors, compiled.Field.Doors)
	s.Equal(plain.Field.Segments, compiled.Field.Segments)
	s.Require().Len(compiled.Field.StructuralWalls, 1)
	s.Require().Len(compiled.Field.StructuralWalls[0].Sections, 1)
	section := compiled.Field.StructuralWalls[0].Sections[0]
	s.Equal("window", section.ID)
	s.Equal("opaque:window", section.Ref)
	s.InDelta(8*scenePerFoot(), section.Position, 1e-10)
	s.InDelta(3.75*scenePerFoot(), section.Width, 1e-10)
	s.Empty(compiled.Field.StructuralWalls[0].Openings)
	spec.Room.Gameplay.Walls[0].Appearance.Sections[0].AssetRef = "later:edit"
	s.Equal("opaque:window", compiled.Field.StructuralWalls[0].Sections[0].Ref)
}

func (s *SingleRoomWallSuite) TestNativeSectionInvalidShapesAndIntervalsRefuse() {
	spec := s.baseSpec()
	spec.Room.Gameplay.Walls[0].Appearance.Sections = []dungeonspec.RoomWallSection{{ID: "window", AssetRef: "opaque:window", Position: 8, Width: 2}}
	raw := s.encoded(spec)
	for _, tc := range []struct {
		name     string
		old, new []byte
	}{
		{"null-position", []byte("position: 8"), []byte("position: null")},
		{"missing-width", []byte("width: 2"), []byte("ignoredWidth: 2")},
		{"zero-width", []byte("width: 2"), []byte("width: 0")},
		{"blank-ref", []byte("assetRef: opaque:window"), []byte("assetRef: ''")},
		{"outside", []byte("position: 8"), []byte("position: 25")},
	} {
		s.Run(tc.name, func() {
			s.Require().Contains(string(raw), string(tc.old))
			_, err := dungeonspec.Load(bytes.Replace(raw, tc.old, tc.new, 1))
			s.Error(err)
		})
	}
	spec.Room.Gameplay.Walls[0].Appearance.Sections = append(spec.Room.Gameplay.Walls[0].Appearance.Sections, dungeonspec.RoomWallSection{ID: "window", AssetRef: "opaque:other", Position: 12, Width: 2})
	_, err := dungeonspec.Load(s.encoded(spec))
	s.Error(err)
}
