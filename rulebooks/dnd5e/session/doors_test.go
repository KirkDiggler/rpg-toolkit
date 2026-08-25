// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// doors_test.go drives the door verbs (rpg-project#268): Doors reads the
// live state the Atlas deliberately does not carry, OpenDoor pushes a shut
// door open, and Unlock rolls the sheet against the authored DC and TELLS
// the composition the verdict. The walk refusals are the rpg-toolkit#1135
// split, seen from the host's side: a locked door says locked, a shut door
// says shut, and neither hides behind "bad position".

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// deftCharacter is a sheet whose DEX the unlock check reads: score 14 is a
// +2 modifier, so testDice's flat 10 totals 12 — exactly the reference
// tomb's DC, and a tie goes to the roller.
func deftCharacter(id string, dex int) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Delve", Level: 3,
		ProficiencyBonus: 2, RaceID: races.Dwarf, ClassID: classes.Rogue,
		HitPoints: 20, MaxHitPoints: 20, ArmorClass: 14,
		AbilityScores: shared.AbilityScores{abilities.DEX: dex},
	}
}

// gatedWorld is read_test's hexWorld with the gate in a caller-chosen state
// and alice standing at its west cell, one step from crossing it.
func gatedWorld(t fataler, state encounter.DoorState) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("corridor", 0, 0, 6, 6),
				rectRegion("vault", 6, 0, 6, 6),
			},
			Walls: hexSeamWalls(6, 6, 0),
			Doors: []encounter.DoorInput{{
				ID:    "gate",
				Edges: []encounter.DoorEdge{{From: hexCell(5, 0), To: hexCell(6, 0)}},
				State: state,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 0}},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		t.Fatalf("building gated world: %v", err)
	}
	data := enc.ToData()
	return &data
}

const tombDC = 12

func tombLock() encounter.DoorState {
	return encounter.DoorIsLocked(encounter.Lock{DC: tombDC, Ability: "dex"})
}

type DoorsSuite struct {
	suite.Suite

	stream *fakeStream
	mgr    *session.Manager
}

func TestDoorsSuite(t *testing.T) {
	suite.Run(t, new(DoorsSuite))
}

// startWith wires a fresh manager around the given world and cast, with the
// stream recorded — the beats are half of what this suite pins.
func (s *DoorsSuite) startWith(world *encounter.EncounterData, cast ...*character.Data) {
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(cast...), Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)
}

// doorEvents filters what the stream heard down to the door beats one
// audience received, typed.
func (s *DoorsSuite) doorEvents(audience string) []session.DoorBody {
	s.T().Helper()

	out := make([]session.DoorBody, 0)
	for _, e := range s.stream.published {
		if e.Kind != session.EventDoor || e.Recipient != audience {
			continue
		}
		body, ok := e.Body.(session.DoorBody)
		s.Require().True(ok, "a door event carries its typed body")
		out = append(out, body)
	}
	return out
}

func (s *DoorsSuite) TestDoorsReadsTheLiveState() {
	ctx := context.Background()

	s.Run("an open gate has no lock to report", func() {
		s.startWith(hexWorld(s.T()))
		out, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess"})
		s.Require().NoError(err)
		s.Require().Len(out.Doors, 1)
		s.Equal(session.Door{ID: "gate", State: "open"}, out.Doors[0])
	})

	s.Run("a locked gate reports its lock, DC and all", func() {
		s.startWith(gatedWorld(s.T(), tombLock()))
		out, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess"})
		s.Require().NoError(err)
		s.Require().Len(out.Doors, 1)
		s.Equal(session.Door{ID: "gate", State: "locked",
			Lock: &session.DoorLock{DC: tombDC, Ability: "dex"}}, out.Doors[0])
	})
}

func (s *DoorsSuite) TestAWalkIntoTheDoorSaysWhatStoppedIt() {
	ctx := context.Background()

	s.Run("locked says locked — the fiction beat, not a bad cell", func() {
		s.startWith(gatedWorld(s.T(), tombLock()))
		_, err := s.mgr.Move(ctx, &session.MoveInput{
			Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(6, 0)}})
		s.Require().ErrorIs(err, session.ErrLocked)
		s.NotErrorIs(err, session.ErrBadPosition,
			"a caller told bad-position goes looking for arithmetic that is fine")
	})

	s.Run("shut says shut — the remedy is OpenDoor, not new coordinates", func() {
		s.startWith(gatedWorld(s.T(), encounter.DoorIsClosed()))
		_, err := s.mgr.Move(ctx, &session.MoveInput{
			Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(6, 0)}})
		s.Require().ErrorIs(err, session.ErrDoorShut)
		s.NotErrorIs(err, session.ErrBadPosition)
	})
}

