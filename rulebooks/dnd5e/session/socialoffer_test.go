// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// socialoffer_test.go is the ruling of rpg-project#494 in one room: the social
// offer comes from the NPC.
//
// TWO GOBLINS STANDING SIDE BY SIDE, one the World Builder wrote `intimidate:`
// and `persuade:` on and one it did not. Everything else about them is the
// same — same stat block, same faction, same sightline to the player — so the
// only thing that can explain the difference in what the panel offers and what
// the verbs accept is the author's hand. That is the whole claim.
//
// IT IS ASSERTED ON BOTH CLOCKS, because the clock changes what a row COSTS
// and must never change who it is offered on: a creature that cannot be
// threatened in a fight cannot be threatened in the doorway either.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SocialOfferSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestSocialOfferSuite(t *testing.T) {
	suite.Run(t, new(SocialOfferSuite))
}

func (s *SocialOfferSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(talkingFighter("alice"))
}

// aRoom opens the two-goblin hall. `entries` says what each goblin's binding
// authored, keyed by member ID; a goblin missing from it is spawned with
// nothing, which is the creature the ruling is about.
func (s *SocialOfferSuite) aRoom(
	entries map[string]session.SpawnInput, props ...encounter.PropInput,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: []int{10, 10, 10, 10}},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Props:    props,
			Factions: []encounter.FactionInput{{ID: "goblins"}},
			// NEUTRAL, so no fight forms on sight and the default clock is
			// the world's — the front room this slice exists for.
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	for i, id := range []string{"written", "unwritten"} {
		spawn := entries[id]
		spawn.Session, spawn.ID = "sess", id
		spawn.Ref = refs.Monsters.Goblin().String()
		spawn.Position = spatial.Position{X: 5, Y: float64(1 + i*2)}
		spawn.Faction = "goblins"
		_, err = mgr.Spawn(ctx, &spawn)
		s.Require().NoError(err)
	}

	return mgr
}

// written is the goblin an author gave both social verbs to; unwritten is its
// twin, spawned with nothing.
func (s *SocialOfferSuite) bothVerbsOnTheWrittenGoblin() map[string]session.SpawnInput {
	return map[string]session.SpawnInput{"written": {
		Intimidate: []session.DoorApproach{{Ability: "intimidation", DC: 9}},
		Persuade:   []session.DoorApproach{{Ability: "persuasion", DC: 9}},
	}}
}

// intoAFight rolls the room into an authored turn order, so the same room can
// be asked the same questions on the other clock.
func (s *SocialOfferSuite) intoAFight() {
	ctx := context.Background()
	stored, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Require().NoError(s.encounters.SaveEncounter(ctx, "world",
		turnWorld(stored, []string{"alice", "written", "unwritten"}, 0)))
}

// candidatesFor is who a verb's row offers alice, by member ID and in the
// order the row lists them.
func (s *SocialOfferSuite) candidatesFor(mgr *session.Manager, verb session.Verb) []string {
	row := s.rowFor(mgr, verb)
	out := make([]string, 0, len(row.Candidates))
	for _, candidate := range row.Candidates {
		out = append(out, candidate.Member)
	}

	return out
}

func (s *SocialOfferSuite) rowFor(mgr *session.Manager, verb session.Verb) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, decl := range out.Declarations {
		if decl.Verb == verb {
			return decl
		}
	}
	s.Require().Fail("no row", string(verb))

	return session.Declaration{}
}

func (s *SocialOfferSuite) clockOf(mgr *session.Manager) session.ClockKind {
	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	return turn.Clock
}

// THE HEADLINE. Both goblins are standing in the room and both can see alice;
// only the one an author wrote the verb on is offered.
func (s *SocialOfferSuite) TestOnlyTheAuthoredGoblinIsACandidateInFreeRoam() {
	mgr := s.aRoom(s.bothVerbsOnTheWrittenGoblin())
	s.Require().Equal(session.ClockWorld, s.clockOf(mgr), "precondition: nothing here is fighting")

	for _, verb := range []session.Verb{session.VerbIntimidate, session.VerbPersuade} {
		s.Equal([]string{"written"}, s.candidatesFor(mgr, verb),
			"%s: the unwritten goblin is not offered at all", verb)
		s.True(s.rowFor(mgr, verb).Available, "%s: there is somebody to talk to", verb)
	}
}

// The same room, the same question, the other clock. A fight changes what the
// row costs and not who is in it.
func (s *SocialOfferSuite) TestOnlyTheAuthoredGoblinIsACandidateOnTheTurnClock() {
	mgr := s.aRoom(s.bothVerbsOnTheWrittenGoblin())
	s.intoAFight()
	s.Require().Equal(session.ClockTurn, s.clockOf(mgr), "precondition: alice is in a fight")

	for _, verb := range []session.Verb{session.VerbIntimidate, session.VerbPersuade} {
		s.Equal([]string{"written"}, s.candidatesFor(mgr, verb),
			"%s: the same audience law on both clocks", verb)
		s.Equal(session.SlotAction, s.rowFor(mgr, verb).Slot, "%s: and the fight's price", verb)
	}
}

