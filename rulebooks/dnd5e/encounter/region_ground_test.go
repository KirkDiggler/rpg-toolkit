// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const groundGrass = "dnd5e:ground:fantasy-kingdom:grass_01"
const groundDirt = "dnd5e:ground:fantasy-kingdom:dirt_01"

func groundField() encounter.FieldInput {
	return encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{{
		ID: "courtyard", Name: "Courtyard", Archetype: "crypt", Lighting: &encounter.Lighting{Intensity: 1},
		Cells: []spatial.Position{{X: 6, Y: 4}, {X: 7, Y: 4}, {X: 8, Y: 4}},
		Ground: &encounter.RegionGround{BaseRef: groundGrass, Paint: []encounter.GroundPaint{
			{Ref: groundGrass, Cells: []spatial.Position{{X: 6, Y: 4}}},
			{Ref: groundDirt, Cells: []spatial.Position{{X: 7, Y: 4}, {X: 8, Y: 4}}},
		}},
	}}}
}

func (s *AtlasRegionsSuite) TestGroundCapturedCopiedAndReloadedWithoutChangingCoordinates() {
	input := groundField()
	enc := s.open(input)
	input.Regions[0].Ground.BaseRef = "mutated"
	input.Regions[0].Ground.Paint[1].Cells[0].X = 999
	back := s.reload(enc)
	atlas, err := back.Atlas()
	s.Require().NoError(err)
	expected := &encounter.RegionGround{BaseRef: groundGrass, Paint: []encounter.GroundPaint{
		{Ref: groundGrass, Cells: []spatial.Position{{X: 4, Y: 4}}},
		{Ref: groundDirt, Cells: []spatial.Position{{X: 5, Y: 4}, {X: 6, Y: 4}}},
	}}
	s.Require().Equal(expected, atlas.Regions[0].Ground)
	s.Equal(groundField().Regions[0].Ground.BaseRef, back.ToData().Field.Regions[0].Ground.BaseRef)
	s.Equal([]encounter.PositionData{{X: 7, Y: 4}, {X: 8, Y: 4}}, back.ToData().Field.Regions[0].Ground.Paint[1].Cells)
	atlas.Regions[0].Ground.Paint[0].Cells[0].X = 999
	atlas.Regions[0].Ground.BaseRef = "outside"
	fresh, err := back.Region("courtyard")
	s.Require().NoError(err)
	s.Equal(expected, fresh.Ground)
	fresh.Ground.Paint[0].Cells[0].X = 888
	again, err := back.Atlas()
	s.Require().NoError(err)
	s.Equal(expected, again.Regions[0].Ground)
}

func (s *AtlasRegionsSuite) TestGroundRefusedIfItAssignsOutsideOwnerOrMoreThanOnce() {
	cases := []struct {
		name   string
		change func(*encounter.RegionGround)
	}{
		{"empty paint ref", func(g *encounter.RegionGround) { g.Paint[0].Ref = "" }},
		{"padded base", func(g *encounter.RegionGround) { g.BaseRef = " grass " }},
		{"outside", func(g *encounter.RegionGround) { g.Paint[0].Cells[0].X = 42 }},
		{"fractional", func(g *encounter.RegionGround) { g.Paint[0].Cells[0].X = 6.5 }},
		{"duplicate", func(g *encounter.RegionGround) { g.Paint[1].Cells[0] = g.Paint[0].Cells[0] }},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			input := groundField()
			tc.change(input.Regions[0].Ground)
			_, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: input})
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
}

func (s *AtlasRegionsSuite) TestGroundDoesNotChangePartitionsAndAbsentGroundStaysAbsent() {
	painted := groundField()
	before, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: painted})
	s.Require().NoError(err)
	painted.Regions[0].Ground = nil
	after, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: painted})
	s.Require().NoError(err)
	s.Equal(before, after)
	enc := s.reload(s.open(painted))
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Nil(atlas.Regions[0].Ground)
	s.Nil(enc.ToData().Field.Regions[0].Ground)
}

