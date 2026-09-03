// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DeathTestSuite is the death lane arriving at the seam (rpg-toolkit#1079).
//
// DOWNED, not "down": at zero hit points and out of the fight, as distinct
// from PRONE, which is a posture the rulebook tracks and this package never
// gates on (Kirk's ruling, rpg-toolkit#1084). The opposite state is UP. The
// composition underneath keeps saying "down" in its own beat kind, and the
// scenes below assert on both sides of that translation deliberately.
//
// Everything here runs on REAL SHEETS. The composition holds no hit points and
// never will (law C1), so the whole question of who is standing is one this
// package answers, out of the sheets it already loads for a verb — and a suite
// that drove a fake capability would be testing the composition's side of a
// seam this slice is not building.
//
// The scenes are therefore end to end: a skeleton instantiated from the
// catalog with its own 13 hit points, a fighter with a longsword, and damage
// that really persists. What is asserted is what a client would see.
type DeathTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestDeathSuite(t *testing.T) { suite.Run(t, new(DeathTestSuite)) }

// cryptWorld is one room split by a solid wall of occluders, with a player on
// each side of it.
//
// The wall is the fixture's whole reason for existing. Bob has to be able to
// take an ordinary step WITHOUT joining the fight on the other side, so the
// scenes that ask what a SECOND player's walk does can ask it. A gap anywhere in
// the column and he sees the skeleton, contact forms, and the scene stops being
// the one under test.
//
// Unbounded retention because the story is the ledger the composition reads
// back to decide whether a death is news (rpg-toolkit#1077). A window that
// trimmed mid-scene would re-narrate a death these tests are counting.
func cryptWorld(t fataler) *encounter.EncounterData {
	occluders := make([]spatial.Position, 0, 10)
	for y := 0; y < 10; y++ {
		occluders = append(occluders, spatial.Position{X: 5, Y: float64(y)})
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("crypt", 0, 0, 10, 10)},
			Props:   occludingProps(occluders...),
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 8, Y: 8}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the crypt: %v", err)
	}
	data := enc.ToData()

	return &data
}

func (s *DeathTestSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *DeathTestSuite) startCrypt() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(s.T()),
	})
	s.Require().NoError(err)
	s.stream.published = nil
}

// spawnSkeleton puts a catalog skeleton next to alice, which starts a fight.
//
// Adjacent and in plain sight, so contact is immediate — asserted rather than
// assumed, because every scene below depends on there being a fight to end.
func (s *DeathTestSuite) spawnSkeleton() {
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "arriving in plain sight of alice starts a fight")
	s.Require().Equal(13, out.NPC.HitPoints, "the catalog skeleton, whole")
}

// aliceSwings runs one strike at the skeleton and returns what it did.
func (s *DeathTestSuite) aliceSwings() *session.AttackOutput {
	out, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err)

	return out
}

// bobSteps takes one ordinary step on the far side of the wall.
//
// It is the plainest thing a second player does, and it is what makes the
// world look: every sight refresh consults the rulebook about who is standing.
func (s *DeathTestSuite) bobSteps() {
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{hexCell(8, 7)},
	})
	s.Require().NoError(err)
}

// storedHP reads a spawned NPC's hit points back out of the session record.
func (s *DeathTestSuite) storedHP(id string) int {
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == id {
			return npc.HitPoints
		}
	}
	s.Require().Fail("no stored sheet for " + id)

	return -1
}

// eventsOfKind collects everything of one kind published since the last reset.
func (s *DeathTestSuite) eventsOfKind(kind session.EventKind) []session.Event {
	var out []session.Event
	for _, event := range s.stream.published {
		if event.Kind == kind {
			out = append(out, event)
		}
	}

	return out
}

// payloadOf decodes a beat into the fields these scenes assert on.
func (s *DeathTestSuite) payloadOf(event session.Event) map[string]any {
	var beat map[string]any
	s.Require().NoError(json.Unmarshal(event.Payload, &beat))

	return beat
}

