// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type StructuralPatchSuite struct{ suite.Suite }

func TestStructuralPatchSuite(t *testing.T) { suite.Run(t, new(StructuralPatchSuite)) }

func (s *StructuralPatchSuite) TestNewWallIsIntroducedWithoutReplacement() {
	wall := AtlasStructuralWall{ID: "wall", Openings: []AtlasStructuralOpening{{ID: "one", Position: 4, Width: 2}}}
	walls, doors, patches := structuralRevealPayload(Atlas{}, Atlas{StructuralWalls: []AtlasStructuralWall{wall}})
	s.Require().Len(walls, 1)
	s.Equal(PropID("wall"), walls[0]["id"])
	s.Empty(doors)
	s.Empty(patches, "an introduction and replacement must not collide")
}

func (s *StructuralPatchSuite) TestKnownWallCarriesTheCompleteReplacementOnly() {
	one := AtlasStructuralOpening{ID: "one", Position: 4, Width: 2}
	two := AtlasStructuralOpening{ID: "two", Position: 8, Width: 2}
	before := Atlas{StructuralWalls: []AtlasStructuralWall{{ID: "wall", Openings: []AtlasStructuralOpening{one}}}}
	after := Atlas{StructuralWalls: []AtlasStructuralWall{{ID: "wall", Openings: []AtlasStructuralOpening{one, two}}}}
	walls, doors, patches := structuralRevealPayload(before, after)
	s.Empty(walls)
	s.Empty(doors)
	s.Require().Len(patches, 1)
	s.Require().Len(patches[0], 2, "only identity and changed component are present")
	s.Equal(PropID("wall"), patches[0]["wall_id"])
	s.Equal([]map[string]interface{}{
		{"id": "one", "position": float64(4), "width": float64(2)},
		{"id": "two", "position": float64(8), "width": float64(2)},
	}, patches[0]["openings"], "replacement retains prior permitted cuts")

	// Captured payload must not alias a later projection.
	after.StructuralWalls[0].Openings[1].ID = "future-secret"
	s.Equal("two", patches[0]["openings"].([]map[string]interface{})[1]["id"])
}

func (s *StructuralPatchSuite) TestPresentEmptyReplacementClearsButAbsentIsNoOp() {
	before := Atlas{StructuralWalls: []AtlasStructuralWall{{ID: "wall", Openings: []AtlasStructuralOpening{{ID: "one", Position: 4, Width: 2}}}}}
	after := Atlas{StructuralWalls: []AtlasStructuralWall{{ID: "wall"}}}
	walls, _, patches := structuralRevealPayload(before, after)
	s.Empty(walls)
	s.Require().Len(patches, 1)
	openings, ok := patches[0]["openings"].([]map[string]interface{})
	s.Require().True(ok)
	s.NotNil(openings, "stored clear is an explicit empty array")
	s.Empty(openings)

	payload := map[string]interface{}{}
	addStructuralReveal(payload, after, after)
	s.Empty(payload, "unchanged layout does not emit even an empty patch collection")
}

func (s *StructuralPatchSuite) TestFrameRowsAreDetachedAndEqualCopiesDoNotRepeatOnReveal() {
	frame := &DoorFramePresentation{Span: &WallFrameSpan{Before: 3, After: 3}, FaceFinishes: []DoorFrameFaceFinish{{Side: WallFacePositiveZ, MaterialRef: "material:gray", PhaseU: 0}}}
	door := AtlasStructuralDoor{ID: "site/door", FramePresentation: frame}
	before := Atlas{StructuralDoors: []AtlasStructuralDoor{door}}
	copied := door
	copied.FramePresentation = cloneDoorFramePresentation(frame)
	after := Atlas{StructuralDoors: []AtlasStructuralDoor{copied}, StructuralWalls: []AtlasStructuralWall{{ID: "new-parent", Openings: []AtlasStructuralOpening{{ID: "gap", Position: 5, Width: 2, FrameSpan: cloneWallFrameSpan(frame.Span)}}}}}
	_, doors, _ := structuralRevealPayload(before, after)
	s.Empty(doors, "parent-later reveal must not repeat an equal independently known door")
	walls, doors, _ := structuralRevealPayload(Atlas{}, after)
	s.Require().Len(doors, 1)
	s.NotContains(doors[0], "parent")
	captured := doors[0]["frame_presentation"].(map[string]interface{})
	s.Equal(float64(0), captured["face_finishes"].([]map[string]interface{})[0]["phase_u"], "zero phase is explicit")
	cuts := walls[0]["openings"].([]map[string]interface{})
	s.NotContains(cuts[0], "door_id")
	s.Equal(map[string]interface{}{"before": float64(3), "after": float64(3)}, cuts[0]["frame_span"])
	after.StructuralDoors[0].FramePresentation.FaceFinishes[0].PhaseU = 0.9
	after.StructuralWalls[0].Openings[0].FrameSpan.Before = 4
	s.Equal(float64(0), captured["face_finishes"].([]map[string]interface{})[0]["phase_u"])
	s.Equal(float64(3), cuts[0]["frame_span"].(map[string]interface{})["before"])
	first := Atlas{StructuralWalls: []AtlasStructuralWall{{ID: "wall", Openings: []AtlasStructuralOpening{{ID: "gap", Position: 5, Width: 2, FrameSpan: &WallFrameSpan{Before: 3, After: 3}}}}}}
	second := Atlas{StructuralWalls: []AtlasStructuralWall{{ID: "wall", Openings: []AtlasStructuralOpening{{ID: "gap", Position: 5, Width: 2, FrameSpan: &WallFrameSpan{Before: 3, After: 3}}}}}}
	_, _, patches := structuralRevealPayload(first, second)
	s.Empty(patches, "equal span values do not depend on pointer identity")
	second.StructuralWalls[0].Openings[0].FrameSpan.After = 3.5
	_, _, patches = structuralRevealPayload(first, second)
	s.Require().Len(patches, 1)
	s.Equal(float64(3.5), patches[0]["openings"].([]map[string]interface{})[0]["frame_span"].(map[string]interface{})["after"])
}

func (s *StructuralPatchSuite) TestIndependentDoorDoesNotNameAWithheldWall() {
	_, doors, patches := structuralRevealPayload(Atlas{}, Atlas{
		StructuralDoors: []AtlasStructuralDoor{{ID: "site/door"}},
	})
	s.Require().Len(doors, 1)
	s.Empty(patches)
	s.NotContains(doors[0], "wall_id")
	s.NotContains(doors[0], "parent")
}
