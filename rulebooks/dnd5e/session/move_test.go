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
	stream     *fakeStream
	mgr        *session.Manager
}

func (s *MoveTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Dice: testDice{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// corridorWorld is a square room with alice at (1,1) and an ending at (4,1),
// three steps to her east — near enough to walk onto, far enough that a walk
// can be interrupted before reaching it.
func corridorWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
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
		Session: "hex", Member: "alice", Path: []spatial.Position{hexCell(1, 0)},
	})
	s.Require().NoError(err, "the next column along is a real neighbour and must be walkable")
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
		Path: []spatial.Position{
			hexCell(1, 0), hexCell(2, 0), hexCell(3, 0), hexCell(4, 0), hexCell(5, 0), hexCell(6, 0),
		},
	})
	s.Require().NoError(err, "the doorway is a step like any other")
	s.Require().Len(out.Steps, 6, "five along the corridor, one through the gate")
	s.Equal(hexCell(6, 0), out.Steps[5].Position, "she is on the far side, in the vault")
	s.NotZero(out.Steps[2].Seq)
}

// TestAStepWithNoDoorwayIsRefused pins the sentinel that replaced
// ErrNoConnection's role here.
//
// A caller can no longer name a connection, so "no such connection" is not a
// mistake it can make. What it CAN do is name a cell in the next room with
// nothing joining it to where the walker stands — two rooms touching without a
// door is a thing W2 allows, and a client reading only the map's cells cannot
// see it coming.
//
// That refusal USED TO HAVE ITS OWN SENTINEL. It no longer does: the
// composition answers a blocked step with a placement refusal carrying the
// reason as text, so "there is no cell there" and "there is no way there from
// here" now arrive as the same thing. The step is still refused — that is what
// this test protects — but a host can no longer tell a wall from a typo.
// rpg-toolkit#1135 is what would give the distinction a source again, and when
// it lands the assertion below should get sharper, not disappear.
func (s *MoveTestSuite) TestAStepWithNoDoorwayIsRefused() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	// Alice walks to a threshold on the seam — the corridor's last column,
	// ROW ONE, because the scene needs her standing somewhere that has a vault
	// neighbour which is not the doorway.
	//
	// She used to walk straight along row 0 to (5,0). That stopped working
	// when rpg-toolkit#1141 corrected the offset schemes: from (5,0) the ONLY
	// adjacent vault cell is now the doorway itself, so there is no wrong step
	// to take from there and the scenario cannot be built. Row 1 has one.
	//
	// The path is computed rather than eyeballed — a breadth-first walk over
	// the corridor's real cells, around the prop at local (1,1).
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{
			hexCell(1, 0), hexCell(2, 0), hexCell(3, 0), hexCell(3, 1), hexCell(4, 1), hexCell(5, 1),
		},
	})
	s.Require().NoError(err)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		// The vault's column 6, row 1: a real cell, a genuine neighbour of the
		// threshold she is standing on, and joined to it by nothing — the gate
		// is on row 0. (This cell is unchanged; only the threshold moved.)
		Path: []spatial.Position{hexCell(6, 1)},
	})
	s.Require().Error(err)
	s.NotErrorIs(err, session.ErrBrokenPath, "the cells ARE adjacent — that is the point")
	s.ErrorIs(err, session.ErrBadPosition,
		"currently the only answer available: the composition does not distinguish "+
			"a walled crossing from a cell that is not there (rpg-toolkit#1135)")
}

// TestAWalkComesBackThroughTheSameDoorway pins the direction a fixture will not
// reach by accident: the way BACK.
//
// A connection is declared with a From room and a To room, and every crossing
// scene in this package walks it the declared way. Match only that direction
// and every crossing test still passes while every door in the game becomes
// one-way — a surviving mutant found exactly that on the composition side
// (rpg-toolkit#1102), and after #1059 this walk runs through the same code a
// monster's pursuit does, so the two can no longer disagree about it.
//
// It runs on hexWorld rather than the offset world for a reason worth stating:
// the offset world declares an ending ON the annex side of its doorway, so a
// walk that crosses it closes the encounter on arrival and CANNOT step back.
// A round trip needs a world with nothing underfoot at the far end.
func (s *MoveTestSuite) TestAWalkComesBackThroughTheSameDoorway() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "hex", Encounter: "hexworld", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	// Out: the corridor owns authored columns 0..5 and the vault 6..11, and the
	// gate joins the corridor's last column to the vault's first on row 0 —
	// one step apart on the map, however far apart their columns read.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{
			hexCell(1, 0), hexCell(2, 0), hexCell(3, 0), hexCell(4, 0), hexCell(5, 0), hexCell(6, 0),
		},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 6, "the walk ran to the end: nothing here stops it")
	s.Require().Nil(out.Outcome, "and nothing ended underfoot")
	s.Equal(hexCell(6, 0), out.Steps[5].Position, "the far side, in the vault")

	// And back, against the direction the connection was declared in.
	back, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "hex", Member: "alice",
		Path: []spatial.Position{hexCell(5, 0)},
	})
	s.Require().NoError(err, "a doorway is crossable both ways")
	s.Require().Len(back.Steps, 1)
	s.Equal(hexCell(5, 0), back.Steps[0].Position)

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "hex", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(hexCell(5, 0), where.Position,
		"standing back on the corridor threshold, read cold")
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
	s.Contains(err.Error(), "is not floor",
		"the composition's account of why survives, even though its sentinel does not")
}

