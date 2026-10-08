// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

// LaunchSuite covers slice 4's launch done-when: one load-act-save that
// refuses before anything is written, stands the whole board, seats and
// rests the party, forms the fight last, and returns one report.
type LaunchSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	seats      *fakeSeats
	stream     *fakeStream
	mgr        *session.Manager
}

func TestLaunchSuite(t *testing.T) { suite.Run(t, new(LaunchSuite)) }

func (s *LaunchSuite) SetupTest() {
	tired := func(id string) *character.Data {
		data := withHitDice(armedFighter(id), 3)
		data.Resources[resources.HitDice] = character.RecoverableResourceData{Current: 0, Maximum: 3, ResetType: coreResources.ResetLongRest}
		data.HitPoints = 5
		return data
	}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(tired("alice"), tired("bob"), tired("carol"))
	s.seats = newFakeSeats()
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: s.seats, PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *LaunchSuite) camp() *dungeonspec.Compiled {
	compiled := compileCamp(s.T(), campSource(s.T()))
	return &compiled
}

func (s *LaunchSuite) launch(dungeon *dungeonspec.Compiled, party ...string) (*session.LaunchOutput, error) {
	return s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", DungeonKey: "reference-raider-camp", Dungeon: dungeon, Party: party,
	})
}

func (s *LaunchSuite) assertNothingWritten() {
	s.T().Helper()
	s.Zero(s.characters.saves, "no character is written")
	s.Zero(s.encounters.saves, "no world is written")
	s.Zero(s.seats.saves, "no seat is written")
	_, err := s.sessions.GetSession(context.Background(), "run")
	s.ErrorIs(err, session.ErrNotFound, "no session is written")
	s.Empty(s.stream.published)
}

func (s *LaunchSuite) TestALaunchSeatsAndRestsEveryPartyMemberAndReportsOnce() {
	out, err := s.launch(s.camp(), "alice", "bob")
	s.Require().NoError(err)

	for _, id := range []string{"alice", "bob"} {
		stored, getErr := s.characters.GetCharacter(context.Background(), id)
		s.Require().NoError(getErr)
		s.Equal(stored.MaxHitPoints, stored.HitPoints, "%s is rested", id)
		s.Equal("run", s.seats.seatOf(id), "%s is seated", id)
		s.Contains(out.Saved.Written, "character:"+id)
		s.Contains(out.Saved.Written, "seat:"+id)
	}
	s.Contains(out.Saved.Written, "encounter:run")
	s.Contains(out.Saved.Written, "session:run")
	s.Less(positionOf(out.Saved.Written, "seat:bob"), positionOf(out.Saved.Written, "encounter:run"),
		"the party is rested and seated before the run that holds them")

	data, err := s.sessions.GetSession(context.Background(), "run")
	s.Require().NoError(err)
	s.Equal("reference-raider-camp", data.Dungeon)

	ids := map[string]bool{}
	for _, member := range out.Members {
		ids[member.ID] = true
	}
	for _, want := range []string{"alice", "bob", "chief", "scout"} {
		s.True(ids[want], "%s is on the board", want)
	}
	s.False(ids["reinforcement-1"], "a reserved placement is not on the board")

	status, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "run"})
	s.Require().NoError(err)
	s.NotNil(status)
}