// swingUntilTheSkeletonFalls is alice hitting it until it stops standing.
//
// The bound is a runaway guard and nothing more — it is deliberately far above
// the three swings this fixture's deterministic damage actually takes, so a
// change to the weapon, the dice or the catalog's hit points moves the number of
// iterations and moves no test. What holds the scene honest is the assertion
// after the loop: if the skeleton is not at zero, this fails saying so rather
// than letting a half-dead monster into a scene about death.
func (s *DeathTestSuite) swingUntilTheSkeletonFalls() {
	s.T().Helper()

	const runaway = 25
	for i := 0; i < runaway && s.storedHP("skeleton") > 0; i++ {
		s.aliceSwings()
		if s.storedHP("skeleton") <= 0 {
			break
		}
		// The lawful damage die caps the longsword at 8 + 3, so the catalog
		// skeleton outlives one swing — and alice gets one attack per turn.
		// End her turn (the skeleton's own driven turn passes) so the next
		// iteration swings on a fresh one.
		_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
			Session: "sess", Member: "alice",
			DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "alice"),
		})
		s.Require().NoError(err)
	}
	s.Require().Zero(s.storedHP("skeleton"), "the blows really landed on the stored sheet")
}

// dropTheSkeleton swings until the skeleton is at zero, and is the whole scene.
//
// It used to be "the scene up to the moment of death, with nobody having looked
// yet", and the change of description is the change of behavior: the killing
// blow is now the moment the world looks (rpg-toolkit#1083). Nothing after this
// helper has to walk, tick, or ask.
func (s *DeathTestSuite) dropTheSkeleton() {
	s.startCrypt()
	s.spawnSkeleton()
	s.swingUntilTheSkeletonFalls()
}

// TestTheSwingNoticesItsOwnKill is this suite's opening pin, FLIPPED.
//
// It used to say the opposite and said so by name: recording an outcome was not
// a sight refresh, so a killing blow landed, persisted, and left the world not
// knowing — no downed event, no ending, the turn order still holding a body —
// until somebody walked. A party that cleared the room and stood still was in a
// fight with a corpse.
//
// rpg-toolkit#1083 put the standing consult inside the composition's own Record,
// and this seam moved to meet it: the sheets the swing changed are written back
// BEFORE the outcome is recorded, because the capability answering the
// composition's question reads those sheets (see Manager.Attack).
//
// So the whole scene is one verb. Nobody walks, nobody ticks, nobody asks.
func (s *DeathTestSuite) TestTheSwingNoticesItsOwnKill() {
	s.dropTheSkeleton()

	s.NotEmpty(s.eventsOfKind(session.EventDowned),
		"the blow is recorded, and the same call notices what the blow did")
	s.NotEmpty(s.eventsOfKind(session.EventFightEnded),
		"and the fight it ended is over")

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, turn.Clock,
		"alice is free-roaming, and the party never had to move to find out")
}

// TestEveryMemberHearsTheDeath is the headline: the stream carries the death, on
// the swing that caused it.
//
// The audience is not scoped to whoever swung — a member downed anywhere on the
// map is news to everybody, including bob, who is behind a solid wall and did
// nothing at all.
//
// The event kind is EventDowned while the beat inside it still says "down":
// that asymmetry is the ruling, and asserting both here is what would notice if
// either half drifted.
func (s *DeathTestSuite) TestEveryMemberHearsTheDeath() {
	s.dropTheSkeleton()

	down := s.eventsOfKind(session.EventDowned)
	s.Require().Len(down, 3, "every member hears it: alice, bob, and the skeleton's own audience")

	heard := map[string]bool{}
	for _, event := range down {
		heard[event.Recipient] = true
		s.Equal(session.EventDowned, event.Kind, "the seam's word is downed")
		s.Equal("down", s.payloadOf(event)["beat"], "while the composition's own beat kind is untouched")
		s.Equal("skeleton", s.payloadOf(event)["member"], "and it names who fell")
	}
	s.Equal(map[string]bool{"alice": true, "bob": true, "skeleton": true}, heard)
}

// TestTheDeathIsNotAnnouncedTwice is the story-ledger dedup surviving the trip
// to a client, which is the thing that makes a second consult site SAFE.
//
// The swing notices, and then bob takes an ordinary step that consults again. A
// table that heard the skeleton fall twice would render it twice.
func (s *DeathTestSuite) TestTheDeathIsNotAnnouncedTwice() {
	s.dropTheSkeleton()
	s.Require().Len(s.eventsOfKind(session.EventDowned), 3)

	s.bobSteps()

	s.Len(s.eventsOfKind(session.EventDowned), 3, "one body, announced once")
	s.Len(s.eventsOfKind(session.EventFightEnded), 3, "and one ending")
}

