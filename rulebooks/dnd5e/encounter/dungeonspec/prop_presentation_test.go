// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"encoding/json"
	"math"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

func (s *SingleRoomWallDoorSuite) TestV2PropsKeepTheirExistingRendererInput() {
	raw, err := os.ReadFile("testdata/reference-tomb.yaml")
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	s.Empty(compiled.Field.PropPresentations, "v2 already carries renderable AtlasProp records")
	enc := s.play(compiled, encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At})
	known, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.NotEmpty(known.Props, "known v2 room props remain on their original path")
	full, err := enc.Atlas()
	s.Require().NoError(err)
	pillars := 0
	for _, p := range full.Props {
		if p.Ref == "dnd5e:props:pillar" {
			pillars++
		}
	}
	s.Equal(4, pillars, "the existing v2 pillars retain their content refs")
}

func (s *SingleRoomWallDoorSuite) TestPropPresentationFollowsKnowledgeWithoutSourceAccess() {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Props
items:
  - {id: books, assetRef: 'dnd5e:props:books', transform: {x: `+fmtFloat(boundaryWallX(20))+`, y: 0.5, z: 0, rotationY: 0.4}, heightScale: 1.5}
  - id: vase
    assetRef: dnd5e:props:vase
    transform: {x: 0, y: 0, z: 0, rotationY: 0}
    pointLight: {enabled: true, offset: {x: 0.1, y: 0.2, z: 0.3}, color: '#00ff11', intensity: 2, range: 4}
  - {id: trap, assetRef: 'dnd5e:props:altar', transform: {x: 0, y: 0, z: 0, rotationY: 0}}
groups: []
`)
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{"books": wallBlocks(false, false, 0.2, 0.2, 1, 1)}
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{"trap": {Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 20}}, Props: []string{"trap"}}}
	enc := s.play(s.load(spec))
	before, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(before.PropPresentations, 1)
	s.Equal("vase", before.PropPresentations[0].ID)
	s.Equal(2.0, before.PropPresentations[0].PointLight.Intensity)
	before.PropPresentations[0].PointLight.Intensity = 999
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "wall-doors-room/gap-door", Actor: "walker"})
	s.Require().NoError(err)
	after, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(after.PropPresentations, 2, "ordinary discovery does not defeat explicit trap concealment")
	books := after.PropPresentations[0]
	s.Equal("books", books.ID)
	s.InDelta(20, books.Origin.X, 1e-10, "blocker local offset must not move art")
	s.InDelta(0.5*5/math.Sqrt(3), books.Elevation, 1e-10)
	s.InDelta(-0.4*180/math.Pi, books.FacingDegrees, 1e-10)
	s.Equal(1.5, books.HeightScale)
	s.Equal(2.0, after.PropPresentations[1].PointLight.Intensity, "returned light does not alias definitions")
	story, err := enc.Story(&encounter.StoryInput{Audience: "walker"})
	s.Require().NoError(err)
	found := false
	for _, e := range story {
		var payload struct {
			Presentations []encounter.PropPresentationData `json:"prop_presentations"`
		}
		s.Require().NoError(json.Unmarshal(e.Payload, &payload))
		for _, p := range payload.Presentations {
			s.NotEqual("trap", p.ID)
			if p.ID == "books" {
				found = true
			}
		}
	}
	s.True(found, "room reveal carries its complete permitted appearance")
	restored, err := s.reload(enc).AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(after.PropPresentations, restored.PropPresentations)
}

func (s *SingleRoomWallDoorSuite) TestBuilderPropsCaptureAppearanceWithoutInventingBlockers() {
	spec := s.baseSpec()
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Props
items:
  - {id: books, kind: prop, assetRef: 'dnd5e:props:books', label: Books, transform: {x: 0, y: 0.5, z: 0, rotationY: 0.4}, heightScale: 1.5}
  - {id: vase, kind: prop, assetRef: 'dnd5e:props:vase', label: Vase, transform: {x: 1, y: 0, z: 0, rotationY: 0}}
  - {id: altar, kind: prop, assetRef: 'dnd5e:props:altar', label: Altar, transform: {x: 2, y: 0, z: 0, rotationY: 0}}
groups: []
`)
	spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{"books": wallBlocks(false, false, 0.2, 0.2, 0, 0)}
	compiled := s.load(spec)
	_, vaseHasCollider := placedByID(compiled.Field, "vase")
	s.False(vaseHasCollider, "decorative scenery must not acquire a fabricated blocker")
	enc := s.play(compiled)
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var saved struct {
		Field struct {
			Props []struct {
				ID  string `json:"id"`
				Ref string `json:"ref"`
			} `json:"prop_presentations"`
		} `json:"field"`
	}
	s.Require().NoError(json.Unmarshal(raw, &saved))
	s.Require().Len(saved.Field.Props, 3, "all authored renderable props need captured appearance, not only declarations")
	s.Equal("altar", saved.Field.Props[0].ID)
	s.Equal("dnd5e:props:altar", saved.Field.Props[0].Ref)
	s.Equal("books", saved.Field.Props[1].ID)
	s.Equal("vase", saved.Field.Props[2].ID)
}
