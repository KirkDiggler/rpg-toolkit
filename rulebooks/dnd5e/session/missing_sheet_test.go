// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

// MissingSheetSuite holds the store's one answer (rpg-project#542, "One sheet
// store per verb"): a member the roster names as a player and whose sheet is
// absent is ErrNoCharacter on the attack, move, boundary and standing paths,
// while Exit, End and the reads still work so a broken run can be left and
// closed.
type MissingSheetSuite struct {
	suite.Suite

	characters *fakeCharacters
	mgr        *session.Manager
}

func TestMissingSheetSuite(t *testing.T) { suite.Run(t, new(MissingSheetSuite)) }

// start opens the duel on world; lose drops a sheet afterwards.
func (s *MissingSheetSuite) start(world *encounter.EncounterData) {
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)
}

func (s *MissingSheetSuite) lose(id string) { delete(s.characters.byID, id) }

func (s *MissingSheetSuite) TestTheAttackPathRefuses() {
	s.start(duelWorld(s.T()))
	id := currentAttackID(s.T(), s.mgr, "sess", "alice")
	s.lose("alice")
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: id,
	})
	s.Require().ErrorIs(err, session.ErrNoCharacter, "the attacker's own sheet is gone")
}

func (s *MissingSheetSuite) TestTheMovePathRefuses() {
	s.start(freeRoamDuelWorld(s.T()))
	s.lose("bob")
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{{X: 3, Y: 1}},
	})
	s.Require().ErrorIs(err, session.ErrNoCharacter)
}

func (s *MissingSheetSuite) TestTheBoundaryPathRefuses() {
	s.start(duelWorld(s.T()))
	id := currentEndTurnID(s.T(), s.mgr, "sess", "alice")
	s.lose("bob")
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{Session: "sess", Member: "alice", DeclarationID: id})
	s.Require().ErrorIs(err, session.ErrNoCharacter, "bob's turn starts, and his boundary has no sheet to land on")
}

func (s *MissingSheetSuite) TestTheStandingPathRefuses() {
	s.start(freeRoamDuelWorld(s.T()))
	s.lose("bob")
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().ErrorIs(err, session.ErrNoCharacter, "a walk's sight refresh asks who is standing, bob included")
}

func (s *MissingSheetSuite) TestTheRunCanStillBeLeftAndClosed() {
	s.start(freeRoamDuelWorld(s.T()))
	s.lose("bob")

	_, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().NoError(err, "reads still work")
	_, err = s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "reads still work")

	_, err = s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err, "the member with no sheet can leave")
	_, err = s.mgr.End(context.Background(), &session.EndInput{Session: "sess", Ending: "withdrawn"})
	s.Require().NoError(err, "the run can be closed")
}