func (s *LaunchSuite) TestALaunchWhoseThirdMonsterCannotBeResolvedWritesNothing() {
	dungeon := s.camp()
	s.Require().GreaterOrEqual(len(dungeon.Monsters), 3)
	dungeon.Monsters[2].Ref = "dnd5e:monsters:no-such-monster"

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().ErrorIs(err, session.ErrUnknownContent)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAPartyBiggerThanTheSeatsWritesNothing() {
	dungeon := s.camp()
	dungeon.PartyStart = dungeon.PartyStart[:1]

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().ErrorIs(err, session.ErrInvalidWorld)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAnIDClaimedTwiceWritesNothing() {
	dungeon := s.camp()
	s.characters.byID["scout"] = armedFighter("scout")

	_, err := s.launch(dungeon, "alice", "scout")
	s.Require().ErrorIs(err, session.ErrDuplicateMember)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAnUnresolvableSheetWritesNothing() {
	_, err := s.launch(s.camp(), "alice", "nobody")
	s.Require().ErrorIs(err, session.ErrNoCharacter)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAFactionTheDungeonDoesNotDeclareWritesNothing() {
	dungeon := s.camp()
	dungeon.Monsters[1].Faction = "strangers"

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().Error(err)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestACharacterAnotherRunHoldsWritesNothing() {
	s.Require().NoError(s.seats.SaveSeat(context.Background(), &session.SeatData{Character: "bob", Session: "elsewhere"}))
	saves := s.seats.saves

	_, err := s.launch(s.camp(), "alice", "bob")
	s.Require().ErrorIs(err, session.ErrSeatedElsewhere)
	s.Equal(saves, s.seats.saves)
	s.Zero(s.characters.saves)
	s.Zero(s.encounters.saves)
}

func (s *LaunchSuite) TestALaunchOverAnExistingSessionIsRefused() {
	_, err := s.launch(s.camp(), "alice")
	s.Require().NoError(err)

	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: s.camp(), Party: []string{"carol"},
	})
	s.Require().ErrorIs(err, session.ErrSessionExists)
	s.Empty(s.seats.seatOf("carol"))
}

// truceSource is two authored factions hostile to EACH OTHER, standing in
// sight of one another, and a party seat far from both behind a wall.
const truceSource = `
version: 2
key: launch-two-sides
name: Two Sides
orientation: pointy
void: opaque
regions:
  - id: hall
    name: The Hall
    archetype: crypt
    lighting: { intensity: 0.8 }
    cells:
      - [[0,0],[1,0],[2,0],[3,0],[4,0],[5,0]]
      - [[0,1],[1,1],[2,1],[3,1],[4,1],[5,1]]
      - [[0,2],[1,2],[2,2],[3,2],[4,2],[5,2]]
start: { at: [0,0], facing: e }
factions:
  - { id: wolves }
  - { id: raiders }
dispositions:
  - { between: [wolves, raiders], stance: hostile }
place:
  - { id: wolf-a,  ref: "dnd5e:monsters:zombie",   at: [4,1], faction: wolves }
  - { id: raider-a, ref: "dnd5e:monsters:skeleton", at: [5,2], faction: raiders }
  - { id: raider-b, ref: "dnd5e:monsters:skeleton", at: [5,0], faction: raiders }
`

// TestTwoHostileFactionsFormOneFightAfterEveryMemberIsPlaced is the board
// law: the whole board stands before any fight forms, so the fight that forms
// holds everyone it should and starts after the last member arrives.
func (s *LaunchSuite) TestTwoHostileFactionsFormOneFightAfterEveryMemberIsPlaced() {
	compiled, err := dungeonspec.Load([]byte(truceSource))
	s.Require().NoError(err)

	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: &compiled, Party: []string{"alice"},
	})
	s.Require().NoError(err)

	// Every member's own stream: exactly one fight started, and it started
	// after every arrival that member was told about — the monsters saw each
	// other while the board stood, and nothing formed until the party came.
	type account struct {
		lastJoined, fight uint64
		fights            int
	}
	streams := map[string]*account{}
	for _, event := range s.stream.published {
		acct := streams[event.Recipient]
		if acct == nil {
			acct = &account{}
			streams[event.Recipient] = acct
		}
		switch event.Kind {
		case session.EventJoined:
			acct.lastJoined = event.Seq
		case session.EventFightStarted:
			acct.fights++
			acct.fight = event.Seq
		}
	}
	for _, member := range []string{"alice", "wolf-a", "raider-a", "raider-b"} {
		acct := streams[member]
		s.Require().NotNil(acct, "%s was told the story", member)
		s.Equal(1, acct.fights, "%s: one contact, one fight", member)
		s.Greater(acct.fight, acct.lastJoined, "%s: the fight forms after every member is placed", member)
	}
}
