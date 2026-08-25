// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

// attackDecl returns the single Attack declaration, failing if there is not
// exactly one.
func (s *AffordSuite) attackDecl(out *session.AffordOutput) session.Declaration {
	s.T().Helper()
	var attack *session.Declaration
	for i := range out.Declarations {
		if out.Declarations[i].Verb == session.VerbAttack {
			s.Require().Nil(attack, "expected exactly one Attack declaration")
			attack = &out.Declarations[i]
		}
	}
	s.Require().NotNil(attack, "expected exactly one Attack declaration")
	return *attack
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

// TestAvailableMeansAttackWillNotRefuse is half the load-bearing invariant:
// Available == true implies a following Attack in the same state is NOT
// refused with ErrCannotAfford.
//
// A fresh turn's first swing is the case: nothing has been spent yet, so
// there is nothing for either verb to disagree about.
func (s *AffordSuite) TestAvailableMeansAttackWillNotRefuse() {
	s.fightScene(1, 15, 5, 1, 1)

	out := s.afford()
	s.Equal(session.ClockTurn, out.Clock)
	s.Require().Len(out.Declarations, 3, "attack, move and end turn")

	decl := s.attackDecl(out)
	s.True(decl.Available, "a fresh turn can still buy its first swing")
	s.Nil(decl.Why, "available carries no shortfall")
	s.Equal(session.SlotAction, decl.Slot, "the first swing of a turn lights the action shape")
	s.NotEmpty(decl.ID, "a compiled Attack carries a selector ID")
	s.NotNil(decl.Attack, "a compiled Attack carries its AttackRef")
	s.Equal(session.TargetMember, decl.TargetKind)

	_, err := s.swing()
	s.Require().NoError(err, "Afford said yes, so Attack must not refuse with ErrCannotAfford")
}

// TestUnavailableMeansAttackRefusesWithTheSameShortfall is the other half:
// Available == false implies a following Attack IS refused with
// ErrCannotAfford, and Why.Text is the same currency text inside that
// refusal — not a paraphrase of it.
func (s *AffordSuite) TestUnavailableMeansAttackRefusesWithTheSameShortfall() {
	s.fightScene(1, 15, 5, 1, 1)

	first, err := s.swing()
	s.Require().NoError(err, "the Attack action buys the first swing")
	s.Require().True(first.Hit)

	out := s.afford()
	s.Require().Len(out.Declarations, 3, "attack, move and end turn")
	decl := s.attackDecl(out)
	s.False(decl.Available, "nothing left to buy a second swing")
	s.Require().NotNil(decl.Why)
	s.NotEmpty(decl.Why.Text, "false carries a reason")
	s.Equal(session.SlotAction, decl.Slot, "the currency that ran out is the action slot")
	s.NotEmpty(decl.ID, "a compiled Attack keeps its selector ID even when the budget fails")
	s.NotNil(decl.Attack, "a compiled Attack keeps its AttackRef even when the budget fails")
	s.NotEmpty(decl.Candidates, "the candidate rows remain alongside the budget refusal")

	_, err = s.swing()
	s.Require().Error(err, "and there is nothing left to buy a second")
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), decl.Why.Text,
		"Afford's Why.Text must be the SAME currency text the refusal carries, not a second copy of it")
}

// TestBankedSwingIsAvailableAndLightsNoSlot walks Extra Attack's second
// swing (the brief's own required fixture): available, and Slot is SlotNone
// because a banked swing spends only capacity, never a per-turn slot.
func (s *AffordSuite) TestBankedSwingIsAvailableAndLightsNoSlot() {
	s.fightScene(5, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "swing 1, bought by the Attack action")

	out := s.afford()
	decl := s.attackDecl(out)
	s.True(decl.Available, "extra attack banked a second swing")
	s.Nil(decl.Why)
	s.Equal(session.SlotNone, decl.Slot,
		"a banked swing spends capacity alone — nothing lights the action/bonus/reaction shapes")

	_, err = s.swing()
	s.Require().NoError(err, "Afford said yes, so Attack must not refuse")
}

// TestExtraAttacksThirdSwingIsUnavailable is the same scene's other end: once
// the bank Extra Attack granted is spent, the action itself has also already
// been spent, and Afford must say so.
func (s *AffordSuite) TestExtraAttacksThirdSwingIsUnavailable() {
	s.fightScene(5, 1, 1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "swing 1")
	_, err = s.swing()
	s.Require().NoError(err, "swing 2, bought by Extra Attack")

	out := s.afford()
	decl := s.attackDecl(out)
	s.False(decl.Available, "the bank Extra Attack granted is spent")
	s.Require().NotNil(decl.Why)
	s.Contains(decl.Why.Text, "action",
		"the action itself ran out too — it was spent buying the bank on swing 1")

	_, err = s.swing()
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), decl.Why.Text)
}

// TestANewTurnRefillsWhatAffordSees is TestANewTurnRefillsTheBank's own claim,
// asked through Afford instead of inferred from a swing succeeding: a bank
// that is charged and never refilled would make Afford permanently say no
// after the first turn.
func (s *AffordSuite) TestANewTurnRefillsWhatAffordSees() {
	s.fightScene(1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err)
	s.False(s.attackDecl(s.afford()).Available, "spent, on turn one")

	s.nextTurn()

	out := s.afford()
	s.Require().Len(out.Declarations, 3, "attack, move and end turn")
	s.True(s.attackDecl(out).Available, "a new turn buys a new swing")
	s.Equal(session.SlotAction, s.attackDecl(out).Slot)
}

