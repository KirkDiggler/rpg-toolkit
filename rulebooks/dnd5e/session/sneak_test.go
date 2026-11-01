// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// sneak_test.go is sneaking through the session (rpg-project#606, sneaking
// plan Task S2; design R6, R8, R10, R12): the verb round-trips and persists,
// a sneaker's world-clock pace is half, a monster's walk on ANOTHER player's
// verb rolls for the sneaker through the real check seam, the snuck beat
// reaches the sneaker's stream alone, and declaring off inside a hostile
// watcher's zone forms a fight that is delivered.
//
// THE SCENE, hearing_test.go's floor. Two rooms joined by one open door on
// row 1: the hall (columns 0–5) and the barracks (6–11). The goblin stands in
// the barracks; alice and bob start in the hall, out of its sight behind the
// wall. testDice rolls every d20 a 10, so alice's Stealth (DEX 14, +2) totals
// 12 against the goblin's passive Perception of 9, and wins.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sneakNeutral is a faction at peace with the party, so the goblin can see
// and be seen without a fight forming (hearing_test.go's reason).
const sneakNeutral encounter.FactionID = "watch"

type SneakSuite struct {
	suite.Suite

	stream     *fakeStream
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestSneakSuite(t *testing.T) { suite.Run(t, new(SneakSuite)) }

func (s *SneakSuite) SetupTest() {
	s.stream = &fakeStream{}
	s.encounters = newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: s.stream,
		Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: newFakeSessions(), Encounters: s.encounters,
		Characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")),
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// launch places alice at hall (2,3), bob at hall (0,4) and the goblin at
// barracks (9,1), neutral or hostile, with the given table. The goblin faces
// west, 270, down its row through the open door into the hall: it is placed to
// watch that door, and since its watch zone keeps only the cells inside its
// arc (design R14, the session's seeded 120), the scene states the facing
// rather than leave it the unstated south, which would turn its back on the
// door.
func (s *SneakSuite) launch(hostile bool, table encounter.Table) {
	s.T().Helper()
	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 9, 1)
	goblin.Table = table
	facingTheDoor := 270.0
	goblin.FacingDegrees = &facingTheDoor
	field := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 5), rectRegion("barracks", 6, 0, 6, 5)},
		Walls:   hexSeamWallsFrom(6, 0, 5, 1),
		Doors: []encounter.DoorInput{{
			ID: "door", Edges: []encounter.DoorEdge{{From: hexCell(5, 1), To: hexCell(6, 1)}}, State: encounter.DoorIsOpen(),
		}},
	}
	if !hostile {
		goblin.Faction = string(sneakNeutral)
		field.Factions = []encounter.FactionInput{{ID: sneakNeutral}}
		field.Dispositions = []encounter.DispositionInput{
			{Between: [2]encounter.FactionID{sneakNeutral, encounter.FactionParty}, Stance: encounter.StanceNeutral},
		}
	}
	launchScene(s.T(), s.mgr, scene{
		Field:    field,
		Party:    []sceneSeat{seatAt("alice", 2, 3), seatAt("bob", 0, 4)},
		Monsters: []dungeonspec.MonsterPlacement{goblin},
		Endings:  []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	s.stream.published = nil
}

func (s *SneakSuite) setSneaking(member string, on bool) *session.SetSneakingOutput {
	s.T().Helper()
	out, err := s.mgr.SetSneaking(context.Background(), &session.SetSneakingInput{
		Session: testSession, Member: member, Sneaking: on,
	})
	s.Require().NoError(err)
	return out
}

func (s *SneakSuite) move(member string, path ...spatial.Position) {
	s.T().Helper()
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: testSession, Member: member, Path: path})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, len(path), "the whole walk happened")
}

// stored is one member's persisted record.
func (s *SneakSuite) stored(member string) encounter.MemberData {
	s.T().Helper()
	for _, m := range s.encounters.byID[testSession].Members {
		if string(m.ID) == member {
			return m
		}
	}
	s.FailNow("no stored member", member)
	return encounter.MemberData{}
}

func (s *SneakSuite) view(member string) *session.ViewOutput {
	s.T().Helper()
	out, err := s.mgr.View(context.Background(), &session.ViewInput{Session: testSession, Member: member})
	s.Require().NoError(err)
	return out
}

// story is one member's whole story, beat by beat.
func (s *SneakSuite) story(member string) []witnessed {
	s.T().Helper()
	return storyBeats(s.T(), s.mgr, testSession, member)
}

