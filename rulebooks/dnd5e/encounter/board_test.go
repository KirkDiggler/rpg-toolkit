// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// BoardSuite is the launch's whole board (rpg-project#542, "Launch"): every
// member placed, then one look, so two camps hostile to each other and the
// party form their fight at the same moment rather than in placement order.
type BoardSuite struct {
	suite.Suite
}

func TestBoardSuite(t *testing.T) { suite.Run(t, new(BoardSuite)) }

// empty is a yard with the goblins and the dogs declared hostile to each
// other, and nobody in it yet.
func (s *BoardSuite) empty() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("yard", 0, 0, 8, 6)},
			Factions: []encounter.FactionInput{{ID: bwGoblins}, {ID: bwDogs}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, bwDogs}, Stance: encounter.StanceHostile,
			}},
		},
		Endings:   []encounter.EndingInput{withdrawn()},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	return enc
}

// launch is the board in placement order: the two camps first, the party
// last — the order that, joined one at a time, starts the fight without her.
func launch() []encounter.JoinInput {
	return []encounter.JoinInput{
		{Member: bwChief, Kind: encounter.KindMonster, Faction: bwGoblins, Cell: cellAt(4, 1)},
		{Member: bwDog, Kind: encounter.KindMonster, Faction: bwDogs, Cell: cellAt(5, 1)},
		{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)},
	}
}

// The whole board stands before anybody looks: one fight forms, after every
// member is placed, and it holds the party as well as both camps.
func (s *BoardSuite) TestTheFightFormsOnceEverybodyIsPlaced() {
	enc := s.empty()

	out, err := enc.Board(&encounter.BoardInput{Members: launch()})
	s.Require().NoError(err)
	s.Require().Len(out.Joined, 3)
	s.Require().Len(out.Formed, 1, "one look, one fight")
	s.ElementsMatch([]encounter.MemberID{bwChief, bwDog, alice}, out.Formed[0].Order, "everybody in contact is in it")
	for i, joined := range out.Joined {
		s.Nil(joined.Formed, "members[%d]: no member's arrival formed anything on its own", i)
		s.Equal(launch()[i].Member, joined.Member.ID)
	}

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	formedSeqs := []uint64{}
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatFightStarted {
			formedSeqs = append(formedSeqs, entry.Seq)
		}
	}
	s.Require().Len(formedSeqs, 1, "exactly one fight formed")
	for i, joined := range out.Joined {
		s.Less(joined.Seq, formedSeqs[0], "members[%d] was placed before the fight formed", i)
	}
}

// THE CONTRAST: joined one at a time, the fight is decided by placement
// order. With the party placed first, it forms the moment the first camp
// lands — goblins and party only — and the dogs, placed next, are not in the
// fight that formed.
func (s *BoardSuite) TestJoiningOneAtATimeFormsTheFightEarly() {
	enc := s.empty()
	members := launch()

	_, err := enc.Join(&members[2])
	s.Require().NoError(err, "the party first")

	chief, err := enc.Join(&members[0])
	s.Require().NoError(err)
	s.Require().NotNil(chief.Formed, "the goblins land in sight of the party and the fight starts")
	s.NotContains(chief.Formed.Order, bwDog, "without the dogs, who are not placed yet")

	dog, err := enc.Join(&members[1])
	s.Require().NoError(err)
	s.Nil(dog.Formed, "the dogs form nothing of their own: the fight already formed without them")
}

// An unplaceable member anywhere on the board refuses the whole board, and
// nothing is written: nobody placed, no beat, nobody in reserve.
func (s *BoardSuite) TestAnUnplaceableMemberPlacesNobody() {
	enc := s.empty()
	nextSeq, err := enc.NextStorySeq()
	s.Require().NoError(err)

	cases := map[string]func([]encounter.JoinInput) []encounter.JoinInput{
		"off the floor": func(m []encounter.JoinInput) []encounter.JoinInput {
			m[2].Cell = cellAt(40, 40)
			return m
		},
		"a record the field never declared": func(m []encounter.JoinInput) []encounter.JoinInput {
			m[2].Holds = []encounter.IntelID{"never-written"}
			return m
		},
		"the same id twice": func(m []encounter.JoinInput) []encounter.JoinInput {
			m[2].Member = bwChief
			return m
		},
		"a blocked cell": func(m []encounter.JoinInput) []encounter.JoinInput {
			m[0].BlocksMovement = true
			m[2].Cell = m[0].Cell
			return m
		},
	}
	for name, edit := range cases {
		members := launch()
		// A reserved member ahead of the bad one, so a partial write would
		// show in the reserve as well as on the roster.
		members[1].Arrives = encounter.TriggerRound{Round: 99}
		_, err := enc.Board(&encounter.BoardInput{Members: edit(members)})
		s.Require().Error(err, name)
		s.Contains(err.Error(), "members[2]", name)

		roster, rerr := enc.Members()
		s.Require().NoError(rerr)
		s.Empty(roster, "%s: nobody placed", name)
		s.Empty(enc.ToData().Reserve, "%s: nobody held in reserve", name)
		after, serr := enc.NextStorySeq()
		s.Require().NoError(serr)
		s.Equal(nextSeq, after, "%s: no beat written", name)
	}

	_, err = enc.Board(&encounter.BoardInput{})
	s.ErrorIs(err, encounter.ErrNoMember, "an empty board")
}
