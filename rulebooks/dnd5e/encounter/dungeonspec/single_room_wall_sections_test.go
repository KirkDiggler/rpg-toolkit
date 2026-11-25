// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"bytes"
	"fmt"

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

func (s *SingleRoomWallSuite) TestTouchingNativeSectionsSurviveCanonicalConstruction() {
	// The same contact can gain a few ULPs during authored arithmetic or the
	// native-to-canonical conversion. Neither boundary may invent an overlap.
	for _, scale := range []float64{1, scenePerFoot()} {
		s.Run(fmt.Sprintf("scale-%g", scale), func() {
			spec := s.baseSpec()
			wall := &spec.Room.Gameplay.Walls[0]
			wall.Line.End.X = 12 * scale
			wall.Appearance.Sections = []dungeonspec.RoomWallSection{
				{ID: "left", AssetRef: "opaque:left", Position: 3 * scale, Width: 6 * scale},
				{ID: "right", AssetRef: "opaque:right", Position: 9 * scale, Width: 6 * scale},
			}
			compiled, err := dungeonspec.Load(s.encoded(spec))
			s.Require().NoError(err)
			sections := compiled.Field.StructuralWalls[0].Sections
			if scale == 1 {
				s.Less(sections[1].Position-sections[1].Width/2, sections[0].Position+sections[0].Width/2,
					"the canonical contact must exercise arithmetic roundoff")
			}
			enc := s.wallEncounter(compiled)
			atlas, err := enc.Atlas()
			s.Require().NoError(err)
			s.Equal(sections, atlas.StructuralWalls[0].Sections, "validation must not rewrite captured lengths")
			// This is a real overlap, many orders above conversion roundoff.
			wall.Appearance.Sections[1].Position -= 1e-8 * scale
			_, err = dungeonspec.Load(s.encoded(spec))
			s.Require().ErrorContains(err, "overlaps")
		})
	}
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
