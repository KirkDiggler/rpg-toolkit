// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *PropPresentationDecodeSuite) TestWallFaceFinishesMatchSnapshotAndReveal() {
	values := []encounter.WallFaceFinish{
		{Side: encounter.WallFacePositiveZ, MaterialRef: "look:brick"},
		{Side: encounter.WallFaceNegativeZ, MaterialRef: "look:stucco"},
	}
	projected := projectStructuralWall(encounter.AtlasStructuralWall{ID: "wall", FaceFinishes: values})
	want := []WallFaceFinish{{Side: "positive-z", MaterialRef: "look:brick"}, {Side: "negative-z", MaterialRef: "look:stucco"}}
	s.Require().Equal(want, projected.FaceFinishes)
	rows, ok := structuralRowsFromPayload([]byte(`{"structural_walls":[{"id":"wall","face_finishes":[{"side":"positive-z","material_ref":"look:brick"},{"side":"negative-z","material_ref":"look:stucco"}]}]}`))
	s.Require().True(ok)
	s.Require().Len(rows.Walls, 1)
	s.Equal(want, rows.Walls[0].FaceFinishes)
	projected.FaceFinishes[0].MaterialRef = "caller:mutation"
	s.Equal("look:brick", values[0].MaterialRef)
	values[1].MaterialRef = "source:mutation"
	s.Equal("look:stucco", projected.FaceFinishes[1].MaterialRef)
}

func (s *PropPresentationDecodeSuite) TestMalformedWallFaceFinishesRefuseWholeReveal() {
	for _, bad := range []string{
		`"not-a-list"`, `[null]`, `[{}]`,
		`[{"side":"front","material_ref":"look"}]`,
		`[{"side":"positive-z","material_ref":" "}]`,
		`[{"side":3,"material_ref":"look"}]`,
		`[{"side":"negative-z","material_ref":"one"},{"side":"negative-z","material_ref":"two"}]`,
	} {
		rows, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"valid"},{"id":"bad","face_finishes":%s}],"structural_doors":[{"id":"door"}],"structural_wall_openings_replacements":[{"wall_id":"another-wall","openings":[]}]}`, bad)))
		s.False(ok, bad)
		s.Empty(rows.Walls)
		s.Empty(rows.Doors)
		s.Empty(rows.Replacements)
	}
}

func (s *PropPresentationDecodeSuite) TestLegacyWallRevealHasNoInventedFinishes() {
	rows, ok := structuralRowsFromPayload([]byte(`{"structural_walls":[{"id":"legacy"}],"structural_wall_openings_replacements":[{"wall_id":"known","openings":[]}]}`))
	s.Require().True(ok)
	s.Require().Len(rows.Walls, 1)
	s.Nil(rows.Walls[0].FaceFinishes)
	s.Require().Len(rows.Replacements, 1)
	s.Empty(rows.Replacements[0].Openings)
}