func (s *DoorsSuite) TestOpenDoorOpensAndTheTableHears() {
	ctx := context.Background()
	s.startWith(gatedWorld(s.T(), encounter.DoorIsClosed()))

	out, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	s.Equal(session.Door{ID: "gate", State: "open"}, out.Door)

	read, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess"})
	s.Require().NoError(err)
	s.Equal("open", read.Doors[0].State, "the read agrees with the verb")

	beats := s.doorEvents("alice")
	s.Require().Len(beats, 1)
	s.Equal(session.DoorBody{Door: "gate", State: "open", Actor: "alice"}, beats[0],
		"the beat names the door, the state, and whose hands")

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(6, 0)}})
	s.NoError(err, "and the way is open")
}

func (s *DoorsSuite) TestOpenDoorRefusesALockedOne() {
	s.startWith(gatedWorld(s.T(), tombLock()))
	_, err := s.mgr.OpenDoor(context.Background(), &session.OpenDoorInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().ErrorIs(err, session.ErrLocked, "Unlock is the way through a lock")
}

func (s *DoorsSuite) TestUnlockRollsTheSheetAgainstTheDC() {
	ctx := context.Background()
	s.startWith(gatedWorld(s.T(), tombLock()), deftCharacter("alice", 14))

	out, err := s.mgr.Unlock(ctx, &session.UnlockInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	s.True(out.Beaten, "10 rolled + 2 dex meets DC 12 — a tie goes to the roller")
	s.Equal(tombDC, out.Total, "the total is public, down to the number")
	s.Equal(tombDC, out.DC)
	s.Equal("open", out.Door.State, "a beaten lock OPENS the door, it does not merely unlock it")

	beats := s.doorEvents("alice")
	s.Require().Len(beats, 1)
	s.Equal(session.DoorBody{Door: "gate", State: "open", Actor: "alice",
		DC: tombDC, Total: tombDC, Beaten: true}, beats[0],
		"the attempt is narrated with its author and its numbers")
}

func (s *DoorsSuite) TestAFailedUnlockIsAnOutcomeNotAnError() {
	ctx := context.Background()
	s.startWith(gatedWorld(s.T(), tombLock()), deftCharacter("alice", 10))

	out, err := s.mgr.Unlock(ctx, &session.UnlockInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err, "a failed check is an outcome, not an error")
	s.False(out.Beaten, "10 rolled + 0 dex misses DC 12")
	s.Equal(10, out.Total)
	s.Equal("locked", out.Door.State, "unchanged")

	beats := s.doorEvents("alice")
	s.Require().Len(beats, 1)
	s.Equal(session.DoorBody{Door: "gate", State: "locked", Actor: "alice",
		DC: tombDC, Total: 10, Beaten: false}, beats[0],
		"the miss is as much fiction as the hit")

	again, err := s.mgr.Unlock(ctx, &session.UnlockInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err, "and the lock is retryable")
	s.False(again.Beaten)
}

func (s *DoorsSuite) TestALockNamingNoRulebookAbilityIsRefusedLoudly() {
	s.startWith(
		gatedWorld(s.T(), encounter.DoorIsLocked(encounter.Lock{DC: tombDC, Ability: "luck"})),
		deftCharacter("alice", 14))
	_, err := s.mgr.Unlock(context.Background(), &session.UnlockInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().ErrorIs(err, session.ErrInvalidWorld,
		"a content defect, refused by name — never a silent -5 from an unknown key")
	s.Contains(err.Error(), "luck")
}

func (s *DoorsSuite) TestUnlockOfAnUnlockedDoorIsRefused() {
	s.startWith(hexWorld(s.T()), deftCharacter("alice", 14))
	_, err := s.mgr.Unlock(context.Background(), &session.UnlockInput{
		Session: "sess", Member: "alice", Door: "gate"})
	s.Require().ErrorIs(err, session.ErrNoConnection,
		"the composition's own refusal, translated — there is no lock to try")
}

func (s *DoorsSuite) TestTheEndedBeatCarriesItsKey() {
	ctx := context.Background()
	s.startWith(hexWorld(s.T()))

	_, err := s.mgr.End(ctx, &session.EndInput{Session: "sess", Ending: "out"})
	s.Require().NoError(err)

	var endeds []session.EndedBody
	for _, e := range s.stream.published {
		if e.Kind != session.EventEnded || e.Recipient != "alice" {
			continue
		}
		body, ok := e.Body.(session.EndedBody)
		s.Require().True(ok, "an ended event carries its typed body")
		endeds = append(endeds, body)
	}
	s.Require().Len(endeds, 1)
	s.Equal(session.EndedBody{Ending: "out"}, endeds[0],
		"a client following the stream finally hears HOW the run ended")
}
