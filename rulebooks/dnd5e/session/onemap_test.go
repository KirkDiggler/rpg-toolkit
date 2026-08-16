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

// offsetWorld is a single 6x6 room anchored at (40,20), plus a second room
// beyond it so that "a cell in another room" is a thing a path can name.
//
// The doorway joins hall-local (5,2) — absolute (45,22) — to annex-local (0,2)
// — absolute (46,22).
func offsetWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: encOrderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "hall", Width: 6, Height: 6, Origin: spatial.Position{X: 40, Y: 20}},
				{ID: "annex", Width: 6, Height: 6, Origin: spatial.Position{X: 46, Y: 20}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "hall", To: "annex",
				FromPosition: spatial.Position{X: 5, Y: 2},
				ToPosition:   spatial.Position{X: 0, Y: 2},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			// Fires where the walk below ends, so the outcome's own placement
			// report is exercised in the same scene.
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Room: "hall", Position: spatial.Position{X: 4, Y: 1}}},
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
		Dice: testDice{}, Sessions: s.sessions, Encounters: s.encounters,
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
	arrival := spatial.Position{X: 41, Y: 21} // hall-local (1,1)

	joined, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: arrival,
	})
	s.Require().NoError(err)
	s.Equal(arrival, joined.Member.Position, "the join answers the cell it was given")

	// He walks two cells east, still inside the hall.
	walked, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob",
		Path: []spatial.Position{{X: 42, Y: 21}, {X: 43, Y: 21}},
	})
	s.Require().NoError(err)
	s.Require().Len(walked.Steps, 2)
	s.Equal(spatial.Position{X: 42, Y: 21}, walked.Steps[0].Position)
	s.Equal(spatial.Position{X: 43, Y: 21}, walked.Steps[1].Position, "walked on the map, reported on it")

	// One more step lands on the stairs and closes the encounter, so the
	// outcome reports where everybody finished — on the map as well.
	ended, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob",
		Path: []spatial.Position{{X: 44, Y: 21}},
	})
	s.Require().NoError(err)
	s.Require().NotNil(ended.Outcome, "the stairs are underfoot")

	placements := map[string]spatial.Position{}
	for _, m := range ended.Outcome.Members {
		placements[m.ID] = m.Position
	}
	s.Equal(spatial.Position{X: 44, Y: 21}, placements["bob"], "bob finished on the stairs")
	s.Equal(spatial.Position{X: 41, Y: 21}, placements["alice"], "alice never moved from her cell")
}

// TestAWalkStillDoesNotCrossADoorway is the deliberate NON-change, pinned so
// that the slice which makes a crossing an ordinary step has to come here and
// change a test on purpose.
//
// Absolute coordinates make the crossing EXPRESSIBLE for the first time — the
// far side of the doorway is simply the next cell along — and it is still
// refused. That is the difference between changing a dialect and changing a
// rule, and only one of them is happening in this slice.
func (s *OneMapSuite) TestAWalkStillDoesNotCrossADoorway() {
	ctx := context.Background()

	// Alice walks to the threshold, which is allowed…
	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 42, Y: 21}, {X: 43, Y: 22}, {X: 44, Y: 22}, {X: 45, Y: 22}},
	})
	s.Require().NoError(err, "the threshold is in her own room")

	// …and then tries to step through it, which is not.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 46, Y: 22}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition, "the far side of a doorway is another room, and a walk is one room")
}

// TestARefusalIsDescribedInTheCallersOwnCoordinates.
//
// A broken path is reported with the cell the caller wrote, at both ends. The
// message used to mix frames — the "from" was room-local and the "to" was
// absolute — which in this world differs by (40,20) and would send whoever
// read it looking for a cell nobody named.
func (s *OneMapSuite) TestARefusalIsDescribedInTheCallersOwnCoordinates() {
	// Alice stands at (41,21). A jump three cells away is not a walk.
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 44, Y: 24}},
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, session.ErrBrokenPath)

	s.Contains(err.Error(), "from (41,21)", "the cell she stands on, on the map")
	s.Contains(err.Error(), "to (44,24)", "the cell the caller asked for, as the caller wrote it")
}

// TestAWalkIntoTheVoidIsRefused: a cell no room owns is not a step, and the
// refusal names the same sentinel as the doorway case — both are "that is not
// a cell you can walk to from here".
func (s *OneMapSuite) TestAWalkIntoTheVoidIsRefused() {
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 900, Y: 900}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition)
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
