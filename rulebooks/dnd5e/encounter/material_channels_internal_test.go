// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MaterialChannelsSuite struct{ suite.Suite }

func TestMaterialChannelsValidateAndDetach(t *testing.T) {
	suite.Run(t, new(MaterialChannelsSuite))
}

func materialChannelsFixture() []MaterialChannelAdjustment {
	return []MaterialChannelAdjustment{{TargetID: " masonry ",
		Normal:   &ChannelAdjustment{Mode: ChannelScale, Multiplier: 1},
		Emissive: &ChannelAdjustment{Mode: ChannelOff}}}
}

func (s *MaterialChannelsSuite) TestExactIntentAndDetachedData() {
	original := materialChannelsFixture()
	s.Require().NoError(validateMaterialChannels(original))
	cloned := cloneMaterialChannels(original)
	data := materialChannelsData(original)
	payload, err := json.Marshal(data)
	s.Require().NoError(err)
	s.JSONEq(`[{"target_id":" masonry ","normal":{"mode":"scale","multiplier":1},"emissive":{"mode":"off"}}]`, string(payload))
	original[0].Normal.Multiplier = 2
	original[0].Emissive.Mode = ChannelScale
	s.Equal(float64(1), cloned[0].Normal.Multiplier)
	s.Equal(ChannelOff, cloned[0].Emissive.Mode)
	s.Equal(float64(1), *data[0].Normal.Multiplier)
	var decoded []MaterialChannelAdjustmentData
	s.Require().NoError(json.Unmarshal(payload, &decoded))
	loaded, err := materialChannelsFromData(decoded)
	s.Require().NoError(err)
	*decoded[0].Normal.Multiplier = 4
	s.Equal(cloned, loaded)
	s.Equal(" masonry ", loaded[0].TargetID, "opaque identity is never trimmed")
	s.Nil(data[0].Emissive.Multiplier, "Off carries no stored multiplier")
}

func (s *MaterialChannelsSuite) TestSharedInputPointersStillProduceTwoDetachedChannels() {
	one := float64(1)
	channel := &ChannelAdjustmentData{Mode: ChannelScale, Multiplier: &one}
	loaded, err := materialChannelsFromData([]MaterialChannelAdjustmentData{{TargetID: "body", Normal: channel, Emissive: channel}})
	s.Require().NoError(err)
	s.Require().NotNil(loaded[0].Normal)
	s.Require().NotNil(loaded[0].Emissive)
	loaded[0].Normal.Multiplier = 2
	s.Equal(float64(1), loaded[0].Emissive.Multiplier)
	s.Equal(float64(1), *channel.Multiplier)
}

func (s *MaterialChannelsSuite) TestAbsenceAndOnlyOneChannel() {
	s.Require().NoError(validateMaterialChannels(nil))
	s.Nil(materialChannelsData(nil))
	loaded, err := materialChannelsFromData(nil)
	s.Require().NoError(err)
	s.NotNil(loaded, "a successful value/error pair never returns nil,nil")
	s.Empty(loaded)
	value := []MaterialChannelAdjustment{{TargetID: "body", Emissive: &ChannelAdjustment{Mode: ChannelScale, Multiplier: .5}}}
	payload, err := json.Marshal(materialChannelsData(value))
	s.Require().NoError(err)
	s.JSONEq(`[{"target_id":"body","emissive":{"mode":"scale","multiplier":0.5}}]`, string(payload))
}

