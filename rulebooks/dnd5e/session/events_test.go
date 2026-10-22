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

// failingStream is an ActStream that refuses to deliver.
type failingStream struct{ err error }

func (f *failingStream) PublishActs(_ context.Context, _ []session.Act) error { return f.err }

// failingRecord is a RecordStream that refuses to deliver.
type failingRecord struct{ err error }

func (f *failingRecord) PublishRecord(_ context.Context, _ []session.RecordEvent) error { return f.err }

// EventsTestSuite covers the multiplayer fan-out of the act stream.
type EventsTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func (s *EventsTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Acts: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// twoRoomParty puts alice and dave in the corridor and carol in the sealed
// vault, so one member is genuinely unable to perceive what the others do.
func twoRoomParty() scene {
	return scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("corridor", 0, 0, 8, 8),
				rectRegion("vault", 20, 0, 8, 8),
			},
		},
		Party:   []sceneSeat{seatAt("alice", 1, 1), seatAt("dave", 2, 2), seatAt("carol", 1, 1)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
}

func (s *EventsTestSuite) start() {
	launchScene(s.T(), s.mgr, twoRoomParty())
	s.stream.published = nil // setup beats predate any client
}

func (s *EventsTestSuite) recipients() map[string]int {
	out := map[string]int{}
	for _, e := range s.stream.published {
		out[e.Recipient]++
	}
	return out
}

// TestOneBeatBecomesPerRecipientActs pins that a single beat fans out as
// addressed acts per recipient, and that the recipient list is whatever the
// composition says it is.
//
// It does NOT pin perception scoping, because the composition does not do that
// yet: every beat is currently addressed to every member of the encounter, so
// carol — sealed in another room with no line of sight — is told where alice
// moved. That is toolkit#940, and it is the composition's to fix rather than
// this package's: filtering here would mean reimplementing visibility outside
// the module that owns perception, which is the very thing S11 exists to
// prevent.
//
// The layering is what makes that acceptable. When the composition scopes its
// audiences, this fan-out inherits the fix with no change at all — which is
// also why this test asserts on "whoever the composition addressed" rather than
// on a hard-coded roster.
func (s *EventsTestSuite) TestOneBeatBecomesPerRecipientActs() {
	s.start()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)

	byRecipient := s.recipients()
	s.NotZero(byRecipient["alice"], "the actor hears about their own move")
	s.NotZero(byRecipient["dave"], "someone in the same room hears about it")

	// Documents current behaviour rather than endorsing it. When #940 lands and
	// audiences become perception-scoped, this flips to Zero and the failure is
	// the reminder to update scenes.md's scene 7, which currently describes the
	// behaviour we want rather than the behaviour we have.
	s.NotZero(byRecipient["carol"],
		"TODAY the composition addresses every beat to every member (toolkit#940); "+
			"when that is scoped by perception this becomes Zero")
}

// TestActsAreAddressed pins that acts are per-recipient rather than
// broadcast with a viewer list attached.
//
// A single act carrying "here is everyone who may see this" would put the
// filtering decision back on the host, and would hand every client the roster
// of who else was watching.
func (s *EventsTestSuite) TestActsAreAddressed() {
	s.start()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(s.stream.published)

	for _, e := range s.stream.published {
		s.NotEmpty(e.Recipient, "every act names exactly one recipient")
		s.Equal("sess", e.Session)
		s.NotEmpty(e.ID, "and carries the id a client resumes after")
	}
}

// TestNothingIsPublishedWhenTheSaveFails pins S9's ordering.
//
// Announcing a fact that failed to persist is the one mistake with no recovery:
// the client is told the world changed, the world did not, and there is no
// missing id to betray the difference. The client would never resync, because
// nothing looks missing.
func (s *EventsTestSuite) TestNothingIsPublishedWhenTheSaveFails() {
	encounters := &failingEncounters{fakeEncounters: newFakeEncounters()}
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: encounters, Characters: testCharacters(), Acts: stream,
	})
	s.Require().NoError(err)

	ctx := context.Background()
	launchScene(s.T(), mgr, twoRoomParty())
	encounters.saveErr = errBroken
	stream.published = nil

	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().Error(err)
	s.Empty(stream.published,
		"a fact that failed to persist must never be announced")
}

// TestDeliveryFailureDoesNotFailTheVerb pins S10.
//
// The world already changed by the time delivery is attempted, so failing the
// verb would roll back nothing and would turn a transient stream outage into an
// error the host has to interpret. The record is the truth, so clients
// self-heal by catching up after the last act id they hold.
func (s *EventsTestSuite) TestDeliveryFailureDoesNotFailTheVerb() {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Acts: &failingStream{err: errBroken},
	})
	s.Require().NoError(err)

	ctx := context.Background()
	launchScene(s.T(), mgr, twoRoomParty())

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err, "a stream outage must not fail the move")
	s.Equal([]string{session.DeliveryActs}, out.Delivery.Failed, "but it is reported, not hidden, naming the stream")

	// And the move really happened.
	status, err := mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open)
}

// TestDiscardingActsIsNotFailing pins that the explicit no-op is a real
// stream rather than a hole in the flow: DiscardActs accepts the batch and
// succeeds, so the report says nothing failed — which is true: the acts were
// handed to a stream that chose to drop them.
func (s *EventsTestSuite) TestDiscardingActsIsNotFailing() {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Acts: session.DiscardActs{},
	})
	s.Require().NoError(err)

	ctx := context.Background()
	launchScene(s.T(), mgr, twoRoomParty())

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Empty(out.Delivery.Failed, "discarding is not failing")
}

// TestDepartingMemberHearsTheirOwnExit pins the roster choice.
//
// The fan-out addresses everyone who has ever been a member, not the current
// roster. Using the live one would drop the single act the departing player
// most needs — the one saying they left.
func (s *EventsTestSuite) TestDepartingMemberHearsTheirOwnExit() {
	s.start()

	_, err := s.mgr.Exit(context.Background(),
		&session.ExitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	s.NotZero(s.recipients()["alice"],
		"a member who just left must still be told that they left")
}

func TestEventsSuite(t *testing.T) {
	suite.Run(t, new(EventsTestSuite))
}

// TestDeliveryFailureNamesEachRefusingStream: the report names which stream
// refused, so a host knows which catch-up to run — the record's, the acts',
// or both — and names none when both landed.
func (s *EventsTestSuite) TestDeliveryFailureNamesEachRefusingStream() {
	cases := []struct {
		name   string
		record session.RecordStream
		acts   session.ActStream
		want   []string
	}{
		{"both land", session.DiscardRecord{}, session.DiscardActs{}, nil},
		{"the record refuses", &failingRecord{err: errBroken}, session.DiscardActs{}, []string{session.DeliveryRecord}},
		{"the acts refuse", session.DiscardRecord{}, &failingStream{err: errBroken}, []string{session.DeliveryActs}},
		{"both refuse", &failingRecord{err: errBroken}, &failingStream{err: errBroken},
			[]string{session.DeliveryRecord, session.DeliveryActs}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			sessions, encounters := newFakeSessions(), newFakeEncounters()
			mgr, err := session.NewManager(&session.Config{Record: tc.record, Acts: tc.acts, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
				Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
			})
			s.Require().NoError(err)
			launchScene(s.T(), mgr, twoRoomParty())

			out, err := mgr.Move(context.Background(), &session.MoveInput{
				Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
			})
			s.Require().NoError(err, "a stream outage never fails the verb")
			s.Equal(tc.want, out.Delivery.Failed)
		})
	}
}
