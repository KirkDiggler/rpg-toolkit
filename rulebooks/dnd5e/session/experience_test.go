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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ExperienceTestSuite is the first in-toolkit source of experience arriving at
// the seam (rpg-project#496).
//
// THE WHOLE SCENE IS ONE VERB, like the death suite beside it. A monster's
// fall is noticed by the composition inside the Record that reported the
// killing blow, and the party is paid in that same commit — nobody walks,
// nobody ends a turn, nobody asks. What the scenes assert is what a client
// would see and what the character store would hold afterwards.
//
// EVERYTHING RUNS ON REAL SHEETS AND REAL CATALOG MONSTERS. The worth being
// divided is the goblin's own authored 50, read back off the session record
// the way every verb that damages a monster reads it; the totals asserted are
// the totals the fake repository actually holds after the verb. A suite that
// handed the manager a number would be testing its own arithmetic.
type ExperienceTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestExperienceSuite(t *testing.T) { suite.Run(t, new(ExperienceTestSuite)) }

// xpRoom is one open room with the named players standing in it.
//
// OPEN, with no wall anywhere: most of this suite is about who is on the
// ROSTER when a monster falls, not about who could see it, and a fixture that
// hid somebody would invite the reader to think sight was the thing being
// tested. The two scenes that DO need a second player kept out of the fight
// use xpCrypt below and say why.
//
// Unbounded retention for the death suite's reason: the story is the ledger
// the settlement reads back to find this act's falls, and a window that
// trimmed mid-scene would hide a fall these tests are counting.
func xpRoom(t fataler, endings []encounter.EndingInput, players ...string) *encounter.EncounterData {
	members := make([]encounter.MemberInput, 0, len(players))
	for i, id := range players {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(id), Kind: encounter.KindPlayer,
			Position: spatial.Position{X: float64(1 + 2*i), Y: 1},
		})
	}

	return denWith(t, members, nil, endings)
}

// xpCrypt is alice's half of a room, a solid wall, and bob on the far side of
// it — the death suite's own crypt, borrowed for the two scenes that need a
// player who can take an ORDINARY STEP without joining the fight.
//
// The wall is the whole reason it exists. Those scenes put bodies on the floor
// by writing the stored sheets and then let somebody walk, so the world looks;
// if the walker were in contact with the monsters, the step would be a turn in
// a fight and the scene would be about initiative instead.
func xpCrypt(t fataler) *encounter.EncounterData { return xpCryptWith(t) }

// xpCryptWith is the crypt plus any extra players, parked on bob's side of the
// wall where the fight cannot reach them.
func xpCryptWith(t fataler, extra ...string) *encounter.EncounterData {
	occluders := make([]spatial.Position, 0, 10)
	for y := 0; y < 10; y++ {
		occluders = append(occluders, spatial.Position{X: 5, Y: float64(y)})
	}

	members := []encounter.MemberInput{
		{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 8, Y: 8}},
	}
	for i, id := range extra {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(id), Kind: encounter.KindPlayer,
			Position: spatial.Position{X: float64(7 - i), Y: 8},
		})
	}

	return denWith(t, members, occludingProps(occluders...), withdrawable())
}

// denWith builds the one world shape both fixtures above are variations of.
func denWith(
	t fataler, members []encounter.MemberInput, props []encounter.PropInput, endings []encounter.EndingInput,
) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("den", 0, 0, 10, 10)},
			Props:   props,
		},
		Members:   members,
		Endings:   endings,
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the den: %v", err)
	}
	data := enc.ToData()

	return &data
}

// startCrypt opens a session on the walled den.
func (s *ExperienceTestSuite) startCrypt() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: xpCrypt(s.T()),
	})
	s.Require().NoError(err)
	s.stream.published = nil
}

// bobSteps is the plainest thing a player on the far side of the wall does,
// and it is what makes the world look: every sight refresh consults the
// rulebook about who is standing.
func (s *ExperienceTestSuite) bobSteps() {
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{hexCell(8, 7)},
	})
	s.Require().NoError(err)
}

// dropPlayerTo writes a stored character's hit points without anybody swinging.
func (s *ExperienceTestSuite) dropPlayerTo(id string, hp int) {
	data, ok := s.characters.byID[id]
	s.Require().True(ok, "no stored sheet for "+id)
	data.HitPoints = hp
	s.characters.byID[id] = data
}

// withdrawable is the one ending every scene but the boss-down one wants: a
// way out that nothing in the scene ever takes.
func withdrawable() []encounter.EndingInput {
	return []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}
}

