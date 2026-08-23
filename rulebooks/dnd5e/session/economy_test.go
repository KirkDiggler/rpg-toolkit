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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EconomySuite is what rpg-toolkit#1097 bought: a swing that costs something.
//
// It is the other half of TestFreeRoamChargesNothing, and the two are the whole
// ruling between them — the action economy is a FIGHT's economy, so the same
// verb charges nothing on the world clock and refuses a second swing on the turn
// clock. Neither test means much without the other.
//
// EVERY SCENE HERE IS A REAL FIGHT, driven through the session's own verbs: a
// skeleton arrives in plain sight, the composition forms a bubble, and the
// swings below are swings inside it. Nothing constructs an action economy by
// hand — if the seam stopped lighting one, these tests would fail rather than
// pass against a fixture that did the lighting for it.
type EconomySuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestEconomySuite(t *testing.T) { suite.Run(t, new(EconomySuite)) }

// skeletonAC is what the catalog skeleton defends with, and the reason the
// scenes below can script a miss: a d20 of 1 totals 6 against it.
const skeletonAC = 13

// fightScene puts an armed fighter of the given level in a hall and drops a
// skeleton next to her, which starts a fight.
//
// The level is the parameter because it is the only thing the cases differ in —
// Extra Attack is a class table, and asserting it at this seam means asserting
// that the seam ASKED, rather than that it counted.
//
// A MISS IS THE DEFAULT SCRIPT, and that is a scene requirement rather than
// squeamishness: this fixture's fighter deals 8 and the catalog skeleton holds
// 13, so two landed blows drop it, the last monster falling ends the fight, and
// alice is back on the world clock — where swings are free and the case under
// test has quietly stopped existing. Rolls of 1 keep the fight running so the
// economy is what the refusal is about.
//
// The rolls given are the SWINGS' rolls. Forming the bubble rolls initiative
// first — one die per member — and those are prepended here rather than left for
// every caller to remember, because a scene whose scripted 15 silently became an
// initiative roll reads as a miss nobody can explain. The first swing asserts it
// got the die it asked for, so this stops being true loudly rather than quietly.
func (s *EconomySuite) fightScene(level int, rolls ...int) {
	alice := armedFighter("alice")
	alice.Level = level

	s.mgr, s.sessions, s.encounters, s.characters = aFight(s.T(), alice, rolls)
}

// aFight puts one armed fighter in a plain hall and drops a catalog skeleton
// next to her, which starts a fight.
//
// A package-level function rather than a suite method because two suites need
// the same scene: this file's economy pins, and sentinels_test.go, whose whole
// job is to check what a host can match on when a refusal really happens. A
// second arrangement of the same fixture is a second thing to drift.
//
// It asserts the bubble for its callers. A scene that quietly formed none would
// leave every member on the world clock, where nothing is charged — so every
// economy claim built on it would pass by testing nothing at all.
func aFight(
	t fataler, alice *character.Data, rolls []int,
) (*session.Manager, *fakeSessions, *fakeEncounters, *fakeCharacters) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(alice)

	// initiativeRolls is one die per member of the bubble the spawn forms.
	const initiativeRolls = 2
	scripted := append(make([]int, initiativeRolls), rolls...)

	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: scripted}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	if err != nil {
		t.Fatalf("building manager: %v", err)
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: encounter.MemberID(alice.ID), Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the hall: %v", err)
	}
	data := enc.ToData()

	ctx := context.Background()
	if _, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	}); err != nil {
		t.Fatalf("starting the session: %v", err)
	}

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	if err != nil {
		t.Fatalf("spawning the skeleton: %v", err)
	}
	if spawned.Formed == nil {
		t.Fatalf("arriving in plain sight of %s must start a fight", alice.ID)
	}

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: alice.ID})
	if err != nil {
		t.Fatalf("asking what %s is waiting on: %v", alice.ID, err)
	}
	if turn.Clock != session.ClockTurn {
		t.Fatalf("%s is on the %s clock, where nothing costs anything", alice.ID, turn.Clock)
	}
	if turn.Round != 1 {
		t.Fatalf("a fight starts in round 1, not %d", turn.Round)
	}

	return mgr, sessions, encounters, characters
}

func (s *EconomySuite) swing() (*session.AttackOutput, error) {
	return s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton",
	})
}

// nextTurn takes the fight all the way round to alice again.
//
// ONE END, NOT TWO — rpg-toolkit#1162. The skeleton has no player: ending
// alice's own turn now drives the skeleton's through in the same call
// (ADR-0043), so a second, explicit EndTurn for the skeleton is refused —
// its turn already ended, and alice is active again by the time this
// returns.
func (s *EconomySuite) nextTurn() {
	s.T().Helper()
	ctx := context.Background()

	_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "ending alice's turn")

	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(2, turn.Round, "the fight really came back around")
}

