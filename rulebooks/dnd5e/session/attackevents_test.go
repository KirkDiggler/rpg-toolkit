// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// AttackEventsTestSuite covers what a client is told when somebody swings.
//
// Separate from AttackTestSuite because it asks a different question. That one
// asks what the RULES produced — the roll, the arithmetic, the beat in the
// story. This one asks what reached the TABLE: the same swing, delivered to a
// real act stream as an attack act every recipient can read, with nothing in
// the verb left undecoded.
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

// countingPresentationIDs mints a fresh token every time it is asked, so a
// test can tell one roll from the next the way a client has to.
type countingPresentationIDs struct{ minted int }

func (g *countingPresentationIDs) Generate() string {
	g.minted++
	return fmt.Sprintf("roll-%d", g.minted)
}

// duelWithStream is AttackTestSuite's duel wired to a stream that records.
func (s *AttackEventsTestSuite) duelWithStream(dice session.Roller) *session.Manager {
	return s.duelWithStreamAndIDs(dice, testPresentationIDs{}, armedFighter("alice"))
}

// duelWithStreamAndIDs is duelWithStream with the correlation entropy and the
// attacker's own sheet named — for the cases that ask what a SECOND roll was
// given, which needs an attacker who gets a second swing.
func (s *AttackEventsTestSuite) duelWithStreamAndIDs(
	dice session.Roller, ids session.PresentationIDGenerator, alice *character.Data,
) *session.Manager {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(alice, armedFighter("bob"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: s.stream, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: ids,
		Dice: dice, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)

	launchDuel(s.T(), mgr, s.encounters)
	s.stream.published = nil // the opening beats predate any client
	return mgr
}

// swing runs alice at bob and returns what the caller was told.
func (s *AttackEventsTestSuite) swing(mgr *session.Manager) *session.AttackOutput {
	return s.swingBy(mgr, "alice", "bob")
}

// swingBy is swing with both ends named, for the turn that is not alice's.
func (s *AttackEventsTestSuite) swingBy(mgr *session.Manager, attacker, target string) *session.AttackOutput {
	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: attacker, Target: target,
		DeclarationID: currentAttackID(s.T(), mgr, "sess", attacker),
	})
	s.Require().NoError(err)
	return out
}

// swingsTo is every attack act delivered to recipient, in delivery order.
func (s *AttackEventsTestSuite) swingsTo(recipient string) []session.AttackAct {
	var out []session.AttackAct
	for _, act := range s.stream.published {
		if act.Recipient != recipient {
			continue
		}
		if body, ok := act.Body.(session.AttackAct); ok {
			out = append(out, body)
		}
	}
	return out
}

// swingTo is the one attack act delivered to recipient.
func (s *AttackEventsTestSuite) swingTo(recipient string) session.AttackAct {
	s.T().Helper()
	swings := s.swingsTo(recipient)
	s.Require().Len(swings, 1, "%s was told about the swing once", recipient)
	return swings[0]
}

// TestAHitIsNamedOnTheStream is the headline, and the reason the swing is real
// rather than a payload handed to the fold: the swing is driven end to end
// with scripted dice, so both ends of the coupling — the composition's beat
// and this package's reading of it — are exercised.
func (s *AttackEventsTestSuite) TestAHitIsNamedOnTheStream() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	out := s.swing(mgr)
	s.Require().True(out.Hit, "15 + 3 STR + 2 proficiency clears AC 12")

	for _, recipient := range []string{"alice", "bob"} {
		s.Equal(session.AttackHit, s.swingTo(recipient).Outcome,
			"both the striker and the struck hear that it landed (%s)", recipient)
	}
}

// TestAMissIsNamedOnTheStream pins the other arm: a table that narrates hits
// but not misses is arguably worse than one that narrates neither, because
// the silence looks like nothing happened.
func (s *AttackEventsTestSuite) TestAMissIsNamedOnTheStream() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{2, 5}})

	out := s.swing(mgr)
	s.Require().False(out.Hit, "2 + 5 is under AC 12")

	for _, recipient := range []string{"alice", "bob"} {
		s.Equal(session.AttackMiss, s.swingTo(recipient).Outcome,
			"a miss is a fact of the fight too (%s)", recipient)
	}
}

