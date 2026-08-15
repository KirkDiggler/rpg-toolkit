// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// errBroken stands for a store that is down — distinct from a store that
// simply does not hold the key.
var errBroken = errors.New("store unavailable")

// failingSessions fails whichever operations a test arms.
type failingSessions struct {
	*fakeSessions
	getErr  error
	saveErr error
}

func (f *failingSessions) GetSession(ctx context.Context, id string) (*session.SessionData, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.fakeSessions.GetSession(ctx, id)
}

func (f *failingSessions) SaveSession(ctx context.Context, data *session.SessionData) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.fakeSessions.SaveSession(ctx, data)
}

// failingEncounters fails saves on demand.
type failingEncounters struct {
	*fakeEncounters
	saveErr error
}

func (f *failingEncounters) SaveEncounter(
	ctx context.Context, id string, data *encounter.EncounterData,
) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.fakeEncounters.SaveEncounter(ctx, id, data)
}

// StartSessionTestSuite covers the verb that brings a session into existence.
type StartSessionTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
	world      *encounter.EncounterData
}

func (s *StartSessionTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()

	mgr, err := session.NewManager(&session.Config{Dice: testDice{},
		Sessions:   s.sessions,
		Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
	s.world = authoredWorld(s.T())
}

// authoredWorld builds a small valid encounter blob standing in for content
// from an authoring pipeline.
func authoredWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: encOrderAsGiven{},
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 5, Height: 5}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerReachedPosition{
				Room: "hall", Position: spatial.Position{X: 4, Y: 4},
			}},
		},
	})
	if err != nil {
		t.Fatalf("building authored world: %v", err)
	}
	data := enc.ToData()
	return &data
}

// fataler is the sliver of *testing.T authoredWorld needs.
type fataler interface {
	Fatalf(format string, args ...any)
}

// TestValidationRejectsBeforeAnythingIsWritten pins R5 atomicity at this seam:
// every rejection leaves both stores untouched.
//
// The assertion that both repositories are still empty is the load-bearing
// half. Checking only the returned error would pass just as happily if the
// verb wrote the world and then noticed the session ID was blank — leaving an
// orphaned encounter behind on every malformed request.
func (s *StartSessionTestSuite) TestValidationRejectsBeforeAnythingIsWritten() {
	cases := []struct {
		name   string
		input  *session.StartSessionInput
		expect error
	}{
		{"nil input", nil, session.ErrNilInput},
		{
			"empty session id",
			&session.StartSessionInput{Encounter: "enc-1", World: s.world},
			session.ErrNoSessionID,
		},
		{
			"empty encounter id",
			&session.StartSessionInput{Session: "sess-1", World: s.world},
			session.ErrNoEncounterID,
		},
		{
			"nil world",
			&session.StartSessionInput{Session: "sess-1", Encounter: "enc-1"},
			session.ErrInvalidWorld,
		},
		{
			"world that cannot be loaded",
			&session.StartSessionInput{
				Session: "sess-1", Encounter: "enc-1",
				World: &encounter.EncounterData{}, // no rooms, no endings
			},
			session.ErrInvalidWorld,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			out, err := s.mgr.StartSession(context.Background(), tc.input)
			s.Require().Error(err)
			s.ErrorIs(err, tc.expect)
			s.Nil(out)
			s.Empty(s.sessions.byID, "no session may be written on a rejection")
			s.Empty(s.encounters.byID, "no world may be written on a rejection")
		})
	}
}

// TestExistingSessionIsNotOverwritten pins that an ID in use is refused. The ID
// names a game in progress, and silently clobbering it would destroy a party's
// state because someone reused a string.
func (s *StartSessionTestSuite) TestExistingSessionIsNotOverwritten() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-1", World: s.world,
	})
	s.Require().NoError(err)

	second := authoredWorld(s.T())
	_, err = s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-2", World: second,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSessionExists)

	s.Equal("enc-1", s.sessions.byID["sess-1"].Encounter, "the original session is intact")
	s.NotContains(s.encounters.byID, "enc-2", "and the second world was never written")
}

// TestBrokenStoreIsNotMistakenForAFreeID is the discriminating case the
// existence check exists to get right.
//
// A store that is down and a store that does not hold the key are different
// answers. If the lookup treated every error as "absent", a transient outage
// would make StartSession succeed against an ID that is actually taken — and
// the moment the store recovered, a live session would have been overwritten
// by one that thought the name was free.
func (s *StartSessionTestSuite) TestBrokenStoreIsNotMistakenForAFreeID() {
	sessions := &failingSessions{fakeSessions: newFakeSessions(), getErr: errBroken}
	mgr, err := session.NewManager(&session.Config{Dice: testDice{},
		Sessions: sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	out, err := mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-1", World: s.world,
	})
	s.Require().Error(err)
	s.ErrorIs(err, errBroken, "the store's failure must propagate, not be swallowed")
	s.NotErrorIs(err, session.ErrSessionExists)
	s.Nil(out)
	s.Empty(s.encounters.byID, "and nothing may be written on an inconclusive check")
}

