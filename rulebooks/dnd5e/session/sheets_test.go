// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SheetFactsSuite is "sheet facts asked at use time" (rpg-project#538) seen
// from the host: the world asks a member's sheet how fast it walks at the
// moment it paces a walk, so a sheet changed between two walks paces the
// second at its new speed with nothing re-joined and nothing re-written.
type SheetFactsSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestSheetFactsSuite(t *testing.T) {
	suite.Run(t, new(SheetFactsSuite))
}

func (s *SheetFactsSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"))
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sheets: encStandStill{},
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{}, Initiative: encOrderAsGiven{},
		TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 16, 3)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)
}

// walk moves alice n cells east along row 1 from column from.
func (s *SheetFactsSuite) walk(from, n int) {
	s.T().Helper()
	path := make([]spatial.Position, 0, n)
	for x := from; x < from+n; x++ {
		path = append(path, spatial.Position{X: float64(x), Y: 1})
	}
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: path})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, n, "the whole walk happened")
}

// rounds is how many world rounds the stored clock has seen.
func (s *SheetFactsSuite) rounds() int {
	s.T().Helper()
	return s.encounters.byID["world"].Clock.HighWater
}

// TestAPlayerWalkingOutsideAFightPacesFromTheSheetsSpeedOfThatMoment is slice
// 4's done-when. Six cells is a round at a Human's 30 feet. Then her sheet
// changes — a Dwarf now, 25 feet, five cells a round — with no Join and no
// write to the world, and five more cells are a round: the second walk was
// paced by the sheet of its own moment. A speed copied at Join would want six.
func (s *SheetFactsSuite) TestAPlayerWalkingOutsideAFightPacesFromTheSheetsSpeedOfThatMoment() {
	s.walk(2, 6)
	s.Require().Equal(1, s.rounds(), "precondition: six cells at a Human's thirty feet is one round")

	s.characters.byID["alice"].RaceID = races.Dwarf

	s.walk(8, 5)
	s.Equal(2, s.rounds(), "five cells at a Dwarf's twenty-five feet is a round, read at the walk")
}