// TestTheSwingLeavesNothingUndecoded guards the verb as a whole rather than
// the one beat under test.
//
// Attack records its outcome, and anything the world does in response — a fight
// starting underfoot, an ending firing — records its own beats behind it. If any
// of those reached a client undecoded, naming the strike would have fixed the
// headline and left the scene around it unreadable.
func (s *AttackEventsTestSuite) TestTheSwingLeavesNothingUndecoded() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	s.swing(mgr)

	s.Require().NotEmpty(s.stream.published, "a swing with two members in the room reaches clients")
	for _, act := range s.stream.published {
		s.Empty(act.Undecoded, "act %s reached %s undecoded", act.ID, act.Recipient)
	}
}

// requireSameSwing asserts a recipient's attack act states the numbers and
// weapon identity AttackOutput gave the attacker.
func (s *AttackEventsTestSuite) requireSameSwing(out *session.AttackOutput, body session.AttackAct, recipient string) {
	s.T().Helper()
	s.Equal("alice", body.By.ID, recipient)
	s.Equal("bob", body.To.ID, recipient)
	s.Require().NotNil(body.Roll, recipient)
	s.Contains(body.Roll.Dice, out.Roll, recipient)
	s.Equal(out.Total, body.Roll.Total, recipient)
	s.Require().NotNil(body.Roll.Against, recipient)
	s.Equal(out.Against, *body.Roll.Against, recipient)
	s.Equal(out.Damage, body.Damage, recipient)
	s.Equal(out.Critical, body.Critical, recipient)
	s.Equal(out.Attack, body.Attack, "the SAME weapon identity AttackOutput reported (%s)", recipient)
}

// TestAHitCarriesATypedBody pins rpg-toolkit#941/#866 at the stream: a
// witness who is NOT the attacker — bob, reading his own attack act — gets
// the SAME numbers and weapon identity AttackOutput gave alice.
func (s *AttackEventsTestSuite) TestAHitCarriesATypedBody() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	out := s.swing(mgr)
	s.Require().True(out.Hit)

	for _, recipient := range []string{"alice", "bob"} {
		s.requireSameSwing(out, s.swingTo(recipient), recipient)
	}
}

// TestAMissCarriesATypedBody is the hit's own twin.
func (s *AttackEventsTestSuite) TestAMissCarriesATypedBody() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{2, 5}})

	out := s.swing(mgr)
	s.Require().False(out.Hit)

	body := s.swingTo("bob")
	s.Equal(session.AttackMiss, body.Outcome)
	s.requireSameSwing(out, body, "bob")
}

// TestAHitCarriesTheTokenTheAttackerWasGiven is the load-bearing property of
// shared dice: the attacker and every witness hold the SAME string for one
// roll.
//
// The client that rolls an attack simulates the d20 falling through the room
// and publishes that throw so everybody watches the same die bounce off the
// same wall. That only works if the roller and the witnesses can agree on
// WHICH roll a throw belongs to.
//
// So the assertion is equality, not presence. Two non-empty tokens that
// disagree is exactly the bug this exists to prevent, and a test that only
// checked both were populated would pass while the feature stayed broken.
func (s *AttackEventsTestSuite) TestAHitCarriesTheTokenTheAttackerWasGiven() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{15, 5}})

	out := s.swing(mgr)
	s.Require().True(out.Hit, "15 + 3 STR + 2 proficiency clears AC 12")
	s.Require().NotEmpty(out.PresentationID, "the attacker is told which roll this was")

	for _, recipient := range []string{"alice", "bob"} {
		body := s.swingTo(recipient)
		s.Equal(session.AttackHit, body.Outcome)
		s.Equal(out.PresentationID, body.PresentationID,
			"%s reads the SAME token the attacker was handed", recipient)
	}
}

