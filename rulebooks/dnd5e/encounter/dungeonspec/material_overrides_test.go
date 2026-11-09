// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"gopkg.in/yaml.v3"
	"os"
)

func (s *SingleRoomWallSuite) materialCastle() (map[string]interface{}, map[string]interface{}, map[string]interface{}) {
	raw, err := os.ReadFile("testdata/world-builder-v4-castle.yaml")
	s.Require().NoError(err)
	var doc map[string]interface{}
	s.Require().NoError(yaml.Unmarshal(raw, &doc))
	room := doc["room"].(map[string]interface{})
	wall := room["room"].(map[string]interface{})["walls"].([]interface{})[0].(map[string]interface{})["appearance"].(map[string]interface{})
	prop := room["scene"].(map[string]interface{})["items"].([]interface{})[0].(map[string]interface{})
	return doc, wall, prop
}

func (s *SingleRoomWallSuite) TestNativeMaterialOverridesPreserveEveryNonAppearanceField() {
	doc, wall, prop := s.materialCastle()
	beforeBytes, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	before, err := dungeonspec.Load(beforeBytes)
	s.Require().NoError(err)
	selected := []interface{}{map[string]interface{}{"surfaceId": "atlas", "materialRef": "dnd5e:material:fantasy-kingdom:atlas_04_c"}}
	wall["materialOverrides"] = selected
	prop["materialOverrides"] = selected
	raw, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	after, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	want := []encounter.MaterialOverride{{SurfaceID: "atlas", MaterialRef: "dnd5e:material:fantasy-kingdom:atlas_04_c"}}
	s.Equal(want, after.Field.StructuralWalls[0].MaterialOverrides)
	found := false
	for i := range after.Field.PropPresentations {
		if after.Field.PropPresentations[i].ID == prop["id"].(string) {
			s.Equal(want, after.Field.PropPresentations[i].MaterialOverrides)
			after.Field.PropPresentations[i].MaterialOverrides = nil
			found = true
		}
	}
	s.True(found, "the actual source prop must be captured")
	after.Field.StructuralWalls[0].MaterialOverrides = nil
	s.Equal(before.Field, after.Field, "texture selection changes no geometry, ground, roster or rules")
}

func (s *SingleRoomWallSuite) TestNativeMaterialOverrideShapesRefuseAtBothOwners() {
	for _, value := range []struct {
		name  string
		value interface{}
	}{
		{"null", nil}, {"scalar", "atlas"}, {"map", map[string]interface{}{}},
		{"null-entry", []interface{}{nil}},
		{"missing", []interface{}{map[string]interface{}{"surfaceId": "atlas"}}},
		{"number", []interface{}{map[string]interface{}{"surfaceId": 3, "materialRef": "look"}}},
		{"blank", []interface{}{map[string]interface{}{"surfaceId": "atlas", "materialRef": " "}}},
		{"duplicate", []interface{}{map[string]interface{}{"surfaceId": "atlas", "materialRef": "one"}, map[string]interface{}{"surfaceId": "atlas", "materialRef": "two"}}},
	} {
		for _, owner := range []string{"wall", "prop"} {
			s.Run(value.name+"/"+owner, func() {
				doc, wall, prop := s.materialCastle()
				target := wall
				if owner == "prop" {
					target = prop
				}
				target["materialOverrides"] = value.value
				raw, err := yaml.Marshal(doc)
				s.Require().NoError(err)
				_, err = dungeonspec.Load(raw)
				s.Require().Error(err)
				s.Contains(err.Error(), "materialOverrides")
			})
		}
	}
}
