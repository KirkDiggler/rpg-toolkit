// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"gopkg.in/yaml.v3"
)

func (s *SingleRoomSourceSuite) groundSource(value string) []byte {
	s.Require().Contains(string(s.raw), "        lighting: { intensity: 1 }\n")
	return []byte(strings.Replace(string(s.raw), "        lighting: { intensity: 1 }\n", "        lighting: { intensity: 1 }\n        ground: "+value+"\n", 1))
}

func (s *SingleRoomSourceSuite) TestGroundCourtyardWalkFixture() {
	raw, err := os.ReadFile("testdata/ground/courtyard.yaml")
	s.Require().NoError(err)
	compiled, err := Load(raw)
	s.Require().NoError(err)
	s.Require().Len(compiled.Field.Regions, 2)
	s.Equal("courtyard", compiled.Field.Regions[0].ID)
	s.Equal("dnd5e:ground:fantasy-kingdom:grass_01", compiled.Field.Regions[0].Ground.BaseRef)
	s.Require().Len(compiled.Field.Regions[0].Ground.Paint, 1)
	s.Equal("dnd5e:ground:fantasy-kingdom:dirt_01", compiled.Field.Regions[0].Ground.Paint[0].Ref)
	s.Len(compiled.Field.Regions[0].Ground.Paint[0].Cells, 3)
	s.Nil(compiled.Field.Regions[1].Ground, "the gate retains its existing floor")
}

func (s *SingleRoomSourceSuite) TestGroundSourceLowersAndRoundTripsExplicitPaint() {
	raw := s.groundSource("{baseRef: grass, paint: [{ref: grass, cells: [{r: -1, q: [[0, 1]]}]}]}")
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Require().NotNil(decoded.Spec.Room.Gameplay.Regions[0].Ground)
	encoded, err := yaml.Marshal(decoded.Spec)
	s.Require().NoError(err)
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	s.Equal(decoded.Spec.Room.Gameplay.Regions[0].Ground, again.Spec.Room.Gameplay.Regions[0].Ground)
	compiled, err := Load(raw)
	s.Require().NoError(err)
	ground := compiled.Field.Regions[0].Ground
	s.Require().NotNil(ground)
	s.Equal("grass", ground.BaseRef)
	s.Require().Len(ground.Paint, 1)
	s.Equal("grass", ground.Paint[0].Ref, "same-as-base remains explicit")
	s.Equal([]spatial.Position{{X: -1, Y: -1}, {X: 0, Y: -1}}, ground.Paint[0].Cells, "native axial converts once to the input's offset frame")
	plain, err := Load(s.raw)
	s.Require().NoError(err)
	s.Nil(plain.Field.Regions[0].Ground)
	compiled.Field.Regions[0].Ground = nil
	s.Equal(plain, compiled, "ground changes no other compiled fact")
}

func (s *SingleRoomSourceSuite) TestGroundSourceRefusesMalformedConflictingOrOrphanPaint() {
	for _, value := range []string{
		"null", "{baseRef: null}", "{baseRef: ''}", "{baseRef: ' grass '}", "{paint: null}",
		"{paint: [{ref: '', cells: []}]}",
		"{paint: [{ref: dirt, cells: [{r: 0, q: [[9, 9]]}]}]}",
		"{paint: [{ref: dirt, cells: [{r: 0, q: [[0, 0]]}]}, {ref: grass, cells: [{r: 0, q: [[0, 0]]}]}]}",
		"{paint: [{ref: dirt, cells: [{r: 0, q: [[0, 1], [2, 2]]}]}]}",
		"{paint: [{ref: dirt, cells: [{r: 0.0, q: [[0, 0]]}]}]}",
		"{paint: [{ref: dirt, cells: [{r: 0, q: [[0, 16777216]]}]}]}",
	} {
		s.Run(value, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.groundSource(value)})
			s.Require().Error(err)
			s.Contains(err.Error(), "room.room.regions[0].ground")
		})
	}
}
