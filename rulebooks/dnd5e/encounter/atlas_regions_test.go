// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// atlas_regions_test.go pins what the map reports since regions replaced
// rooms (rpg-project#256): every floor cell, every region with its cells and
// the per-area facts it carries, and every prop, wall and doorway — flat,
// sorted, copied out, and computed from construction data alone.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type AtlasRegionsSuite struct {
	suite.Suite
}

func TestAtlasRegionsSuite(t *testing.T) {
	suite.Run(t, new(AtlasRegionsSuite))
}

// atlasField is threeRegions with a prop in the entrance, the entrance/hall
// seam walled except for one crossing, and a door in that crossing —
// declared in an order no sorted list would produce.
func atlasField() encounter.FieldInput {
	regions := threeRegions()
	regions[0], regions[2] = regions[2], regions[0] // tomb first, so the sort has work to do
	return encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: regions,
		Props:   []encounter.PropInput{rubble(8, 6), rubble(7, 5)},
		// The seam, plus one wall inside the entrance authored BACKWARDS —
		// east cell first — so the Atlas's normalization has something to do.
		Walls: append(seamWallRows(9, 4, 5, 6), wall(8, 5, 7, 5)),
		Doors: []encounter.DoorInput{
			openDoorway("seam", 9, 6, 10, 6),
			{ID: "inner", State: encounter.DoorIsClosed(), Edges: []encounter.DoorEdge{{From: cellAt(15, 5), To: cellAt(15, 6)}}},
		},
	}
}

func (s *AtlasRegionsSuite) open(field encounter.FieldInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 4}}},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *AtlasRegionsSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)
	return back
}

// TestAtlas_RegionsCarryConcealment: the concealed marker round-trips through
// ToData -> Load -> Atlas exactly as lighting does — carried, never read
// (rpg-project#351, living-world wave 1a) — and a region that authored
// nothing reports false on the far side, the same fact it went in as.
func (s *AtlasRegionsSuite) TestAtlas_RegionsCarryConcealment() {
	field := atlasField()
	for i := range field.Regions {
		if field.Regions[i].ID == "tomb" {
			field.Regions[i].Concealed = true
		}
	}

	atlas, err := s.reload(s.open(field)).Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Regions, 3)
	for _, r := range atlas.Regions {
		s.Equal(r.ID == "tomb", r.Concealed, "region %q", r.ID)
	}
}

// TestAtlas_RegionsCarryLighting: archetype and intensity round-trip through
// ToData -> Load -> Atlas, unread and unchanged.
func (s *AtlasRegionsSuite) TestAtlas_RegionsCarryLighting() {
	enc := s.open(atlasField())
	back := s.reload(enc)

	atlas, err := back.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Regions, 3)

	byID := map[string]encounter.AtlasRegion{}
	for _, r := range atlas.Regions {
		byID[r.ID] = r
	}
	s.Equal("cavern", byID["hall"].Archetype)
	s.Equal(0.4, byID["hall"].Lighting.Intensity)
	s.Equal("crypt", byID["tomb"].Archetype)
	s.Equal(1.0, byID["tomb"].Lighting.Intensity)
	s.Equal("hall", byID["hall"].Name)

	region, err := back.Region("hall")
	s.Require().NoError(err)
	s.Equal(byID["hall"], region, "Region(id) is the same report, one at a time")

	_, err = back.Region("oubliette")
	s.ErrorIs(err, encounter.ErrNoRegion)
}

// TestAtlasDeterministicOrdering pins C8: every list in one order, derived
// from the coordinates (or the ID) and never from declaration order.
func (s *AtlasRegionsSuite) TestAtlasDeterministicOrdering() {
	atlas, err := s.open(atlasField()).Atlas()
	s.Require().NoError(err)

	s.Equal("entrance", atlas.Regions[0].ID)
	s.Equal("hall", atlas.Regions[1].ID)
	s.Equal("tomb", atlas.Regions[2].ID)

	sorted := func(cells []spatial.Position) bool {
		for i := 1; i < len(cells); i++ {
			if cells[i].X < cells[i-1].X || (cells[i].X == cells[i-1].X && cells[i].Y <= cells[i-1].Y) {
				return false
			}
		}
		return true
	}
	s.True(sorted(atlas.Cells), "cells sorted by X then Y, strictly")
	for _, r := range atlas.Regions {
		s.True(sorted(r.Cells), "region %q's cells too", r.ID)
	}

	s.Require().Len(atlas.Props, 2)
	s.Equal(cellAt(7, 5), atlas.Props[0].At, "props by cell, not by declaration")
	s.Equal(cellAt(8, 6), atlas.Props[1].At)

	for i := 1; i < len(atlas.Boundaries); i++ {
		a, b := atlas.Boundaries[i-1], atlas.Boundaries[i]
		s.True(a.From.X < b.From.X || (a.From.X == b.From.X && (a.From.Y < b.From.Y ||
			(a.From.Y == b.From.Y && (a.To.X < b.To.X || (a.To.X == b.To.X && a.To.Y < b.To.Y))))),
			"boundaries sorted by From then To")
	}
	for _, b := range atlas.Boundaries {
		s.True(b.From.X < b.To.X || (b.From.X == b.To.X && b.From.Y < b.To.Y), "and each normalized From before To")
	}
	s.Contains(atlas.Boundaries, encounter.AtlasBoundary{From: cellAt(7, 5), To: cellAt(8, 5), BlocksMovement: true, BlocksLineOfSight: true},
		"the wall authored east-first is reported west-first: a wall has no side")

	s.Require().Len(atlas.Doorways, 2)
	s.Equal(encounter.DoorID("inner"), atlas.Doorways[0].Door, "doorways by door ID")
	s.Equal(encounter.DoorID("seam"), atlas.Doorways[1].Door)
	s.Equal(cellAt(9, 6), atlas.Doorways[1].From)
	s.Equal(cellAt(10, 6), atlas.Doorways[1].To)
}

