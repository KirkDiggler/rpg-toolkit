// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// second_wind_walk_test.go plays "a second wind" (rpg-project#1147, story
// rpg-project#1131) on its own small site, testdata/second-wind-walk.yaml: a
// fighter in a fight with one Sparring Guard is hurt, uses Second Wind (run
// from its description and paid at the door), finds the row spent and a
// second use refused, ends the fight and rests.
//
// The dice are scripted: every die is min(size, 10), so the fighter's d10 is
// a 10 and the initiative leaves the fighter first; once the guard's turn is
// near, every d20 is a 20, so the guard's dagger lands as a critical hit.

// secondWindDice is min(size, 10) per die, with every d20 a natural 20 once
// the guard is meant to strike.
type secondWindDice struct {
	mu     sync.Mutex
	strike bool
}

func (d *secondWindDice) Roll(_ context.Context, size int) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if size == 20 && d.strike {
		return 20, nil
	}
	if size > 10 {
		return 10, nil
	}
	return size, nil
}

// SecondWindWalkSuite plays the site through session.Manager.
type SecondWindWalkSuite struct {
	suite.Suite
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	dice       *secondWindDice
}

func TestSecondWindWalkSuite(t *testing.T) { suite.Run(t, new(SecondWindWalkSuite)) }

func (s *SecondWindWalkSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/second-wind-walk.yaml")
	s.Require().NoError(err, "the site file is there")
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the site compiles")
	s.characters = newFakeCharacters(secondWindFighter(s.T(), "alice", 30, 30))
	s.sessions, s.encounters, s.dice = newFakeSessions(), newFakeEncounters(), &secondWindDice{}
}

// manager is a Manager over the site's stores; a second one is the host
// reloading what was saved.
func (s *SecondWindWalkSuite) manager() *session.Manager {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: s.dice,
		TurnDriver: session.Driver(), Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *SecondWindWalkSuite) stored() *character.Data {
	return storedSheet(s.T(), s.characters, "alice")
}

// row is alice's Second Wind row, or fails the test.
func (s *SecondWindWalkSuite) row(mgr *session.Manager) session.Declaration {
	panel, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == refs.Features.SecondWind().String() {
			return d
		}
	}
	s.FailNow("alice is offered no Second Wind row")
	return session.Declaration{}
}

func (s *SecondWindWalkSuite) TestASecondWind() {
	ctx := context.Background()
	mgr := s.manager()

	// 1. A fighter is in a fight with one weak hostile, and is hurt: the
	// fight begins at launch, alice acts first, and she ends her turn so the
	// guard's turn is played, a critical dagger hit.
	_, err := mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &s.compiled, Party: []string{"alice"}})
	s.Require().NoError(err, "the site launches")
	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", turn.Active, "the fight is on and alice is first")
	s.Equal(30, s.stored().HitPoints)

	s.dice.mu.Lock()
	s.dice.strike = true
	s.dice.mu.Unlock()
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err)
	s.dice.mu.Lock()
	s.dice.strike = false
	s.dice.mu.Unlock()
	hurt := s.stored().HitPoints
	s.Equal(22, hurt, "a critical dagger hit: two d4 (4 each) off her 30")
	turn, err = mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", turn.Active, "it is alice's turn again")

	// 2. She uses Second Wind: the d10 (a 10) plus fighter level 3, capped at
	// her maximum. The bonus action is spent. A reload happens after it.
	row := s.row(mgr)
	s.True(row.Available)
	_, err = mgr.Activate(ctx, &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: row.ID})
	s.Require().NoError(err)
	const want = 30
	sheet := s.stored()
	s.Equal(want, sheet.HitPoints, "22 + the d10 (10) + fighter level 3 is 35, capped at her maximum 30")
	s.Zero(sheet.Resources[resources.SecondWind].Current, "the use is spent from the shared pool")
	s.Require().NotNil(sheet.ActionEconomy)
	s.Zero(sheet.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")

	mgr = s.manager()

	// 3. The row is no longer available this turn, and using it is refused.
	again := s.row(mgr)
	s.False(again.Available)
	s.Require().NotNil(again.Why)
	out, err := mgr.Activate(ctx, &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: row.ID})
	s.Nil(out)
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(want, s.stored().HitPoints, "nobody is healed twice")

	// 4. The fight ends: the guard is one blow from done and alice strikes it
	// with a critical hit. After a short rest Second Wind is back.
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "guard" {
			s.sessions.byID["sess"].NPCs[i].HitPoints = 1
		}
	}
	s.dice.mu.Lock()
	s.dice.strike = true
	s.dice.mu.Unlock()
	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "guard",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err)
	_, err = mgr.Rest(ctx, &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice"}},
	})
	s.Require().NoError(err, "the fight is over and she may rest")
	s.Equal(1, s.stored().Resources[resources.SecondWind].Current, "the short rest restored Second Wind")
}
