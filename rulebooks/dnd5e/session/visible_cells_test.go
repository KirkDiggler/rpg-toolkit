// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// VisibleCellsSuite pins fog of war at the session seam (rpg-project#586,
// Task 2): the view carries each seat's own seen-now answer through
// unchanged, and the per-recipient stream delivers the composition's
// per-cell reveal as its own event. The session neither observes nor computes
// either; these tests hold the delivery, not the sight rules.
type VisibleCellsSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestVisibleCellsSuite(t *testing.T) { suite.Run(t, new(VisibleCellsSuite)) }

func (s *VisibleCellsSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters()
	s.stream = &fakeStream{}
	s.mgr = s.manager()
}

// manager is a fresh Manager over the suite's stores — a second one is a
// restart of the host process with the same repositories.
func (s *VisibleCellsSuite) manager() *session.Manager {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *VisibleCellsSuite) startWith(world scene) {
	stockAuthoredPlayers(world, s.characters)
	launchScene(s.T(), s.mgr, world)
}

// twoSeatWorld is hexWorld with bob standing deep in the vault, out of
// alice's sight through the gate and she out of his.
func twoSeatWorld() scene {
	world := hexWorld()
	world.Party = append(world.Party, seatAt("bob", 10, 4))
	return world
}

// closetWorld seats carol in a one-cell room the void walls off: there is
// nothing for her to see but where she stands.
func closetWorld() scene {
	return scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 3, 3), rectRegion("closet", 6, 0, 1, 1)},
		},
		Party:   []sceneSeat{seatAt("alice", 1, 1), seatAt("carol", 6, 0)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
}

func (s *VisibleCellsSuite) view(member string) *session.ViewOutput {
	view, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return view
}

func (s *VisibleCellsSuite) where(member string) spatial.Position {
	where, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return where.Position
}

func (s *VisibleCellsSuite) atlas(member string) *session.Atlas {
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return atlas
}

// TestViewCarriesVisibleCellsForTheCallerOnly: two seats, two answers. Each
// view holds the viewer's own cell and not the other seat's, each is a subset
// of that viewer's own atlas, and the two differ — seen now is never a party
// union.
func (s *VisibleCellsSuite) TestViewCarriesVisibleCellsForTheCallerOnly() {
	s.startWith(twoSeatWorld())

	alice, bob := s.view("alice"), s.view("bob")
	s.Contains(alice.VisibleCells, s.where("alice"), "a placed seat always sees where it stands")
	s.Contains(bob.VisibleCells, s.where("bob"))
	s.NotContains(alice.VisibleCells, s.where("bob"), "bob's corner of the vault is not alice's to see")
	s.NotContains(bob.VisibleCells, s.where("alice"))
	s.Subset(s.atlas("alice").Cells, alice.VisibleCells, "seen now is a subset of what alice knows")
	s.Subset(s.atlas("bob").Cells, bob.VisibleCells, "and of what bob knows")
	s.NotEqual(alice.VisibleCells, bob.VisibleCells, "two seats, two answers")
}

// TestKnowledgeViewAndAtlasAgree: the restore snapshot carries the same
// seen-now answer the view read does, and every one of its cells is in the
// snapshot's own atlas.
func (s *VisibleCellsSuite) TestKnowledgeViewAndAtlasAgree() {
	s.startWith(twoSeatWorld())

	known, err := s.mgr.Knowledge(context.Background(),
		&session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(known.View.VisibleCells)
	s.Equal(s.view("alice").VisibleCells, known.View.VisibleCells, "the snapshot and the view are one answer")
	s.Subset(known.Atlas.Cells, known.View.VisibleCells, "every seen cell is a known cell")
}

// TestASeatThatSeesNothingCarriesItsOwnCell: a seat with nothing in sight
// still answers its own cell, so a host may read an empty list as "not sent"
// (plan Task 4) — and the wire key is present, never null.
func (s *VisibleCellsSuite) TestASeatThatSeesNothingCarriesItsOwnCell() {
	s.startWith(closetWorld())

	carol := s.view("carol")
	s.Equal([]spatial.Position{s.where("carol")}, carol.VisibleCells)
	raw, err := json.Marshal(carol)
	s.Require().NoError(err)
	var fields map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &fields))
	s.Require().Contains(fields, "visible_cells")
	s.NotEqual("null", string(fields["visible_cells"]))
}

// TestAStepThatRevealsCellsReachesOnlyTheStepper: one step that brings new
// cells into alice's sight emits one cells-revealed event, to alice alone,
// carrying exactly the cells her atlas gained — so a client applying it to
// the prior atlas lands on the fresh one. bob, who learned nothing, hears no
// reveal; and the whole-room event is never produced.
func (s *VisibleCellsSuite) TestAStepThatRevealsCellsReachesOnlyTheStepper() {
	s.startWith(twoSeatWorld())
	before := s.atlas("alice")

	s.stream.published = nil
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(1, 0)}})
	s.Require().NoError(err)

	for _, event := range s.stream.published {
		s.NotEqual(session.EventRoomRevealed, event.Kind, "the whole-room event is no longer produced")
		if event.Kind == session.EventCellsRevealed {
			s.Equal("alice", event.Recipient, "a cells reveal reaches its learner alone")
		}
	}
	reveals := eventsOfKind(s.stream.published, "alice", session.EventCellsRevealed)
	s.Require().Len(reveals, 1, "one step, one sight pass, one reveal")
	body, ok := reveals[0].Body.(session.CellsRevealedBody)
	s.Require().True(ok)

	gained := append([]spatial.Position{}, body.Scenery...)
	for _, region := range body.Regions {
		gained = append(gained, region.Cells...)
	}
	after := s.atlas("alice")
	s.ElementsMatch(after.Cells, append(append([]spatial.Position{}, before.Cells...), gained...),
		"prior atlas plus the reveal is the fresh atlas")
	s.Subset(after.Cells, s.view("alice").VisibleCells)
}

// TestKnownCellsSurviveARestart: discovery lands in the stored world as the
// composition's per-cell journal fact, and a fresh Manager over the same
// repositories answers the same atlas and the same seen-now cells.
func (s *VisibleCellsSuite) TestKnownCellsSurviveARestart() {
	s.startWith(twoSeatWorld())
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(1, 0)}})
	s.Require().NoError(err)
	atlas, view := s.atlas("alice"), s.view("alice")

	stored, err := json.Marshal(s.encounters.byID["sess"])
	s.Require().NoError(err)
	s.Contains(string(stored), `"known:cells"`, "discovery is stored as the per-cell fact")

	s.mgr = s.manager()
	s.Equal(atlas, s.atlas("alice"), "a restart restores what alice learned")
	s.Equal(view.VisibleCells, s.view("alice").VisibleCells, "and derives the same seen-now answer")
}