// storedSkeleton reads the NPC's hit points back out of the session record.
func (s *EconomySuite) storedSkeleton() int {
	s.T().Helper()
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == "skeleton" {
			return npc.HitPoints
		}
	}
	s.FailNow("no skeleton in the record")
	return 0
}

// storedEconomy reads alice's persisted action economy.
func (s *EconomySuite) storedEconomy() *character.ActionEconomyData {
	s.T().Helper()
	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok, "alice must be in the repository")
	return stored.ActionEconomy
}

func (s *EconomySuite) story() []session.StoryEntry {
	s.T().Helper()
	out, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	return out
}

// TestALevel1FightersSecondSwingIsRefused is the headline, and the thing
// rpg-toolkit#1035 was opened to make true.
//
// One action, one swing, and the second ask is refused BY NAME rather than
// silently ignored or quietly resolved. The currency is in the message because a
// refusal a client cannot explain reads to a player as a bug: "action: 1 needed,
// 0 left" is the difference between "you can't do that" and "you have already
// taken your action this turn".
func (s *EconomySuite) TestALevel1FightersSecondSwingIsRefused() {
	s.fightScene(1, 15, 5, 1, 1)

	first, err := s.swing()
	s.Require().NoError(err, "the Attack action buys the first swing")
	s.Require().Equal(15, first.Roll, "the swing got the die the scene scripted for it")
	s.Require().Equal(skeletonAC, first.Against, "and resolved against the skeleton's own armour class")
	s.True(first.Hit, "15 + 3 STR + 2 proficiency beats it")

	_, err = s.swing()
	s.Require().Error(err, "and there is nothing left to buy a second")
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), "action",
		"the refusal names the currency that ran out")
	s.Contains(err.Error(), "alice",
		"and who ran out of it")
}

// TestTheRefusalIsOursAndNotResolutions is the #1058/#1066 law reaching the
// economy.
//
// resolution.ErrCannotPay is the gate's word for this and it must not be the
// host's: a client that branched on it would be coupled to the module the strike
// machine lives in, through the one channel boundary_test.go cannot see. The
// reason still travels — as TEXT, which is what the assertion above about the
// currency is really checking — and only the sentinel is ours alone.
func (s *EconomySuite) TestTheRefusalIsOursAndNotResolutions() {
	s.fightScene(1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err)
	_, err = s.swing()
	s.Require().Error(err)

	s.ErrorIs(err, session.ErrCannotAfford, "the host is answered in this package's vocabulary")
	s.NotErrorIs(err, resolution.ErrCannotPay,
		"and cannot reach resolution's, which would couple it to a module we intend to replace")
	s.NotErrorIs(err, resolution.ErrBadCost)
	s.NotErrorIs(err, resolution.ErrNoPayer)
	s.NotErrorIs(err, session.ErrBadCost,
		"an actor who ran out is not a malformed price, and the two must not be one sentinel")
}

// TestExtraAttackBuysASecondSwing pins the class table at the seam.
//
// A level-5 fighter's Attack action banks two swings, so the second lands and
// the third is refused — and the SAME code produced the level-1 case above,
// which is the actual claim. Nothing here counts to two; it asks the rulebook
// what the action bought and spends that.
func (s *EconomySuite) TestExtraAttackBuysASecondSwing() {
	s.fightScene(5, 1, 1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "swing 1, bought by the Attack action")
	_, err = s.swing()
	s.Require().NoError(err, "swing 2, bought by Extra Attack")

	_, err = s.swing()
	s.Require().Error(err, "swing 3 has nothing left")
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), "action",
		"and it is the action that ran out — the bank was emptied by the swing before")
}

// TestARefusedSwingWritesNothing is the #1056 shape asked of the economy.
//
// E2 refuses at the door — before the machine starts, before any damage exists —
// so this is the clean case that shape was named for: there is nothing to undo
// because nothing was done. A refusal that had persisted a sheet, or recorded a
// beat for a swing that never happened, would leave a host holding a story about
// an attack nobody made.
//
// All three stores are checked rather than one. The skeleton's hit points are
// the world, the story is the transcript, and alice's own economy is the sheet —
// and it is the third that could plausibly have moved, since the refused payment
// runs against a sheet this verb had already readied.
func (s *EconomySuite) TestARefusedSwingWritesNothing() {
	s.fightScene(1, 15, 5, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err)

	hpBefore := s.storedSkeleton()
	storyBefore := len(s.story())
	economyBefore := *s.storedEconomy()

	_, err = s.swing()
	s.Require().ErrorIs(err, session.ErrCannotAfford)

	s.Equal(hpBefore, s.storedSkeleton(), "a refused swing damaged nobody")
	s.Len(s.story(), storyBefore, "and recorded no beat")
	s.Equal(economyBefore, *s.storedEconomy(),
		"and did not spend, refill or otherwise touch the sheet it was refused against")

	var saved *session.SaveError
	s.False(errors.As(err, &saved),
		"nothing was written, so there is no partial save to report and no repair to do")
}