// TestAMissCarriesTheTokenTheAttackerWasGiven is the whiff's own half: a miss
// is the one a client is most likely to want the shared throw for — the die
// that clatters and comes up short is the whole drama of the roll.
func (s *AttackEventsTestSuite) TestAMissCarriesTheTokenTheAttackerWasGiven() {
	mgr := s.duelWithStream(&sequenceDice{rolls: []int{2, 5}})

	out := s.swing(mgr)
	s.Require().False(out.Hit, "2 + 5 is under AC 12")
	s.Require().NotEmpty(out.PresentationID)

	for _, recipient := range []string{"alice", "bob"} {
		body := s.swingTo(recipient)
		s.Equal(session.AttackMiss, body.Outcome)
		s.Equal(out.PresentationID, body.PresentationID,
			"%s reads the SAME token the attacker was handed", recipient)
	}
}

// TestTwoSwingsAreTwoDifferentRolls pins the other half of correlation: the
// token identifies ONE roll, so the next roll must not answer to it.
//
// A generator whose value never changed would satisfy every equality above and
// still be useless — every throw in the fight would replay as the first
// throw. Extra Attack is what makes this two DECLARED swings inside one turn
// rather than two fixtures compared side by side: a level-5 fighter banks two
// attacks from one Attack action, and each is its own roll a client simulates
// on its own.
func (s *AttackEventsTestSuite) TestTwoSwingsAreTwoDifferentRolls() {
	alice := armedFighter("alice")
	setLevel(alice, 5)
	mgr := s.duelWithStreamAndIDs(&sequenceDice{rolls: []int{2, 2}}, &countingPresentationIDs{}, alice)

	first := s.swing(mgr)
	second := s.swing(mgr)

	s.Require().False(first.Hit, "both swings whiff, so each costs one die")
	s.Require().False(second.Hit)
	s.Require().NotEmpty(first.PresentationID)
	s.Require().NotEmpty(second.PresentationID)
	s.NotEqual(first.PresentationID, second.PresentationID, "one token, one roll")

	// Read as bob, the witness: each swing reached him as its own act,
	// carrying its own token.
	toBob := s.swingsTo("bob")
	s.Require().Len(toBob, 2, "and they really are two swings")
	s.Equal(first.PresentationID, toBob[0].PresentationID)
	s.Equal(second.PresentationID, toBob[1].PresentationID)
}

// TestAnUnusableTokenRefusesTheSwingBeforeItRolls is the write side's fail-closed
// half, and the reason it is checked at all: correlation is only worth
// anything if it is ALWAYS there on a declared swing.
//
// A host whose generator hands back an empty or non-wire-safe string has a
// defect, not a swing without shared dice, and the honest answer is to refuse
// the command rather than record a beat nobody can correlate. It is refused
// BEFORE the dice, so a rejected swing rolls nothing, damages nobody, and
// writes nothing at all — the same discipline the payment door keeps, and the
// same one DeathSave already applies to its own token.
func (s *AttackEventsTestSuite) TestAnUnusableTokenRefusesTheSwingBeforeItRolls() {
	for _, token := range []string{"", "not/wire-safe", strings.Repeat("a", 129)} {
		s.Run(token, func() {
			rolls := 0
			mgr := s.duelWithStreamAndIDs(
				testDice{calls: &rolls}, literalPresentationIDs{value: token}, armedFighter("alice"))
			declaration := currentAttackID(s.T(), mgr, "sess", "alice")
			rollsBefore, savesBefore := rolls, s.characters.saves

			out, err := mgr.Attack(context.Background(), &session.AttackInput{
				Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: declaration,
			})
			s.Require().Nil(out)
			s.Require().Error(err)
			s.Equal(rollsBefore, rolls, "a swing with no usable token rolls nothing")
			s.Equal(savesBefore, s.characters.saves, "and writes nothing")
			s.Empty(s.stream.published, "and tells nobody")
		})
	}
}