func (s *ExperienceTestSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"), armedFighter("carol"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// startDen opens a session on a den holding the named players.
func (s *ExperienceTestSuite) startDen(endings []encounter.EndingInput, players ...string) {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: xpRoom(s.T(), endings, players...),
	})
	s.Require().NoError(err)
	s.stream.published = nil
}

// spawnGoblin puts a catalog goblin on a cell, and says what it is worth.
//
// The worth assertion is the fixture declaring its own premise: every number
// the scenes below divide comes from this one, so a catalog change that moved
// it would fail here, saying so, instead of moving every expected total
// silently.
func (s *ExperienceTestSuite) spawnGoblin(id string, at spatial.Position) {
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: id, Ref: refs.Monsters.Goblin().String(), Position: at,
	})
	s.Require().NoError(err)
	s.Require().Equal(7, out.NPC.HitPoints, "the catalog goblin, whole")
	s.Require().Equal(50, s.storedWorth(id), "and worth 50, which is every share below")
}

// storedWorth reads a spawned NPC's authored experience back out of the
// session record — the same field the settlement reads.
func (s *ExperienceTestSuite) storedWorth(id string) int {
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == id {
			return npc.Experience
		}
	}
	s.Require().Fail("no stored sheet for " + id)

	return -1
}

// setStoredWorth rewrites what the session record says a monster is worth.
//
// The store hands out a copy on every load, so this is the honest way to put a
// worthless monster in front of the settlement: the sheet the session plays
// with says zero, which is exactly what an unvalued monster's sheet says.
func (s *ExperienceTestSuite) setStoredWorth(id string, worth int) {
	data := s.sessions.byID["sess"]
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			data.NPCs[i].Experience = worth

			return
		}
	}
	s.Require().Fail("no stored sheet for " + id)
}

// dropTo puts a member's stored hit points at a value without anybody swinging.
func (s *ExperienceTestSuite) dropTo(id string, hp int) {
	data := s.sessions.byID["sess"]
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			data.NPCs[i].HitPoints = hp

			return
		}
	}
	s.Require().Fail("no stored sheet for " + id)
}

// swing is one strike, which is all it takes: the fixture's longsword deals 11
// and the catalog goblin has 7.
func (s *ExperienceTestSuite) swing(attacker, target string) {
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: attacker, Target: target,
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", attacker),
	})
	s.Require().NoError(err)
}

// storedExperience is what the character repository actually holds.
func (s *ExperienceTestSuite) storedExperience(id string) int {
	data, ok := s.characters.byID[id]
	s.Require().True(ok, "no stored sheet for "+id)

	return data.Experience
}

// grantEvents is every experience beat published since the last reset.
func (s *ExperienceTestSuite) grantEvents() []session.Event {
	var out []session.Event
	for _, event := range s.stream.published {
		if event.Kind == session.EventExperienceGained {
			out = append(out, event)
		}
	}

	return out
}

// grantBodies is one body per DISTINCT beat, keyed by the member that fell.
//
// The same beat reaches every recipient, so the raw event list holds one copy
// per player; what the scenes want to count is beats, not deliveries. A second
// beat naming the same fallen monster would collide here and the assertion on
// its grants would fail — which is the settle-once claim, checked rather than
// assumed.
func (s *ExperienceTestSuite) grantBodies() map[string]session.ExperienceGainedBody {
	out := map[string]session.ExperienceGainedBody{}
	for _, event := range s.grantEvents() {
		body, ok := event.Body.(session.ExperienceGainedBody)
		s.Require().True(ok, "an experience beat carries a typed body")
		if seen, dup := out[body.Member]; dup {
			s.Require().Equal(seen, body, "two beats for one fall must at least agree")
		}
		out[body.Member] = body
	}

	return out
}

// TestOnePlayerTakesTheWholeWorth is the slice's own sentence.
//
// A party of one is not a special case anywhere in the rule — it is the equal
// division with nobody to divide among — which is why it is the opening scene:
// if the share arithmetic were wrong in either direction, this is where it
// shows as a number a reader can check against the stat block.
func (s *ExperienceTestSuite) TestOnePlayerTakesTheWholeWorth() {
	s.startDen(withdrawable(), "alice")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})

	s.swing("alice", "goblin")

	s.Equal(50, s.storedExperience("alice"), "the whole worth, on the sheet the store holds")

	bodies := s.grantBodies()
	s.Require().Len(bodies, 1, "one fall, one beat")
	s.Require().Len(bodies["goblin"].Grants, 1, "and one grant on it")
	s.Equal(session.ExperienceGrant{Character: "alice", Amount: 50, Total: 50}, bodies["goblin"].Grants[0])
}

