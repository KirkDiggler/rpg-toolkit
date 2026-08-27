// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

// ClockBoundaryTestSuite is the acceptance for rpg-project#294, and every test in it
// is deliberately shaped the same way: put a condition on a sheet, END A TURN
// THROUGH THE REAL VERB, and read the answer back out of what was PERSISTED.
//
// Not one of them publishes a turn event by hand. That is the whole point. The
// suites that did — conditions/dodging_test.go, helped_test.go,
// unconscious_test.go — have passed for the entire time these rules were inert,
// because they were the only thing in the codebase publishing a turn boundary
// at all. combat.TurnManager, the sole publisher in the product, has zero call
// sites anywhere including its own tests.
//
// So the assertion that matters is not "the condition reacted". It is "the game
// told it to".
type ClockBoundaryTestSuite struct {
	suite.Suite

	ctx        context.Context
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestClockBoundarySuite(t *testing.T) { suite.Run(t, new(ClockBoundaryTestSuite)) }

func (s *ClockBoundaryTestSuite) SetupTest() { s.ctx = context.Background() }

// fight wires a two-player turn clock with alice active, and gives alice
// whatever conditions the test is about.
func (s *ClockBoundaryTestSuite) fight(aliceConditions ...json.RawMessage) *session.Manager {
	alice := armedFighter("alice")
	alice.Conditions = aliceConditions

	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(alice, armedFighter("bob"))

	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{15, 5}}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(s.ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)
	return mgr
}

func (s *ClockBoundaryTestSuite) endTurn(mgr *session.Manager, member string) {
	s.T().Helper()
	_, err := mgr.EndTurn(s.ctx, &session.EndTurnInput{
		Session: "sess", Member: member,
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", member),
	})
	s.Require().NoError(err)
}

// storedConditions reads back what the repository actually holds — not what an
// in-memory object thinks. A condition that expired but never reached disk
// expires again next request, forever.
func (s *ClockBoundaryTestSuite) storedConditions(id string) []json.RawMessage {
	s.T().Helper()
	sheet, err := s.characters.GetCharacter(s.ctx, id)
	s.Require().NoError(err)
	s.Require().NotNil(sheet)
	return sheet.Conditions
}

func (s *ClockBoundaryTestSuite) raw(c interface {
	ToJSON() (json.RawMessage, error)
}) json.RawMessage {
	s.T().Helper()
	b, err := c.ToJSON()
	s.Require().NoError(err)
	return b
}

// TestDodgeLapsesWhenItsOwnersTurnComesAround.
//
// RAW: dodging lasts until the start of your next turn. Before this slice it
// lasted until the end of the encounter, because nothing ever published a turn
// start — so a player who dodged once was harder to hit for the whole fight.
//
// alice dodges and ends her turn; bob ends his; the clock comes back to alice
// and HER turn start is what removes it.
func (s *ClockBoundaryTestSuite) TestDodgeLapsesWhenItsOwnersTurnComesAround() {
	mgr := s.fight(s.raw(&conditions.DodgingCondition{CharacterID: "alice"}))
	s.Require().Len(s.storedConditions("alice"), 1, "alice starts the fight dodging")

	s.endTurn(mgr, "alice")
	s.Require().Len(s.storedConditions("alice"), 1,
		"and is STILL dodging through bob's turn — that is the whole rule")

	s.endTurn(mgr, "bob")
	s.Empty(s.storedConditions("alice"),
		"alice's own turn starting is what ends it")
}

// TestSneakAttackForgetsItsDiceWhenTheTurnEnds.
//
// Once per turn, not once per fight. The flag is persisted state on the
// condition, so this asserts on the STORED BLOB rather than on presence:
// the condition is meant to survive, with its memory cleared.
func (s *ClockBoundaryTestSuite) TestSneakAttackForgetsItsDiceWhenTheTurnEnds() {
	mgr := s.fight(s.raw(&conditions.SneakAttackCondition{
		CharacterID: "alice", Level: 1, DamageDice: 1, UsedThisTurn: true,
	}))

	s.endTurn(mgr, "alice")

	stored := s.storedConditions("alice")
	s.Require().Len(stored, 1, "the condition survives; only its memory of this turn is cleared")

	var blob struct {
		Used bool `json:"used_this_turn"`
	}
	s.Require().NoError(json.Unmarshal(stored[0], &blob))
	s.False(blob.Used, "a rogue sneak attacks once per TURN, not once per fight")
}

