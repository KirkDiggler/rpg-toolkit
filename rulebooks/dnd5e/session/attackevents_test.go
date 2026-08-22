// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// AttackEventsTestSuite covers what a client is told when somebody swings.
//
// Separate from AttackTestSuite because it asks a different question. That one
// asks what the RULES produced — the roll, the arithmetic, the beat in the
// story. This one asks what reached the TABLE: the same swing, delivered to a
// real stream, named as something a client can branch on rather than as the
// EventUnknown every attack arrived as before rpg-toolkit#1038.
type AttackEventsTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
}

func TestAttackEventsSuite(t *testing.T) {
	suite.Run(t, new(AttackEventsTestSuite))
}

// duelWithStream is AttackTestSuite's duel wired to a stream that records.
func (s *AttackEventsTestSuite) duelWithStream(dice session.Roller) *session.Manager {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{
		Dice: dice, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)
	s.stream.published = nil // the opening beats predate any client
	return mgr
}

// swing runs alice at bob and returns what the caller was told.
func (s *AttackEventsTestSuite) swing(mgr *session.Manager) *session.AttackOutput {
	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
	})
	s.Require().NoError(err)
	return out
}

// kindsAtSeq collects the event kinds delivered for one story sequence, keyed
// by recipient — so an assertion can name WHO was told WHAT, rather than
// counting events and hoping the count means what it looks like.
func (s *AttackEventsTestSuite) kindsAtSeq(seq uint64) map[string]session.EventKind {
	out := map[string]session.EventKind{}
	for _, event := range s.stream.published {
		if event.Seq != seq {
			continue
		}
		s.Require().NotContains(out, event.Recipient, "one event per recipient per beat")
		out[event.Recipient] = event.Kind
	}
	return out
}

// TestAHitIsNamedOnTheStream is the headline, and the reason the swing is real
// rather than a payload handed to the mapper.
//
// The beat string is written by the composition (encounter's Record) and read
// by this package (kindOf), and nothing else checks that the two agree. A test
// that fed the mapper a literal "struck" would pass just as happily with a
// composition that had renamed the beat, which is the exact failure mode the
// mapper's own doc warns about: detection fails SILENTLY and everything
// degrades to unknown.
//
// So the swing is driven end to end with scripted dice, and the event is found
// by the sequence the caller's own AttackOutput reports. Both ends of the
// coupling are exercised, and the assertion is tied to the swing that produced
// it rather than to a string typed twice.
func (s *AttackEventsTestSuite) TestAHitIsNamedOnTheStream() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	out := s.swing(mgr)
	s.Require().True(out.Hit, "15 + 3 STR + 2 proficiency clears AC 12")

	s.Equal(
		map[string]session.EventKind{"alice": session.EventStruck, "bob": session.EventStruck},
		s.kindsAtSeq(out.Seq),
		"both the striker and the struck hear that it landed",
	)
}

// TestAMissIsNamedOnTheStream pins the other arm.
//
// Worth its own case rather than a subtest of the hit: the two beats are
// separate strings on the composition's side and separate cases on this one, so
// a mapper that named only the hit would leave every miss unknown — and a table
// that narrates hits but not misses is arguably worse than one that narrates
// neither, because the silence looks like nothing happened.
func (s *AttackEventsTestSuite) TestAMissIsNamedOnTheStream() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{2, 5}})

	out := s.swing(mgr)
	s.Require().False(out.Hit, "2 + 5 is under AC 12")

	s.Equal(
		map[string]session.EventKind{"alice": session.EventMissed, "bob": session.EventMissed},
		s.kindsAtSeq(out.Seq),
		"a miss is a fact of the fight too",
	)
}

// TestTheSwingLeavesNothingUnknown guards the verb as a whole rather than the
// one beat under test.
//
// Attack records its outcome, and anything the world does in response — a fight
// starting underfoot, an ending firing — records its own beats behind it. If any
// of those reached a client as unknown, naming the strike would have fixed the
// headline and left the scene around it unreadable.
func (s *AttackEventsTestSuite) TestTheSwingLeavesNothingUnknown() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	s.swing(mgr)

	s.Require().NotEmpty(s.stream.published, "a swing with two members in the room reaches clients")
	for _, event := range s.stream.published {
		s.NotEqual(session.EventUnknown, event.Kind,
			"beat at seq %d reached %s unnamed", event.Seq, event.Recipient)
	}
}

// bodiesAtSeq is kindsAtSeq's own twin for Body: the typed value each
// recipient's event carries for one story sequence.
func (s *AttackEventsTestSuite) bodiesAtSeq(seq uint64) map[string]session.EventBody {
	out := map[string]session.EventBody{}
	for _, event := range s.stream.published {
		if event.Seq != seq {
			continue
		}
		out[event.Recipient] = event.Body
	}
	return out
}

// TestAHitCarriesATypedBody pins rpg-toolkit#941/#866 at the stream: a
// witness who is NOT the attacker — bob, reading his own struck event —
// gets the SAME numbers and weapon identity AttackOutput gave alice, from
// StruckBody rather than from decoding Payload by hand.
func (s *AttackEventsTestSuite) TestAHitCarriesATypedBody() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	out := s.swing(mgr)
	s.Require().True(out.Hit)

	bodies := s.bodiesAtSeq(out.Seq)
	for _, recipient := range []string{"alice", "bob"} {
		body, ok := bodies[recipient].(session.StruckBody)
		s.Require().True(ok, "%s's event carries a StruckBody, got %T", recipient, bodies[recipient])
		s.Equal("alice", body.Attacker)
		s.Equal("bob", body.Target)
		s.Equal(out.Roll, body.Roll)
		s.Equal(out.Total, body.Total)
		s.Equal(out.Against, body.Against)
		s.Equal(out.Damage, body.Damage)
		s.Equal(out.Critical, body.Critical)
		s.Equal(out.Attack, body.Attack, "the SAME weapon identity AttackOutput reported")
	}
}

// TestAMissCarriesATypedBody is the hit's own twin: MissedBody, with no
// Damage or Critical field to even get wrong.
func (s *AttackEventsTestSuite) TestAMissCarriesATypedBody() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{2, 5}})

	out := s.swing(mgr)
	s.Require().False(out.Hit)

	bodies := s.bodiesAtSeq(out.Seq)
	body, ok := bodies["bob"].(session.MissedBody)
	s.Require().True(ok, "bob's event carries a MissedBody, got %T", bodies["bob"])
	s.Equal("alice", body.Attacker)
	s.Equal("bob", body.Target)
	s.Equal(out.Roll, body.Roll)
	s.Equal(out.Total, body.Total)
	s.Equal(out.Against, body.Against)
	s.Equal(out.Attack, body.Attack)
}