// TestTheWholePartyHearsItsShare is R2's division and R5's audience in one
// scene.
//
// THREE PLAYERS, AND 50/3 IS 16. The odd two points are dropped rather than
// handed to somebody, which is the ruling being arithmetic rather than
// generous. Carol never moves and never sees the goblin; she is paid the same
// as alice, who killed it, because taking part is being on the roster.
func (s *ExperienceTestSuite) TestTheWholePartyHearsItsShare() {
	s.startDen(withdrawable(), "alice", "bob", "carol")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})

	s.swing("alice", "goblin")

	for _, id := range []string{"alice", "bob", "carol"} {
		s.Equal(16, s.storedExperience(id), id+" was paid an equal share")
	}

	bodies := s.grantBodies()
	s.Require().Len(bodies, 1, "one fall is still one beat, however many it pays")
	s.Equal([]session.ExperienceGrant{
		{Character: "alice", Amount: 16, Total: 16},
		{Character: "bob", Amount: 16, Total: 16},
		{Character: "carol", Amount: 16, Total: 16},
	}, bodies["goblin"].Grants, "every payee on the one beat, sorted by character")
}

// TestEveryPlayerIsToldTheWholeGrant pins the delivery half of R5.
//
// The beat is whole-party and so is its audience: bob reads the same grants
// alice does, including alice's own line, so a client can narrate what the
// party gained rather than only what its own character did.
func (s *ExperienceTestSuite) TestEveryPlayerIsToldTheWholeGrant() {
	s.startDen(withdrawable(), "alice", "bob")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})

	s.swing("alice", "goblin")

	heard := map[string]bool{}
	for _, event := range s.grantEvents() {
		heard[event.Recipient] = true
		s.Equal(session.EventExperienceGained, event.Kind)
		body, ok := event.Body.(session.ExperienceGainedBody)
		s.Require().True(ok)
		s.Equal("goblin", body.Member, "and it names what fell")
		s.Len(body.Grants, 2, "the whole grant reaches each of them, not their own line alone")
	}
	s.True(heard["alice"] && heard["bob"], "both players hear it, got %v", heard)
}

// TestAMonsterWorthNothingPaysNothing is R1's zero, which is a rule rather
// than a gap.
//
// No grant, no beat, and no error: an encounter whose fallen monster was worth
// nothing simply has no experience in it. The assertion that matters is the
// absence of a beat — a zero-amount grant would have been refused by the
// composition, so a settlement that tried would have failed the verb.
func (s *ExperienceTestSuite) TestAMonsterWorthNothingPaysNothing() {
	s.startDen(withdrawable(), "alice")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.setStoredWorth("goblin", 0)

	s.swing("alice", "goblin")

	s.Zero(s.storedExperience("alice"), "nobody valued it, so nobody was paid")
	s.Empty(s.grantEvents(), "and nothing was said about it")
}

// TestAShareThatFloorsToZeroIsNotABeat is the same silence reached from the
// other side: the monster is worth something and the party is too big for it
// to divide.
//
// character.AddExperience refuses a grant of nothing by design, so the choice
// here is not whether to pay zero but whether to say so — and there is nothing
// to say.
func (s *ExperienceTestSuite) TestAShareThatFloorsToZeroIsNotABeat() {
	s.startDen(withdrawable(), "alice", "bob", "carol")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.setStoredWorth("goblin", 2)

	s.swing("alice", "goblin")

	for _, id := range []string{"alice", "bob", "carol"} {
		s.Zero(s.storedExperience(id), id+" got no share out of two points between three")
	}
	s.Empty(s.grantEvents())
}

