// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "encoding/json"

func (s *StructuralPatchSuite) TestSectionsTravelWithCompleteWallRevealNotOpeningReplacement() {
	wall := AtlasStructuralWall{ID: "wall", Sections: []WallSection{{ID: "window", Ref: "opaque:window", Position: 8, Width: 2}}}
	rows, _, _ := structuralRevealPayload(Atlas{}, Atlas{StructuralWalls: []AtlasStructuralWall{wall}})
	s.Require().Len(rows, 1)
	s.Require().Contains(rows[0], "sections")
	raw, err := json.Marshal(rows[0])
	s.Require().NoError(err)
	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	s.Equal([]interface{}{map[string]interface{}{"id": "window", "ref": "opaque:window", "position": float64(8), "width": float64(2)}}, decoded["sections"])
	wall.Sections[0].Ref = "later:mutation"
	s.Equal("opaque:window", rows[0]["sections"].([]WallSectionData)[0].Ref)
	after := wall
	after.Openings = []AtlasStructuralOpening{{ID: "gap", Position: 3, Width: 1}}
	full, doors, replacements := structuralRevealPayload(Atlas{StructuralWalls: []AtlasStructuralWall{wall}}, Atlas{StructuralWalls: []AtlasStructuralWall{after}})
	s.Empty(full)
	s.Empty(doors)
	s.Require().Len(replacements, 1)
	s.NotContains(replacements[0], "sections")
	s.NotContains(structuralWallRow(AtlasStructuralWall{ID: "legacy"}), "sections")
}
