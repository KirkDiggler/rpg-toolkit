// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *PropPresentationDecodeSuite) TestDoorFrameSnapshotAndRevealCarryDetachedZeroPhase() {
	source := &encounter.DoorFramePresentation{Span: &encounter.WallFrameSpan{Before: 3, After: 3}, FaceFinishes: []encounter.DoorFrameFaceFinish{{Side: encounter.WallFacePositiveZ, MaterialRef: "look:gray", PhaseU: 0}}}
	projected := projectStructuralDoor(encounter.AtlasStructuralDoor{ID: "door", From: spatial.Point{}, To: spatial.Point{X: 2}, FramePresentation: source})
	raw, err := json.Marshal(projected)
	s.Require().NoError(err)
	s.Require().Contains(string(raw), `"frame_presentation"`)
	rows, ok := structuralRowsFromPayload([]byte(`{"structural_doors":[` + string(raw) + `],"structural_wall_openings_replacements":[{"wall_id":"wall","openings":[{"id":"gap","width":2,"frame_span":{"before":3,"after":3}}]}]}`))
	s.Require().True(ok)
	s.Require().Len(rows.Doors, 1)
	s.Equal(projected, rows.Doors[0])
	s.Require().Len(rows.Replacements, 1)
	s.Require().NotNil(projected.FramePresentation.FaceFinishes[0].PhaseU)
	s.Equal(float64(0), *projected.FramePresentation.FaceFinishes[0].PhaseU)
	wall := projectStructuralWall(encounter.AtlasStructuralWall{ID: "wall", Openings: []encounter.AtlasStructuralOpening{{ID: "gap", Width: 2, FrameSpan: source.Span}}})
	wall.Openings[0].FrameSpan.Before = 88
	s.Equal(float64(3), source.Span.Before)
	source.Span.Before = 99
	source.FaceFinishes[0].PhaseU = 0.9
	again, err := json.Marshal(projected)
	s.Require().NoError(err)
	s.Equal(raw, again, "snapshot must not alias provider facts")
	projected.FramePresentation.Span.After = 77
	*projected.FramePresentation.FaceFinishes[0].PhaseU = 0.5
	s.Equal(float64(3), source.Span.After)
	s.Equal(0.9, source.FaceFinishes[0].PhaseU)
	legacy, ok := structuralRowsFromPayload([]byte(`{"structural_doors":[{"id":"legacy"}]}`))
	s.Require().True(ok)
	s.Nil(legacy.Doors[0].FramePresentation)
}

func (s *PropPresentationDecodeSuite) TestMalformedFrameRefusesAllSiblingRevealRows() {
	for _, bad := range []string{
		`{}`, `{"span":{}}`,
		`{"span":{"before":-1,"after":3}}`,
		`{"span":{"before":0.5,"after":3}}`,
		`{"span":{"before":3,"after":3},"face_finishes":[{"side":"positive-z","material_ref":"look"}]}`,
		`{"span":{"before":3,"after":3},"face_finishes":[{"side":"positive-z","material_ref":"look","phase_u":1}]}`,
		`{"span":{"before":3,"after":3},"face_finishes":[{"side":"front","material_ref":"look","phase_u":0}]}`,
		`{"span":{"before":3,"after":3},"face_finishes":[{"side":"positive-z","material_ref":" ","phase_u":0}]}`,
		`{"span":{"before":3,"after":3},"face_finishes":[{"side":"positive-z","material_ref":"look","phase_u":0},{"side":"positive-z","material_ref":"look","phase_u":0.5}]}`,
	} {
		rows, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"valid"}],"structural_doors":[{"id":"door","from":{"x":0,"y":0},"to":{"x":2,"y":0},"frame_presentation":%s}],"structural_wall_openings_replacements":[{"wall_id":"other","openings":[]}]}`, bad)))
		s.False(ok, bad)
		s.Empty(rows.Walls)
		s.Empty(rows.Doors)
		s.Empty(rows.Replacements)
	}
	rows, ok := structuralRowsFromPayload([]byte(`{"structural_doors":[{"id":"legacy"}],"structural_wall_openings_replacements":[{"wall_id":"wall","openings":[{"id":"gap","width":2,"frame_span":{"before":0,"after":2}}]}]}`))
	s.False(ok)
	s.Empty(rows.Doors)
	s.Empty(rows.Replacements)
}
