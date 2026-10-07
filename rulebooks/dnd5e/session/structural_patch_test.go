// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func structuralDoorOnlyWorld(t fataler, extra ...encounter.ConcealmentInput) *encounter.EncounterData {
	centre := hallPlane().CellCentre(hexCell(4, 0))
	point := func(dx float64) spatial.Point { return spatial.Point{X: centre.X + dx, Y: centre.Y} }
	wallBox := structuralBox(hexCell(4, 0), 6, 0.25)
	doorBox := structuralBox(hexCell(4, 0), 2, 0.25)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Equipment: encounter.UnobservedEquipment{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		CheckResolver: encNeverResolves{}, Witness: encNeverWitnesses{},
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
		Members: []encounter.MemberInput{{ID: "alice", Kind: encounter.KindPlayer, Position: cell(1, 1)}},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("structural door-only world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *StructuralSessionSuite) TestIndependentDoorIntroductionDoesNotRequireItsHiddenParent() {
	ctx := context.Background()
	world := structuralDoorOnlyWorld(s.T(), encounter.ConcealmentInput{
		ID: "hidden-parent", Props: []encounter.PropID{structWallPresence},
		Checks: []encounter.CheckApproach{{Ability: "perception", DC: 100}},
	})
	s.startWith(world, sharpEyed("alice"))
	in := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	before, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Empty(before.Atlas.StructuralWalls)
	s.Empty(before.Atlas.StructuralDoors)
	s.stream.published = nil
	_, err = s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)
	reveals := eventsOfKind(s.stream.published, "alice", session.EventConcealmentRevealed)
	s.Require().Len(reveals, 1)
	body := reveals[0].Body.(session.ConcealmentRevealedBody)
	s.Empty(body.StructuralWalls)
	s.Empty(body.StructuralWallOpeningsReplacements, "a hidden parent must not be named by a patch")
	s.Require().Len(body.StructuralDoors, 1)
	s.Equal(structDoorID, body.StructuralDoors[0].ID)
	after, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Empty(after.Atlas.StructuralWalls)
	s.Equal(body.StructuralDoors, after.Atlas.StructuralDoors)
	s.Equal(before.Atlas.Cells, after.Atlas.Cells)
	replay, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice", FromSeq: before.Seq + 1})
	s.Require().NoError(err)
	s.Equal(eventsFor(s.stream.published, "alice"), replay)
}

func (s *StructuralSessionSuite) TestKnownWallOpeningReplacementMatchesSnapshotAndReplay() {
	ctx := context.Background()
	s.startWith(structuralDoorOnlyWorld(s.T()), sharpEyed("alice"))
	in := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	before, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Require().Len(before.Atlas.StructuralWalls, 1)
	s.Empty(before.Atlas.StructuralWalls[0].Openings)
	s.Empty(before.Atlas.StructuralDoors)

	s.stream.published = nil
	_, err = s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)
	reveals := eventsOfKind(s.stream.published, "alice", session.EventConcealmentRevealed)
	s.Require().Len(reveals, 1)
	body, ok := reveals[0].Body.(session.ConcealmentRevealedBody)
	s.Require().True(ok)
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

	replay, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice", FromSeq: before.Seq + 1})
	s.Require().NoError(err)
	s.Equal(eventsFor(s.stream.published, "alice"), replay)
	reloaded, err := s.mgr.Knowledge(ctx, in)
	s.Require().NoError(err)
	s.Equal(after.Atlas.StructuralWalls, reloaded.Atlas.StructuralWalls)
	s.Equal(after.Atlas.StructuralDoors, reloaded.Atlas.StructuralDoors)

	s.Require().Len(body.StructuralWallOpeningsReplacements[0].Openings, 1)
	body.StructuralWallOpeningsReplacements[0].Openings[0].ID = "consumer-edit"
	replayAgain, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice", FromSeq: before.Seq + 1})
	s.Require().NoError(err)
	replayedReveals := eventsOfKind(replayAgain, "alice", session.EventConcealmentRevealed)
	s.Require().Len(replayedReveals, 1)
	replayedBody := replayedReveals[0].Body.(session.ConcealmentRevealedBody)
	s.Equal("gap", replayedBody.StructuralWallOpeningsReplacements[0].Openings[0].ID,
		"a consumer cannot mutate persisted event payloads through a DTO slice")
}
