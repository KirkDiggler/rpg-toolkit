// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *StructuralRevealSuite) TestWallSectionRevealIsRecipientScopedAndCaptured() {
	field := hiddenWallRevealField()
	field.StructuralWalls[0].Sections = []encounter.WallSection{{ID: "window", Ref: "private:window", Position: 8, Width: 2}}
	enc := s.open(field, findsNothing{})
	for _, id := range []encounter.MemberID{"finder", "peer"} {
		before, err := enc.AtlasFor(id)
		s.Require().NoError(err)
		s.Empty(before.StructuralWalls)
	}
	s.Require().NoError(encounter.ExportedFind(enc, "finder", "secret"))
	found, err := enc.AtlasFor("finder")
	s.Require().NoError(err)
	s.Require().Len(found.StructuralWalls, 1)
	s.Equal("private:window", found.StructuralWalls[0].Sections[0].Ref)
	peer, err := enc.AtlasFor("peer")
	s.Require().NoError(err)
	s.Empty(peer.StructuralWalls)
	beats := s.structuralBeats(enc, "finder", encounter.BeatConcealmentRevealed)
	s.Require().Len(beats, 1)
	raw, err := json.Marshal(beats[0])
	s.Require().NoError(err)
	s.Contains(string(raw), "private:window")
	peerBeats := s.structuralBeats(enc, "peer", encounter.BeatConcealmentRevealed)
	s.Empty(peerBeats)
	field.StructuralWalls[0].Sections[0].Ref = "later:source"
	again, err := json.Marshal(s.structuralBeats(enc, "finder", encounter.BeatConcealmentRevealed)[0])
	s.Require().NoError(err)
	s.Equal(string(raw), string(again))
}

func (s *StructuralWallSuite) TestWallSectionsPersistProjectAndStayDetached() {
	want := []encounter.WallSection{{ID: "window", Ref: "opaque:window", Position: 8, Width: 2}}
	field := structuralField()
	plain := s.play(field)
	field.StructuralWalls[0].Sections = append([]encounter.WallSection(nil), want...)
	enc := s.play(field)
	hash := enc.ToData().SiteHash
	s.NotEqual(plain.ToData().SiteHash, hash)
	field.StructuralWalls[0].Sections[0].Ref = "later:source"
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Require().Equal(want, atlas.StructuralWalls[0].Sections)
	original, err := plain.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(original.Placed, atlas.Placed)
	s.Equal(original.StructuralDoors, atlas.StructuralDoors)
	s.Equal(original.StructuralWalls[0].Openings, atlas.StructuralWalls[0].Openings)
	atlas.StructuralWalls[0].Sections[0].Ref = "caller:mutation"
	exported := enc.Site().Data()
	exported.Field.StructuralWalls[0].Sections[0].Ref = "export:mutation"
	raw, err := json.Marshal(enc.Site().Data())
	s.Require().NoError(err)
	var site encounter.SiteData
	s.Require().NoError(json.Unmarshal(raw, &site))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Site: &site, Sites: encounter.NewMemorySites(nil),
		Capabilities: encounter.Capabilities{Sheets: zeroSheets{}, Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{}, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{}, Actors: encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}}},
	})
	s.Require().NoError(err)
	after, err := loaded.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(want, after.StructuralWalls[0].Sections)
	author, err := loaded.Atlas()
	s.Require().NoError(err)
	s.Equal(want, author.StructuralWalls[0].Sections)
	author.StructuralWalls[0].Sections[0].Ref = "author:mutation"
	s.Equal(want, after.StructuralWalls[0].Sections)
	s.Equal(hash, loaded.ToData().SiteHash)
}

func (s *StructuralWallSuite) TestWallSectionsRefuseInvalidIntervalsAndDoorSurroundOverlap() {
	for _, tc := range []struct {
		name     string
		sections []encounter.WallSection
	}{
		{"blank-id", []encounter.WallSection{{ID: " ", Ref: "x", Position: 8, Width: 2}}},
		{"blank-ref", []encounter.WallSection{{ID: "x", Ref: " ", Position: 8, Width: 2}}},
		{"nonfinite", []encounter.WallSection{{ID: "x", Ref: "x", Position: math.NaN(), Width: 2}}},
		{"zero-width", []encounter.WallSection{{ID: "x", Ref: "x", Position: 8}}},
		{"outside", []encounter.WallSection{{ID: "x", Ref: "x", Position: 10, Width: 2}}},
		{"duplicate", []encounter.WallSection{{ID: "x", Ref: "x", Position: 8, Width: 1}, {ID: "x", Ref: "y", Position: 9, Width: 1}}},
		{"overlap", []encounter.WallSection{{ID: "x", Ref: "x", Position: 8, Width: 2}, {ID: "y", Ref: "y", Position: 8.5, Width: 1}}},
		{"opening", []encounter.WallSection{{ID: "x", Ref: "x", Position: 5, Width: 1}}},
	} {
		s.Run(tc.name, func() {
			field := structuralField()
			field.StructuralWalls[0].Sections = tc.sections
			_, err := s.build(field)
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
	field := structuralField()
	field.StructuralWalls[0].Sections = []encounter.WallSection{{ID: "window", Ref: "x", Position: 8, Width: 2}}
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = &encounter.DoorFramePresentation{Span: &encounter.WallFrameSpan{Before: 1, After: 2.5}}
	_, err := s.build(field)
	s.ErrorIs(err, encounter.ErrNoField)
	field.StructuralWalls[0].Openings[0].Door.FramePresentation.Span.After = 2
	_, err = s.build(field)
	s.NoError(err, "touching a surround is valid")
}

func (s *StructuralWallSuite) TestHiddenWallWithholdsItsSectionsAndEmptyLegacyListKeepsHash() {
	field := concealedStructuralField([]encounter.PropID{"wall-presence"}, nil, nil)
	field.StructuralWalls[0].Sections = []encounter.WallSection{{ID: "private-window", Ref: "private:art", Position: 8, Width: 2}}
	enc := s.play(field)
	atlas, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(atlas.StructuralWalls)
	s.NotEmpty(atlas.StructuralDoors)
	plain := structuralField()
	old := s.play(plain)
	plain.StructuralWalls[0].Sections = []encounter.WallSection{}
	s.Equal(old.ToData().SiteHash, s.play(plain).ToData().SiteHash)
}