// TestAFallenPlayerPaysNobodyAndIsStillPaid is both halves of R2 in one act.
//
// Alice and the goblin are on the floor together when bob takes his step, so
// the act carries two down beats and the settlement has to tell them apart.
// What decides is the ENCOUNTER's roster kind, never whether an ID loads out
// of a store:
//
//   - Alice falling pays nobody. There is no beat naming her, and bob is not
//     credited for his own party member.
//   - Alice is still PAID for the goblin, dying on the floor. RAW pays
//     everyone who took part and a dying character took part, so the share is
//     halved between the two of them exactly as if she were standing.
func (s *ExperienceTestSuite) TestAFallenPlayerPaysNobodyAndIsStillPaid() {
	s.startCrypt()
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.dropTo("goblin", 0)
	s.dropPlayerTo("alice", 0)

	s.bobSteps()

	downed := map[string]bool{}
	for _, event := range s.stream.published {
		if body, ok := event.Body.(session.DownedBody); ok {
			downed[body.Member] = true
		}
	}
	s.Require().True(downed["alice"] && downed["goblin"], "both fell in this act, got %v", downed)

	bodies := s.grantBodies()
	s.Require().Len(bodies, 1, "one of the two falls is a payday")
	s.Require().Contains(bodies, "goblin", "and it is the monster's")
	s.Equal([]session.ExperienceGrant{
		{Character: "alice", Amount: 25, Total: 25},
		{Character: "bob", Amount: 25, Total: 25},
	}, bodies["goblin"].Grants, "the dying character takes her share")
	s.Equal(25, s.storedExperience("alice"))
	s.Equal(25, s.storedExperience("bob"))
}

// TestTwoFallsInOneActAreTwoBeats is the per-cause law (R5).
//
// Both goblins are at zero when the world next looks, so one commit settles
// two falls — and each one gets its own beat with its own member, rather than
// the act's grants being folded into a single total nobody could attribute.
//
// They are dropped by writing the stored sheets rather than by swinging twice,
// because a swing is one target and this scene needs the two falls INSIDE one
// act. What the settlement reads is the act's down beats, and the composition
// writes one per body it notices on the next consult, whatever put them there.
func (s *ExperienceTestSuite) TestTwoFallsInOneActAreTwoBeats() {
	s.startCrypt()
	s.spawnGoblin("first", spatial.Position{X: 2, Y: 1})
	s.spawnGoblin("second", spatial.Position{X: 3, Y: 1})
	s.dropTo("first", 0)
	s.dropTo("second", 0)

	s.bobSteps()

	s.Equal(50, s.storedExperience("alice"), "two goblins at 25 a head")
	s.Equal(50, s.storedExperience("bob"))

	bodies := s.grantBodies()
	s.Require().Len(bodies, 2, "one beat per fall, never one per act")
	s.Equal([]session.ExperienceGrant{
		{Character: "alice", Amount: 25, Total: 25},
		{Character: "bob", Amount: 25, Total: 25},
	}, bodies["first"].Grants)
	s.Equal([]session.ExperienceGrant{
		{Character: "alice", Amount: 25, Total: 50},
		{Character: "bob", Amount: 25, Total: 50},
	}, bodies["second"].Grants,
		"and the second beat carries the running total, not a second first payment")
}

// TestTheFallSettlesExactlyOnce is the idempotence the baseline buys.
//
// The act's delta is bounded by the scope's baseline and the composition
// appends a down beat once per fall, so the verb that comes after the kill
// sees nothing to settle. A settlement re-reading the whole story instead
// would pay the party again on every step anybody took for the rest of the
// run.
func (s *ExperienceTestSuite) TestTheFallSettlesExactlyOnce() {
	s.startDen(withdrawable(), "alice")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})

	s.swing("alice", "goblin")
	s.Require().Equal(50, s.storedExperience("alice"))
	s.Require().Len(s.grantBodies(), 1)
	s.stream.published = nil

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(1, 2)},
	})
	s.Require().NoError(err)

	s.Equal(50, s.storedExperience("alice"), "one body, paid for once")
	s.Empty(s.grantEvents(), "and announced once")
}

// TestTheFallThatEndsTheRunStillPays is the door the encounter left open.
//
// The goblin's fall fires a member-down ending INSIDE the Record that reported
// the killing blow, so the encounter is already closed by the time the party's
// share is settled. Every other outcome kind is refused on a closed encounter;
// this one is not, because refusing would mean the one death that mattered
// most is the only death nobody was paid for.
func (s *ExperienceTestSuite) TestTheFallThatEndsTheRunStillPays() {
	s.startDen([]encounter.EndingInput{
		{Key: "boss-down", Trigger: encounter.TriggerMemberDown{Member: "goblin"}},
		{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
	}, "alice", "bob")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})

	s.swing("alice", "goblin")

	ended := false
	for _, event := range s.stream.published {
		if event.Kind == session.EventEnded {
			ended = true
		}
	}
	s.Require().True(ended, "the boss going down ends the run")

	s.Equal(25, s.storedExperience("alice"), "and the sheets were still written")
	s.Equal(25, s.storedExperience("bob"))
	bodies := s.grantBodies()
	s.Require().Len(bodies, 1, "and the beat was still recorded on the closed encounter")
	s.Len(bodies["goblin"].Grants, 2)
}

