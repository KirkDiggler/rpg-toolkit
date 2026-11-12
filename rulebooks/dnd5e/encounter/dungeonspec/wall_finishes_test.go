// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"gopkg.in/yaml.v3"
)

func (s *SingleRoomWallSuite) withWallFinishes(value interface{}) *dungeonspec.SingleRoomSpec {
	spec := s.baseSpec()
	var scene map[string]interface{}
	s.Require().NoError(spec.Room.Scene.Decode(&scene))
	scene["wallFinishes"] = value
	s.Require().NoError(spec.Room.Scene.Encode(scene))
	return spec
}

func (s *SingleRoomWallSuite) TestNativeWallFinishesPreserveNonAppearanceFacts() {
	base := s.baseSpec()
	s.Require().NotEmpty(base.Room.Gameplay.Walls)
	id := base.Room.Gameplay.Walls[0].ID
	before, err := dungeonspec.Load(s.encoded(base))
	s.Require().NoError(err)
	spec := s.withWallFinishes(map[string]interface{}{id: map[string]interface{}{
		"positive-z":       "dnd5e:material:fantasy-kingdom:wall_brick_01",
		"negative-z":       "dnd5e:material:fantasy-kingdom:wall_stucco_01",
		"future-side-note": map[string]interface{}{"keep": true},
	}})
	source := s.encoded(spec)
	after, err := dungeonspec.Load(source)
	s.Require().NoError(err)
	s.Require().Equal([]encounter.WallFaceFinish{
		{Side: encounter.WallFacePositiveZ, MaterialRef: "dnd5e:material:fantasy-kingdom:wall_brick_01"},
		{Side: encounter.WallFaceNegativeZ, MaterialRef: "dnd5e:material:fantasy-kingdom:wall_stucco_01"},
	}, after.Field.StructuralWalls[0].FaceFinishes)
	after.Field.StructuralWalls[0].FaceFinishes = nil
	s.Equal(before.Field, after.Field, "no geometry, ground, rules or door fact changes")
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: source})
	s.Require().NoError(err)
	var scene map[string]interface{}
	s.Require().NoError(decoded.Spec.Room.Scene.Decode(&scene))
	row := scene["wallFinishes"].(map[string]interface{})[id].(map[string]interface{})
	s.Equal(map[string]interface{}{"keep": true}, row["future-side-note"])
}

func (s *SingleRoomWallSuite) TestMalformedNativeFaceValuesRefuseWithPath() {
	id := s.baseSpec().Room.Gameplay.Walls[0].ID
	for _, tc := range []struct {
		name  string
		value interface{}
	}{
		{"null", nil}, {"scalar", "look"}, {"list", []interface{}{}},
		{"orphan", map[string]interface{}{"missing-wall": map[string]interface{}{"positive-z": "look"}}},
		{"null-row", map[string]interface{}{id: nil}},
		{"scalar-row", map[string]interface{}{id: "look"}},
		{"empty-row", map[string]interface{}{id: map[string]interface{}{}}},
		{"number", map[string]interface{}{id: map[string]interface{}{"positive-z": 3}}},
		{"null-ref", map[string]interface{}{id: map[string]interface{}{"positive-z": nil}}},
		{"blank", map[string]interface{}{id: map[string]interface{}{"negative-z": " \t"}}},
	} {
		s.Run(tc.name, func() {
			_, err := dungeonspec.Load(s.encoded(s.withWallFinishes(tc.value)))
			s.Require().Error(err)
			s.Contains(err.Error(), "wallFinishes")
		})
	}
}

func (s *SingleRoomWallSuite) TestNativeWallFinishesRefuseDuplicateFaceKeys() {
	id := s.baseSpec().Room.Gameplay.Walls[0].ID
	spec := s.withWallFinishes(map[string]interface{}{id: map[string]interface{}{"positive-z": "look"}})
	scene := &spec.Room.Scene
	var finishes *yaml.Node
	for i := 0; i < len(scene.Content); i += 2 {
		if scene.Content[i].Value == "wallFinishes" {
			finishes = scene.Content[i+1]
			break
		}
	}
	s.Require().NotNil(finishes)
	row := finishes.Content[1]
	row.Content = append(row.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "positive-z"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "another-look"})
	_, err := dungeonspec.Load(s.encoded(spec))
	s.Require().Error(err)
	s.Contains(err.Error(), "wallFinishes")
}

func (s *SingleRoomWallSuite) TestRegionFinishDoesNotInventWallOwnership() {
	spec := s.baseSpec()
	var scene map[string]interface{}
	s.Require().NoError(spec.Room.Scene.Decode(&scene))
	scene["regionPresentation"] = map[string]interface{}{"hall": map[string]interface{}{
		"wallFinish": map[string]interface{}{"materialRef": "look:unapplied-region"},
	}}
	s.Require().NoError(spec.Room.Scene.Encode(scene))
	compiled, err := dungeonspec.Load(s.encoded(spec))
	s.Require().NoError(err)
	for _, wall := range compiled.Field.StructuralWalls {
		s.Empty(wall.FaceFinishes)
	}
}
