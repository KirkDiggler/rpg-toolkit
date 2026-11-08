// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "encoding/json"

func (s *StructuralPatchSuite) TestMaterialOverridesReachCapturedWallRevealRows() {
	wall := AtlasStructuralWall{ID: "wall", MaterialOverrides: []MaterialOverride{{SurfaceID: "atlas", MaterialRef: "look"}}}
	row := structuralWallRow(wall)
	raw, err := json.Marshal(row)
	s.Require().NoError(err)
	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	s.Equal([]interface{}{map[string]interface{}{"surface_id": "atlas", "material_ref": "look"}}, decoded["material_overrides"])
	wall.MaterialOverrides[0].MaterialRef = "later-source-change"
	s.Equal("look", row["material_overrides"].([]MaterialOverrideData)[0].MaterialRef)
	s.NotContains(structuralWallRow(AtlasStructuralWall{ID: "legacy"}), "material_overrides", "legacy wire shape remains unchanged")
}