// TestCrossingAThresholdOpensALevel is the ruling's own done-when, read back
// through the verb a host actually calls.
//
// The seeded total is two hundred and fifty: one short of nothing in
// particular, and fifty short of the 2014 table's 300 for level 2. One goblin
// closes it. What flips is ENTITLEMENT, not the level — Advance still refuses
// in combat and the character still levels between runs through the verbs that
// already exist — which is why this asserts the gap rather than a new level.
func (s *ExperienceTestSuite) TestCrossingAThresholdOpensALevel() {
	seeded := armedFighter("alice")
	setLevel(seeded, 1)
	seeded.Experience = 250
	s.characters = newFakeCharacters(seeded)

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	before, err := s.mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(1, before.EntitledLevel, "250 has earned nothing past level 1")
	s.Require().Equal(character.ExperienceThresholdForLevel(2), before.NextLevelThreshold)

	s.startDen(withdrawable(), "alice")
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.swing("alice", "goblin")

	s.Require().Equal(300, s.storedExperience("alice"))

	after, err := s.mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "alice"})
	s.Require().NoError(err)
	s.Equal(300, after.Experience, "the verb reads the total the fall wrote")
	s.Equal(2, after.EntitledLevel, "which has earned level 2")
	s.Equal(1, after.Level, "and the sheet still holds level 1: the gap IS the offer")
}

// errSettlementSave is a character store refusing one exact write during the
// settlement, so a scene can ask what a half-paid party leaves behind.
var errSettlementSave = errors.New("the settlement's character save was refused")

// TestAMonsterWithNoStoredSheetNeverFalls is the upstream half of the
// settlement's missing-sheet guard, and the reason that guard has no reachable
// scene of its own.
//
// settleOneFall refuses a fallen monster whose sheet is not in the session
// record (ErrInvalidSession). Nothing in this module can produce that state:
// standing.go answers "no sheet, no death" — a KindMonster with no entry in
// SessionData.NPCs is in neither provider list and takes the Conscious
// compatibility fact — so a sheetless monster never goes down at all, and
// SessionData.NPCs is only ever appended to (Spawn) or rewritten in place
// (saveDirty), never shortened. The guard is therefore fail-closed defence
// against a hand-edited record, not a live path.
//
// What IS worth pinning is the behavior above it, because it is what makes the
// guard unreachable: strip the sheet and the monster stops being able to fall,
// so the party is never paid and nothing is recorded. A change that let a
// sheetless monster fall would land in the settlement's refusal instead of
// here, and this scene is what would notice.
func (s *ExperienceTestSuite) TestAMonsterWithNoStoredSheetNeverFalls() {
	s.startCrypt()
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.dropTo("goblin", 0)

	data := s.sessions.byID["sess"]
	kept := make([]monster.Data, 0, len(data.NPCs))
	for _, npc := range data.NPCs {
		if npc.ID != "goblin" {
			kept = append(kept, npc)
		}
	}
	data.NPCs = kept

	s.bobSteps()

	for _, event := range s.stream.published {
		s.NotEqual(session.EventDowned, event.Kind, "a sheetless monster cannot be seen to fall")
	}
	s.Empty(s.grantEvents(), "so nothing is paid and nothing is recorded")
	s.Zero(s.storedExperience("alice"))
	s.Zero(s.storedExperience("bob"))
}

