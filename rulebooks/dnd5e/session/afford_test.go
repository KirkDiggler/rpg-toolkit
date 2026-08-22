// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// afford_test.go pins rpg-toolkit#1138: the affordability read
// [session.Manager.Afford] and the invariant its own doc promises — it answers
// from the SAME price and the SAME gate a real swing pays through, never a
// second copy of the arithmetic, so the two structurally cannot disagree.
//
// It reuses aFight and armedFighter from economy_test.go / attack_test.go
// rather than building a fixture of its own: a scene built only for Afford
// could quietly stop being the scene Attack's own economy runs through, which
// is exactly the drift the invariant exists to rule out.
type AffordSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestAffordSuite(t *testing.T) { suite.Run(t, new(AffordSuite)) }

// fightScene is economy_test.go's own, reused rather than copied so the two
// suites are driving the identical scene.
func (s *AffordSuite) fightScene(level int, rolls ...int) {
	alice := armedFighter("alice")
	alice.Level = level

	s.mgr, s.sessions, s.encounters, s.characters = aFight(s.T(), alice, rolls)
}

func (s *AffordSuite) afford() *session.AffordOutput {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	return out
}

func (s *AffordSuite) swing() (*session.AttackOutput, error) {
	return s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton",
	})
}

// nextTurn is economy_test.go's own: ONE end, not two — rpg-toolkit#1162.
// The skeleton has no player, so ending alice's own turn drives the
// skeleton's through in the same call; a second, explicit EndTurn for the
// skeleton would be refused, since its turn already ended.
func (s *AffordSuite) nextTurn() {
	s.T().Helper()
	ctx := context.Background()

	_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "ending alice's turn")
}

// storedEconomy reads alice's persisted action economy, the same read
// economy_test.go's own TestARefusedSwingWritesNothing checks against.
func (s *AffordSuite) storedEconomy() *character.ActionEconomyData {
	s.T().Helper()
	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok, "alice must be in the repository")
	return stored.ActionEconomy
}

// TestAffordableMeansAttackWillNotRefuse is half the load-bearing invariant:
// Affordable == true implies a following Attack in the same state is NOT
// refused with ErrCannotAfford.
//
// A fresh turn's first swing is the case: nothing has been spent yet, so
// there is nothing for either verb to disagree about.
func (s *AffordSuite) TestAffordableMeansAttackWillNotRefuse() {
	s.fightScene(1, 15, 5, 1, 1)

	out := s.afford()
	s.Equal(session.ClockTurn, out.Clock)
	s.Require().Len(out.Declarations, 2, "attack and move (rpg-toolkit#1169)")

	decl := out.Declarations[0]
	s.Equal(session.VerbAttack, decl.Verb)
	s.True(decl.Affordable, "a fresh turn can still buy its first swing")
	s.Empty(decl.Shortfall, "affordable carries no shortfall")
	s.Equal(session.SlotAction, decl.Slot, "the first swing of a turn lights the action shape")

	_, err := s.swing()
	s.Require().NoError(err, "Afford said yes, so Attack must not refuse with ErrCannotAfford")
}

// TestUnaffordableMeansAttackRefusesWithTheSameShortfall is the other half:
// Affordable == false implies a following Attack IS refused with
// ErrCannotAfford, and Shortfall is the same currency text inside that
// refusal — not a paraphrase of it.
func (s *AffordSuite) TestUnaffordableMeansAttackRefusesWithTheSameShortfall() {
	s.fightScene(1, 15, 5, 1, 1)

	first, err := s.swing()
	s.Require().NoError(err, "the Attack action buys the first swing")
	s.Require().True(first.Hit)

	out := s.afford()
	s.Require().Len(out.Declarations, 2, "attack and move (rpg-toolkit#1169)")
	decl := out.Declarations[0]
	s.False(decl.Affordable, "nothing left to buy a second swing")
	s.NotEmpty(decl.Shortfall, "false carries a reason")
	s.Equal(session.SlotAction, decl.Slot, "the currency that ran out is the action slot")

	_, err = s.swing()
	s.Require().Error(err, "and there is nothing left to buy a second")
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), decl.Shortfall,
		"Afford's Shortfall must be the SAME currency text the refusal carries, not a second copy of it")
}

