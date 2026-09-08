// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// R11's flag, on the lane it belongs to.
//
// # Why the turn lane and not the roster
//
// The roster is identity and side: who is at the table, what they look like,
// which faction they fight for. Whether somebody is holding a spell together
// right now is per-turn state about a member of a fight, which is exactly what
// Participant carries and exactly the audience Active is for. Putting it on
// the roster would have given a client two lanes to read one member from, and
// the proto row that carries this has no field on the roster side at all.
type TurnConcentrationSuite struct {
	suite.Suite

	characters *fakeCharacters
	mgr        *session.Manager
}

func TestTurnConcentrationSuite(t *testing.T) {
	suite.Run(t, new(TurnConcentrationSuite))
}

func (s *TurnConcentrationSuite) SetupTest() {
	s.characters = testCharacters()

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)
}

// fight walks alice into the ogre so there is a turn clock to read. The world
// clock has no order and therefore no participants at all.
func (s *TurnConcentrationSuite) fight() {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: ambushPath(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the scene must actually put them in a fight")
}

// concentrateOn puts a concentrating condition on a stored character exactly
// as the game server holds one: an opaque blob the sheet loads back.
func (s *TurnConcentrationSuite) concentrateOn(member, spellRef, spellName string) {
	s.T().Helper()

	holding := conditions.NewConcentratingCondition(member, spellRef, spellName, 10)
	blob, err := holding.ToJSON()
	s.Require().NoError(err)

	stored, ok := s.characters.byID[member]
	s.Require().True(ok, "the fixture holds a character called %q", member)
	stored.Conditions = append(stored.Conditions, json.RawMessage(blob))
}

// rows indexes the participants of one member's turn read by member id.
func (s *TurnConcentrationSuite) rows(asker string) map[string]session.Participant {
	s.T().Helper()

	out, err := s.mgr.Turn(context.Background(), &session.TurnInput{
		Session: "sess", Member: asker,
	})
	s.Require().NoError(err)

	byID := make(map[string]session.Participant, len(out.Participants))
	for _, row := range out.Participants {
		byID[row.Member] = row
	}
	return byID
}

// TestTheTurnLaneSaysWhoIsConcentrating is R11 at the read a client actually
// makes.
func (s *TurnConcentrationSuite) TestTheTurnLaneSaysWhoIsConcentrating() {
	s.concentrateOn("alice", refs.Spells.TrueStrike().String(), "True Strike")
	s.fight()

	rows := s.rows("alice")
	s.Require().Len(rows, 2, "the whole order, and nobody else")

	s.True(rows["alice"].Concentrating,
		"the member holding True Strike together is concentrating")
	s.False(rows["ogre"].Concentrating,
		"a monster is false by construction: the value is a character sheet's own answer, "+
			"and a monster row has none")
}

// TestEverybodyInTheFightReadsTheSameFlag is what makes the row worth having.
//
// The flag exists for the people who cannot see the sheet, so the ogre's own
// read of alice must say what alice's read of alice says. A value that only
// appeared on the holder's own request would answer nobody's question.
func (s *TurnConcentrationSuite) TestEverybodyInTheFightReadsTheSameFlag() {
	s.concentrateOn("alice", refs.Spells.TrueStrike().String(), "True Strike")
	s.fight()

	s.True(s.rows("ogre")["alice"].Concentrating,
		"the same fight reads the same to everybody in it")
}

// TestAMemberHoldingNothingIsNotConcentrating is the zero, asserted rather
// than assumed: the flag has to be able to say no.
func (s *TurnConcentrationSuite) TestAMemberHoldingNothingIsNotConcentrating() {
	s.fight()

	s.False(s.rows("alice")["alice"].Concentrating,
		"a member holding no spell is not concentrating, and false is an answer")
}

// TestTheFlagFollowsTheSheetRatherThanTheRead pins the direction the value
// travels.
//
// A concentration that ends between two reads shows as ended on the second.
// The row is the sheet's answer at read time, not something this seam
// remembers from the last one.
func (s *TurnConcentrationSuite) TestTheFlagFollowsTheSheetRatherThanTheRead() {
	s.concentrateOn("alice", refs.Spells.TrueStrike().String(), "True Strike")
	s.fight()
	s.Require().True(s.rows("alice")["alice"].Concentrating)

	// The spell ends, by whatever route ended it. This seam is told nothing
	// and asks again.
	s.characters.byID["alice"].Conditions = nil

	s.False(s.rows("alice")["alice"].Concentrating,
		"a concentration that ended is gone from the next read")
}
