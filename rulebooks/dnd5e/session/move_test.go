// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MoveTestSuite covers walking a path and crossing a doorway — the first verbs
// that do something the composition cannot do alone.
type MoveTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func (s *MoveTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// corridorWorld is a square room with alice at (1,1) and an ending at (4,1),
// three steps to her east — near enough to walk onto, far enough that a walk
// can be interrupted before reaching it.
func corridorWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Room: "hall", Position: spatial.Position{X: 4, Y: 1},
			}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building corridor world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *MoveTestSuite) startCorridor() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: corridorWorld(s.T()),
	})
	s.Require().NoError(err)
}

// TestWalksEveryCellOfThePath is the property the composition cannot provide.
//
// Its single-hop Move would report one movement from (1,1) to (3,1); the walk
// must report two, having actually stood in (2,1) on the way. Anything that
// fires because a member ENTERED a particular cell depends on this being true,
// so it is asserted per step rather than on the destination.
func (s *MoveTestSuite) TestWalksEveryCellOfThePath() {
	s.startCorridor()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 2, "each cell of the path is entered in turn")
	s.Equal(spatial.Position{X: 2, Y: 1}, out.Steps[0].Position)
	s.Equal(spatial.Position{X: 3, Y: 1}, out.Steps[1].Position)
	s.Less(out.Steps[0].Seq, out.Steps[1].Seq, "each step is its own recorded beat")
}

// TestEndingUnderfootStopsTheWalk is scene 6.
//
// Alice is handed a four-step path whose third cell is the exit. She takes
// three steps, the encounter closes, and steps four and five never happen. The
// caller learns from the LENGTH of Steps, not from an error — the movement that
// happened is exactly what was asked for, up to the point the world changed.
func (s *MoveTestSuite) TestEndingUnderfootStopsTheWalk() {
	s.startCorridor()
	ctx := context.Background()

	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{
			{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}, // the exit
			{X: 5, Y: 1}, {X: 6, Y: 1}, // never walked
		},
	})
	s.Require().NoError(err, "a walk cut short by an ending is not an error")
	s.Require().Len(out.Steps, 3, "the walk stopped where the world did")
	s.Equal(spatial.Position{X: 4, Y: 1}, out.Steps[2].Position)

	s.Require().NotNil(out.Outcome, "and the reason is reported")
	s.Equal("stairs", out.Outcome.Ending)

	// The abandoned steps really did not happen: the encounter is closed and
	// alice is at the exit, not two cells past it.
	status, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.False(status.Open)
}

// TestBrokenPathIsRejectedWhole pins R5 at this seam.
//
// A path whose second step teleports is refused entirely — the member does not
// take the legal first step and stop at the gap. A caller who mis-computed a
// route wants none of it, not a prefix leaving the party somewhere nobody chose.
func (s *MoveTestSuite) TestBrokenPathIsRejectedWhole() {
	s.startCorridor()
	ctx := context.Background()

	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 6, Y: 1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBrokenPath)

	// She has not moved at all.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err, "alice must still be adjacent to (2,1), i.e. still at (1,1)")
	s.Len(out.Steps, 1)
}

// TestFirstStepMustBeReachable catches the version of the bug that only shows
// up at the start of a walk: a path whose cells are adjacent to each other but
// whose first cell is nowhere near the member.
//
// A validator checking only consecutive pairs passes this happily, and the
// member teleports on step one.
func (s *MoveTestSuite) TestFirstStepMustBeReachable() {
	s.startCorridor()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 6, Y: 6}, {X: 6, Y: 5}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBrokenPath,
		"the first cell must be adjacent to where the member actually stands")
}

// TestSingleCellPathIsLegal is the must-accept row: the degenerate walk is how
// the game moves today, and a validator over-tightened into demanding multiple
// cells would break every ordinary step.
func (s *MoveTestSuite) TestSingleCellPathIsLegal() {
	s.startCorridor()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Len(out.Steps, 1)
}

// TestDiagonalIsAdjacentOnSquare is the other must-accept row, and it is
// grid-family-specific on purpose.
//
// On a square grid a diagonal step is adjacent. A validator that used
// orthogonal-only adjacency would reject half of all legal movement while
// passing every straight-line test in this file.
func (s *MoveTestSuite) TestDiagonalIsAdjacentOnSquare() {
	s.startCorridor()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 2}},
	})
	s.Require().NoError(err, "a diagonal step is adjacent on a square grid")
	s.Len(out.Steps, 1)
}

// TestHexAdjacencyUsesCubeDistance is the discriminating hex case.
//
// Axial (1,1) is cube distance 2 from the origin — two steps, not one — while
// Chebyshev distance calls it 1. Substituting the square formula for the hex one
// therefore passes almost every hex fixture and fails only here. It is a
// previously-shipped defect class in this codebase, which is why adjacency is
// delegated to spatial's own grid rather than hand-rolled.
func (s *MoveTestSuite) TestHexAdjacencyUsesCubeDistance() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	// alice is at axial (0,0) in the corridor. (1,1) is cube distance 2.
	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "hex", Member: "alice", Path: []spatial.Position{{X: 1, Y: 1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBrokenPath,
		"axial (1,1) is two hex steps away; only Chebyshev would call it adjacent")

	// And a genuine hex neighbour is accepted, so the check is not simply
	// rejecting everything.
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "hex", Member: "alice", Path: []spatial.Position{{X: 1, Y: 0}},
	})
	s.Require().NoError(err, "axial (1,0) is a real neighbour and must be walkable")
	s.Len(out.Steps, 1)
}

// TestEmptyPathIsRejected pins that a walk to nowhere is a caller mistake
// rather than a silent no-op, which would hide a route computation that
// produced nothing.
func (s *MoveTestSuite) TestEmptyPathIsRejected() {
	s.startCorridor()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: nil,
	})
	s.ErrorIs(err, session.ErrEmptyPath)
}

// TestWalkPersists checks the walk through a separate read, so a version that
// moved in memory and never saved cannot pass.
func (s *MoveTestSuite) TestWalkPersists() {
	s.startCorridor()
	ctx := context.Background()

	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}},
	})
	s.Require().NoError(err)

	// If the walk had not been saved, alice would still be at (1,1) and this
	// one-step path from (3,1) would be a broken path.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 3, Y: 2}},
	})
	s.Require().NoError(err, "the walk must have been persisted, not just reported")
	s.Len(out.Steps, 1)
}

// TestTraverseCrossesTheDoorway pins the crossing and its projection.
func (s *MoveTestSuite) TestTraverseCrossesTheDoorway() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	// Walk to the gate's own cell, then cross.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}},
	})
	s.Require().NoError(err)

	out, err := s.mgr.Traverse(ctx, &session.TraverseInput{
		Session: "hex", Member: "alice", Connection: "gate",
	})
	s.Require().NoError(err)
	s.Equal("corridor", out.FromRoom)
	s.Equal("vault", out.ToRoom)
	s.NotZero(out.Seq)
}

// TestTraverseRejectsUnknownConnection pins the sentinel translation.
func (s *MoveTestSuite) TestTraverseRejectsUnknownConnection() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = s.mgr.Traverse(ctx, &session.TraverseInput{
		Session: "hex", Member: "alice", Connection: "not-a-door",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoConnection)

	_, err = s.mgr.Traverse(ctx, &session.TraverseInput{Session: "hex", Member: "alice"})
	s.ErrorIs(err, session.ErrNoConnection, "an empty connection is refused before loading")
}

func TestMoveSuite(t *testing.T) {
	suite.Run(t, new(MoveTestSuite))
}
