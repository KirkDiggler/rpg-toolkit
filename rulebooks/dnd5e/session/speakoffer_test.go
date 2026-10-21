// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// speakoffer_test.go is the offer law in one room: the lines come from the
// target (rpg-project#494 R1, carried onto options by rpg-project#569).
//
// TWO GOBLINS STANDING SIDE BY SIDE, one the World Builder wrote lines on and
// one it did not. Everything else about them is the same — same stat block,
// same faction, same sightline to the player — so the only thing that can
// explain the difference in what the panel offers and what the verb accepts is
// the author's hand.
//
// IT IS ASSERTED ON BOTH CLOCKS, because the clock changes what a row COSTS
// and must never change who it is offered on.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SpeakOfferSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestSpeakOfferSuite(t *testing.T) {
	suite.Run(t, new(SpeakOfferSuite))
}

func (s *SpeakOfferSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(talkingFighter("alice"))
}

// aRoom opens the two-goblin hall. `lines` says what each goblin's binding
// authored, keyed by member ID; a goblin missing from it is spawned with
// nothing, which is the creature the ruling is about.
func (s *SpeakOfferSuite) aRoom(
	lines map[string][]encounter.Option, props ...encounter.PropInput,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: []int{10, 10, 10, 10}},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	sc := scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Props:    props,
			Factions: []encounter.FactionInput{{ID: "goblins"}},
			// NEUTRAL, so no fight forms on sight and the default clock is
			// the world's.
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
	}

	for i, id := range []string{"written", "unwritten"} {
		goblin := dungeonspec.MonsterPlacement{Options: lines[id]}
		goblin.ID, goblin.MemberID = id, id
		goblin.Ref = refs.Monsters.Goblin().String()
		goblin.At = authoredOf(spatial.Position{X: 5, Y: float64(1 + i*2)})
		goblin.Faction = "goblins"
		sc.Monsters = append(sc.Monsters, goblin)
	}
	launchScene(s.T(), mgr, sc)

	return mgr
}

// bothLinesOnTheWrittenGoblin is the goblin an author gave a threat and an
// appeal to; unwritten is its twin, spawned with nothing.
func bothLinesOnTheWrittenGoblin() map[string][]encounter.Option {
	return map[string][]encounter.Option{"written": {threatLine(), appealLine()}}
}

// intoAFight rolls the room into an authored turn order, so the same room can
// be asked the same questions on the other clock.
func (s *SpeakOfferSuite) intoAFight() {
	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "written", "unwritten"}, 0)
}

// candidates is who the Speak row offers alice, by member ID and in the order
// the row lists them.
func (s *SpeakOfferSuite) candidates(mgr *session.Manager) []string {
	row := s.row(mgr)
	out := make([]string, 0, len(row.Candidates))
	for _, candidate := range row.Candidates {
		out = append(out, candidate.Member)
	}

	return out
}

func (s *SpeakOfferSuite) row(mgr *session.Manager) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, decl := range out.Declarations {
		if decl.Verb == session.VerbSpeak {
			return decl
		}
	}
	s.Require().Fail("no Speak row")

	return session.Declaration{}
}

func (s *SpeakOfferSuite) clockOf(mgr *session.Manager) session.ClockKind {
	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	return turn.Clock
}

func (s *SpeakOfferSuite) say(mgr *session.Manager, target, option string) (*session.SpeakOutput, error) {
	return mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: target, OptionID: option,
	})
}

// THE HEADLINE. Both goblins are standing in the room and both can see alice;
// only the one an author wrote lines on is offered.
func (s *SpeakOfferSuite) TestOnlyTheAuthoredGoblinIsACandidateInFreeRoam() {
	mgr := s.aRoom(bothLinesOnTheWrittenGoblin())
	s.Require().Equal(session.ClockWorld, s.clockOf(mgr), "precondition: nothing here is fighting")

	s.Equal([]string{"written"}, s.candidates(mgr), "the unwritten goblin is not offered at all")
	s.Equal([][2]string{{"written", "threaten"}, {"written", "appeal"}}, menu(s.row(mgr)))
	s.True(s.row(mgr).Available, "there is somebody to talk to")
}