// TestBankedSwingIsAffordableAndLightsNoSlot walks Extra Attack's second
// swing (the brief's own required fixture): affordable, and Slot is SlotNone
// because a banked swing spends only capacity, never a per-turn slot.
func (s *AffordSuite) TestBankedSwingIsAffordableAndLightsNoSlot() {
	s.fightScene(5, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "swing 1, bought by the Attack action")

	out := s.afford()
	decl := out.Declarations[0]
	s.True(decl.Affordable, "extra attack banked a second swing")
	s.Empty(decl.Shortfall)
	s.Equal(session.SlotNone, decl.Slot,
		"a banked swing spends capacity alone — nothing lights the action/bonus/reaction shapes")

	_, err = s.swing()
	s.Require().NoError(err, "Afford said yes, so Attack must not refuse")
}

// TestExtraAttacksThirdSwingIsUnaffordable is the same scene's other end: once
// the bank Extra Attack granted is spent, the action itself has also already
// been spent, and Afford must say so.
func (s *AffordSuite) TestExtraAttacksThirdSwingIsUnaffordable() {
	s.fightScene(5, 1, 1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "swing 1")
	_, err = s.swing()
	s.Require().NoError(err, "swing 2, bought by Extra Attack")

	out := s.afford()
	decl := out.Declarations[0]
	s.False(decl.Affordable, "the bank Extra Attack granted is spent")
	s.Contains(decl.Shortfall, "action",
		"the action itself ran out too — it was spent buying the bank on swing 1")

	_, err = s.swing()
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), decl.Shortfall)
}

// TestANewTurnRefillsWhatAffordSees is TestANewTurnRefillsTheBank's own claim,
// asked through Afford instead of inferred from a swing succeeding: a bank
// that is charged and never refilled would make Afford permanently say no
// after the first turn.
func (s *AffordSuite) TestANewTurnRefillsWhatAffordSees() {
	s.fightScene(1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err)
	s.False(s.afford().Declarations[0].Affordable, "spent, on turn one")

	s.nextTurn()

	out := s.afford()
	s.Require().Len(out.Declarations, 2, "attack and move (rpg-toolkit#1169)")
	s.True(out.Declarations[0].Affordable, "a new turn buys a new swing")
	s.Equal(session.SlotAction, out.Declarations[0].Slot)
}

// TestFreeRoamAffordsNothing is TestFreeRoamChargesNothing's own claim, asked
// of Afford: a member with no bubble has no economy to report, and the answer
// is EMPTY, not a Declaration reporting Affordable:true for a free action —
// the two would look identical on a boolean and mean different things about
// why nothing is spent.
func (s *AffordSuite) TestFreeRoamAffordsNothing() {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: offsetWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, out.Clock)
	s.Empty(out.Declarations, "empty, not zero — the economy does not apply on the world clock at all")
}

// TestAffordSavesNothing is TestARefusedSwingWritesNothing's own claim, asked
// of a read that has no refusal to hide behind: Afford prices a turn by
// readying it exactly as priceSwing does for a real swing, and that readying
// must land on a sheet nobody saves.
//
// Both states are checked: a cold sheet Afford must not light and persist on
// its own, and a sheet a real swing already lit and spent, which a second ask
// must not touch any further.
func (s *AffordSuite) TestAffordSavesNothing() {
	s.fightScene(1, 15, 5, 1, 1)

	s.Nil(s.storedEconomy(), "a stored sheet starts cold: no economy at all")

	before := s.afford()
	s.True(before.Declarations[0].Affordable)
	s.Nil(s.storedEconomy(),
		"asking what alice can afford must not light and persist her turn — a read that ignited "+
			"the ledger on the way to answering would make asking indistinguishable from swinging")

	_, err := s.swing()
	s.Require().NoError(err, "the real swing lights and spends the economy")
	afterSwing := *s.storedEconomy()

	s.afford()
	s.Equal(afterSwing, *s.storedEconomy(),
		"asking again must not touch the sheet the real swing already wrote")
}
