// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "encoding/json"

func (s *StructuralPatchSuite) TestWallFaceFinishesReachCapturedWallRevealRows() {
	wall := AtlasStructuralWall{ID: "wall", FaceFinishes: []WallFaceFinish{
		{Side: WallFacePositiveZ, MaterialRef: "look:brick"},
		{Side: WallFaceNegativeZ, MaterialRef: "look:stucco"},
	}}
	walls, _, _ := structuralRevealPayload(Atlas{}, Atlas{StructuralWalls: []AtlasStructuralWall{wall}})
	s.Require().Len(walls, 1)
	raw, err := json.Marshal(walls[0])
	s.Require().NoError(err)
	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	s.Require().Equal([]interface{}{
		map[string]interface{}{"side": "positive-z", "material_ref": "look:brick"},
		map[string]interface{}{"side": "negative-z", "material_ref": "look:stucco"},
	}, decoded["face_finishes"])
	wall.FaceFinishes[0].MaterialRef = "later:source"
	s.Equal("look:brick", walls[0]["face_finishes"].([]WallFaceFinishData)[0].MaterialRef)
	s.NotContains(structuralWallRow(AtlasStructuralWall{ID: "legacy"}), "face_finishes")
}

func (s *StructuralPatchSuite) TestOpeningReplacementRetainsWallAppearance() {
	before := AtlasStructuralWall{ID: "wall", FaceFinishes: []WallFaceFinish{{Side: WallFacePositiveZ, MaterialRef: "look:brick"}}}
	after := before
	after.Openings = []AtlasStructuralOpening{{ID: "gap", Position: 3, Width: 1}}
	walls, doors, replacements := structuralRevealPayload(
		Atlas{StructuralWalls: []AtlasStructuralWall{before}}, Atlas{StructuralWalls: []AtlasStructuralWall{after}})
	s.Empty(walls)
	s.Empty(doors)
	s.Require().Len(replacements, 1)
	s.Equal("wall", string(replacements[0]["wall_id"].(PropID)))
	s.NotContains(replacements[0], "face_finishes", "opening replacement cannot clear captured appearance")
	s.Equal([]WallFaceFinish{{Side: WallFacePositiveZ, MaterialRef: "look:brick"}}, after.FaceFinishes)
}
