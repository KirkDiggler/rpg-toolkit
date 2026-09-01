// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// onemap_test.go pins that the seam's typed surface speaks one map
// (rpg-toolkit#1046): a caller hands in cells, reads back cells, and never
// names a room while playing.
//
// Every fixture here anchors its room AWAY from the origin. A room at (0,0)
// makes local and absolute the same numbers, which is why the rest of this
// package's move and join tests pass unchanged through this reshape — they
// cannot tell the two frames apart, and neither could a bug.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type OneMapSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestOneMapSuite(t *testing.T) {
	suite.Run(t, new(OneMapSuite))
}

// offsetWorld is a single 6x6 region painted at [40,20], plus a second region
// beyond it so that "a cell in another region" is a thing a path can name.
//
// The door joins authored [45,22] to [46,22], the one open crossing on the
// seam. Every authored pair in this fixture is ABSOLUTE offset, and every
// cell a verb takes or reports is the axial one hexCell makes of it.
func offsetWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 40, 20, 6, 6),
				rectRegion("annex", 46, 20, 6, 6),
			},
			Walls: hexSeamWallsFrom(46, 20, 6, 22),
			Doors: []encounter.DoorInput{{
				ID:    "gate",
				Edges: []encounter.DoorEdge{{From: hexCell(45, 22), To: hexCell(46, 22)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 41, Y: 21}},
		},
		Endings: []encounter.EndingInput{
			// Fires where the walk below ends, so the outcome's own placement
			// report is exercised in the same scene.
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 44, Y: 21}}},
			// And one on the FAR SIDE of the doorway, for the pin that a
			// crossing is an ordinary step: an ending there must fire as the
			// crossing lands, inside the same Move.
			{Key: "beyond", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 46, Y: 22}}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the offset world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *OneMapSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: offsetWorld(s.T()),
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// TestAJoinIsAnsweredInTheSameCellItWasAskedFor is the slice's headline case.
//
// One cell goes in, and every surface that mentions the joiner afterwards
// answers with the same cell: the join's own report, the steps of a later
// walk, and the ending's final placement. A seam that projected in some places
// and not others would satisfy any single one of these.
func (s *OneMapSuite) TestAJoinIsAnsweredInTheSameCellItWasAskedFor() {
	ctx := context.Background()
	arrival := hexCell(41, 21) // authored [41,21], inside the hall

	joined, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: arrival,
	})
	s.Require().NoError(err)
	s.Equal(arrival, joined.Member.Position, "the join answers the cell it was given")

	// He walks two cells east, still inside the hall.
	walked, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob",
		Path: []spatial.Position{hexCell(42, 21), hexCell(43, 21)},
	})
	s.Require().NoError(err)
	s.Require().Len(walked.Steps, 2)
	s.Equal(hexCell(42, 21), walked.Steps[0].Position)
	s.Equal(hexCell(43, 21), walked.Steps[1].Position, "walked on the map, reported on it")

	// One more step lands on the stairs and closes the encounter, so the
	// outcome reports where everybody finished — on the map as well.
	ended, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob",
		Path: []spatial.Position{hexCell(44, 21)},
	})
	s.Require().NoError(err)
	s.Require().NotNil(ended.Outcome, "the stairs are underfoot")

	placements := map[string]spatial.Position{}
	for _, m := range ended.Outcome.Members {
		placements[m.ID] = m.Position
	}
	s.Equal(hexCell(44, 21), placements["bob"], "bob finished on the stairs")
	s.Equal(hexCell(41, 21), placements["alice"], "alice never moved from her cell")
}

// TestAWalkCrossesTheDoorway is what four slices of reshaping were for, and it
// replaces the pin that used to say the opposite.
//
// TestAWalkStillDoesNotCrossADoorway stood here through #1046, refusing this
// exact path so that the change would have to be made ON PURPOSE rather than
// inherited when the coordinates stopped preventing it. This is that purpose:
// one Move call, out of the hall, through the gate, into the annex.
func (s *OneMapSuite) TestAWalkCrossesTheDoorway() {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{
			hexCell(42, 22), hexCell(43, 22), hexCell(44, 22), hexCell(45, 22), // the hall
			hexCell(46, 22), // through the doorway
		},
	})
	s.Require().NoError(err, "the doorway is a step like any other")

	s.Require().Len(out.Steps, 5)
	s.Equal(hexCell(45, 22), out.Steps[3].Position, "the threshold")
	s.Equal(hexCell(46, 22), out.Steps[4].Position, "the far side, in the annex")

	// A crossing is a step, so an ending declared on the arrival cell fires as
	// it lands — in THIS Move's own output, not on the next call.
	s.Require().NotNil(out.Outcome, "the ending on the far side fired underfoot")
	s.Equal("beyond", out.Outcome.Ending)
}

