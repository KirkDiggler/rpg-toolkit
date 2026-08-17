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
	mgr, err := session.NewManager(&session.Config{Dice: testDice{},
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: encOrderAsGiven{},
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

// TestAWalkCrossesTheDoorway is what the two Traverse tests here became.
//
// hexWorld's gate joins corridor-local (2,0) — absolute (2,0), the corridor
// being anchored at the origin — to vault-local (-3,0), absolute (3,0). One
// Move walks alice to the threshold and through it, and the far side is just
// the next cell in the path.
func (s *MoveTestSuite) TestAWalkCrossesTheDoorway() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}},
	})
	s.Require().NoError(err, "the doorway is a step like any other")
	s.Require().Len(out.Steps, 3, "two in the corridor, one through the gate")
	s.Equal(spatial.Position{X: 3, Y: 0}, out.Steps[2].Position, "she is on the far side")
	s.NotZero(out.Steps[2].Seq)
}

// TestAStepWithNoDoorwayIsRefused pins the sentinel that replaced
// ErrNoConnection's role here.
//
// A caller can no longer name a connection, so "no such connection" is not a
// mistake it can make. What it CAN do is name a cell in the next room with
// nothing joining it to where the walker stands — and that has its own
// refusal, because two rooms touching without a door is a thing W2 allows and
// a client reading only the map's cells cannot see.
func (s *MoveTestSuite) TestAStepWithNoDoorwayIsRefused() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	// Alice walks to the threshold itself.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}},
	})
	s.Require().NoError(err)

	// (3,-1) is a vault cell and an axial neighbour of the threshold she is
	// standing on — adjacent, in the next room, and joined to nothing. The
	// doorway is at (3,0), one cell over.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{{X: 3, Y: -1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoCrossing)
	s.NotErrorIs(err, session.ErrBrokenPath, "the cells ARE adjacent — that is the point")
	s.NotErrorIs(err, session.ErrBadPosition, "and the cell exists; there is just no way through")
}

// TestAStepOntoNoCellUsesOurSentinelNotTheirs is
// TestTrimmedStoryUsesOurSentinelNotTheirs' walk-shaped twin, and it covers the
// leak channel a routine caller mistake reaches (rpg-toolkit#1058).
//
// Pathing one cell too far is arithmetic any client can get wrong — not corrupt
// state and not a defect here. Until this test existed the refusal carried the
// composition's own ErrBadPlacement alongside our ErrBadPosition, matchable by
// errors.Is, so a host handling "off the map" would have been coupled to the
// module this seam exists to keep replaceable — through the one channel the AST
// boundary test cannot see.
func (s *MoveTestSuite) TestAStepOntoNoCellUsesOurSentinelNotTheirs() {
	s.startCorridor()

	// The hall runs (0,0)-(7,7). Alice steps to its corner, then off the map.
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 0, Y: 0}, {X: -1, Y: -1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition, "the caller sees our vocabulary")
	s.NotErrorIs(err, encounter.ErrBadPlacement,
		"and must NOT be able to reach the composition's sentinel — a host that matched "+
			"on it would break the day the composition is replaced")

	// Demoting the inner error must cost its MESSAGE nothing: whoever debugs
	// this still needs to know it was owned by no room rather than, say, a
	// fractional hex cell.
	s.Contains(err.Error(), "owned by no room",
		"the composition's account of why survives, even though its sentinel does not")
}

func TestMoveSuite(t *testing.T) {
	suite.Run(t, new(MoveTestSuite))
}