// TestTheLastOneDownedEndsTheFightByDefeat is the slice's own sentence.
//
// Nobody called Dissolve, and nobody walked either. The only verb in this scene
// is alice's swing, and the ending rides out on that swing's own fan-out.
func (s *DeathTestSuite) TestTheLastOneDownedEndsTheFightByDefeat() {
	s.dropTheSkeleton()

	ended := s.eventsOfKind(session.EventFightEnded)
	s.Require().NotEmpty(ended, "the last one down ends the fight")

	beat := s.payloadOf(ended[0])
	s.Equal(string(session.DissolveByDefeat), beat["cause"],
		"and it says the fight was lost rather than left")
	s.ElementsMatch([]any{"alice", "skeleton"}, beat["members"],
		"everyone the fight held comes back out of it")

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, turn.Clock, "alice is free-roaming again, and nobody asked for that")
}

// TestDownedAndFightEndedCarryTypedBodies pins rpg-toolkit#941's other pair:
// the killing blow's own consequences — the body, and the fight ending it —
// reach a client as DownedBody and FightEndedBody, not as payload a client
// decodes itself.
func (s *DeathTestSuite) TestDownedAndFightEndedCarryTypedBodies() {
	s.dropTheSkeleton()

	downed := s.eventsOfKind(session.EventDowned)
	s.Require().NotEmpty(downed)
	body, ok := downed[0].Body.(session.DownedBody)
	s.Require().True(ok, "got %T", downed[0].Body)
	s.Equal("skeleton", body.Member)

	ended := s.eventsOfKind(session.EventFightEnded)
	s.Require().NotEmpty(ended)
	endedBody, ok := ended[0].Body.(session.FightEndedBody)
	s.Require().True(ok, "got %T", ended[0].Body)
	s.Equal(session.DissolveByDefeat, endedBody.Cause)
}

// TestTheBeatOrderReachesTheClientAsCauseThenEffect is the composition's
// ordering law arriving where a table actually reads it.
//
// The strike, the body, the ending — in that order, in one response's worth of
// events. A client renders in sequence order, so an ending delivered ahead of
// the death that caused it would narrate the fight as won before anybody fell.
func (s *DeathTestSuite) TestTheBeatOrderReachesTheClientAsCauseThenEffect() {
	s.dropTheSkeleton()

	var kinds []session.EventKind
	for _, event := range s.stream.published {
		if event.Recipient != "alice" {
			continue
		}
		switch event.Kind {
		case session.EventStruck, session.EventDowned, session.EventFightEnded:
			kinds = append(kinds, event.Kind)
		default:
		}
	}

	s.Require().GreaterOrEqual(len(kinds), 3)
	s.Equal([]session.EventKind{session.EventDowned, session.EventFightEnded}, kinds[len(kinds)-2:],
		"the last strike, then the body, then the ending the body explains")
	s.Equal(session.EventStruck, kinds[len(kinds)-3])
}

// TestTheEncounterOutlivesTheFight is ruled fork (c) surviving the seam, and
// ruled fork (a) with it.
//
// The old stack closed the whole encounter when the hostiles were defeated,
// which in a multi-room dungeon means clearing room one ends the dungeon. Only
// the BUBBLE ends here. And the downed member is still a member: on the map,
// answerable by Where, and walkable past.
func (s *DeathTestSuite) TestTheEncounterOutlivesTheFight() {
	s.dropTheSkeleton()

	ctx := context.Background()

	status, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open, "the fight ended; the dungeon did not")
	s.Nil(status.Outcome)

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "skeleton"})
	s.Require().NoError(err, "a downed member still has a position, and asking for it is not an error")
	s.Equal(spatial.Position{X: 2, Y: 1}, where.Position, "it fell where it stood")
}

// TestTheSurvivorWalksAgain is the consequence a player actually feels.
//
// Alice is refused every free-roam verb with ErrInBubble while the fight runs
// (rpg-toolkit#1024). Her own killing blow ends it, so the walk is hers on the
// very next call — and this is the assertion that would fail if the ending were
// narrated without actually re-homing anyone.
//
// The control moved. It used to sit AFTER the killing blow, because the fight
// outlived it; it now has to sit before, which is the whole change stated as a
// scene rather than as a claim.
func (s *DeathTestSuite) TestTheSurvivorWalksAgain() {
	s.startCrypt()
	s.spawnSkeleton()

	// Control: mid-fight, she is on the turn clock, not free-roaming — a
	// direct blocked Move no longer proves this alone since rpg-toolkit#1169,
	// which lets the active member of a bubble still walk on the turn clock,
	// at a cost. See move_test.go's own turn-clock suite for that behavior.
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockTurn, turn.Clock, "control: mid-fight, she is on the turn clock")

	s.swingUntilTheSkeletonFalls()

	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().NoError(err, "the fight is over, so the walk is hers again")
}