// snuckBeat is a snuck beat's payload, the fields these tests read.
type snuckBeat struct {
	Member      string          `json:"member"`
	DC          int             `json:"dc"`
	Total       int             `json:"total"`
	Beaten      bool            `json:"beaten"`
	Calculation json.RawMessage `json:"calculation"`
}

// snuckIn is every snuck beat in one member's story.
func (s *SneakSuite) snuckIn(member string) []snuckBeat {
	s.T().Helper()
	var out []snuckBeat
	for _, e := range beatsNamed(s.story(member), encounter.BeatSnuck) {
		var body snuckBeat
		e.decode(s.T(), &body)
		out = append(out, body)
	}
	return out
}

// TestTheVerbRoundTripsAndPersists: the mode lands on the stored world and
// on the viewer's own View, a repeat is a no-op, off clears it, and a
// monster is refused.
func (s *SneakSuite) TestTheVerbRoundTripsAndPersists() {
	s.launch(false, nil)

	out := s.setSneaking("alice", true)
	s.True(out.Sneaking)
	s.NotEmpty(out.Saved.Written, "the verb persists")
	s.True(s.stored("alice").Sneaking, "the mode is on the stored world")
	s.True(s.view("alice").Sneaking, "and on the sneaker's own view")
	s.False(s.view("bob").Sneaking, "and only on hers")

	// THE REPEAT IS A NO-OP in the composition: no beat, no refresh. The
	// verb still commits, so the save report still names the records.
	before := s.story("alice")
	again := s.setSneaking("alice", true)
	s.True(again.Sneaking, "declaring the mode held returns it")
	s.Equal(before, s.story("alice"), "a repeated declaration tells nothing")
	s.True(s.stored("alice").Sneaking)

	off := s.setSneaking("alice", false)
	s.False(off.Sneaking)
	s.False(s.stored("alice").Sneaking, "off persists")
	s.False(s.view("alice").Sneaking)

	_, err := s.mgr.SetSneaking(context.Background(), &session.SetSneakingInput{
		Session: testSession, Member: "goblin", Sneaking: true,
	})
	s.ErrorIs(err, session.ErrNoMember, "a monster never sneaks")
	_, err = s.mgr.SetSneaking(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)
}

// TestASneakersPaceIsHalf: thirty feet is six cells a round on the world
// clock; sneaking it is three. alice sneaking spends her round on the third
// cell, bob walking the same three cells has not, and alice off sneaking is
// back to six.
func (s *SneakSuite) TestASneakersPaceIsHalf() {
	s.launch(false, nil)
	s.setSneaking("alice", true)
	start := s.encounters.byID[testSession].Clock.HighWater

	s.move("alice", hexCell(3, 3), hexCell(4, 3), hexCell(5, 3))
	s.Equal(0, s.stored("alice").Pace, "three sneaking cells are a whole round")
	s.Greater(s.encounters.byID[testSession].Clock.HighWater, start, "and the round was paid")

	s.move("bob", hexCell(1, 4), hexCell(2, 4), hexCell(3, 4))
	s.Equal(3, s.stored("bob").Pace, "three walking cells are half a round")

	s.setSneaking("alice", false)
	s.move("alice", hexCell(4, 3), hexCell(3, 3), hexCell(2, 3))
	s.Equal(3, s.stored("alice").Pace, "off sneaking, alice walks at full pace again")
}