// The same room, the same question, the other clock. A fight changes what the
// row costs and not who is in it.
func (s *SpeakOfferSuite) TestOnlyTheAuthoredGoblinIsACandidateOnTheTurnClock() {
	mgr := s.aRoom(bothLinesOnTheWrittenGoblin())
	s.intoAFight()
	s.Require().Equal(session.ClockTurn, s.clockOf(mgr), "precondition: alice is in a fight")

	s.Equal([]string{"written"}, s.candidates(mgr), "the same audience law on both clocks")
	s.Equal([][2]string{{"written", "threaten"}, {"written", "appeal"}}, menu(s.row(mgr)))
	s.Equal(session.SlotAction, s.row(mgr).Slot, "and the fight's price")
}

// THE DOOR AGREES WITH THE OFFER. A stale client that aims at the goblin the
// panel never offered is refused by name, before any die is thrown.
func (s *SpeakOfferSuite) TestTheVerbRefusesTheUnauthoredGoblin() {
	mgr := s.aRoom(bothLinesOnTheWrittenGoblin())

	_, err := s.say(mgr, "unwritten", "threaten")
	s.Require().ErrorIs(err, session.ErrNoOption)
	_, err = s.say(mgr, "unwritten", "appeal")
	s.Require().ErrorIs(err, session.ErrNoOption)
}

// And its twin answers both lines, so the refusal above is about the authoring
// and not about anything else in the room.
func (s *SpeakOfferSuite) TestTheAuthoredGoblinAnswersBothLines() {
	mgr := s.aRoom(bothLinesOnTheWrittenGoblin())

	threat, err := s.say(mgr, "written", "threaten")
	s.Require().NoError(err)
	s.Equal(9, threat.DC, "the author's number")
	s.Equal("intimidation", threat.Applied.Ability)

	appeal, err := s.say(mgr, "written", "appeal")
	s.Require().NoError(err)
	s.Equal(9, appeal.DC)
	s.Equal("persuasion", appeal.Applied.Ability)
}

// LINES ARE AUTHORED PER TARGET. A creature written to be reasoned with and not
// to be threatened is a thing about the creature — so the menu carries each
// target's own lines, and a line id names a line only on the target that
// offered it.
func (s *SpeakOfferSuite) TestALineBelongsToTheTargetThatOffersIt() {
	mgr := s.aRoom(map[string][]encounter.Option{
		"written":   {threatLine()},
		"unwritten": {appealLine()},
	})

	s.Equal([]string{"unwritten", "written"}, s.candidates(mgr), "the witnesses, in the composition's sorted order")
	s.Equal([][2]string{{"unwritten", "appeal"}, {"written", "threaten"}}, menu(s.row(mgr)),
		"each target's own lines, and only those")

	_, err := s.say(mgr, "unwritten", "threaten")
	s.Require().ErrorIs(err, session.ErrNoOption, "being persuadable is not being intimidatable")
}

// A room full of creatures nobody wrote a line on says so in its own words,
// and as a DIFFERENT reason from having nobody to talk to: one means there is
// nothing here to say, the other means make yourself seen.
func (s *SpeakOfferSuite) TestARoomNobodyWroteALineOnSaysSoByName() {
	mgr := s.aRoom(nil)

	row := s.row(mgr)
	s.False(row.Available, "nobody in the room offers a line")
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoOption, row.Why.Reason)
	s.Equal("nothing here has a line for you", row.Why.Text)
	s.Empty(row.Candidates, "nobody is offered, not offered-unavailable")
	s.Empty(row.SpeakOptions)
}

// AND THE AUDIENCE COMES FIRST. With nobody able to see alice at all, the row
// reports the missing audience rather than the missing lines.
func (s *SpeakOfferSuite) TestNoAudienceOutRanksNoOption() {
	// A wall down the middle: both goblins are still in the room and still
	// unwritten, so both reasons are true at once and only one is reported.
	mgr := s.aRoom(nil, occludingProps(
		spatial.Position{X: 3, Y: 0}, spatial.Position{X: 3, Y: 1},
		spatial.Position{X: 3, Y: 2}, spatial.Position{X: 3, Y: 3},
		spatial.Position{X: 3, Y: 4}, spatial.Position{X: 3, Y: 5},
		spatial.Position{X: 3, Y: 6}, spatial.Position{X: 3, Y: 7},
	)...)

	row := s.row(mgr)
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoTargetInReach, row.Why.Reason, "the audience is the first thing missing")
}