// TestNothingReachesAClientUnnamed is the whole scene's guard rail.
//
// Three beats now leave one Attack — the strike, the death, and the ending — and
// any of them reaching a client as EventUnknown would leave a table unable to
// narrate the moment the fight was won.
//
// It is also the answer to the question rpg-toolkit#1083 asked of this seam:
// whether the new call site needs new vocabulary. It does not. The beats are the
// same kinds kindOf already names; only the verb they arrive on changed.
func (s *DeathTestSuite) TestNothingReachesAClientUnnamed() {
	s.dropTheSkeleton()

	s.Require().NotEmpty(s.stream.published)
	for _, event := range s.stream.published {
		s.NotEqual(session.EventUnknown, event.Kind,
			"beat at seq %d reached %s unnamed", event.Seq, event.Recipient)
	}
}

// duelAtZero drops alice with two of bob's swings and returns nothing but the
// fact of it.
//
// Two characters standing next to each other, which is the only arrangement
// this package can drive a character to zero in: Attack compiles character
// attackers only, so a monster cannot be the one who does it.
func (s *DeathTestSuite) duelAtZero() {
	s.characters.byID["bob"].Level = 5
	// The lawful damage die caps the longsword at 8 + 3 per swing, so two
	// swings (22) must be enough to end alice: 12 hit points keeps the whole
	// takedown inside bob's own two-attack turn, which is what the fixture
	// always needed from the dice.
	s.characters.byID["alice"].HitPoints = 12
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world",
		World: turnWorld(freeRoamDuelWorld(s.T()), []string{"alice", "bob"}, 1),
	})
	s.Require().NoError(err)

	for i := 0; i < 4 && s.characters.byID["alice"].HitPoints > 0; i++ {
		_, err = s.mgr.Attack(context.Background(), &session.AttackInput{
			Session: "sess", Attacker: "bob", Target: "alice",
			DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "bob"),
		})
		s.Require().NoError(err)
	}
	s.Require().Zero(s.characters.byID["alice"].HitPoints, "alice is down")

	// Dying retains initiative. Settle Bob's completed attack turn so the
	// explicit-save turn reaches Alice instead of relying on the old splice.
	_, err = s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "bob",
		DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "bob"),
	})
	s.Require().NoError(err)
}

// TestTheKillingBlowNoticesACHARACTERToo is the other store's half of the same
// ordering, and it is the half that could rot silently.
//
// The seam answers about monsters out of the session record — aliased, so a
// sheet this verb folded in is what the next consult reads — and about players
// out of the HOST'S REPOSITORY, which is only current once SaveCharacter has run.
// A version of Attack that folded the NPC sheets early and left the character
// write where it was would pass every skeleton scene above and fail here.
//
// Two characters, because Attack compiles character attackers only: bob is the
// only thing in this package that can drive alice to zero.
func (s *DeathTestSuite) TestTheKillingBlowNoticesACHARACTERToo() {
	s.characters.byID["bob"].Level = 5
	// Same arithmetic duelAtZero keeps: 12 hit points lets bob's two-attack
	// turn carry the takedown under the lawful damage die.
	s.characters.byID["alice"].HitPoints = 12
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world",
		World: turnWorld(freeRoamDuelWorld(s.T()), []string{"alice", "bob"}, 1),
	})
	s.Require().NoError(err)
	s.stream.published = nil

	for i := 0; i < 4 && s.characters.byID["alice"].HitPoints > 0; i++ {
		_, aerr := s.mgr.Attack(context.Background(), &session.AttackInput{
			Session: "sess", Attacker: "bob", Target: "alice",
			DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "bob"),
		})
		s.Require().NoError(aerr)
	}
	s.Require().Zero(s.characters.byID["alice"].HitPoints)

	down := s.eventsOfKind(session.EventDowned)
	s.Require().NotEmpty(down, "the blow that dropped her is the blow that says so")
	s.Equal("alice", s.payloadOf(down[0])["member"])
}

// brokenAfterWriting is a repository that answers reads until something is
// written to it, and then stops — the shape of a store that goes away
// mid-verb, at the one moment this verb now has something durable to lose.
type brokenAfterWriting struct {
	*fakeCharacters
	wrote bool
}

