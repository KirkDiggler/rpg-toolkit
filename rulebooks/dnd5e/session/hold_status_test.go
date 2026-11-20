// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hold_status_test.go is the Keep's hold at the session door
// (rpg-project#990), on the kitchen's small site (kitchen_status_test.go): a
// plain line to the cook gives alice a hold, and the session refuses her
// walk with a sentence she can be shown, keeps refusing it across a reload,
// lets her sneak, and greys her Move out on her turn with what holds her.
const (
	// heldStatus ends when a fight she is in starts, or with her run.
	heldStatus = `{"name":"Held","changes":[{"cannot":["move"]}],` +
		`"maintenance":[{"ends_on":"fight starts"},{"ends_on":"run"}],"again":"ignored"}`
	// snaredStatus ends only at a rest, so it can be held inside a fight.
	snaredStatus = `{"name":"Snared","changes":[{"cannot":["move"]}],` +
		`"maintenance":[{"ends_on":"rest"}],"again":"ignored"}`
)

type HoldStatusSuite struct {
	suite.Suite
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestHoldStatusSuite(t *testing.T) { suite.Run(t, new(HoldStatusSuite)) }

func (s *HoldStatusSuite) manager() *session.Manager {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *HoldStatusSuite) SetupTest() {
	s.characters, s.sessions, s.encounters = newFakeCharacters(armedFighter("alice")), newFakeSessions(), newFakeEncounters()
	s.mgr = s.manager()
	// A bear's hit points, so the swing that starts the fight does not end it.
	cook := monsterAt("cook", refs.Monsters.BrownBear().String(), 0, 0)
	cook.Faction = "kitchen"
	cook.Options = []encounter.Option{cookLine("hold", "held", heldStatus), cookLine("snare", "snared", snaredStatus)}
	sc := ambushWorld(cook)
	sc.Monsters[0].ID, sc.Monsters[0].MemberID = "a-goblin", "a-goblin"
	sc.Field.Factions = []encounter.FactionInput{{ID: "kitchen", Mind: "cook"}}
	sc.Field.Dispositions = []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{"kitchen", encounter.FactionParty}, Stance: encounter.StanceNeutral,
	}}
	launchScene(s.T(), s.mgr, sc)
}

func (s *HoldStatusSuite) say(option string) {
	_, err := s.mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "cook", OptionID: option})
	s.Require().NoError(err)
}

func (s *HoldStatusSuite) where(mgr *session.Manager) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	return out.Position
}

// requireHeld is a refused own move: the session's CannotMoveError naming
// what holds her, the plain sentence, no composition error, nothing moved.
func (s *HoldStatusSuite) requireHeld(err error, sentence string) {
	s.Require().ErrorIs(err, session.ErrCannotMove)
	var held *session.CannotMoveError
	s.Require().ErrorAs(err, &held)
	s.Equal("alice", held.Member)
	s.Equal(sentence, held.Error())
	s.False(errors.Is(err, encounter.ErrCannotMove), "the composition's error stays behind the seam")
}

// heldRows counts name among the conditions alice's saved sheet shows, as the
// host reads it: one status is one row, whatever halves it is stored as.
func (s *HoldStatusSuite) heldRows(name string) int {
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	loaded, err := character.Load(context.Background(), stored)
	s.Require().NoError(err)
	view, err := loaded.StatusView(&character.StatusViewInput{})
	s.Require().NoError(err)
	n := 0
	for _, condition := range view.View.Conditions {
		if condition.Name == name {
			n++
		}
	}
	return n
}

// Catches the sheet fact left unfilled, or the refusal leaking the
// composition's error: held, alice's walk is refused whole with her sentence,
// across a reload of every store, and the hold is no reason to refuse a sneak.
func (s *HoldStatusSuite) TestAHeldPlayerCannotWalk() {
	s.say("hold")
	s.Equal(1, s.heldRows("Held"), "Held is on her saved sheet, once")
	before := s.where(s.mgr)

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: ambushPath()})
	s.requireHeld(err, "You are Held; you cannot move.")
	s.Equal(before, s.where(s.mgr), "she has not moved")

	reloaded := s.manager()
	_, err = reloaded.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: ambushPath()})
	s.requireHeld(err, "You are Held; you cannot move.")

	// Here the cook stands beside her, so sneaking is refused for being SEEN;
	// the hold is not a reason (the castle scene shows her sneak while held).
	_, err = reloaded.SetSneaking(context.Background(), &session.SetSneakingInput{Session: "sess", Member: "alice", Sneaking: true})
	s.NotErrorIs(err, session.ErrCannotMove, "a hold does not refuse sneaking")
}