// TestRageLapsesWhenTheBarbarianDidNothing.
//
// RAW: rage ends if your turn ends and you have neither attacked a hostile
// creature nor taken damage since your last turn. raging.go has implemented
// that correctly since long before this slice, on a turn end that never came —
// so rage was permanent, and TurnsActive was permanently 0.
//
// This is also the case Kirk used to rule out round boundaries: rage runs from
// your turn to your turn, so ending ALICE's turn is what decides it, whatever
// the round is doing.
func (s *ClockBoundaryTestSuite) TestRageLapsesWhenTheBarbarianDidNothing() {
	mgr := s.fight(s.raw(&conditions.RagingCondition{
		CharacterID: "alice", DamageBonus: 2, Level: 1, Source: "dnd5e:features:rage",
	}))
	s.Require().Len(s.storedConditions("alice"), 1, "alice starts raging")

	s.endTurn(mgr, "alice")

	s.Empty(s.storedConditions("alice"),
		"a barbarian who neither swung nor was hit stops raging when their turn ends")
}

// TestOneAdvanceReachesEveryoneAndEachDecidesForItself.
//
// Ending ALICE's turn crosses two boundaries: alice's turn ended, and bob's
// began. Both reach both members — R3 passes everyone in, and R1's shared bus
// is the reason an effect on one member can observe what happens to another.
//
// So the same announcement produces OPPOSITE outcomes for two identical
// conditions, decided entirely by each one's own guard
// (`event.SubjectID != c.CharacterID`): alice's dodge survives, because dodging
// ends at its owner's turn START and hers merely ended. Bob's ends, because his
// began.
//
// If the cast were scoped out here instead — "announce only to whoever's turn
// it is" — the arithmetic would still come out right for bob and wrong for
// nobody visible, while quietly making cross-participant effects impossible.
// That is the failure R1 exists to prevent, and this is what noticing it looks
// like.
func (s *ClockBoundaryTestSuite) TestOneAdvanceReachesEveryoneAndEachDecidesForItself() {
	mgr := s.fight(s.raw(&conditions.DodgingCondition{CharacterID: "alice"}))

	bob, err := s.characters.GetCharacter(s.ctx, "bob")
	s.Require().NoError(err)
	bob.Conditions = []json.RawMessage{s.raw(&conditions.DodgingCondition{CharacterID: "bob"})}
	s.Require().NoError(s.characters.SaveCharacter(s.ctx, bob))

	s.endTurn(mgr, "alice")

	s.Len(s.storedConditions("alice"), 1,
		"alice's dodge survives her turn ENDING — dodging lapses at its owner's turn start")
	s.Empty(s.storedConditions("bob"),
		"and bob's ends on the very same announcement, because his turn is the one that STARTED")
}

// TestNoCodePathLoadsAWorldWithoutNamingAnAnnouncer is the STRUCTURAL half, and
// it is deliberately not a behavioural one.
//
// A behavioural suite can only demonstrate that the paths it happened to
// exercise announced something. The defect this whole slice exists to close is
// the opposite shape — a capability that production never wires while every
// test wires one by hand — so the tests are the LAST place it shows up. That is
// exactly how TurnStartTopic came to have seven subscribers and no publisher,
// and how gamectx came to have five installers and one install
// (rpg-toolkit#1251).
//
// So this reads the package's own source and asserts the claim mechanically:
// every LoadEncounterInput literal names an Announcer. Which one is a judgement
// the load site makes and this test refuses to make for it — the point is that
// nobody gets to be silent.
//
// The sibling of TestNoCodePathProducesACastlessInteraction and
// TestNoCodePathProducesARoomlessInteraction, one module up.
func TestNoCodePathLoadsAWorldWithoutNamingAnAnnouncer(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}

	literals := 0
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			t.Fatalf("parsing %s: %v", path, perr)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "LoadEncounterInput" {
				return true
			}
			literals++

			named := false
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Announcer" {
					named = true
				}
			}
			if !named {
				t.Errorf("%s: a LoadEncounterInput names no Announcer — "+
					"a world whose clock can advance with nobody listening",
					fset.Position(lit.Pos()))
			}
			return true
		})
	}

	// A guard on the guard. If this package stops loading encounters the way it
	// does today, the loop above would pass by examining nothing at all — which
	// is the failure mode of every structural test, and worth one line to close.
	if literals == 0 {
		t.Fatal("found no LoadEncounterInput literals at all: this test has stopped testing anything")
	}
}