var errStoreWentAway = errors.New("the character store went away")

func (b *brokenAfterWriting) SaveCharacter(ctx context.Context, data *character.Data) error {
	b.wrote = true

	return b.fakeCharacters.SaveCharacter(ctx, data)
}

func (b *brokenAfterWriting) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if b.wrote {
		return nil, errStoreWentAway
	}

	return b.fakeCharacters.GetCharacter(ctx, id)
}

// failNthArmedSaveCharacters lets setup writes land, then refuses one exact
// save after it is armed. The successful writes still delegate to the copying
// store, so the assertions below inspect durable values rather than call counts.
type failNthArmedSaveCharacters struct {
	*fakeCharacters
	armed    bool
	attempts int
	failAt   int
	err      error
}

func (f *failNthArmedSaveCharacters) SaveCharacter(ctx context.Context, data *character.Data) error {
	if f.armed {
		f.attempts++
		if f.attempts == f.failAt {
			return f.err
		}
	}
	return f.fakeCharacters.SaveCharacter(ctx, data)
}

var errBoundaryCharacterSave = errors.New("the boundary character save was refused")

// TestAKillingAttackReportsTheNestedBoundarySaveFailure is the reachable
// rpg-toolkit#1403 regression. One real Attack pays and saves Alice's sheet,
// drops the last monster, records the blow, notices the body, dissolves the
// fight, and announces its combat-end boundary. That announcement removes her
// rage and attempts a newer save of the same sheet; only that second save is
// refused.
//
// Record returns the nested SaveError through noticeDown. reportUnrecorded must
// expose one complete outer report to ordinary errors.As: Alice is in Written
// because the paid attack sheet landed, and independently in Failed because the
// newer boundary sheet did not, followed by the encounter whose new story was
// never committed. The stored world and NPC sheet stay at their pre-call truth,
// while the already-written action-economy state remains durable.
func (s *DeathTestSuite) TestAKillingAttackReportsTheNestedBoundarySaveFailure() {
	alice := armedFighter("alice")
	rage, err := (&conditions.RagingCondition{
		CharacterID: "alice", DamageBonus: 2, Level: 1, Source: "dnd5e:features:rage",
	}).ToJSON()
	s.Require().NoError(err)
	alice.Conditions = []json.RawMessage{rage}

	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(alice, armedFighter("bob"))
	failing := &failNthArmedSaveCharacters{
		fakeCharacters: s.characters,
		failAt:         2,
		err:            errBoundaryCharacterSave,
	}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: failing, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	s.startCrypt()
	s.spawnSkeleton()
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			s.sessions.byID["sess"].NPCs[i].HitPoints = 1
		}
	}
	s.Require().Equal(1, s.storedHP("skeleton"), "the next hit will decide the fight")

	declaration := currentAttackID(s.T(), mgr, "sess", "alice")
	beforeWorld, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	beforeAlice, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Require().Nil(beforeAlice.ActionEconomy)
	failing.armed = true

	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: declaration,
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBoundaryCharacterSave, "the repository cause stays matchable")
	s.Equal(2, failing.attempts, "the paid sheet landed before the boundary save failed")

	var reported *session.SaveError
	s.Require().True(errors.As(err, &reported), "ordinary errors.As reaches the complete report")
	s.Equal([]string{"character:alice"}, reported.Report.Written)
	s.Equal([]string{"character:alice", "encounter:world"}, reported.Report.Failed,
		"the nested failed aggregate must not be hidden by the recording boundary")

	afterWorld, getErr := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(getErr)
	s.Equal(beforeWorld, afterWorld, "the failed recording never commits its in-memory story or dissolve")
	s.Equal(1, s.storedHP("skeleton"), "the session-held monster damage was not persisted")

	afterAlice, getErr := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(getErr)
	s.NotNil(afterAlice.ActionEconomy, "the earlier paid-sheet write remains durable")
	s.Len(afterAlice.Conditions, 1,
		"the newer combat-end removal failed, matching the report's Failed identity")
}