// TestACrossingReachesClientsAsACrossing: one map does not mean one narration.
//
// The step that changed rooms is still a distinguishable beat, so a client can
// narrate a doorway differently from a corridor — which is the whole reason the
// traversed event kind survives a reshape that removed rooms from everything
// else a client sees.
func (s *OneMapSuite) TestACrossingReachesClientsAsACrossing() {
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: stream,
	})
	s.Require().NoError(err)

	_, err = mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{
			hexCell(42, 22), hexCell(43, 22), hexCell(44, 22), hexCell(45, 22), hexCell(46, 22),
		},
	})
	s.Require().NoError(err)

	kinds := map[session.EventKind]int{}
	for _, e := range stream.published {
		kinds[e.Kind]++
	}
	s.Positive(kinds[session.EventMoved], "the cells inside the hall are moves")
	// The step through the gate is an ordinary move now, not its own kind: the
	// composition stopped narrating crossings separately once a crossing became
	// an ordinary step (rpg-toolkit#1048). A client that wants to draw a door
	// matches the step against Atlas.Doorways, which still lists every pair.
	s.Zero(kinds[session.EventUnknown], "with nothing unnameable in between")
}

// TestARefusalIsDescribedInTheCallersOwnCoordinates.
//
// A broken path is reported with the cell the caller wrote, at both ends. The
// message used to mix frames — the "from" was room-local and the "to" was
// absolute — which in this world differs by (40,20) and would send whoever
// read it looking for a cell nobody named.
func (s *OneMapSuite) TestARefusalIsDescribedInTheCallersOwnCoordinates() {
	// Alice stands at [41,21]. A jump three cells away is not a walk.
	from, to := hexCell(41, 21), hexCell(44, 24)
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{to},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, session.ErrBrokenPath)

	s.Contains(err.Error(), fmt.Sprintf("from (%v,%v)", from.X, from.Y), "the cell she stands on, on the map")
	s.Contains(err.Error(), fmt.Sprintf("to (%v,%v)", to.X, to.Y), "the cell the caller asked for, as the caller wrote it")
}

// TestAWalkIntoTheVoidIsRefused: a cell no room owns is not a step, and the
// refusal names the same sentinel as the doorway case — both are "that is not
// a cell you can walk to from here".
func (s *OneMapSuite) TestAWalkIntoTheVoidIsRefused() {
	ctx := context.Background()

	// To the hall's west edge first: the void has to be ADJACENT for this to
	// be the refusal under test. A cell far away is refused earlier, as a
	// path that is not a walk, which is a different mistake and is checked
	// first.
	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{hexCell(40, 21)},
	})
	s.Require().NoError(err)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{hexCell(39, 21)},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition, "no room owns that cell")
}

// TestNothingOnThePlaySurfaceNamesARoom checks the claim structurally rather
// than by reading the diff.
//
// The types are the contract: a room id cannot come back through an input a
// caller fills in or an output a client renders, because there is nowhere to
// put one. Authoring is deliberately not on this list — StartSessionInput
// carries an authored world, which is construction data and still speaks
// rooms.
func (s *OneMapSuite) TestNothingOnThePlaySurfaceNamesARoom() {
	for _, shape := range []any{
		session.JoinInput{}, session.SpawnInput{}, session.MoveInput{},
		session.Member{}, session.MemberOutcome{}, session.Step{},
		session.Atlas{}, session.AtlasDoorway{},
	} {
		for _, field := range fieldsOf(shape) {
			s.NotEqual("Room", field, "%T still names a room", shape)
			s.NotEqual("FromRoom", field, "%T still names a room", shape)
			s.NotEqual("ToRoom", field, "%T still names a room", shape)
		}
	}

	// And the composition's own room-shaped types are not reachable from here
	// either, which the boundary test already guards — this only states the
	// half that is about rooms specifically.
	s.NotContains(fieldsOf(session.Member{}), "Room")
	s.Contains(fieldsOf(session.Member{}), "Position")
}

// TestASightingIsReportedOnTheMap is #1053's own case, and the last frame this
// seam had left to converge.
//
// A sighting's payload is opaque bytes here — the session hands them through
// without reading them, because intel's testimony is the composition's
// encoding, not this package's — so the pin decodes them the way a client
// does. Two halves, and both matter: the cell is the one the Atlas names for
// that spot, and there is no room key to reinterpret it by. A payload carrying
// hall-local (2,1) would put bob at a cell no room in this world contains,
// while looking exactly as plausible.
func (s *OneMapSuite) TestASightingIsReportedOnTheMap() {
	ctx := context.Background()
	bobsCell := hexCell(42, 21) // authored [42,21], inside the hall

	_, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: bobsCell,
	})
	s.Require().NoError(err)

	seen, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	var payload map[string]any
	for _, sighting := range seen {
		if sighting.Subject != "bob" {
			continue
		}
		s.Require().NoError(json.Unmarshal(sighting.Payload, &payload))
	}
	s.Require().NotNil(payload, "alice and bob share an open hall in plain sight")

	s.Equal(map[string]any{"state": string(encounter.LocationKnown), "x": bobsCell.X, "y": bobsCell.Y}, payload,
		"bob's cell on the dungeon map — authored [42,21] as one axial cell")
	s.NotContains(payload, "room", "a sighting names no room; there is one map")

	atlas, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess"})
	s.Require().NoError(err)
	s.Contains(atlas.Cells, bobsCell,
		"the sighted cell is a cell of the map the same client renders")
}

