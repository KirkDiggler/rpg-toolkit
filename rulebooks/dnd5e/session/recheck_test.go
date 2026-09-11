// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// RecheckSuite covers the verb a host calls after it has changed something an
// observer could SEE about a member — equipment today.
//
// It shares the heirloom set with the holdings scenes because that set already
// has two players who can see each other and a body that cannot see anybody,
// which is exactly the three-way split this verb's audience rules turn on.
type RecheckSuite struct {
	suite.Suite

	stream     *fakeStream
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestRecheckSuite(t *testing.T) { suite.Run(t, new(RecheckSuite)) }

func (s *RecheckSuite) SetupTest() {
	s.stream = &fakeStream{}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(sharpEyed("alice"), dullEyed("bob"))

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: heirloomWorld(s.T(), true),
	})
	s.Require().NoError(err)
	s.stream.published = nil
}

// sightedFor is every sighting body one recipient was told about.
func (s *RecheckSuite) sightedFor(recipient string) []session.SightedBody {
	out := make([]session.SightedBody, 0)
	for _, e := range eventsFor(s.stream.published, recipient) {
		if body, ok := e.Body.(session.SightedBody); ok {
			out = append(out, body)
		}
	}
	return out
}

func (s *RecheckSuite) recheck(members ...string) error {
	_, err := s.mgr.Recheck(context.Background(), &session.RecheckInput{
		Session: "sess", Members: members,
	})
	return err
}

// THE WATCHER IS TOLD, and told only that their view is stale. This is the
// whole point of the verb: alice's gear changed on a sheet the session does
// not watch, and bob — who is looking at her — learns to read his view again.
func (s *RecheckSuite) TestAWatcherIsToldTheirViewIsStale() {
	s.Require().NoError(s.recheck("alice"))

	told := s.sightedFor("bob")
	s.Require().Len(told, 1, "one beat, for the one thing he is owed")
	s.Equal([]string{"alice"}, told[0].Changed)
	s.Empty(told[0].Gained, "she did not arrive — he could already see her")
	s.Empty(told[0].Lost)
}

// THE SUBJECT HEARS NOTHING. A member is never in their own percept, so
// nobody is told that they themselves changed — they are the one who did it.
func (s *RecheckSuite) TestTheSubjectIsNotToldAboutItself() {
	s.Require().NoError(s.recheck("alice"))

	s.Empty(s.sightedFor("alice"),
		"she changed her own hands; she does not need telling what is in them")
}

// IT NAMES WHO, NOT WHAT. There is nowhere on the wire to say a longsword
// was put away, and that absence is the design: the watcher re-reads their
// OWN view and sees what they are entitled to see, which is the only shape in
// which it can differ from the truth.
func (s *RecheckSuite) TestTheBeatCarriesNoFactAboutTheChange() {
	s.Require().NoError(s.recheck("alice"))

	told := s.sightedFor("bob")
	s.Require().Len(told, 1)
	s.Equal([]string{"alice"}, told[0].Changed,
		"a name, and the whole body is names — there is no item, slot or verb here")
}

// TWO NAMES, ONE BEAT. A host that changed several members says so once, and
// the watcher is owed one nudge rather than one per member.
func (s *RecheckSuite) TestSeveralMembersArriveInOneBeat() {
	s.Require().NoError(s.recheck("alice", "captain"))

	told := s.sightedFor("bob")
	s.Require().Len(told, 1, "one pass, one beat")
	s.Equal([]string{"alice", "captain"}, told[0].Changed, "both, sorted")
}

// The door, checked before any world is loaded where it can be.
func (s *RecheckSuite) TestRecheckRefusesWhatItCannotDo() {
	s.Require().ErrorIs(s.recheckNil(), session.ErrNilInput)
	s.Require().ErrorIs(s.recheck(), session.ErrNoMemberID, "a re-look of nobody")
	s.Require().ErrorIs(s.recheck("alice", ""), session.ErrNoMemberID,
		"an empty name in the list is the host's mistake, and needs no world to say so")
	s.Require().ErrorIs(s.recheck("a-stranger"), session.ErrNoMember)

	s.Empty(s.sightedFor("bob"), "and no refusal published anything")
}

func (s *RecheckSuite) recheckNil() error {
	_, err := s.mgr.Recheck(context.Background(), nil)
	return err
}