// TestASwingThatCannotRecordStillNamesTheSheetItWrote is rpg-toolkit#1056's
// lesson held against the new ordering, and it is why that ordering needed a
// guard rather than just a comment.
//
// The sheets are now written BEFORE the outcome is recorded, so a Record that
// fails has damage already on disk. A caller told only "it failed" would retry a
// swing whose damage landed — which is the difference between repair and retry
// that the report exists to carry.
func (s *DeathTestSuite) TestASwingThatCannotRecordStillNamesTheSheetItWrote() {
	chars := &brokenAfterWriting{
		fakeCharacters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
	}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: s.stream,
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world",
		World: turnWorld(freeRoamDuelWorld(s.T()), []string{"alice", "bob"}, 1),
	})
	s.Require().NoError(err)

	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "bob", Target: "alice",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "bob"),
	})
	s.Require().Error(err)
	s.ErrorIs(err, errStoreWentAway, "the host's own failure is not flattened into ours")

	var reported *session.SaveError
	s.Require().ErrorAs(err, &reported,
		"a swing that wrote a sheet and then failed reports what it wrote")
	s.NotEmpty(reported.Report.Written, "and names it, so the caller repairs rather than retries")
	s.Contains(reported.Report.Failed, "encounter:world", "while the world it describes did not land")
}

// TestADownedActorCannotSwing is rpg-toolkit#845 refused by name.
//
// DOWNED is the word, and the distinction is not pedantry: a PRONE character
// swings perfectly well (at disadvantage), so a gate that conflated the two
// would silently disarm anybody knocked flat. Nothing here reads posture.
//
// D1 closed it inside a fight, structurally: a body is spliced out of the turn
// order, so the seam above has nothing to offer it. Free roam has no turn
// order, and this duel is free roam — nobody is in a bubble — so the defect
// survives there until something refuses it, and this is that refusal.
func (s *DeathTestSuite) TestADownedActorCannotSwing() {
	s.duelAtZero()

	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
	})
	s.Require().ErrorIs(err, session.ErrDowned)
	s.Contains(err.Error(), "alice", "and the refusal names who could not swing")
	s.NotErrorIs(err, session.ErrNoMember, "she is still a member; she is down")
}

// TestADownedActorCannotWalk is the same ruling on the other verb a body could
// still drive.
func (s *DeathTestSuite) TestADownedActorCannotWalk() {
	s.duelAtZero()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().ErrorIs(err, session.ErrDowned)
	s.Contains(err.Error(), "alice")
}

// TestAMemberStillUpIsNotRefused is the negative control that makes the two
// refusals above mean something.
//
// Bob is in the same world, on the same verbs, at full hit points. A gate that
// refused everybody would pass both tests above and fail this one.
func (s *DeathTestSuite) TestAMemberStillUpIsNotRefused() {
	s.duelAtZero()

	// Alice's Dying slot remains in initiative. Her independently available
	// End Turn advances to Bob, whose ordinary move remains available.
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice",
		DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err)
	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{{X: 2, Y: 2}},
		DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "bob"),
	})
	s.Require().NoError(err, "the one still standing walks")
}

// TestAKillingBlowAboutADownedMemberIsStillLegal is ruled fork (a) at the seam.
//
// Recording an outcome ABOUT a downed member must stay legal — the killing
// stroke is itself a beat about somebody who is now down, and a gate that could
// not tell the actor from the target would make the death un-narratable.
// Swinging at a downed member may be narratively silly; refusing it is a
// different ruling, and nobody made it.
func (s *DeathTestSuite) TestAKillingBlowAboutADownedMemberIsStillLegal() {
	s.duelAtZero()

	story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)
	lastStrike := false
	for _, event := range story {
		var beat struct {
			Beat    string   `json:"beat"`
			Targets []string `json:"targets"`
		}
		if json.Unmarshal(event.Payload, &beat) == nil && beat.Beat == "struck" && slices.Contains(beat.Targets, "alice") {
			lastStrike = true
		}
	}
	s.True(lastStrike, "the blow that made alice down was still recorded about her")
}

