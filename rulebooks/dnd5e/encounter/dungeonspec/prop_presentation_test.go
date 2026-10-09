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

type presentationSearchCheck struct{}

func (presentationSearchCheck) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: true, Applied: in.Approaches[0], Total: 30}, nil
}

func (s *SingleRoomWallDoorSuite) TestConcealmentRevealCapturesDecorativePresentation() {
	spec := s.baseSpec()
	spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Secret art
items: [{id: art, assetRef: 'dnd5e:props:vase', label: Vase, transform: {x: 0.3, y: 0.2, z: 0.4, rotationY: 0.7}, heightScale: 1.3}]
groups: []
`)
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{"secret": {Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 20}}, Props: []string{"art"}}}
	compiled := s.load(spec)
	initial := s.play(compiled)
	before, err := initial.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(before.PropPresentations)
	load := &encounter.LoadEncounterInput{Data: initial.ToData(), Capabilities: encounter.RefusingCapabilities()}
	load.Standing = everyoneStanding{}
	load.Sight = everyoneSeesTheWholeMap{}
	load.CheckResolver = presentationSearchCheck{}
	enc, err := encounter.LoadEncounter(load)
	s.Require().NoError(err)
	_, err = enc.Search(&encounter.SearchInput{Member: "walker", Region: compiled.PartyStart[0].Region})
	s.Require().NoError(err)
	after, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(after.PropPresentations, 1)
	story, err := enc.Story(&encounter.StoryInput{Audience: "walker"})
	s.Require().NoError(err)
	found := 0
	for _, entry := range story {
		var body struct {
			Beat string                           `json:"beat"`
			Rows []encounter.PropPresentationData `json:"prop_presentations"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		if body.Beat != encounter.BeatConcealmentRevealed {
			continue
		}
		found++
		s.Require().Len(body.Rows, 1)
		p := body.Rows[0]
		s.Equal("art", p.ID)
		s.Equal(after.PropPresentations[0].Ref, p.Ref)
		s.Equal(after.PropPresentations[0].Origin, p.Origin)
		s.Equal(after.PropPresentations[0].Elevation, p.Elevation)
		s.Equal(after.PropPresentations[0].FacingDegrees, p.FacingDegrees)
		s.Equal(after.PropPresentations[0].HeightScale, p.HeightScale)
		s.Equal(after.PropPresentations[0].Label, p.Label)
	}
	s.Equal(1, found)
	reloaded := s.reload(enc)
	replay, err := reloaded.Story(&encounter.StoryInput{Audience: "walker"})
	s.Require().NoError(err)
	s.Equal(story, replay)
}

func (s *SingleRoomWallDoorSuite) TestStandaloneDoorPresentationMintProjectionAndReload() {
	raw, err := os.ReadFile("testdata/world-builder-v4-site.yaml")
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	var definition *encounter.PropPresentation
	for i := range compiled.Field.PropPresentations {
		if compiled.Field.PropPresentations[i].ID == "cellar-door" {
			definition = &compiled.Field.PropPresentations[i]
		}
	}
	s.Require().NotNil(definition)
	s.Equal("front-room-site/cellar-door", definition.DoorID)
	enc := s.play(compiled, encounter.MemberInput{ID: "walker", Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At})
	before, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	var presented *encounter.PropPresentation
	for i := range before.PropPresentations {
		if before.PropPresentations[i].ID == "cellar-door" {
			presented = &before.PropPresentations[i]
		}
	}
	s.Require().NotNil(presented, "the ordinary closed door is a visible boundary, not withheld with its far-side floor")
	s.Equal(*definition, *presented)
	s.NotContains(before.Cells, axial(2, 0), "presenting a closed boundary does not teach its far-side floor")
	_, err = enc.Step(&encounter.StepInput{Member: "walker", To: axial(1, 0)})
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Actor: "walker", Door: definition.DoorID})
	s.Require().NoError(err)
	after, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	restored, err := s.reload(enc).AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(after.PropPresentations, restored.PropPresentations)
	compiled.Field.Concealments = []encounter.ConcealmentInput{{ID: "door-secret", Doors: []encounter.DoorID{definition.DoorID}, Checks: []encounter.CheckApproach{{Ability: "perception", DC: 20}}}}
	hidden, err := s.play(compiled).AtlasFor("walker")
	s.Require().NoError(err)
	for _, p := range hidden.PropPresentations {
		s.NotEqual("cellar-door", p.ID)
	}
	for _, p := range hidden.Placed {
		s.NotEqual("cellar-door", p.ID, "a hidden canonical door must not leak its raw boundary identity")
	}
}

func (s *SingleRoomWallDoorSuite) TestRawPresentationCannotDuplicateStructuralChannels() {
	compiled := s.load(s.baseSpec())
	for _, id := range []string{"long-wall", "long-wall-door"} {
		field := compiled.Field
		field.PropPresentations = []encounter.PropPresentation{{ID: id, Ref: "test:props:door", HeightScale: 1}}
		_, err := encounter.NewEncounter(&encounter.SetupInput{Field: field, Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}}, Capabilities: encounter.RefusingCapabilities()})
		s.ErrorIs(err, encounter.ErrNoField)
	}
	field := compiled.Field
	bound, ok := placedByID(field, "long-wall-door")
	s.Require().True(ok)
	bound.ID = "duplicate"
	field.Placed = append(append([]encounter.PlacedPropInput(nil), field.Placed...), bound)
	field.PropPresentations = []encounter.PropPresentation{{ID: "duplicate", Ref: "test:props:door", Origin: bound.Placement.Origin, FacingDegrees: bound.Placement.Facing, HeightScale: 1, DoorID: boundDoorID}}
	_, err := encounter.NewEncounter(&encounter.SetupInput{Field: field, Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}}, Capabilities: encounter.RefusingCapabilities()})
	s.ErrorIs(err, encounter.ErrNoField, "a different raw ID cannot duplicate the structural DoorID")
}

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