// Catches a Move row that stays lit for a held player: inside a fight, on
// her turn, a hold given then greys her Move out with the hold's own
// sentence, and a step is refused with it.
func (s *HoldStatusSuite) TestAHeldPlayersMoveIsUnavailableOnHerTurn() {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: ambushPath()})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "precondition: the walk meets the goblin")
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal("alice", turn.Active, "precondition: alice's turn")

	s.say("snare")
	bar, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	var move *session.Declaration
	for i, d := range bar.Declarations {
		if d.Verb == session.VerbMove {
			move = &bar.Declarations[i]
		}
	}
	s.Require().NotNil(move, "her turn offers Move")
	s.False(move.Available, "held, Move is unavailable")
	s.Require().NotNil(move.Why)
	s.Equal(session.ShortfallHeld, move.Why.Reason)
	s.Equal("You are Snared; you cannot move.", move.Why.Text)

	at := s.where(s.mgr)
	_, err = s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", DeclarationID: move.ID,
		Path: []spatial.Position{{X: at.X, Y: at.Y - 1}}})
	s.requireHeld(err, "You are Snared; you cannot move.")
}

// Catches a hold that silences its holder, or one that stacks: held, alice
// says the line again, and still holds one Held.
func (s *HoldStatusSuite) TestAHeldPlayerStillSpeaks() {
	s.say("hold")
	s.say("hold")
	s.Equal(1, s.heldRows("Held"), "said again, still one Held")
}

// Catches the fight start not reaching the hold, or its ending not saved:
// held, alice attacks the cook beside her, the fight forms, Held is gone
// from her saved sheet, and on her turn she can step.
func (s *HoldStatusSuite) TestAFightStartingFreesTheHeld() {
	ctx := context.Background()
	s.say("hold")
	s.Require().Equal(1, s.heldRows("Held"))
	turn := s.swingAtTheCook()
	s.Zero(s.heldRows("Held"), "the fight starting ended Held, on her saved sheet")

	s.Require().Equal("alice", turn.Active, "precondition: alice's turn")
	bar, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	move := ""
	for _, d := range bar.Declarations {
		if d.Verb == session.VerbMove && d.Available {
			move = d.ID
		}
	}
	s.Require().NotEmpty(move, "her Move is offered again")
	at := s.where(s.mgr)
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", DeclarationID: move,
		Path: []spatial.Position{{X: at.X, Y: at.Y + 1}}})
	s.Require().NoError(err, "freed, she steps")
	s.NotEqual(at, s.where(s.mgr))
}

// swingAtTheCook has alice attack the cook beside her and returns the turn
// after: the swing turns the kitchen hostile and forms a fight.
func (s *HoldStatusSuite) swingAtTheCook() *session.TurnOutput {
	ctx := context.Background()
	attack := ""
	bar, err := s.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, d := range bar.Declarations {
		for _, c := range d.Candidates {
			if d.Verb == session.VerbAttack && c.Member == "cook" && c.Available {
				attack = d.ID
			}
		}
	}
	s.Require().NotEmpty(attack, "alice can swing at the cook beside her")
	_, err = s.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "alice", Target: "cook", DeclarationID: attack})
	s.Require().NoError(err)
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "the swing formed a fight")
	return turn
}

// Catches a fight's start ending a hold not written to end then (rpg-toolkit
// #2493): Snared ends only at a rest, so once alice's swing forms the fight
// she still holds it, her Move is unavailable, and a step on her turn is
// refused.
func (s *HoldStatusSuite) TestAHoldNotWrittenToEndAtAFightKeepsHolding() {
	s.say("snare")
	turn := s.swingAtTheCook()
	s.Equal(1, s.heldRows("Snared"), "the fight did not end Snared")
	s.Require().Equal("alice", turn.Active, "precondition: alice's turn")

	bar, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	var move *session.Declaration
	for i, d := range bar.Declarations {
		if d.Verb == session.VerbMove {
			move = &bar.Declarations[i]
		}
	}
	s.Require().NotNil(move)
	s.False(move.Available, "held, her Move is unavailable")
	at := s.where(s.mgr)
	_, err = s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", DeclarationID: move.ID,
		Path: []spatial.Position{{X: at.X, Y: at.Y + 1}}})
	s.requireHeld(err, "You are Snared; you cannot move.")
	s.Equal(at, s.where(s.mgr), "she has not moved")
}

// Catches a hold that outlives its holder's run: alice, held, leaves, and
// Held is gone from her saved sheet.
func (s *HoldStatusSuite) TestLeavingTheRunEndsTheHold() {
	s.say("hold")
	s.Require().Equal(1, s.heldRows("Held"))
	_, err := s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "a held player may always leave")
	s.Zero(s.heldRows("Held"), "her run ended, and Held with it")
}