// TestAPayeeTheStoreDoesNotHoldFailsTheVerb pins the second of the three
// returns the settlement's safety argument runs through.
//
// Carol is a KindPlayer on the encounter's roster and the character store does
// not hold her. That is a real inconsistency, not an ordinary absence, and the
// settlement returns rather than skipping her: a party quietly paid short is
// exactly the silent wrong this fails closed against.
//
// SHE IS NEITHER IN THE FIGHT NOR WALKING, and that is the fixture's whole
// point. A missing player who is in the dissolving fight is caught one
// settlement earlier, by exitDissolvedCombatants, so a scene built on her
// would pass while saying nothing about this code. Carol stands behind the
// wall doing nothing, which leaves this settlement the first to look for her.
//
// What the verb leaves behind is the ordering law holding: the world is
// untouched, no beat was recorded, and the two sheets paid BEFORE the failure
// are named in the report — which is the difference between a caller who
// repairs and one who retries.
func (s *ExperienceTestSuite) TestAPayeeTheStoreDoesNotHoldFailsTheVerb() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: xpCryptWith(s.T(), "carol"),
	})
	s.Require().NoError(err)
	s.stream.published = nil

	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.dropTo("goblin", 0)
	delete(s.characters.byID, "carol")

	before, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)

	// The verb's own error keeps its own name. Reusing err for the reads below
	// would quietly replace it with a nil, and every assertion after that point
	// would be asking questions of the wrong error.
	_, verbErr := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{hexCell(8, 7)},
	})

	s.Require().Error(verbErr)
	s.ErrorIs(verbErr, session.ErrNoCharacter, "a roster player the store does not hold is an inconsistency")

	after, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	s.Equal(before, after, "the verb failed before persist, so the world never moved")
	s.Empty(s.grantEvents(), "and no grant was announced")

	var reported *session.SaveError
	s.Require().True(errors.As(verbErr, &reported), "the failure carries the report")
	// TWO NAMES, AND THE COUNT IS THE POINT. The settlement pays each player
	// and saves that sheet before moving to the next, so alice and bob are
	// already durable when carol is found missing. A settlement restructured
	// to load everyone first, or to record its beat before writing, leaves
	// this list EMPTY — which is how this assertion notices an ordering change
	// that nothing else in the suite can see.
	s.Equal([]string{"character:alice", "character:bob"}, reported.Report.Written,
		"the sheets paid before the failure are named, so the caller repairs rather than retries")
}

// TestASheetThatWillNotSaveFailsTheVerbBeforeTheBeat pins the third return.
//
// Alice's share lands and bob's save is refused. The verb fails, the world is
// untouched, and no grant reaches a client.
//
// WHAT THIS SCENE DOES NOT PIN, said plainly because the neighbouring comment
// could be read as claiming it: it does not by itself catch a settlement that
// recorded the beat BEFORE saving the sheets. Delivery waits for persist, so a
// verb that fails anywhere publishes nothing either way, and the stored world
// is unchanged whichever order the two ran in. The assertion sensitive to that
// restructure is the Written list in
// [ExperienceTestSuite.TestAPayeeTheStoreDoesNotHoldFailsTheVerb]: pay-then-
// next-payee leaves two sheets named, load-everyone-first leaves none. Checked
// by mutation rather than assumed.
//
// ALICE'S SHEET IS DURABLE AT 25 AND THAT IS ADMITTED, NOT HIDDEN. It is the
// wedge settleExperience's own doc names and the one Manager.persist already
// admits for a swing's damage (rpg-toolkit#1056): sheets are durable before
// persist runs, so a retry of this verb would pay her twice. The report naming
// her is what lets a caller tell that apart from nothing having happened, and
// asserting it here is the difference between a known cost and a surprise.
func (s *ExperienceTestSuite) TestASheetThatWillNotSaveFailsTheVerbBeforeTheBeat() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.stream = &fakeStream{}
	failing := &failNthArmedSaveCharacters{
		fakeCharacters: s.characters, failAt: 3, err: errSettlementSave,
	}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: failing, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	s.startCrypt()
	s.spawnGoblin("goblin", spatial.Position{X: 2, Y: 1})
	s.dropTo("goblin", 0)

	before, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	failing.armed = true

	// Its own name, for the reason the scene above states.
	_, verbErr := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{hexCell(8, 7)},
	})

	s.Require().Error(verbErr)
	s.ErrorIs(verbErr, session.ErrSaveFailed, "this seam's vocabulary")
	s.ErrorIs(verbErr, errSettlementSave, "and the host's own cause stays matchable")
	s.Equal(3, failing.attempts, "combat-end cleanup and alice's share landed before bob's was refused")

	after, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	s.Equal(before, after, "the verb failed before persist, so the world never moved")
	s.Empty(s.grantEvents(), "and no grant reached a client")

	var reported *session.SaveError
	s.Require().True(errors.As(verbErr, &reported), "the failure carries the report")
	s.Equal([]string{"character:alice"}, reported.Report.Written,
		"the share written before the failure is named — settleExperience's own admitted wedge")
	s.Equal([]string{"character:bob"}, reported.Report.Failed, "and the one that did not land is too")
	s.Equal(25, s.characters.byID["alice"].Experience, "alice's grant really is durable; a retry pays her twice")
	s.Zero(s.characters.byID["bob"].Experience)
}