func (s *AtlasRegionsSuite) TestGroundConcealmentMasksOnlyTheHiddenPaintCells() {
	input := groundField()
	input.Concealments = []encounter.ConcealmentInput{{ID: "secret", Checks: []encounter.CheckApproach{{Ability: "perception", DC: 15}}, Cells: []spatial.Position{{X: 8, Y: 4}}}}
	enc := s.open(input)
	scoped, err := enc.AtlasFor(alice)
	s.Require().NoError(err)
	s.Require().Len(scoped.Regions, 1)
	s.Equal([]spatial.Position{{X: 5, Y: 4}}, scoped.Regions[0].Ground.Paint[1].Cells)
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Len(full.Regions[0].Ground.Paint[1].Cells, 2)
}

func (s *CellKnowledgeSuite) TestGroundSavedOutsideOwnerIsRefused() {
	field := s.field()
	field.Regions[0].Ground = &encounter.RegionGround{Paint: []encounter.GroundPaint{{Ref: groundDirt, Cells: field.Regions[0].Cells[:1]}}}
	s.startWith(field)
	data := s.enc.ToData()
	data.Field.Regions[0].Ground.Paint[0].Cells[0].X = 999
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities()})
	s.ErrorIs(err, encounter.ErrNoField)
}

func (s *ConcealmentSuite) TestGroundRevealDoesNotExposeAnotherSecretInTheSameRegion() {
	field := groundField()
	field.Concealments = []encounter.ConcealmentInput{
		{ID: "near", Checks: vaultCheck(), Cells: []spatial.Position{{X: 7, Y: 4}}},
		{ID: "far", Checks: vaultCheck(), Cells: []spatial.Position{{X: 8, Y: 4}}},
	}
	enc := s.play(field, findsNothing{}, encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 4}})
	s.Require().NoError(encounter.ExportedFind(enc, alice, "near"))
	reveals := s.revealsFor(enc, alice)
	s.Require().Len(reveals, 1)
	encoded, err := json.Marshal(reveals[0]["regions"])
	s.Require().NoError(err)
	var regions []encounter.AtlasRegion
	s.Require().NoError(json.Unmarshal(encoded, &regions))
	s.Require().Len(regions, 1)
	s.Require().NotNil(regions[0].Ground)
	s.Require().Len(regions[0].Ground.Paint, 2)
	s.Equal([]spatial.Position{{X: 5, Y: 4}}, regions[0].Ground.Paint[1].Cells)
}

func (s *CellKnowledgeSuite) TestGroundSnapshotAndDiscoveryCarryOnlyDeliveredPaint() {
	field := s.field()
	for i := range field.Regions {
		region := &field.Regions[i]
		region.Ground = &encounter.RegionGround{BaseRef: groundGrass, Paint: []encounter.GroundPaint{{Ref: groundDirt, Cells: append([]spatial.Position(nil), region.Cells...)}}}
	}
	s.startWith(field)
	before := s.atlas("a")
	for _, region := range before.Regions {
		s.Require().NotNil(region.Ground)
		s.Equal(region.Cells, region.Ground.Paint[0].Cells)
	}
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"})
	s.Require().NoError(err)
	beats := s.cellsRevealed("a")
	s.Require().NotEmpty(beats)
	for _, beat := range beats {
		var regions []encounter.AtlasRegion
		s.Require().NoError(json.Unmarshal(beat["regions"], &regions))
		for _, region := range regions {
			s.Require().NotNil(region.Ground)
			s.Require().Len(region.Ground.Paint, 1)
			s.Equal(region.Cells, region.Ground.Paint[0].Cells, "partial discoveries must not include the rest of the region's paint")
		}
	}
	current := s.atlas("a")
	s.reload()
	s.Equal(current.Regions, s.atlas("a").Regions)
}
