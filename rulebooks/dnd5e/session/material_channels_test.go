// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

type MaterialChannelRosterSuite struct{ suite.Suite }

func TestMaterialChannelRosterSuite(t *testing.T) { suite.Run(t, new(MaterialChannelRosterSuite)) }

func rosterChannels() []encounter.MaterialChannelAdjustment {
	return []encounter.MaterialChannelAdjustment{{TargetID: "body", Normal: &encounter.ChannelAdjustment{Mode: encounter.ChannelScale, Multiplier: 1}, Emissive: &encounter.ChannelAdjustment{Mode: encounter.ChannelOff}}}
}

func (s *MaterialChannelRosterSuite) TestObserversRetainTheirOwnChannelAppearanceAcrossReload() {
	f := newRosterFixture(s.T())
	data := rosterWorldWithPresentation(s.T(), rosterPresentation{appearanceRef: merchantAppearance, materialChannels: rosterChannels()}, encEveryoneSees{})
	for i := range data.Members {
		if data.Members[i].ID == "skel-1" {
			*data.Members[i].MaterialChannels[0].Normal.Multiplier = 2
		}
	}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: *data, Capabilities: encounter.Capabilities{
		Sheets: encStandStill{}, Sight: aliceCannotSee{}, Equipment: encNoHandsObserved{}, Conditions: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, Driver: encPassDriver{}, Participation: encEveryoneStanding{},
		Actors: encounter.Actors{Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}},
	}})
	s.Require().NoError(err)
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{"skel-1"}})
	s.Require().NoError(err)
	encoded, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var saved encounter.EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	f.encounters.byID["world"] = &saved
	for observer, expected := range map[string]float64{"alice": 1, "bob": 2} {
		out, err := f.manager.Roster(context.Background(), &session.RosterInput{Session: "sess", Member: observer, Player: "player-" + observer})
		s.Require().NoError(err)
		found := false
		for _, member := range out.Members {
			if member.ID != "skel-1" {
				continue
			}
			found = true
			s.Require().Len(member.MaterialChannels, 1)
			s.Equal(expected, *member.MaterialChannels[0].Normal.Multiplier)
			s.Equal(session.ChannelOff, member.MaterialChannels[0].Emissive.Mode)
			s.Nil(member.MaterialChannels[0].Emissive.Multiplier)
			*member.MaterialChannels[0].Normal.Multiplier = 9
		}
		s.True(found)
	}
}

func (s *MaterialChannelRosterSuite) TestNeverSeenAndLegacyAbsenceAreNotEnriched() {
	f := newRosterFixture(s.T())
	f.encounters.byID["world"] = rosterWorldWithPresentation(s.T(), rosterPresentation{materialChannels: rosterChannels()}, aliceCannotSee{})
	out, err := f.manager.Roster(context.Background(), &session.RosterInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	for _, member := range out.Members {
		s.NotEqual("skel-1", member.ID)
	}
	legacy := rosterWorld(s.T())
	for i := range legacy.Members {
		if legacy.Members[i].ID == "skel-1" {
			one := float64(1)
			legacy.Members[i].MaterialChannels = []encounter.MaterialChannelAdjustmentData{{TargetID: "body", Normal: &encounter.ChannelAdjustmentData{Mode: encounter.ChannelScale, Multiplier: &one}}}
		}
	}
	f.encounters.byID["world"] = legacy
	out, err = f.manager.Roster(context.Background(), &session.RosterInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	found := false
	for _, member := range out.Members {
		if member.ID == "skel-1" {
			found = true
			s.Empty(member.MaterialChannels)
		}
	}
	s.True(found)
}

func (s *LaunchSuite) TestMaterialChannelsLaunchReachesActiveAndReservedMembers() {
	dungeon := s.camp()
	expected := map[string]bool{}
	for i := range dungeon.Monsters {
		dungeon.Monsters[i].MaterialChannels = rosterChannels()
		expected[dungeon.Monsters[i].MemberID] = true
	}
	_, err := s.launch(dungeon, "alice")
	s.Require().NoError(err)
	world := s.encounters.byID["run"]
	found := map[string]bool{}
	for _, member := range world.Members {
		if expected[string(member.ID)] {
			s.Require().Len(member.MaterialChannels, 1)
			s.Equal(float64(1), *member.MaterialChannels[0].Normal.Multiplier)
			found[string(member.ID)] = true
		}
	}
	for _, member := range world.Reserve {
		if expected[string(member.ID)] {
			s.Require().Len(member.MaterialChannels, 1)
			s.Equal(float64(1), *member.MaterialChannels[0].Normal.Multiplier)
			found[string(member.ID)] = true
		}
	}
	s.Equal(expected, found)
}