// TestAtlasUnaffectedByLiveState pins ruling 3: construction truth, never
// live truth — a step, a tick and a door opening change nothing here.
func (s *AtlasRegionsSuite) TestAtlasUnaffectedByLiveState() {
	enc := s.open(atlasField())
	before, err := enc.Atlas()
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(7, 4)})
	s.Require().NoError(err)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "inner"})
	s.Require().NoError(err)

	after, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(before, after)
}

// TestAtlasCopyOutImmunity mutates every slice a returned Atlas exposes and
// re-fetches — the second must be unaffected.
func (s *AtlasRegionsSuite) TestAtlasCopyOutImmunity() {
	enc := s.open(atlasField())
	first, err := enc.Atlas()
	s.Require().NoError(err)
	pristine := deepCopyAtlas(first)
	hall, err := enc.Region("hall")
	s.Require().NoError(err)
	hallCells := append([]spatial.Position(nil), hall.Cells...)

	poison := spatial.Position{X: -999, Y: -999}
	for i := range first.Cells {
		first.Cells[i] = poison
	}
	for i := range first.Regions {
		first.Regions[i].Archetype = "poisoned"
		for j := range first.Regions[i].Cells {
			first.Regions[i].Cells[j] = poison
		}
	}
	for i := range first.Props {
		first.Props[i].At = poison
	}
	for i := range first.Boundaries {
		first.Boundaries[i].From, first.Boundaries[i].To = poison, poison
	}
	for i := range first.Doorways {
		first.Doorways[i].From = poison
	}

	for i := range hall.Cells {
		hall.Cells[i] = poison
	}

	second, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(pristine, second, "mutating one snapshot must not reach internal state")
	again, err := enc.Region("hall")
	s.Require().NoError(err)
	s.Equal(hallCells, again.Cells, "nor may a Region read alias the cells it hands out")
}

// deepCopyAtlas is a copy that shares no slice with its source — what a
// copy-out test has to compare against, since a second Atlas call would
// itself be the thing under test.
func deepCopyAtlas(a encounter.Atlas) encounter.Atlas {
	out := a
	out.Cells = append([]spatial.Position(nil), a.Cells...)
	out.Regions = make([]encounter.AtlasRegion, len(a.Regions))
	for i, r := range a.Regions {
		out.Regions[i] = r
		out.Regions[i].Cells = append([]spatial.Position(nil), r.Cells...)
	}
	out.Props = append([]encounter.AtlasProp(nil), a.Props...)
	out.Boundaries = append([]encounter.AtlasBoundary(nil), a.Boundaries...)
	out.Doorways = append([]encounter.AtlasDoorway(nil), a.Doorways...)
	return out
}

// TestAtlasIdenticalAfterReload — the reload-identity property, for the
// whole map.
func (s *AtlasRegionsSuite) TestAtlasIdenticalAfterReload() {
	enc := s.open(atlasField())
	before, err := enc.Atlas()
	s.Require().NoError(err)

	after, err := s.reload(enc).Atlas()
	s.Require().NoError(err)
	s.Equal(before, after)
}

// TestAPropsCellIsStillFloor: what a prop blocks is not ownership.
func (s *AtlasRegionsSuite) TestAPropsCellIsStillFloor() {
	enc := s.open(atlasField())
	onTheRubble := cellAt(8, 6)

	region, floor := enc.RegionAt(onTheRubble)
	s.Require().True(floor, "a prop stands ON the floor; it is not a hole in it")
	s.Equal(encounter.RegionID("entrance"), region)

	_, err := enc.Join(&encounter.JoinInput{Member: "onTheRubble", Kind: encounter.KindPlayer, Cell: onTheRubble})
	s.ErrorIs(err, encounter.ErrBadPlacement, "solid rubble is somewhere you cannot stand — blockage, not ownership")

	_, err = enc.Join(&encounter.JoinInput{Member: "besideIt", Kind: encounter.KindPlayer, Cell: cellAt(9, 7)})
	s.Require().NoError(err)
}

// TestVoidIsNotFloor: the canvas spans the bounding box, so a cell can be on
// the map and belong to no region; arriving there is refused.
func (s *AtlasRegionsSuite) TestVoidIsNotFloor() {
	enc := s.open(atlasField())
	_, err := enc.Join(&encounter.JoinInput{Member: "nowhere", Kind: encounter.KindPlayer, Cell: cellAt(12, 7)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "not floor")
}

// TestAPropMustStandOnTheFloor — a prop in the void is a prop nobody can
// draw, refused at construction by name.
func (s *AtlasRegionsSuite) TestAPropMustStandOnTheFloor() {
	field := atlasField()
	field.Props = append(field.Props, rubble(12, 7))
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   field,
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "[12,7]")
}
