// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"math"
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"gopkg.in/yaml.v3"
)

func (s *SingleRoomWallSuite) TestMaterialChannelsNamedFixtureAndContentIdentity() {
	raw, err := os.ReadFile("testdata/material-channels/site.yaml")
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	s.Require().Len(compiled.Monsters, 1)
	s.Equal(float64(1), compiled.Monsters[0].MaterialChannels[0].Normal.Multiplier)
	s.Equal(encounter.ChannelOff, compiled.Field.PropPresentations[0].MaterialChannels[0].Emissive.Mode)
	s.Equal(.5, compiled.Field.StructuralWalls[0].MaterialChannels[0].Normal.Multiplier)
	first, err := compiled.ContentHash()
	s.Require().NoError(err)
	compiled.Field.StructuralWalls[0].MaterialChannels[0].Normal.Multiplier = 1
	second, err := compiled.ContentHash()
	s.Require().NoError(err)
	s.NotEqual(first, second, "authored strength is content even though it is not a rule")
}

func nativeChannels() []interface{} {
	return []interface{}{
		map[string]interface{}{"targetId": "masonry", "normal": map[string]interface{}{"mode": "scale", "multiplier": 1}, "emissive": map[string]interface{}{"mode": "off"}},
		map[string]interface{}{"targetId": "other", "emissive": map[string]interface{}{"mode": "scale", "multiplier": .5}},
	}
}

func canonicalChannels() []encounter.MaterialChannelAdjustment {
	return []encounter.MaterialChannelAdjustment{
		{TargetID: "masonry", Normal: &encounter.ChannelAdjustment{Mode: encounter.ChannelScale, Multiplier: 1}, Emissive: &encounter.ChannelAdjustment{Mode: encounter.ChannelOff}},
		{TargetID: "other", Emissive: &encounter.ChannelAdjustment{Mode: encounter.ChannelScale, Multiplier: .5}},
	}
}

func channelMonster(doc map[string]interface{}) map[string]interface{} {
	room := doc["room"].(map[string]interface{})["room"].(map[string]interface{})
	return room["monsterDeclarations"].([]interface{})[0].(map[string]interface{})
}

func (s *SingleRoomWallSuite) TestMaterialChannelsNativeOwnersPreserveIntentWithoutUnitConversion() {
	doc, wall, prop := s.materialCastle()
	beforeRaw, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	before, err := dungeonspec.Load(beforeRaw)
	s.Require().NoError(err)
	wall["materialChannels"] = nativeChannels()
	prop["materialChannels"] = nativeChannels()
	channelMonster(doc)["materialChannels"] = nativeChannels()
	raw, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	// Re-encode the new typed value into the unchanged native document. The
	// entire compiled authoring spec is not a lossless serializer of unrelated
	// behavior words (the unmodified castle's temperament also refuses that path).
	channelBytes, err := yaml.Marshal(decoded.Spec.Room.Gameplay.Walls[0].Appearance.MaterialChannels)
	s.Require().NoError(err)
	var roundTrip []interface{}
	s.Require().NoError(yaml.Unmarshal(channelBytes, &roundTrip))
	s.Equal(nativeChannels(), roundTrip)
	wall["materialChannels"] = roundTrip
	roundTripDocument, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(roundTripDocument)
	s.Require().NoError(err)
	s.Equal(canonicalChannels(), compiled.Field.StructuralWalls[0].MaterialChannels)
	s.Equal(canonicalChannels(), compiled.Monsters[0].MaterialChannels)
	found := false
	for i := range compiled.Field.PropPresentations {
		if compiled.Field.PropPresentations[i].ID == prop["id"].(string) {
			s.Equal(canonicalChannels(), compiled.Field.PropPresentations[i].MaterialChannels)
			compiled.Field.PropPresentations[i].MaterialChannels = nil
			found = true
		}
	}
	s.True(found)
	compiled.Field.StructuralWalls[0].MaterialChannels = nil
	compiled.Monsters[0].MaterialChannels = nil
	s.Equal(before.Field, compiled.Field, "appearance does not change geometry, doors or blockers")
	s.Equal(before.Monsters, compiled.Monsters, "appearance changes no rules or behavior identity")
}

func (s *SingleRoomWallSuite) TestMaterialChannelsDoNotDisappearOnMechanicalOnlyItems() {
	doc, _, prop := s.materialCastle()
	delete(prop, "assetRef")
	prop["materialChannels"] = nativeChannels()
	raw, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	_, err = dungeonspec.Load(raw)
	s.Require().Error(err)
	s.Contains(err.Error(), "materialChannels")
}

func (s *SingleRoomWallSuite) TestMaterialChannelsNativeInvalidShapesNameTheOwner() {
	cases := []interface{}{
		nil, "scalar", []interface{}{nil},
		[]interface{}{map[string]interface{}{"targetId": "body"}},
		[]interface{}{map[string]interface{}{"targetId": "body", "normal": nil}},
		[]interface{}{map[string]interface{}{"targetId": "body", "normal": map[string]interface{}{"mode": "off", "multiplier": 0}}},
		[]interface{}{map[string]interface{}{"targetId": "body", "normal": map[string]interface{}{"mode": "scale", "multiplier": 0}}},
		[]interface{}{map[string]interface{}{"targetId": "body", "normal": map[string]interface{}{"mode": "scale", "multiplier": math.NaN()}}},
	}
	for _, owner := range []string{"wall", "prop", "monster"} {
		for _, value := range cases {
			doc, wall, prop := s.materialCastle()
			target := wall
			if owner == "prop" {
				target = prop
			}
			if owner == "monster" {
				target = channelMonster(doc)
			}
			target["materialChannels"] = value
			raw, err := yaml.Marshal(doc)
			s.Require().NoError(err)
			_, err = dungeonspec.Load(raw)
			s.Require().Error(err, owner)
			s.Contains(err.Error(), "materialChannels", owner)
		}
	}
}

func (s *SingleRoomWallDoorSuite) TestMaterialChannelsFixedFrameNativeCapture() {
	frame := strings.TrimSuffix(nativeFrame, "}") + ", materialChannels: [{targetId: masonry, normal: {mode: scale, multiplier: 0.5}, emissive: {mode: off}}]}"
	raw := s.framedSource(frame)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(s.encoded(decoded.Spec))
	s.Require().NoError(err)
	captured := compiled.Field.StructuralWalls[0].Openings[0].Door.FramePresentation
	s.Require().NotNil(captured)
	s.Equal([]encounter.MaterialChannelAdjustment{{TargetID: "masonry", Normal: &encounter.ChannelAdjustment{Mode: encounter.ChannelScale, Multiplier: .5}, Emissive: &encounter.ChannelAdjustment{Mode: encounter.ChannelOff}}}, captured.MaterialChannels)
	for _, replacement := range []string{"multiplier: null", "multiplier: 0", "multiplier: .inf"} {
		_, err = dungeonspec.Load([]byte(strings.Replace(string(raw), "multiplier: 0.5", replacement, 1)))
		s.Require().Error(err)
		s.Contains(err.Error(), "materialChannels")
	}
}