// TestTheReadsStayOpenToTheDowned is the rest of fork (a): a downed member is
// still a member, and every question about one still has an answer.
//
// Downed, not dead — the distinction has teeth. Zero hit points has no exit in
// v1, but the name does not claim the character is gone, and when death saves
// arrive these reads are what a client renders them from.
// TestADownedMemberIsSplicedOutOfParticipantsToo pins that Participants
// mirrors Order EXACTLY, including a fact Order already held before this
// feature: the composition splices a downed member out of the turn order
// the moment it notices them (encounter.noticeDown, rpg-toolkit#1078 — "a
// body keeps no turn, and the order closes over the gap rather than
// holding it open"). Participant is a projection of Order, not a second
// opinion about who belongs in it, so it does not re-add what the
// composition removed.
//
// TWO SKELETONS, deliberately: dropping the last standing member of a side
// dissolves the bubble entirely (TestTheLastOneDownedEndsTheFightByDefeat),
// which would leave nothing for THIS test to read Participants off of. A
// second skeleton keeps the fight alive with the first one down inside it.
func (s *DeathTestSuite) TestADownedMemberIsSplicedOutOfParticipantsToo() {
	s.startCrypt()
	s.spawnSkeleton()

	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skeleton-2", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 2},
	})
	s.Require().NoError(err)

	s.swingUntilTheSkeletonFalls()

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockTurn, turn.Clock, "skeleton-2 is still standing; the fight must still be running")
	s.NotContains(turn.Order, "skeleton", "the composition already spliced the body out (rpg-toolkit#1078)")

	for _, p := range turn.Participants {
		s.NotEqual("skeleton", p.Member, "Participants must not re-add what Order does not hold")
	}

	// The downed member is still a MEMBER, still answerable — just not a
	// PARTICIPANT in this fight's rotation any more.
	_, err = s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: "skeleton"})
	s.Require().NoError(err, "a downed member is still a member, on the map and answerable")
}

func (s *DeathTestSuite) TestTheReadsStayOpenToTheDowned() {
	s.duelAtZero()
	ctx := context.Background()

	_, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	s.NoError(err, "a downed member has a position")

	_, err = s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.NoError(err, "and holdings")

	_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.NoError(err, "and a story, which is how a client renders what just happened to it")

	_, err = s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.NoError(err, "and a clock")

	status, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open, "and the encounter is not over because somebody fell")
}

// TestTheAnswerIsAskedAgainNotRemembered is what makes this a PULL.
//
// Heal the stored sheet and the very next verb lets her walk. Nothing was
// invalidated, because nothing was remembered: the sheets are the truth and the
// question is asked again every time it matters. A cache here would be the
// smallest possible version of the dual state this whole design exists to
// avoid — a healed character the world still calls dead.
func (s *DeathTestSuite) TestTheAnswerIsAskedAgainNotRemembered() {
	s.duelAtZero()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().ErrorIs(err, session.ErrDowned)

	s.characters.byID["alice"].HitPoints = 7

	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
		DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err, "she is up, so she walks, and nothing had to be told")
}

// TestAMemberWithNoSheetIsUp is the case every existing fixture is.
//
// Authored content placed straight into a world has no sheet until something
// spawns it — the ambush's ogre is exactly that — and there is nothing to read
// hit points off. Reporting it DOWNED would kill every monster ever authored
// into a tomb; failing the verb would make every one of those worlds
// unplayable. So no sheet means UP, and this is the fight that has to keep
// starting.
func (s *DeathTestSuite) TestAMemberWithNoSheetIsUp() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)
	s.stream.published = nil

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: ambushPath()[:3],
	})
	s.Require().NoError(err, "a sheetless monster is not a broken world")
	s.Require().NotNil(out.Formed, "and it is an enemy, not a casualty")
	s.Empty(s.eventsOfKind(session.EventDowned), "nothing died")
}

// TestTheAnswerNamesOnlyWhoWasAsked is D1's carry-forward, and it is reachable
// rather than hypothetical.
//
// The session's own NPC list is not the encounter's roster: a spawned sheet
// stays in the session record after its member leaves, and a hand-seeded record
// can hold one that never was a member. A capability that answered out of its
// whole store would name a stranger, and the composition refuses a stranger in
// the answer rather than ignoring one (ErrNotMember) — so every verb in the
// session would start failing because of a sheet nobody is playing.
func (s *DeathTestSuite) TestTheAnswerNamesOnlyWhoWasAsked() {
	s.startCrypt()

	stored := s.sessions.byID["sess"]
	stored.NPCs = append(stored.NPCs, monster.Data{
		ID: "ghost", Name: "Ghost", HitPoints: 0, MaxHitPoints: 9, ArmorClass: 11,
	})

	s.bobSteps()

	_, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.Empty(s.eventsOfKind(session.EventDowned),
		"a sheet for somebody who is not a member is not news about anybody")
}