// THE DOOR AGREES WITH THE OFFER. A stale client that aims at the goblin the
// panel never offered is refused by name, before any die is thrown.
func (s *SocialOfferSuite) TestTheVerbsRefuseTheUnauthoredGoblin() {
	mgr := s.aRoom(s.bothVerbsOnTheWrittenGoblin())
	ctx := context.Background()

	_, err := mgr.Intimidate(ctx, &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "unwritten",
	})
	s.Require().ErrorIs(err, session.ErrNoSocialEntry)

	_, err = mgr.Persuade(ctx, &session.PersuadeInput{
		Session: "sess", Member: "alice", Target: "unwritten",
	})
	s.Require().ErrorIs(err, session.ErrNoSocialEntry)
}

// And its twin answers both, so the refusal above is about the authoring and
// not about anything else in the room.
func (s *SocialOfferSuite) TestTheAuthoredGoblinAnswersBothVerbs() {
	mgr := s.aRoom(s.bothVerbsOnTheWrittenGoblin())
	ctx := context.Background()

	threat, err := mgr.Intimidate(ctx, &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "written",
	})
	s.Require().NoError(err)
	s.Equal(9, threat.DC, "the author's number")

	appeal, err := mgr.Persuade(ctx, &session.PersuadeInput{
		Session: "sess", Member: "alice", Target: "written",
	})
	s.Require().NoError(err)
	s.Equal(9, appeal.DC)
}

// THE TWO VERBS ARE AUTHORED INDEPENDENTLY. A creature written to be reasoned
// with and not to be threatened is a thing about the creature, not a gap — so
// one row offers it and the other does not, in the same room on the same
// sightline.
func (s *SocialOfferSuite) TestAVerbIsOfferedPerVerbAndNotPerCreature() {
	mgr := s.aRoom(map[string]session.SpawnInput{
		"written":   {Intimidate: []session.DoorApproach{{Ability: "intimidation", DC: 9}}},
		"unwritten": {Persuade: []session.DoorApproach{{Ability: "persuasion", DC: 9}}},
	})

	s.Equal([]string{"written"}, s.candidatesFor(mgr, session.VerbIntimidate),
		"only the one with `intimidate:`")
	s.Equal([]string{"unwritten"}, s.candidatesFor(mgr, session.VerbPersuade),
		"only the one with `persuade:`")

	_, err := mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "unwritten",
	})
	s.Require().ErrorIs(err, session.ErrNoSocialEntry,
		"being persuadable is not being intimidatable")
}

// A room full of creatures nobody wrote a social verb on says so in its own
// words, and says it as a DIFFERENT reason from having nobody to talk to: one
// means there is nothing here for this verb, the other means make yourself
// seen. A client that greyed the button the same way for both would tell a
// player to walk toward a goblin that was never written to be talked to.
func (s *SocialOfferSuite) TestARoomNobodyWroteTheVerbOnSaysSoByName() {
	mgr := s.aRoom(nil)

	for verb, text := range map[session.Verb]string{
		session.VerbIntimidate: "nobody here can be intimidated",
		session.VerbPersuade:   "nobody here can be persuaded",
	} {
		row := s.rowFor(mgr, verb)
		s.False(row.Available, "%s: nobody in the room carries it", verb)
		s.Require().NotNil(row.Why)
		s.Equal(session.ShortfallNoSocialEntry, row.Why.Reason, "%s", verb)
		s.Equal(text, row.Why.Text, "%s: the verb's own words", verb)
		s.Empty(row.Candidates, "%s: nobody is offered, not offered-unavailable", verb)
	}
}

// AND THE AUDIENCE COMES FIRST. With nobody able to see alice at all, the row
// reports the missing audience rather than the missing authoring — the second
// answers a question she has not reached yet.
func (s *SocialOfferSuite) TestNoAudienceOutRanksNoEntry() {
	// A wall down the middle: both goblins are still in the room and still
	// unwritten, so both reasons are true at once and only one is reported.
	mgr := s.aRoom(nil, occludingProps(
		spatial.Position{X: 3, Y: 0}, spatial.Position{X: 3, Y: 1},
		spatial.Position{X: 3, Y: 2}, spatial.Position{X: 3, Y: 3},
		spatial.Position{X: 3, Y: 4}, spatial.Position{X: 3, Y: 5},
		spatial.Position{X: 3, Y: 6}, spatial.Position{X: 3, Y: 7},
	)...)

	row := s.rowFor(mgr, session.VerbIntimidate)
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoTargetInReach, row.Why.Reason,
		"the audience is the first thing missing")
}