func (s *MaterialChannelsSuite) TestRevealRowsDetachChannelsAndOpeningOnlyPatchesPreserveThem() {
	wall := AtlasStructuralWall{ID: "wall", MaterialChannels: materialChannelsFixture()}
	rows, _, _ := structuralRevealPayload(Atlas{}, Atlas{StructuralWalls: []AtlasStructuralWall{wall}})
	s.Require().Len(rows, 1)
	captured := rows[0]["material_channels"].([]MaterialChannelAdjustmentData)
	wall.MaterialChannels[0].Normal.Multiplier = 5
	s.Equal(float64(1), *captured[0].Normal.Multiplier)
	after := wall
	after.Openings = []AtlasStructuralOpening{{ID: "gap", Position: 3, Width: 1}}
	full, _, replacements := structuralRevealPayload(Atlas{StructuralWalls: []AtlasStructuralWall{wall}}, Atlas{StructuralWalls: []AtlasStructuralWall{after}})
	s.Empty(full)
	s.Require().Len(replacements, 1)
	s.NotContains(replacements[0], "material_channels")
	s.NotContains(structuralWallRow(AtlasStructuralWall{ID: "legacy"}), "material_channels")
}

func (s *MaterialChannelsSuite) TestInvalidRuntimeIntent() {
	cases := [][]MaterialChannelAdjustment{
		{{TargetID: " ", Normal: &ChannelAdjustment{Mode: ChannelOff}}},
		{{TargetID: "body"}},
		{{TargetID: "body", Normal: &ChannelAdjustment{Mode: "unknown"}}},
		{{TargetID: "body", Normal: &ChannelAdjustment{Mode: ChannelOff, Multiplier: 2}}},
		{{TargetID: "body", Normal: &ChannelAdjustment{Mode: ChannelScale}}},
		{{TargetID: "body", Normal: &ChannelAdjustment{Mode: ChannelScale, Multiplier: -1}}},
		{{TargetID: "body", Emissive: &ChannelAdjustment{Mode: ChannelScale, Multiplier: math.Inf(1)}}},
		{{TargetID: "body", Normal: &ChannelAdjustment{Mode: ChannelScale, Multiplier: math.NaN()}}},
		{{TargetID: "body", Normal: &ChannelAdjustment{Mode: ChannelOff}}, {TargetID: "body", Emissive: &ChannelAdjustment{Mode: ChannelOff}}},
	}
	for _, value := range cases {
		s.ErrorIs(validateMaterialChannels(value), ErrNoField)
	}
}

func (s *MaterialChannelsSuite) TestPersistedPresenceCannotTurnInvalidDataIntoDefault() {
	for _, text := range []string{
		`[{"target_id":"body","normal":null,"emissive":{"mode":"off"}}]`,
		`[{"target_id":"body","normal":{}}]`,
		`[{"target_id":"body","normal":{"mode":"off","multiplier":null}}]`,
		`[{"target_id":"body","normal":{"mode":"off","multiplier":0}}]`,
		`[{"target_id":"body","normal":{"mode":"scale"}}]`,
		`[{"target_id":"body","normal":{"mode":"scale","multiplier":null}}]`,
		`[{"target_id":"body","normal":{"mode":"scale","multiplier":0}}]`,
		`[{"target_id":"body","normal":{"mode":"scale","multiplier":"1"}}]`,
		`[{"target_id":"body"}]`, `[null]`,
		`[{"target_id":"body","normal":{"mode":"off","source_path":"private.glb"}}]`,
		`[{"target_id":"body","normal":{"mode":"off"},"parent_wall":"hidden"}]`,
		`[{"target_id":"body","normal":{"mode":"off","mode":"scale","multiplier":1}}]`,
		`[{"target_id":"body","normal":{"mode":"scale","multiplier":1,"multiplier":2}}]`,
		`[{"target_id":"body","target_id":"other","normal":{"mode":"off"}}]`,
		`[{"target_id":"body","normal":{"mode":"off","Mode":"scale","multiplier":1}}]`,
	} {
		var data []MaterialChannelAdjustmentData
		err := json.Unmarshal([]byte(text), &data)
		if err == nil {
			_, err = materialChannelsFromData(data)
		}
		s.Error(err, text)
	}
	zero := float64(0)
	_, err := materialChannelsFromData([]MaterialChannelAdjustmentData{{TargetID: "body",
		Normal: &ChannelAdjustmentData{Mode: ChannelOff, Multiplier: &zero}}})
	s.Error(err)
}
