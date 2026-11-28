// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func sessionChannelSource() []encounter.MaterialChannelAdjustment {
	return []encounter.MaterialChannelAdjustment{{TargetID: "body",
		Normal:   &encounter.ChannelAdjustment{Mode: encounter.ChannelScale, Multiplier: 1},
		Emissive: &encounter.ChannelAdjustment{Mode: encounter.ChannelOff}}}
}

func (s *PropPresentationDecodeSuite) TestMaterialChannelsProjectWithoutAliasing() {
	values := sessionChannelSource()
	wall := projectStructuralWall(encounter.AtlasStructuralWall{ID: "wall", MaterialChannels: values})
	prop := projectPropPresentation(encounter.PropPresentation{ID: "prop", MaterialChannels: values})
	frame := projectDoorFramePresentation(&encounter.DoorFramePresentation{Span: &encounter.WallFrameSpan{Before: 3, After: 3}, MaterialChannels: values})
	want := projectMaterialChannels(sessionChannelSource())
	s.Equal(want, wall.MaterialChannels)
	s.Equal(want, prop.MaterialChannels)
	s.Equal(want, frame.MaterialChannels)
	*wall.MaterialChannels[0].Normal.Multiplier = 2
	*prop.MaterialChannels[0].Normal.Multiplier = 3
	*frame.MaterialChannels[0].Normal.Multiplier = 4
	s.Equal(float64(1), values[0].Normal.Multiplier)
	s.Nil(want[0].Emissive.Multiplier)
	values[0].Emissive.Mode = encounter.ChannelScale
	s.Equal(ChannelOff, wall.MaterialChannels[0].Emissive.Mode)
}

func (s *PropPresentationDecodeSuite) TestMaterialChannelsMatchSnapshotAndLivePayloads() {
	selected := `[{"target_id":"body","normal":{"mode":"scale","multiplier":1},"emissive":{"mode":"off"}}]`
	walls, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"wall","material_channels":%s}]}`, selected)))
	s.Require().True(ok)
	props, ok := propPresentationsFromPayload([]byte(fmt.Sprintf(`{"prop_presentations":[{"id":"prop","ref":"model","origin":{"x":0,"y":0},"height_scale":1,"material_channels":%s}]}`, selected)))
	s.Require().True(ok)
	s.Equal(projectMaterialChannels(sessionChannelSource()), walls.Walls[0].MaterialChannels)
	s.Equal(walls.Walls[0].MaterialChannels, props[0].MaterialChannels)
	frames, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_doors":[{"id":"door","from":{"x":0,"y":0},"to":{"x":2,"y":0},"frame_presentation":{"span":{"before":3,"after":3},"material_channels":%s}}]}`, selected)))
	s.Require().True(ok)
	s.Require().Len(frames.Doors, 1)
	s.Equal(walls.Walls[0].MaterialChannels, frames.Doors[0].FramePresentation.MaterialChannels)
}

func (s *PropPresentationDecodeSuite) TestInvalidMaterialChannelsRefuseWholePayload() {
	for _, bad := range []string{
		`"not-list"`, `[null]`, `[{"target_id":"body"}]`,
		`[{"target_id":"body","normal":null,"emissive":{"mode":"off"}}]`,
		`[{"target_id":"body","normal":{"mode":"off","multiplier":0}}]`,
		`[{"target_id":"body","normal":{"mode":"scale","multiplier":null}}]`,
		`[{"target_id":"body","normal":{"mode":"scale","multiplier":0}}]`,
		`[{"target_id":"body","normal":{"mode":"off","Mode":"scale","multiplier":1}}]`,
		`[{"target_id":"body","normal":{"mode":"off"}},{"target_id":"body","emissive":{"mode":"off"}}]`,
	} {
		walls, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"wall","material_channels":%s}],"structural_doors":[{"id":"door"}]}`, bad)))
		s.False(ok, bad)
		s.Empty(walls.Walls)
		props, ok := propPresentationsFromPayload([]byte(fmt.Sprintf(`{"prop_presentations":[{"id":"valid","ref":"model","origin":{"x":0,"y":0},"height_scale":1},{"id":"bad","ref":"model","origin":{"x":0,"y":0},"height_scale":1,"material_channels":%s}]}`, bad)))
		s.False(ok, bad)
		s.Empty(props)
		frames, ok := structuralRowsFromPayload([]byte(fmt.Sprintf(`{"structural_walls":[{"id":"valid"}],"structural_doors":[{"id":"door","from":{"x":0,"y":0},"to":{"x":2,"y":0},"frame_presentation":{"span":{"before":3,"after":3},"material_channels":%s}}]}`, bad)))
		s.False(ok, bad)
		s.Empty(frames.Walls)
		s.Empty(frames.Doors)
	}
}