// TestACallerEndingAFightStillSaysDecision pins the other cause, and pins it as
// something DERIVED.
//
// A fight ends two ways now, and the answer a caller gets has to come from what
// the world did rather than from what the caller said. Echoing the input back
// would report a decision for a fight that ended in defeat the instant the two
// could disagree.
func (s *DeathTestSuite) TestACallerEndingAFightStillSaysDecision() {
	s.startCrypt()
	s.spawnSkeleton()

	out, err := s.mgr.Dissolve(context.Background(), &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDecision(),
	})
	s.Require().NoError(err)
	s.Equal(session.DissolveByDecision, out.Cause, "the party walked away, and it says so")
	s.ElementsMatch([]string{"alice", "skeleton"}, out.Members)
}

// TestACallerCannotDeclareDefeat is the derivation's other half, and the one
// that makes it a claim rather than a coincidence.
//
// The cause on the response comes from what the WORLD did. A caller handing in
// defeat is describing something it cannot know — being defeated is noticed,
// not chosen, and the composition refuses to take a cause from a caller at all
// for exactly that reason. This seam does take one, because its input crosses a
// wire and a proto field is not a sealed Go type; what it does not do is
// believe it. The fight ended because somebody called Dissolve, so it ended by
// decision, and that is what comes back.
//
// The call is not FAILED over it. Nothing was harmed: the verb did what it
// says, and the answer corrects the account. A refusal here would be a fourth
// sentinel earning nothing.
func (s *DeathTestSuite) TestACallerCannotDeclareDefeat() {
	s.startCrypt()
	s.spawnSkeleton()

	out, err := s.mgr.Dissolve(context.Background(), &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDefeat(),
	})
	s.Require().NoError(err)
	s.Equal(session.DissolveByDecision, out.Cause,
		"the verb IS the decision, whatever the caller wrote on the form")
}

// ctxKey is the marker markedCharacters looks for.
type ctxKey struct{}

// markedCharacters refuses any read that did not arrive on the caller's own
// context.
type markedCharacters struct {
	*fakeCharacters
}

var errWrongContext = errors.New("the character store was read on a context the caller never handed over")

func (m *markedCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if ctx.Value(ctxKey{}) == nil {
		return nil, errWrongContext
	}

	return m.fakeCharacters.GetCharacter(ctx, id)
}

// TestTheSeamReadsOnTheCallersContext pins something no assertion about hit
// points can see.
//
// The capability's own method takes no context — the composition calling it has
// none to give — so the verb's context is captured when the seam is built. That
// is the one detail of this design that could rot invisibly: swapping the
// capture for context.Background() compiles, passes every other test in this
// file, and silently drops the caller's deadline and cancellation into a
// repository read that runs once per member per sight refresh. On a walk, that
// is a request the host can no longer stop.
func (s *DeathTestSuite) TestTheSeamReadsOnTheCallersContext() {
	chars := &markedCharacters{fakeCharacters: newFakeCharacters(armedFighter("alice"), armedFighter("bob"))}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: s.stream,
	})
	s.Require().NoError(err)

	marked := context.WithValue(context.Background(), ctxKey{}, true)
	_, err = mgr.StartSession(marked, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = mgr.Move(marked, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().NoError(err, "the standing question is asked on the context the verb was called with")

	_, err = mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 1}},
	})
	s.Require().ErrorIs(err, errWrongContext,
		"and the control: a different context really is refused, so the row above means something")
}

// failingCharacters is a repository that is up for some sheets and broken for
// one, which is what a partly-unavailable store looks like.
type failingCharacters struct {
	*fakeCharacters
	broken string
}

var errCharacterStoreDown = errors.New("the character store is down")

func (f *failingCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if id == f.broken {
		return nil, errCharacterStoreDown
	}

	return f.fakeCharacters.GetCharacter(ctx, id)
}

// TestAStoreThatCannotAnswerFailsTheVerb is the error arm, and it matters
// because the alternative is worse than an error.
//
// A store that is unreachable is not a store saying "no such sheet". Reading it
// as UP would let a fight run on against a sheet nobody can read; reading it as
// DOWNED would kill a character because a database blinked. The capability's
// contract is that an error aborts the verb, atomically, and this is that
// contract reaching a host.
//
// (The word for a broken server and the word for a member at zero hit points
// were the same one until rpg-toolkit#1084, which is a small illustration of
// why the ruling was worth making.)
func (s *DeathTestSuite) TestAStoreThatCannotAnswerFailsTheVerb() {
	chars := &failingCharacters{
		fakeCharacters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
		broken:         "bob",
	}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: chars, Events: s.stream,
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().Error(err, "a world that cannot find out who is standing does not half-act on a guess")
	s.ErrorIs(err, errCharacterStoreDown, "and the host's own failure is not flattened into ours")
}
