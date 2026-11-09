// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *PropPresentationDecodeSuite) TestMaterialOverridesProjectWithoutAliasing() {
	values := []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "look:04_C"}}
	wall := projectStructuralWall(encounter.AtlasStructuralWall{ID: "wall", MaterialOverrides: values})
	prop := projectPropPresentation(encounter.PropPresentation{ID: "prop", MaterialOverrides: values})
	want := []MaterialOverride{{SurfaceID: "atlas", MaterialRef: "look:04_C"}}
	s.Equal(want, wall.MaterialOverrides)
	s.Equal(want, prop.MaterialOverrides)
	wall.MaterialOverrides[0].MaterialRef = "changed-wall"
	prop.MaterialOverrides[0].MaterialRef = "changed-prop"
	s.Equal("look:04_C", values[0].MaterialRef)
}

func (s *PropPresentationDecodeSuite) TestMaterialOverridesMatchSnapshotAndPayload() {
	selected := `[{"surface_id":"atlas","material_ref":"look:04_C"}]`
	walls, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"wall","material_overrides":%s}]}`, selected)))
	s.Require().True(ok)
	s.Require().Len(walls.Walls, 1)
	props, ok := propPresentationsFromPayload([]byte(fmt.Sprintf(`{"prop_presentations":[{"id":"prop","ref":"model","origin":{"x":0,"y":0},"height_scale":1,"material_overrides":%s}]}`, selected)))
	s.Require().True(ok)
	s.Require().Len(props, 1)
	want := projectPropPresentation(encounter.PropPresentation{MaterialOverrides: []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "look:04_C"}}}).MaterialOverrides
	s.Equal(want, walls.Walls[0].MaterialOverrides)
	s.Equal(want, props[0].MaterialOverrides)
}

func (s *PropPresentationDecodeSuite) TestInvalidMaterialOverridesRefuseWholePayload() {
	for _, bad := range []string{
		`"not a list"`, `[null]`, `[{"surface_id":"","material_ref":"look"}]`,
		`[{"surface_id":"atlas","material_ref":" "}]`,
		`[{"surface_id":"atlas","material_ref":"one"},{"surface_id":"atlas","material_ref":"two"}]`,
		`[{"surface_id":3,"material_ref":"look"}]`,
	} {
		walls, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"wall","material_overrides":%s}],"structural_doors":[{"id":"door"}]}`, bad)))
		s.False(ok, bad)
		s.Empty(walls.Walls)
		s.Empty(walls.Doors)
		props, ok := propPresentationsFromPayload([]byte(fmt.Sprintf(`{"prop_presentations":[{"id":"valid","ref":"model","origin":{"x":0,"y":0},"height_scale":1},{"id":"prop","ref":"model","origin":{"x":0,"y":0},"height_scale":1,"material_overrides":%s}]}`, bad)))
		s.False(ok, bad)
		s.Empty(props)
	}
}
