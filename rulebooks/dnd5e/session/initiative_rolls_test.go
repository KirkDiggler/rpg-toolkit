// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// EVERYONE IN A FIGHT READS EVERY MEMBER'S INITIATIVE ROLL
// (rpg-project#1035): the fight-forming entry of the debug record carries each
// member's roll, and the act stream's fight-formed entry carries the same.
type InitiativeRollsSuite struct{ suite.Suite }

func TestInitiativeRollsSuite(t *testing.T) { suite.Run(t, new(InitiativeRollsSuite)) }

// formed launches a hall with the party and one monster in plain sight, with
// the given faces in sorted-ID order, and returns what the record and the act
// stream were handed.
func (s *InitiativeRollsSuite) formed(party []*character.Data, monster dungeonspec.MonsterPlacement, faces ...int) (*fakeRecord, *fakeStream) {
	rec, stream := &fakeRecord{}, &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Record: rec, Acts: stream, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: &sequenceDice{rolls: faces}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(party...),
	})
	s.Require().NoError(err)
	seats := make([]sceneSeat, 0, len(party))
	for i, member := range party {
		seats = append(seats, seatAt(member.ID, 1+i, 1))
	}
	launched := launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 40, 8)}},
		Party:    seats,
		Monsters: []dungeonspec.MonsterPlacement{monster},
	})
	s.Require().Len(launched.Formed, 1, "standing in plain sight starts one fight")
	return rec, stream
}

func (s *InitiativeRollsSuite) twoAndAZombie() (*fakeRecord, *fakeStream) {
	return s.formed(
		[]*character.Data{fighterWithDex("a-slow", 8), fighterWithDex("b-quick", 18)},
		monsterAt("m-zombie", refs.Monsters.Zombie().String(), 6, 1),
		10, 10, 10)
}

// fightFormedFor is the fight-formed clock entry delivered to a recipient.
func fightFormedFor(stream *fakeStream, recipient string) (session.ClockEntry, bool) {
	for _, act := range stream.published {
		if act.Recipient != recipient {
			continue
		}
		if entry, ok := act.Body.(session.ClockEntry); ok && entry.Mark == session.ClockFightFormed {
			return entry, true
		}
	}
	return session.ClockEntry{}, false
}

func rollOfMember(rolls []session.InitiativeRoll, id string) *session.Roll {
	for _, r := range rolls {
		if r.Member.ID == id {
			return r.Roll
		}
	}
	return nil
}

func modifierNamed(roll *session.Roll, name string) *session.RollModifier {
	for i := range roll.Modifiers {
		if roll.Modifiers[i].Source == name {
			return &roll.Modifiers[i]
		}
	}
	return nil
}

// Catches a record entry with no rolls, a roll not found by its member, or a
// calculation rebuilt rather than carried (the d20 source and Dexterity
// modifier come from the rulebook).
func (s *InitiativeRollsSuite) TestTheRecordCarriesEveryMembersInitiativeRoll() {
	rec, _ := s.twoAndAZombie()
	var started []session.RecordEvent
	for _, batch := range rec.batches {
		started = append(started, recordOf(batch, encounter.BeatFightStarted)...)
	}
	s.Require().Len(started, 1)
	var fields struct {
		Rolls []struct {
			Member      string `json:"member"`
			Roll        int    `json:"roll"`
			Total       int    `json:"total"`
			Calculation struct {
				Components []struct {
					Kind   string `json:"kind"`
					Source struct {
						Name     string `json:"name"`
						Ref      string `json:"ref"`
						SourceID string `json:"source_id"`
					} `json:"source"`
					Modifier *int `json:"modifier"`
				} `json:"components"`
			} `json:"calculation"`
		} `json:"rolls"`
	}
	s.Require().NoError(json.Unmarshal(started[0].Fields, &fields))
	s.Require().Len(fields.Rolls, 3)
	type want struct{ roll, total int }
	expected := map[string]want{"b-quick": {10, 14}, "a-slow": {10, 9}, "m-zombie": {10, 8}}
	for _, r := range fields.Rolls {
		w, ok := expected[r.Member]
		s.Require().True(ok, "unexpected member %q", r.Member)
		s.Equal(w.roll, r.Roll, r.Member)
		s.Equal(w.total, r.Total, r.Member)
		if r.Member != "b-quick" {
			continue
		}
		s.Require().GreaterOrEqual(len(r.Calculation.Components), 2)
		first, second := r.Calculation.Components[0], r.Calculation.Components[1]
		s.Equal("Initiative", first.Source.Name)
		s.Equal("dnd5e:rules:initiative", first.Source.Ref)
		s.Equal("b-quick", first.Source.SourceID)
		s.Equal("Dexterity", second.Source.Name)
		s.Require().NotNil(second.Modifier)
		s.Equal(4, *second.Modifier)
	}
}

// Catches rolls missing from the act stream's fight-formed entry, out of the
// order's order, or without the Dexterity line.
func (s *InitiativeRollsSuite) TestTheActStreamCarriesEveryMembersInitiativeRoll() {
	_, stream := s.twoAndAZombie()
	entry, ok := fightFormedFor(stream, "a-slow")
	s.Require().True(ok, "a-slow is told the fight formed")
	s.Require().NotNil(entry.Fight)
	var order, rolled []string
	for _, p := range entry.Fight.Order {
		order = append(order, p.ID)
	}
	for _, r := range entry.Fight.Rolls {
		rolled = append(rolled, r.Member.ID)
	}
	s.Equal(order, rolled, "one roll per member, in the order's order")
	quick := rollOfMember(entry.Fight.Rolls, "b-quick")
	s.Require().NotNil(quick)
	s.Equal(14, quick.Total)
	s.Equal([]int{10}, quick.Dice)
	dex := modifierNamed(quick, "Dexterity")
	s.Require().NotNil(dex)
	s.Equal(4, dex.Value)
}

// Catches a score of ten dropped from the roll instead of shown as plus zero.
func (s *InitiativeRollsSuite) TestDexterityTenIsShownAsZero() {
	_, stream := s.formed(
		[]*character.Data{fighterWithDex("a-ten", 10)},
		monsterAt("m-zombie", refs.Monsters.Zombie().String(), 6, 1),
		10, 10)
	entry, ok := fightFormedFor(stream, "a-ten")
	s.Require().True(ok)
	roll := rollOfMember(entry.Fight.Rolls, "a-ten")
	s.Require().NotNil(roll)
	dex := modifierNamed(roll, "Dexterity")
	s.Require().NotNil(dex, "a modifier of zero is still shown")
	s.Equal(0, dex.Value)
	s.Equal(10, roll.Total)
}