// TestTheFirstSwingSurvivesTheRefusal is the other half of writing nothing:
// what DID happen stays happened.
//
// The obvious wrong implementation rolls the swing back on the way out, or
// re-runs the verb and overwrites it. The blow that landed is durable and the
// beat that recorded it is in the transcript, and a refusal arriving after them
// changes neither.
func (s *EconomySuite) TestTheFirstSwingSurvivesTheRefusal() {
	s.fightScene(1, 15, 5, 1, 1)

	first, err := s.swing()
	s.Require().NoError(err)
	s.Require().True(first.Hit)
	s.Require().Equal(8, first.Damage, "15 + 3 STR + 2 proficiency lands; the die scripted to 5 plus 3 deals 8")

	_, err = s.swing()
	s.Require().ErrorIs(err, session.ErrCannotAfford)

	s.Equal(13-8, s.storedSkeleton(), "the blow that landed is still on the stored sheet")
	s.NotEmpty(s.story(), "and its beat is still in the story")
}

// TestANewTurnRefillsTheBank is the pin that keeps this a TURN economy rather
// than a session-long one.
//
// It is the failure the whole turn-marker question exists to prevent: a bank
// that is charged and never refilled gives every character exactly one action
// for the entire game, which would look like a working economy in the test above
// and be unplayable in a fight. The refill runs through the door's own refresh,
// keyed on the round the composition reports.
func (s *EconomySuite) TestANewTurnRefillsTheBank() {
	s.fightScene(1, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err)
	_, err = s.swing()
	s.Require().ErrorIs(err, session.ErrCannotAfford, "spent, on turn one")

	s.nextTurn()

	_, err = s.swing()
	s.Require().NoError(err, "a new turn buys a new swing")
	s.Equal(2, s.storedEconomy().TurnNumber,
		"and the sheet is now filed under the turn it was refilled for")
}

// TestABankLeftOverFromLastTurnDoesNotMispriceThisOne is the ordering bug this
// seam refreshes to avoid, and it is invisible from anywhere else.
//
// The door refreshes AFTER the caller has compiled the price: payAtTheDoor finds
// the payer, then refreshes, then charges. So a price compiled against the bank
// as STORED is a price for a bank that is about to be replaced.
//
// The scene is the one that makes it bite. A level-5 fighter takes one of her
// two swings on turn one and stops, so the sheet is persisted with a banked
// attack still on it. On turn two that bank is stale: the door is about to wipe
// it and seed a fresh turn. A seam that priced from the stored bank would send
// "spend one banked attack", the door would refresh the bank to empty, and the
// swing would be refused — a fighter unable to attack on her own turn BECAUSE
// she attacked on the last one, which is the worst-shaped bug in the file.
//
// So the refresh happens here, on the sheet this verb is about to hand over, and
// the price is compiled from what it leaves. The assertion is deliberately plain
// — the swing works — because the failure it guards is a refusal that would look
// exactly like the economy working.
func (s *EconomySuite) TestABankLeftOverFromLastTurnDoesNotMispriceThisOne() {
	s.fightScene(5, 1, 1, 1, 1)

	_, err := s.swing()
	s.Require().NoError(err, "one swing of the two Extra Attack bought")
	s.Require().Equal(1, s.storedEconomy().Granted["attacks"],
		"and the other is still banked, which is the state that makes this case real")

	s.nextTurn()

	_, err = s.swing()
	s.Require().NoError(err,
		"a fresh turn buys a fresh swing — a bank left over from last turn must not "+
			"price this one against capacity the door is about to wipe")
}

// TestTheTurnIsLitOnceAndSpentFromThereafter pins the ignition itself.
//
// Nothing in this stack put a character in combat before rpg-toolkit#1097:
// character.RefreshForTurn refuses to create an economy by design, so a cold
// sheet handed to the gate is refused forever rather than once. This seam lights
// it, and the assertion that matters is the SECOND one — a sheet re-lit on every
// ask would look identical on the first swing and would make the economy
// unspendable, since every swing would arrive at a freshly full bank.
func (s *EconomySuite) TestTheTurnIsLitOnceAndSpentFromThereafter() {
	s.fightScene(1, 1, 1, 1, 1)

	s.Nil(s.storedEconomy(), "a stored sheet starts cold: no economy at all")

	_, err := s.swing()
	s.Require().NoError(err)

	lit := s.storedEconomy()
	s.Require().NotNil(lit, "the swing lit the turn it was paid in")
	s.Equal(1, lit.TurnNumber, "filed under the round the composition reported")
	s.Zero(lit.ActionsRemaining, "and the action it cost is spent")

	_, err = s.swing()
	s.ErrorIs(err, session.ErrCannotAfford,
		"a second ask must not re-light the turn — that would refill the bank it just spent")
}