// TestAMonsterWalkOnAnotherPlayersVerbRollsForTheSneaker: bob's walk pays
// the world a round, the goblin's table walks it through the door into the
// hall, and its zone crosses alice — who did nothing. The roll is alice's,
// made through the real check seam with her staged record, and it reaches
// her stream alone.
func (s *SneakSuite) TestAMonsterWalkOnAnotherPlayersVerbRollsForTheSneaker() {
	walkIn := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, Toward: &encounter.Selector{Path: []spatial.Position{hexCell(3, 1)}}},
	}}
	s.launch(false, walkIn)
	s.setSneaking("alice", true)
	s.Empty(s.snuckIn("alice"), "out of the goblin's sight, declaring rolls nothing")
	goblinAt := persistedMonsterPosition(s.T(), s.encounters, "goblin")

	// bob paces his row until the world has had a round: six cells at
	// thirty feet, so three two-cell walks. Bounded at four, so a pace or
	// clock regression fails here rather than hanging the package.
	before := s.encounters.byID[testSession].Clock.HighWater
	for walks := 0; walks < 4 && s.encounters.byID[testSession].Clock.HighWater == before; walks++ {
		at := persistedMonsterPosition(s.T(), s.encounters, "bob")
		if at == hexCell(0, 4) {
			s.move("bob", hexCell(1, 4), hexCell(2, 4))
		} else {
			s.move("bob", hexCell(1, 4), hexCell(0, 4))
		}
	}
	s.Require().Greater(s.encounters.byID[testSession].Clock.HighWater, before, "bob's walking paid the world a round")
	s.NotEqual(goblinAt, persistedMonsterPosition(s.T(), s.encounters, "goblin"), "the goblin walked")

	rolls := s.snuckIn("alice")
	s.Require().NotEmpty(rolls, "the goblin's walk rolled for alice on bob's verb")
	for _, roll := range rolls {
		s.Equal("alice", roll.Member, "the roll is the sneaker's")
		s.Equal(9, roll.DC, "against the goblin's passive Perception")
		s.Equal(12, roll.Total, "a d20 of 10 and DEX 14's +2, off alice's own sheet")
		s.True(roll.Beaten)
		s.NotEmpty(roll.Calculation, "the real resolver's arithmetic rides the beat")
	}
	s.Empty(s.snuckIn("bob"), "the beat goes to the mover alone")
	for _, a := range s.stream.published {
		if _, ok := a.Body.(session.SneakAct); ok {
			s.Equal("alice", a.Recipient, "delivered to the sneaker's stream only")
		}
	}
	verdicts := s.encounters.byID[testSession].SneakVerdicts["alice"]
	s.Contains(verdicts, encounter.MemberID("goblin"), "the verdict persisted")
}

// TestDeclaringOffInsideAHostileZoneFormsAFight: alice sneaks up the hall
// into the hostile goblin's sight line through the door, wins her roll and
// is not seen; declaring off there is seen at once, and the fight that forms
// is delivered on the streams.
func (s *SneakSuite) TestDeclaringOffInsideAHostileZoneFormsAFight() {
	s.launch(true, nil)
	s.setSneaking("alice", true)
	s.move("alice", hexCell(2, 2), hexCell(2, 1))
	s.Require().NotEmpty(s.snuckIn("alice"), "walking into the zone rolled")
	s.Empty(s.encounters.byID[testSession].Bubbles, "a won roll is not seen: no fight")

	s.stream.published = nil
	off := s.setSneaking("alice", false)
	s.False(off.Sneaking)
	s.NotEmpty(s.encounters.byID[testSession].Bubbles, "stopping in plain view forms a fight")
	delivered := false
	for _, a := range s.stream.published {
		if clock, ok := a.Body.(session.ClockEntry); ok && clock.Mark == session.ClockFightFormed && a.Recipient == "alice" {
			delivered = true
		}
	}
	s.True(delivered, "the formation is delivered to alice's stream")
	s.False(s.stored("alice").Sneaking)
}

// TestTheViewCarriesTheEncountersWatchZonesUnchanged (sneaking Z2, R15): a
// sneaking member's view carries the composition's own WatchZonesFor answer
// value for value (same watchers, same order, same cells) in the session's
// own WatchZone shape, and a walking member's view carries an empty list. alice
// sneaks onto the door row, where she holds the goblin live through it.
func (s *SneakSuite) TestTheViewCarriesTheEncountersWatchZonesUnchanged() {
	s.launch(false, nil)
	s.setSneaking("alice", true)
	s.move("alice", hexCell(2, 2), hexCell(2, 1))

	enc, err := s.mgr.LoadWorldForTest(context.Background(), testSession)
	s.Require().NoError(err)
	want, err := enc.WatchZonesFor(&encounter.WatchZonesInput{Member: "alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(want.Zones, "the scene must give alice a watcher to see, or the test proves nothing")

	got := s.view("alice").WatchZones
	expected := make([]session.WatchZone, 0, len(want.Zones))
	for _, zone := range want.Zones {
		expected = append(expected, session.WatchZone{Watcher: string(zone.Watcher), Cells: zone.Cells})
	}
	s.Require().NotEmpty(want.Zones[0].Cells, "the goblin's zone reaches floor alice knows")
	s.Equal(expected, got, "the same watchers in the same order, each with the encounter's cells unchanged")
	bob := s.view("bob").WatchZones
	s.NotNil(bob, "a walker's list is empty, never nil")
	s.Empty(bob, "a member who is not sneaking is sent no zones")
}