// TestFreeRoamAffordsNothing is TestFreeRoamChargesNothing's own claim, asked
// of Afford: a member with no bubble has no economy to report, and the answer
// is EMPTY, not a Declaration reporting Available:true for a free action —
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
	s.True(s.attackDecl(before).Available)
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

// TestAttackDeclarationCarriesEveryCandidateInReach pins rpg-toolkit#1010/
// rpg-project#249 §6's reshape: a second monster within reach earns its own
// candidate row on the single Attack declaration rather than a second
// declaration. Each candidate carries the SAME slot the economy decided; only
// the member varies.
func (s *AffordSuite) TestAttackDeclarationCarriesEveryCandidateInReach() {
	s.fightScene(1, 15, 5, 1, 1)

	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skeleton-2", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 1, Y: 2}, // adjacent to alice, same as "skeleton"
	})
	s.Require().NoError(err)

	attack := s.attackDecl(s.afford())
	s.ElementsMatch([]string{"skeleton", "skeleton-2"}, candidateIDs(attack.Candidates))
	for _, c := range attack.Candidates {
		s.True(c.Available, "every in-reach candidate is available")
		s.Nil(c.Why)
	}
	s.True(attack.Available)
	s.Equal(session.SlotAction, attack.Slot)
}

// TestNoTargetInReachIsOneAttackDeclarationNotZero pins the other half:
// nothing in reach still answers, once — never an empty attack-declaration
// list a client could mistake for "nothing to ask about yet." The candidate
// row remains, each carrying its own target-specific verdict, while the
// declaration-level Why is NoTargetInReach.
func (s *AffordSuite) TestNoTargetInReachIsOneAttackDeclarationNotZero() {
	alice := armedFighter("alice")
	mgr, _, _, _ := aFight(s.T(), alice, []int{1, 1})
	s.mgr = mgr

	// Move the skeleton out of reach without ending the fight: still in
	// contact (encEveryoneSees keeps sight unlimited), just too far to swing.
	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: pathAwayFromSkeleton(),
	})
	// alice may not be active in every roll of aFight's own initiative, so a
	// refusal here would mean this fixture cannot walk — fail loudly rather
	// than silently asserting nothing.
	s.Require().NoError(err, "test fixture must be able to walk alice out of reach")

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	attack := s.attackDecl(out)
	s.False(attack.Available)
	s.Require().NotNil(attack.Why)
	s.Equal(session.ShortfallNoTargetInReach, attack.Why.Reason)
	s.Require().Len(attack.Candidates, 1, "the candidate row remains, even out of reach")
	s.False(attack.Candidates[0].Available)
	s.Require().NotNil(attack.Candidates[0].Why)
	s.Equal(session.ShortfallTargetOutOfReach, attack.Candidates[0].Why.Reason)
}

// pathAwayFromSkeleton walks alice five cells from her spawn (1,1), well
// past a longsword's one-cell reach from the skeleton at (2,1).
func pathAwayFromSkeleton() []spatial.Position {
	return []spatial.Position{
		{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}, {X: 1, Y: 5}, {X: 1, Y: 6},
	}
}

// TestShortfallCarriesTheStructuredCurrency pins rpg-toolkit#1010's other
// half of #1138's own case: Why is not just text — Reason, Currency,
// Needed and Left are the figures a UI acts on, agreeing with Text by
// construction.
func (s *AffordSuite) TestShortfallCarriesTheStructuredCurrency() {
	s.fightScene(1, 15, 5, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "spends the action")

	decl := s.attackDecl(s.afford())
	s.Require().NotNil(decl.Why)
	s.Equal(session.ShortfallNoBudget, decl.Why.Reason)
	s.Equal(session.CurrencyAction, decl.Why.Currency)
	s.Equal(1, decl.Why.Needed)
	s.Equal(0, decl.Why.Left)
	s.NotEmpty(decl.Why.Text)
}

// TestNotYourTurnIsAnnouncedByAfford is the seam's own promise: a client
// that reads Afford before trying Attack never sends a swing bob's own
// TestNotYourTurnIsRefused (attack_test.go) shows Attack would refuse.
func (s *AffordSuite) TestNotYourTurnIsAnnouncedByAfford() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed)

	out, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Equal(session.ClockTurn, out.Clock)
	s.Require().Len(out.Declarations, 3, "attack, move and end turn, all blocked the same way")
	for _, d := range out.Declarations {
		s.False(d.Available)
		s.Empty(d.ID, "a blocker carries no selector ID")
		s.Nil(d.Attack)
		s.Empty(d.Candidates)
		s.Require().NotNil(d.Why)
		s.Equal(session.ShortfallNotYourTurn, d.Why.Reason)
	}

	_, err = mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "bob", Target: "skel-1"})
	s.ErrorIs(err, session.ErrNotYourTurn, "Attack refuses exactly what Afford announced")
}
