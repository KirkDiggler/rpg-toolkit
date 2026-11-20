// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *PropPresentationDecodeSuite) TestWallSectionsMatchSnapshotAndRecordedReveal() {
	source := []encounter.WallSection{{ID: "window", Ref: "opaque:window", Position: 8, Width: 2}}
	projected := projectStructuralWall(encounter.AtlasStructuralWall{ID: "wall", Sections: source})
	want := []WallSection{{ID: "window", Ref: "opaque:window", Position: 8, Width: 2}}
	s.Require().Equal(want, projected.Sections)
	rows, ok := structuralRowsFromPayload([]byte(`{"structural_walls":[{"id":"wall","sections":[{"id":"window","ref":"opaque:window","position":8,"width":2}]}]}`))
	s.Require().True(ok)
	s.Require().Len(rows.Walls, 1)
	s.Equal(want, rows.Walls[0].Sections)
	projected.Sections[0].Ref = "caller:mutation"
	s.Equal("opaque:window", source[0].Ref)
	source[0].Ref = "source:mutation"
	s.Equal(want, rows.Walls[0].Sections)
	legacy, ok := structuralRowsFromPayload([]byte(`{"structural_walls":[{"id":"legacy"}],"structural_wall_openings_replacements":[{"wall_id":"known","openings":[]}]}`))
	s.Require().True(ok)
	s.Nil(legacy.Walls[0].Sections)
}

func (s *PropPresentationDecodeSuite) TestSectionCentreZeroIsExplicitNotAnAbsentNumericFact() {
	rows, ok := structuralRowsFromPayload([]byte(`{"structural_walls":[{"id":"wall","sections":[{"id":"x","ref":"look","position":0,"width":1e-15}]}]}`))
	s.Require().True(ok)
	s.Equal(float64(0), rows.Walls[0].Sections[0].Position)
}

func (s *PropPresentationDecodeSuite) TestMalformedWallSectionsRefuseTheWholeReveal() {
	for _, bad := range []string{
		`"wrong-kind"`, `[null]`, `[{}]`,
		`[{"id":"x","ref":" ","position":8,"width":2}]`,
		`[{"id":"x","ref":"look","width":2}]`,
		`[{"id":"x","ref":"look","position":null,"width":2}]`,
		`[{"id":"x","ref":"look","position":8}]`,
		`[{"id":"x","ref":"look","position":8,"width":-1}]`,
		`[{"id":"x","ref":"look","position":"8","width":2}]`,
		`[{"id":"x","ref":"look","position":8,"width":2},{"id":"x","ref":"look","position":10,"width":2}]`,
	} {
		payload := fmt.Sprintf(`{"structural_walls":[{"id":"valid"},{"id":"bad","sections":%s}],"structural_doors":[{"id":"door"}],"structural_wall_openings_replacements":[{"wall_id":"other","openings":[]}]}`, bad)
		rows, ok := structuralRowsFromPayload([]byte(payload))
		s.False(ok, bad)
		s.Empty(rows.Walls)
		s.Empty(rows.Doors)
		s.Empty(rows.Replacements)
	}
}