// TestASightingAndAPlacementAgree crosses the two reads a client puts on
// screen together. Where bob IS and where alice SEES bob are answered by
// different paths — a join's own report and an intel payload — and a client
// draws them on one canvas.
func (s *OneMapSuite) TestASightingAndAPlacementAgree() {
	ctx := context.Background()
	joined, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(43, 22),
	})
	s.Require().NoError(err)

	seen, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	var sighted spatial.Position
	found := false
	for _, sighting := range seen {
		if sighting.Subject != "bob" {
			continue
		}
		var payload struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		s.Require().NoError(json.Unmarshal(sighting.Payload, &payload))
		sighted, found = spatial.Position{X: payload.X, Y: payload.Y}, true
	}
	s.Require().True(found)

	s.Equal(joined.Member.Position, sighted,
		"the cell bob joined at is the cell alice sees him on")
}

// The three tests below pin that a cell crosses this seam CONVERTED ONCE.
//
// They were written against anchors: a room-local cell projected through its
// room's origin, where a second projection of an already-absolute cell could
// succeed and land somewhere else entirely (#1072). Rooms and origins are
// gone (rpg-project#256), and the conversion that remains is offset-to-axial
// at the composition's construction. The same shape of mistake is still
// available — converting an axial cell as though it were an authored offset
// pair — and it is invisible in a world painted at the origin's first two
// rows, where the two frames agree. So this world is painted a few rows down,
// where they do not, and every cell a verb reports is compared against the
// ONE conversion the fixture's hexCell does.

// shallowAnchoredWorld is one 10x10 region painted at [2,3], off the origin
// by less than its own span.
func shallowAnchoredWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 2, 3, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 3}},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 5, Y: 3}}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the shallow-anchored world: %v", err)
	}
	data := enc.ToData()
	return &data
}

// shallowSession starts a session on that world. Alice stands on authored
// [2,3] and the stairs are at authored [5,3], three cells east along row 3.
func (s *OneMapSuite) shallowSession() *session.Manager {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "shallow", Encounter: "shallow-world", World: shallowAnchoredWorld(s.T()),
	})
	s.Require().NoError(err)
	return mgr
}

// TestAnOutcomeIsNotAnchoredTwice: the ending's own member list.
func (s *OneMapSuite) TestAnOutcomeIsNotAnchoredTwice() {
	out, err := s.shallowSession().Move(context.Background(), &session.MoveInput{
		Session: "shallow", Member: "alice",
		Path: []spatial.Position{hexCell(3, 3), hexCell(4, 3), hexCell(5, 3)},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome, "the stairs are underfoot")
	s.Require().Len(out.Outcome.Members, 1)

	s.Equal(hexCell(5, 3), out.Outcome.Members[0].Position,
		"the cell she finished on — converting it a second time would name another")
}

// TestAWalkIsNotAnchoredTwice is the standing probe (#1072) aimed at the walk
// itself rather than at what it ends in.
//
// The walk path is where a second conversion mattered longest: it was the
// last place a room-local cell crossed this seam in either direction.
func (s *OneMapSuite) TestAWalkIsNotAnchoredTwice() {
	out, err := s.shallowSession().Move(context.Background(), &session.MoveInput{
		Session: "shallow", Member: "alice",
		Path: []spatial.Position{hexCell(3, 3), hexCell(4, 3)},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 2)

	s.Equal(hexCell(3, 3), out.Steps[0].Position,
		"the cell asked for — converting it a second time would name another")
	s.Equal(hexCell(4, 3), out.Steps[1].Position, "and the next one")
}

// TestAnExitIsNotAnchoredTwice: the leaver's own report, which the composition
// builds on its own path and which reaches a client through the same
// converter. Two shapes carry an outcome across this seam, and a pin on one
// says nothing about the other.
func (s *OneMapSuite) TestAnExitIsNotAnchoredTwice() {
	left, err := s.shallowSession().Exit(context.Background(), &session.ExitInput{
		Session: "shallow", Member: "alice",
	})
	s.Require().NoError(err)

	s.Equal(hexCell(2, 3), left.Outcome.Position,
		"the cell she left from — converting it a second time would name another")
}
