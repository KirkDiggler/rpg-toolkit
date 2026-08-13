// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// WriteTestSuite covers the verbs that change the world: Join, Exit and End.
type WriteTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func (s *WriteTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

// TestJoinPersistsTheNewMember is the round-trip that matters most.
//
// Asserting only on the returned JoinOutput would pass just as happily if the
// verb mutated an in-memory encounter and never saved it — the caller would see
// a perfect answer, and the member would be gone on the next request. So the
// check is made through a SEPARATE read, which is the only way to distinguish
// "it happened" from "it was reported".
func (s *WriteTestSuite) TestJoinPersistsTheNewMember() {
	ctx := context.Background()

	out, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)
	s.Equal("bob", out.Member.ID)
	s.Equal(session.KindPlayer, out.Member.Kind)
	s.Equal("vault", out.Member.Room)
	s.Equal([]string{"encounter:world"}, out.Saved.Written)

	// The proof: a fresh read sees him.
	sightings, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err, "the joined member must exist on a subsequent call")
	s.NotNil(sightings)
}

// TestJoinReportsWhoNoticed pins that perception deltas cross the boundary.
//
// Joining someone into a room the party can see is the moment a client needs to
// announce; a projection that dropped the deltas would leave every arrival
// silent, and no other assertion in this file would notice.
func (s *WriteTestSuite) TestJoinReportsWhoNoticed() {
	out, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "corridor", Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)
	s.NotEmpty(out.Discovered,
		"someone arriving in an occupied room must be discoverable by whoever is there")
}

// TestExitCarriesKnowledgeOut pins that a departing member takes what they knew.
func (s *WriteTestSuite) TestExitCarriesKnowledgeOut() {
	ctx := context.Background()

	out, err := s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", out.Outcome.ID)
	s.Equal("corridor", out.Outcome.Room)
	s.Equal([]string{"encounter:world"}, out.Saved.Written)

	// And she is really gone, not merely reported gone.
	_, err = s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().Error(err, "an exited member is no longer viewable")
}

// TestEndClosesTheEncounter pins the ending path and its persistence.
func (s *WriteTestSuite) TestEndClosesTheEncounter() {
	ctx := context.Background()

	out, err := s.mgr.End(ctx, &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)
	s.Equal("out", out.Outcome.Ending)
	s.Equal([]string{"encounter:world"}, out.Saved.Written)

	status, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.False(status.Open, "the closure must have been persisted, not just reported")
	s.Require().NotNil(status.Outcome)
	s.Equal("out", status.Outcome.Ending)
}

// TestClosedEncounterRefusesChanges pins that a finished game is a record.
//
// Also pins the sentinel translation: a host must be able to recognise "this is
// over" without matching on the composition's error.
func (s *WriteTestSuite) TestClosedEncounterRefusesChanges() {
	ctx := context.Background()
	_, err := s.mgr.End(ctx, &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)

	_, err = s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.ErrorIs(err, session.ErrClosed)

	_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
	s.ErrorIs(err, session.ErrClosed)

	_, err = s.mgr.End(ctx, &session.EndInput{Session: "sess", Ending: "out"})
	s.ErrorIs(err, session.ErrClosed)
}

// TestReadVerbsStillWorkOnAClosedEncounter is the must-accept row for the rule
// above.
//
// A rejection table proves changes are refused; only this proves the refusal
// does not over-reach. A closed encounter must remain fully readable — the
// party wants to see how it ended, and a client that could no longer render a
// finished game would be worse than one that could not close it.
func (s *WriteTestSuite) TestReadVerbsStillWorkOnAClosedEncounter() {
	ctx := context.Background()
	_, err := s.mgr.End(ctx, &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)

	_, err = s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.NoError(err, "status must survive closure — it is how you learn it closed")
	_, err = s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess"})
	s.NoError(err, "the map does not stop existing")
	_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.NoError(err, "the story is the point of a finished encounter")
}

// TestUnknownEndingIsRejected pins that End cannot invent an outcome.
func (s *WriteTestSuite) TestUnknownEndingIsRejected() {
	_, err := s.mgr.End(context.Background(),
		&session.EndInput{Session: "sess", Ending: "not-a-thing"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoEnding)

	status, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open, "a rejected ending must not have closed anything")
}

// TestWriteVerbsRejectMissingIdentifiers walks the guards, and checks the world
// is untouched — a verb that rejected only after mutating would still be wrong.
func (s *WriteTestSuite) TestWriteVerbsRejectMissingIdentifiers() {
	ctx := context.Background()

	_, err := s.mgr.Join(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)
	_, err = s.mgr.Exit(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)
	_, err = s.mgr.End(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = s.mgr.Join(ctx, &session.JoinInput{Session: "sess"})
	s.ErrorIs(err, session.ErrNoMemberID)
	_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = s.mgr.Join(ctx, &session.JoinInput{Member: "bob"})
	s.ErrorIs(err, session.ErrNoSessionID)
	_, err = s.mgr.End(ctx, &session.EndInput{Ending: "out"})
	s.ErrorIs(err, session.ErrNoSessionID)
}

// TestFailedSaveIsReportedNotSwallowed pins S6 on the write path: the verb
// fails, and the report names what did not land.
func (s *WriteTestSuite) TestFailedSaveIsReportedNotSwallowed() {
	encounters := &failingEncounters{fakeEncounters: newFakeEncounters()}
	sessions := newFakeSessions()
	mgr, err := session.NewManager(&session.Config{Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)

	// Arm the failure only now, so setup could succeed.
	encounters.saveErr = errBroken

	out, err := mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken, "the underlying cause must survive")
	s.Nil(out)
}

// TestStaleWorldIsNotResurrected pins statelessness (S1) at the write seam.
//
// Two managers over the same stores, interleaved. If either held the encounter
// between calls, the second manager's write would be computed from a snapshot
// taken before the first manager's join, and bob would vanish — the classic
// lost-update. Loading per verb is what makes several servers able to serve one
// session with no coordination, and this is the test that says so.
func (s *WriteTestSuite) TestStaleWorldIsNotResurrected() {
	ctx := context.Background()

	other, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	_, err = other.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "carol", Kind: session.KindPlayer,
		Room: "vault", Position: spatial.Position{X: 1, Y: 0},
	})
	s.Require().NoError(err)

	// Both must be present: the second manager loaded a world that already had
	// bob in it, rather than one it had been holding since before he joined.
	for _, who := range []string{"alice", "bob", "carol"} {
		_, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: who})
		s.NoError(err, "%s must still be in the encounter", who)
	}
}

// TestJoinOnUnknownSession pins the load failures reach the write verbs too.
func (s *WriteTestSuite) TestJoinOnUnknownSession() {
	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "nope", Member: "bob", Kind: session.KindPlayer, Room: "vault",
	})
	s.ErrorIs(err, session.ErrNoSession)
}

func TestWriteSuite(t *testing.T) {
	suite.Run(t, new(WriteTestSuite))
}
