// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func structuralDoorOnlyWorld(extra ...encounter.ConcealmentInput) scene {
	centre := hallPlane().CellCentre(hexCell(4, 0))
	point := func(dx float64) spatial.Point { return spatial.Point{X: centre.X + dx, Y: centre.Y} }
	wallBox := structuralBox(hexCell(4, 0), 6, 0.25)
	doorBox := structuralBox(hexCell(4, 0), 2, 0.25)

	return scene{
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 6)},
			Concealments: append([]encounter.ConcealmentInput{{
				ID: structSecret, Checks: vaultFind(),
				Doors: []encounter.DoorID{structDoorID}, Props: []encounter.PropID{structDoorPresence},
			}}, extra...),
			Doors: []encounter.DoorInput{{ID: structDoorID, Placement: &doorBox, State: encounter.DoorIsClosed()}},
			Placed: []encounter.PlacedPropInput{
				{ID: structWallPresence, Placement: wallBox},
				{ID: structDoorPresence, Placement: doorBox},
			},
			StructuralWalls: []encounter.StructuralWallInput{{
				ID: structWallPresence, Ref: "dnd5e:env:test:wall",
				From: point(-3), To: point(3), Height: 3, Thickness: 0.25,
				Openings: []encounter.StructuralOpeningInput{{
					ID: "gap", Position: 3, Width: 2,
					Door: &encounter.StructuralDoorBindingInput{
						PlacedID: structDoorPresence, DoorID: structDoorID, Ref: "dnd5e:env:test:door",
						From: point(-1), To: point(1),
					},
				}},
			}},
		},
		Party:   []sceneSeat{{ID: "alice", At: cell(1, 1)}},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
}

// structuralReveal is a concealment_revealed beat's payload, read through
// this package's own atlas types.
type structuralReveal struct {
	Concealment                        string                                      `json:"concealment"`
	Cells                              []spatial.Position                          `json:"cells"`
	StructuralWalls                    []session.AtlasStructuralWall               `json:"structural_walls"`
	StructuralDoors                    []session.AtlasStructuralDoor               `json:"structural_doors"`
	StructuralWallOpeningsReplacements []session.StructuralWallOpeningsReplacement `json:"structural_wall_openings_replacements"`
}

// structuralCellsReveal is a cells_revealed beat's floor.
type structuralCellsReveal struct {
	Scenery []spatial.Position `json:"scenery"`
	Regions []struct {
		Cells []spatial.Position `json:"cells"`
	} `json:"regions"`
}

// onlyReveal is the one concealment_revealed beat in told, decoded.
func (s *StructuralSessionSuite) onlyReveal(told []witnessed) structuralReveal {
	s.T().Helper()
	reveals := beatsNamed(told, encounter.BeatConcealmentRevealed)
	s.Require().Len(reveals, 1)
	var body structuralReveal
	reveals[0].decode(s.T(), &body)
	return body
}

func (s *StructuralSessionSuite) TestIndependentDoorIntroductionDoesNotRequireItsHiddenParent() {
	ctx := context.Background()
	world := structuralDoorOnlyWorld(encounter.ConcealmentInput{
		ID: "hidden-parent", Props: []encounter.PropID{structWallPresence},
		Checks: []encounter.CheckApproach{{Ability: "perception", DC: 100}},
	})
	s.startWith(world, sharpEyed("alice"))
	in := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	before, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Empty(before.Atlas.StructuralWalls)
	s.Empty(before.Atlas.StructuralDoors)
	mark := lastSeq(storyBeats(s.T(), s.mgr, "sess", "alice"))
	_, err = s.find(ctx, "alice")
	s.Require().NoError(err)
	told := storyBeatsAfter(s.T(), s.mgr, "sess", "alice", mark)
	body := s.onlyReveal(told)
	s.Empty(body.StructuralWalls)
	s.Empty(body.StructuralWallOpeningsReplacements, "a hidden parent must not be named by a patch")
	s.Require().Len(body.StructuralDoors, 1)
	s.Equal(structDoorID, body.StructuralDoors[0].ID)
	after, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Empty(after.Atlas.StructuralWalls)
	s.Equal(body.StructuralDoors, after.Atlas.StructuralDoors)
	// Discovery is by cell (rpg-project#586): once the secret is found, the
	// sighted leaf teaches the cell it covers. Whatever floor the find adds
	// arrives on the beats — the prior map plus the reveals is the fresh map.
	patched := append(append([]spatial.Position{}, before.Atlas.Cells...), body.Cells...)
	for _, reveal := range beatsNamed(told, encounter.BeatCellsRevealed) {
		var cells structuralCellsReveal
		reveal.decode(s.T(), &cells)
		patched = append(patched, cells.Scenery...)
		for _, region := range cells.Regions {
			patched = append(patched, region.Cells...)
		}
	}
	s.ElementsMatch(after.Atlas.Cells, patched, "each new cell arrives once, on a beat")
	known := make(map[spatial.Position]bool, len(before.Atlas.Cells))
	for _, cell := range before.Atlas.Cells {
		known[cell] = true
	}
	var added []spatial.Position
	for _, cell := range after.Atlas.Cells {
		if !known[cell] {
			added = append(added, cell)
		}
	}
	s.Equal([]spatial.Position{hexCell(4, 0)}, added,
		"the find adds the leaf's own cell and nothing of the hidden parent's floor")
}

func (s *StructuralSessionSuite) TestKnownWallOpeningReplacementMatchesSnapshotAndReplay() {
	ctx := context.Background()
	s.startWith(structuralDoorOnlyWorld(), sharpEyed("alice"))
	in := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	before, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Require().Len(before.Atlas.StructuralWalls, 1)
	s.Empty(before.Atlas.StructuralWalls[0].Openings)
	s.Empty(before.Atlas.StructuralDoors)

	mark := lastSeq(storyBeats(s.T(), s.mgr, "sess", "alice"))
	_, err = s.find(ctx, "alice")
	s.Require().NoError(err)
	body := s.onlyReveal(storyBeatsAfter(s.T(), s.mgr, "sess", "alice", mark))
	s.Empty(body.StructuralWalls, "a known wall's unchanged fixed fields are not resent")
	s.Require().Len(body.StructuralWallOpeningsReplacements, 1)
	s.Equal(structWallPresence, body.StructuralWallOpeningsReplacements[0].WallID)
	s.Require().Len(body.StructuralDoors, 1)
	s.Empty(body.Cells, "door-only discovery adds no unselected floor")

	after, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	patched := before.Atlas.StructuralWalls[0]
	patched.Openings = body.StructuralWallOpeningsReplacements[0].Openings
	s.Equal(after.Atlas.StructuralWalls[0], patched)
	s.Equal(after.Atlas.StructuralDoors, body.StructuralDoors)
	s.Equal(before.Atlas.Cells, after.Atlas.Cells)

	reloaded, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Equal(after.Atlas.StructuralWalls, reloaded.Atlas.StructuralWalls)
	s.Equal(after.Atlas.StructuralDoors, reloaded.Atlas.StructuralDoors)

	s.Require().Len(body.StructuralWallOpeningsReplacements[0].Openings, 1)
	s.Equal("gap", body.StructuralWallOpeningsReplacements[0].Openings[0].ID)
}
