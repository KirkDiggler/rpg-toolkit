// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *StructuralWallSuite) TestWallFaceChoicesSurviveColdSiteReload() {
	want := []encounter.WallFaceFinish{
		{Side: encounter.WallFacePositiveZ, MaterialRef: "look:brick"},
		{Side: encounter.WallFaceNegativeZ, MaterialRef: "look:stucco"},
	}
	field := structuralField()
	field.StructuralWalls[0].FaceFinishes = append([]encounter.WallFaceFinish(nil), want...)
	enc := s.play(field)
	originalHash := enc.ToData().SiteHash
	field.StructuralWalls[0].FaceFinishes[0].MaterialRef = "later:source"
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(atlas.StructuralWalls, 1)
	s.Require().Equal(want, atlas.StructuralWalls[0].FaceFinishes)
	atlas.StructuralWalls[0].FaceFinishes[0].MaterialRef = "caller:mutation"
	exported := enc.Site().Data()
	s.Require().Len(exported.Field.StructuralWalls[0].FaceFinishes, 2)
	exported.Field.StructuralWalls[0].FaceFinishes[0].MaterialRef = "export:mutation"
	raw, err := json.Marshal(enc.Site().Data())
	s.Require().NoError(err)
	var site encounter.SiteData
	s.Require().NoError(json.Unmarshal(raw, &site))
	data := enc.ToData()
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Site: &site, Sites: encounter.NewMemorySites(nil),
		Capabilities: encounter.Capabilities{
			Sheets: zeroSheets{}, Sight: everyoneSeesTheWholeMap{},
			Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{},
			Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
			Actors: encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	s.Require().NoError(err)
	next, err := loaded.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(want, next.StructuralWalls[0].FaceFinishes)
	s.Equal(atlas.Placed, next.Placed)
	s.Equal(atlas.StructuralWalls[0].Openings, next.StructuralWalls[0].Openings)
	changed := s.play(field)
	s.NotEqual(originalHash, changed.ToData().SiteHash, "appearance belongs to the captured site identity")
}

func (s *StructuralWallSuite) TestWallFaceChoicesAgreeAcrossBothAtlasReaders() {
	want := []encounter.WallFaceFinish{
		{Side: encounter.WallFacePositiveZ, MaterialRef: "look:brick"},
		{Side: encounter.WallFaceNegativeZ, MaterialRef: "look:stucco"},
	}
	field := structuralField()
	field.StructuralWalls[0].FaceFinishes = append([]encounter.WallFaceFinish(nil), want...)
	enc := s.play(field)
	author, err := enc.Atlas()
	s.Require().NoError(err)
	player, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Len(author.StructuralWalls, 1)
	s.Require().Len(player.StructuralWalls, 1)
	s.Require().Equal(want, author.StructuralWalls[0].FaceFinishes)
	s.Require().Equal(want, player.StructuralWalls[0].FaceFinishes)
	author.StructuralWalls[0].FaceFinishes[0].MaterialRef = "author:mutation"
	s.Equal("look:brick", player.StructuralWalls[0].FaceFinishes[0].MaterialRef)
	player.StructuralWalls[0].FaceFinishes[1].MaterialRef = "player:mutation"
	authorAgain, err := enc.Atlas()
	s.Require().NoError(err)
	playerAgain, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(want, authorAgain.StructuralWalls[0].FaceFinishes)
	s.Equal(want, playerAgain.StructuralWalls[0].FaceFinishes)
}

func (s *StructuralWallSuite) TestInvalidFaceChoicesRefuseConstructionAndSiteLoad() {
	for _, tc := range []struct {
		name   string
		values []encounter.WallFaceFinish
	}{
		{"empty-side", []encounter.WallFaceFinish{{MaterialRef: "look"}}},
		{"unknown-side", []encounter.WallFaceFinish{{Side: "front", MaterialRef: "look"}}},
		{"blank-ref", []encounter.WallFaceFinish{{Side: encounter.WallFacePositiveZ, MaterialRef: " \t"}}},
		{"duplicate-side", []encounter.WallFaceFinish{
			{Side: encounter.WallFacePositiveZ, MaterialRef: "one"}, {Side: encounter.WallFacePositiveZ, MaterialRef: "two"},
		}},
	} {
		s.Run(tc.name, func() {
			field := structuralField()
			field.StructuralWalls[0].FaceFinishes = tc.values
			_, err := s.build(field)
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
	enc := s.play(structuralField())
	site := enc.Site().Data()
	site.Field.StructuralWalls[0].FaceFinishes = []encounter.WallFaceFinishData{{Side: "front", MaterialRef: "look"}}
	_, err := loadOverSite(enc.ToData(), site, encounter.LoadEncounterInput{Capabilities: encounter.RefusingCapabilities()})
	s.Error(err)
}

func (s *StructuralWallSuite) TestHiddenWallCarriesNoFinishBeforeReveal() {
	field := concealedStructuralField([]encounter.PropID{"wall-presence"}, nil, nil)
	field.StructuralWalls[0].FaceFinishes = []encounter.WallFaceFinish{{Side: encounter.WallFacePositiveZ, MaterialRef: "private:look"}}
	enc := s.play(field)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(atlas.StructuralWalls)
	s.NotEmpty(atlas.StructuralDoors, "door permission stays independent of the parent wall")
}