// TestBrokenRepositoryIsNotReadAsAnExistingSession pins the third outcome of
// the existence check, which is neither "found" nor "absent".
//
// A repository reporting success with no data has broken its contract, and
// both available guesses are wrong in a way that costs something. Reading it as
// "exists" produces a misleading error about a session that may not be there.
// Reading it as "free" is worse: it would overwrite whatever the repository
// actually holds. So it is refused as the contract violation it is, which also
// points whoever debugs it at the host's storage layer rather than at the
// caller (Copilot, PR #942).
func (s *StartSessionTestSuite) TestBrokenRepositoryIsNotReadAsAnExistingSession() {
	sessions := &nilDataSessions{fakeSessions: newFakeSessions()}
	encounters := newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	out, err := mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-1", World: s.world,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadRepository)
	s.NotErrorIs(err, session.ErrSessionExists,
		"a broken repository is not an existing session")
	s.Nil(out)
	s.Empty(encounters.byID, "and nothing may be written on an inconclusive check")
}

// nilDataSessions reports success while returning no data — the contract
// violation, not an absence.
type nilDataSessions struct{ *fakeSessions }

func (f *nilDataSessions) GetSession(_ context.Context, _ string) (*session.SessionData, error) {
	return nil, nil
}

// TestSuccessWritesWorldThenSession pins the happy path and the report.
func (s *StartSessionTestSuite) TestSuccessWritesWorldThenSession() {
	out, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-1", World: s.world,
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	s.Equal("sess-1", out.Session)
	s.Equal([]string{"encounter:enc-1", "session:sess-1"}, out.Saved.Written,
		"both aggregates are reported, world first")
	s.Empty(out.Saved.Failed)
	s.False(out.Saved.Partial())

	s.Contains(s.encounters.byID, "enc-1")
	s.Equal("enc-1", s.sessions.byID["sess-1"].Encounter)
}

// TestWorldSaveFailureLeavesNoDanglingSession pins the write ORDER, not merely
// that a failure is reported.
//
// If the session were written first, a failed world save would leave a session
// that looks healthy and points at nothing — a record that reads as a live game
// and is permanently unusable. Writing the world first means this failure mode
// cannot exist: there is simply no session yet.
func (s *StartSessionTestSuite) TestWorldSaveFailureLeavesNoDanglingSession() {
	encounters := &failingEncounters{fakeEncounters: newFakeEncounters(), saveErr: errBroken}
	sessions := newFakeSessions()
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-1", World: s.world,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.Empty(sessions.byID, "a session pointing at an unwritten world must never exist")
}

// TestSessionSaveFailureLeavesOnlyAnOrphan pins the other half of the ordering
// argument: when the second write fails, what survives is an orphaned world.
//
// That is the recoverable wreck — garbage a sweep can collect, and a retry can
// simply overwrite. It is the failure we deliberately chose to have.
func (s *StartSessionTestSuite) TestSessionSaveFailureLeavesOnlyAnOrphan() {
	sessions := &failingSessions{fakeSessions: newFakeSessions(), saveErr: errBroken}
	encounters := newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess-1", Encounter: "enc-1", World: s.world,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)

	s.Contains(encounters.byID, "enc-1", "the world landed")
	s.Empty(sessions.byID, "the session did not")
}

// TestSessionsGetSeparateWorlds pins that the authored content is copied rather
// than shared. Two parties running the same tomb must not be able to move each
// other's furniture.
func (s *StartSessionTestSuite) TestSessionsGetSeparateWorlds() {
	ctx := context.Background()

	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "party-a", Encounter: "world-a", World: authoredWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "party-b", Encounter: "world-b", World: authoredWorld(s.T()),
	})
	s.Require().NoError(err)

	s.NotSame(s.encounters.byID["world-a"], s.encounters.byID["world-b"],
		"each session plays in its own world")
	s.Equal("world-a", s.sessions.byID["party-a"].Encounter)
	s.Equal("world-b", s.sessions.byID["party-b"].Encounter)
}

func TestStartSessionSuite(t *testing.T) {
	suite.Run(t, new(StartSessionTestSuite))
}
