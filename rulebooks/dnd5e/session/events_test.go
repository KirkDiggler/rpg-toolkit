// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// failingStream refuses to deliver.
type failingStream struct{ err error }

func (f *failingStream) Publish(_ context.Context, _ []session.Event) error { return f.err }

// EventsTestSuite covers the multiplayer fan-out.
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
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// twoRoomParty puts alice and dave in the corridor and carol in the sealed
// vault, so one member is genuinely unable to perceive what the others do.
func twoRoomParty(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "corridor", Width: 8, Height: 8},
				{ID: "vault", Width: 8, Height: 8, Origin: spatial.Position{X: 20, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "corridor", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "dave", Kind: encounter.KindPlayer, Room: "corridor", Position: spatial.Position{X: 2, Y: 2}},
			{ID: "carol", Kind: encounter.KindPlayer, Room: "vault", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building two-room party: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *EventsTestSuite) start() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: twoRoomParty(s.T()),
	})
	s.Require().NoError(err)
	s.stream.published = nil // setup beats predate any client
}

func (s *EventsTestSuite) recipients() map[string]int {
	out := map[string]int{}
	for _, e := range s.stream.published {
		out[e.Recipient]++
	}
	return out
}

// TestOneBeatBecomesPerRecipientEvents pins that a single beat fans out as one
// addressed event per recipient, and that the recipient list is whatever the
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
func (s *EventsTestSuite) TestOneBeatBecomesPerRecipientEvents() {
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

// TestEventsAreAddressed pins that events are per-recipient rather than
// broadcast with a viewer list attached.
//
// A single event carrying "here is everyone who may see this" would put the
// filtering decision back on the host, and would hand every client the roster
// of who else was watching.
func (s *EventsTestSuite) TestEventsAreAddressed() {
	s.start()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(s.stream.published)

	for _, e := range s.stream.published {
		s.NotEmpty(e.Recipient, "every event names exactly one recipient")
		s.Equal("sess", e.Session)
		s.NotZero(e.Seq, "and carries the sequence a client uses to detect gaps")
	}
}

// TestNothingIsPublishedWhenTheSaveFails pins S9's ordering.
//
// Announcing a fact that failed to persist is the one mistake with no recovery:
// the client is told the world changed, the world did not, and there is no
// sequence gap to betray the difference. The client would never resync, because
// nothing looks missing.
func (s *EventsTestSuite) TestNothingIsPublishedWhenTheSaveFails() {
	encounters := &failingEncounters{fakeEncounters: newFakeEncounters()}
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Sessions: newFakeSessions(), Encounters: encounters, Characters: testCharacters(), Events: stream,
	})
	s.Require().NoError(err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: twoRoomParty(s.T()),
	})
	s.Require().NoError(err)
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
// error the host has to interpret. The log is the truth and sequences are
// gapless, so clients self-heal.
func (s *EventsTestSuite) TestDeliveryFailureDoesNotFailTheVerb() {
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: &failingStream{err: errBroken},
	})
	s.Require().NoError(err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: twoRoomParty(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err, "a stream outage must not fail the move")
	s.True(out.Delivery.Failed, "but it is reported, not hidden")
	s.NotZero(out.Delivery.Events, "including how many events went undelivered")

	// And the move really happened.
	status, err := mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open)
}

// TestDiscardingEventsStillReportsHonestly pins that the explicit no-op is a
// real stream rather than a hole in the flow.
//
// DiscardEvents accepts the batch and succeeds, so the report says events were
// delivered and nothing failed — which is true: they were handed to a stream
// that chose to drop them. Reporting zero would be a lie about what the manager
// did, and would make a genuinely silent run indistinguishable from a broken
// one.
func (s *EventsTestSuite) TestDiscardingEventsStillReportsHonestly() {
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: twoRoomParty(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.NotZero(out.Delivery.Events, "the events were produced and handed over")
	s.False(out.Delivery.Failed, "discarding is not failing")
}

// TestDepartingMemberHearsTheirOwnExit pins the roster choice.
//
// The fan-out addresses everyone who has ever been a member, not the current
// roster. Using the live one would drop the single event the departing player
// most needs — the one saying they left.
func (s *EventsTestSuite) TestDepartingMemberHearsTheirOwnExit() {
	s.start()

	_, err := s.mgr.Exit(context.Background(),
		&session.ExitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	s.NotZero(s.recipients()["alice"],
		"a member who just left must still be told that they left")
}

// TestKindsComeFromTheBeatNotTheTag pins the discriminator choice.
//
// The composition's tags are coarser than its beats: "membership" covers both
// joining and leaving, and "movement" covers both a step and a doorway
// crossing. Switching on the tag would collapse pairs of opposite events into
// one kind, and a client cannot narrate an arrival and a departure the same way.
func (s *EventsTestSuite) TestKindsComeFromTheBeatNotTheTag() {
	s.start()
	ctx := context.Background()

	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Contains(s.kinds(), session.EventMoved)

	s.stream.published = nil
	_, err = s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "erin",
		Room: "corridor", Position: spatial.Position{X: 3, Y: 3},
	})
	s.Require().NoError(err)
	joined := s.kinds()
	s.Contains(joined, session.EventJoined)
	s.NotContains(joined, session.EventExited,
		"joining and leaving share a tag and must not share a kind")

	s.stream.published = nil
	_, err = s.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "erin"})
	s.Require().NoError(err)
	exited := s.kinds()
	s.Contains(exited, session.EventExited)
	s.NotContains(exited, session.EventJoined)
}

func (s *EventsTestSuite) kinds() []session.EventKind {
	var out []session.EventKind
	for _, e := range s.stream.published {
		out = append(out, e.Kind)
	}
	return out
}

// TestPayloadIsCarriedIntact pins that the beat body survives the projection —
// a client needs the detail, not merely the kind.
func (s *EventsTestSuite) TestPayloadIsCarriedIntact() {
	s.start()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(s.stream.published)

	var body map[string]any
	s.Require().NoError(json.Unmarshal(s.stream.published[0].Payload, &body))
	s.Equal("moved", body["beat"])
	s.Equal("alice", body["member"])
}

func TestEventsSuite(t *testing.T) {
	suite.Run(t, new(EventsTestSuite))
}
