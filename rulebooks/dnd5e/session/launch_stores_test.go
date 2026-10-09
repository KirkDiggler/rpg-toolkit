// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// LaunchStoresSuite pins what the stores a run is born into must see:
// rejection before any write, an id in use, a broken store that is neither
// absent nor present, and the write order whose failure modes are chosen.
type LaunchStoresSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
}

func TestLaunchStoresSuite(t *testing.T) { suite.Run(t, new(LaunchStoresSuite)) }

func (s *LaunchStoresSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
}

func (s *LaunchStoresSuite) manager(sessions session.SessionRepository, encounters session.EncounterRepository) *session.Manager {
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(armedFighter("alice")), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

// A rejection leaves both stores empty: checking only the returned error
// would pass as happily if the verb wrote the world and then noticed the
// session id was blank.
func (s *LaunchStoresSuite) TestValidationRejectsBeforeAnythingIsWritten() {
	valid := func() *session.LaunchInput { return sceneInput(authoredWorld()) }
	noID := valid()
	noID.Session = ""
	noDungeon := valid()
	noDungeon.Dungeon = nil
	noParty := valid()
	noParty.Party = nil
	cases := []struct {
		name   string
		input  *session.LaunchInput
		expect error
	}{
		{"nil input", nil, session.ErrNilInput},
		{"empty session id", noID, session.ErrNoSessionID},
		{"nil dungeon", noDungeon, session.ErrInvalidWorld},
		{"no party", noParty, session.ErrNoMemberID},
	}
	mgr := s.manager(s.sessions, s.encounters)
	for _, tc := range cases {
		s.Run(tc.name, func() {
			out, err := mgr.Launch(context.Background(), tc.input)
			s.Require().ErrorIs(err, tc.expect)
			s.Nil(out)
			s.Empty(s.sessions.byID, "no session may be written on a rejection")
			s.Empty(s.encounters.byID, "no world may be written on a rejection")
		})
	}
}

// An id in use names a game in progress, and reusing a string must not
// destroy a party's state.
func (s *LaunchStoresSuite) TestAnExistingRunIsNotOverwritten() {
	mgr := s.manager(s.sessions, s.encounters)
	launchScene(s.T(), mgr, authoredWorld())
	before := s.encounters.byID[testSession]

	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().ErrorIs(err, session.ErrSessionExists)
	s.Same(before, s.encounters.byID[testSession], "the original run is intact")
}

// A store that is down and a store that does not hold the key are different
// answers; treating the outage as "free" would overwrite a live run once the
// store recovered.
func (s *LaunchStoresSuite) TestABrokenStoreIsNotMistakenForAFreeID() {
	mgr := s.manager(&failingSessions{fakeSessions: s.sessions, getErr: errBroken}, s.encounters)

	out, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().ErrorIs(err, errBroken, "the store's failure must propagate, not be swallowed")
	s.NotErrorIs(err, session.ErrSessionExists)
	s.Nil(out)
	s.Empty(s.encounters.byID, "and nothing may be written on an inconclusive check")
}

// A repository reporting success with no data has broken its contract: it is
// refused as that, neither as an existing session nor as a free id.
func (s *LaunchStoresSuite) TestABrokenRepositoryIsNotReadAsAnExistingRun() {
	mgr := s.manager(&nilDataSessions{fakeSessions: s.sessions}, s.encounters)

	out, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().ErrorIs(err, session.ErrBadRepository)
	s.NotErrorIs(err, session.ErrSessionExists, "a broken repository is not an existing session")
	s.Nil(out)
	s.Empty(s.encounters.byID)
}

// nilDataSessions reports success while returning no data: the contract
// violation, not an absence.
type nilDataSessions struct{ *fakeSessions }

func (f *nilDataSessions) GetSession(_ context.Context, _ string) (*session.SessionData, error) {
	return nil, nil
}

// The write ORDER, not merely that a failure is reported: the world lands
// before the session that points at it, so a failed world save can leave no
// session pointing at nothing.
func (s *LaunchStoresSuite) TestAWorldSaveFailureLeavesNoDanglingSession() {
	mgr := s.manager(s.sessions, &failingEncounters{fakeEncounters: s.encounters, saveErr: errBroken})

	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken, "the repository cause is still matchable")
	s.Empty(s.sessions.byID, "a session pointing at an unwritten world must never exist")

	var saveErr *session.SaveError
	s.Require().ErrorAs(err, &saveErr, "the report must reach the host (S6)")
	s.Contains(saveErr.Report.Failed, "encounter:"+testSession)
	s.NotContains(saveErr.Report.Written, "session:"+testSession)
}

// The other half of the ordering argument: when the second write fails, what
// survives is an orphaned world, the recoverable wreck.
func (s *LaunchStoresSuite) TestASessionSaveFailureLeavesOnlyAnOrphanedWorld() {
	mgr := s.manager(&failingSessions{fakeSessions: s.sessions, saveErr: errBroken}, s.encounters)

	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken)
	s.Contains(s.encounters.byID, testSession, "the world landed")
	s.Empty(s.sessions.byID, "the session did not")

	var saveErr *session.SaveError
	s.Require().ErrorAs(err, &saveErr)
	s.Contains(saveErr.Report.Written, "encounter:"+testSession)
	s.Contains(saveErr.Report.Failed, "session:"+testSession)
}