// TestAMidWalkRefusalMovesNobody guards the one thing that narrowed when the
// walk stopped resolving the map for itself (rpg-toolkit#1059).
//
// "A cell no room owns" used to be caught while the path was being resolved,
// before anybody moved. It is now caught as that step is TAKEN, because
// catching it earlier means the seam deciding what the composition decides.
// The caller must not be able to tell: a refused walk still moves nobody and
// tells nobody, because the encounter that moved in memory is discarded
// unsaved.
//
// So the assertions are about the WORLD, not the error. Alice is asked to take
// a legal step and then step off the map; the legal one really does execute
// against the in-memory encounter, and none of it may survive.
func (s *MoveTestSuite) TestAMidWalkRefusalMovesNobody() {
	s.startCorridor()
	ctx := context.Background()

	before, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.stream.published = nil // setup beats predate the walk

	// (1,1) → (0,0) is a real step. (-1,-1) is off the map.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 0, Y: 0}, {X: -1, Y: -1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition)

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 1, Y: 1}, where.Position,
		"she is where she started: the step that DID execute was discarded with the encounter")

	after, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Len(after, len(before), "no beat survived a walk that failed")
	s.Empty(s.stream.published, "and nothing reached a client")
}

// TestAZeroDistanceStepIsRefused pins rpg-toolkit#1060: the cell the walker
// already stands on is not a step.
//
// A step's only check was adjacency, and every grid family reads adjacency as
// Distance <= 1 — so distance ZERO sailed through. The composition's own
// placement then permits the mover's own cell, and the phantom went the whole
// way: a genuine `moved` beat recorded and persisted, sight refreshed, and
// EventMoved fanned out to every client, for a movement that never happened.
// Free-roam has no movement budget, so no accounting layer downstream would
// ever have caught the no-op.
//
// The assertion is therefore not only that it errors. What the defect produced
// was a RECORD, so the record is what has to be absent: no beat, no delivery.
func (s *MoveTestSuite) TestAZeroDistanceStepIsRefused() {
	s.startCorridor()
	ctx := context.Background()

	before, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.stream.published = nil // setup beats predate the walk

	// Alice stands at (1,1). This asks her to step onto herself.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition)
	s.NotErrorIs(err, session.ErrBrokenPath,
		"a broken path has a GAP in it; this one has the opposite problem, and "+
			"a caller told 'not a walk' would go looking for arithmetic that is fine")
	s.Contains(err.Error(), "already standing on (1,1)",
		"and the refusal names the cell, in the caller's own coordinates")

	after, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Len(after, len(before), "no beat was recorded for a movement that did not happen")
	s.Empty(s.stream.published, "and nothing reached a client")
}

// TestARepeatedCellMidPathIsRefused is the same phantom one cell along, and it
// is what stops the fix from being a comparison against where the walk began.
//
// "Where the walker stands" ADVANCES as the path resolves. [A,B,B] is a real
// step followed by a phantom one, so a check against A alone waves the second
// B straight through — the defect intact, one cell further in.
func (s *MoveTestSuite) TestARepeatedCellMidPathIsRefused() {
	s.startCorridor()
	ctx := context.Background()

	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 2, Y: 1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadPosition)
	s.Contains(err.Error(), "step 2 of 2", "and it names which step of the path")

	// Refused WHOLE (R5). The legal first step is not taken either: validation
	// finishes before the composition is touched, so alice never left (1,1).
	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 1, Y: 1}, where.Position,
		"a path refused whole leaves the walker exactly where she was")
}

// TestThereAndBackIsLegal is the negative control, and it is what keeps the
// #1060 fix from turning into a rule about REPEATED CELLS.
//
// [B,A] visits A twice and moves genuinely at every step: out one cell and back
// again is a walk, and a walker may retrace it as often as they like. Only a
// step of zero DISTANCE is a phantom, so only that is refused.
func (s *MoveTestSuite) TestThereAndBackIsLegal() {
	s.startCorridor()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 1, Y: 1}},
	})
	s.Require().NoError(err, "every step of a there-and-back is real movement")
	s.Require().Len(out.Steps, 2, "both are walked, and both are recorded")
	s.Equal(spatial.Position{X: 2, Y: 1}, out.Steps[0].Position)
	s.Equal(spatial.Position{X: 1, Y: 1}, out.Steps[1].Position, "back where she started")
}

func TestMoveSuite(t *testing.T) {
	suite.Run(t, new(MoveTestSuite))
}
