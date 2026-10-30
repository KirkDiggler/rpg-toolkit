// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

const activityThoughtful = "dnd5e:activities:thoughtful"
const activityRelaxed = "dnd5e:activities:relaxed"

type DefaultActivitySuite struct{ suite.Suite }

func TestDefaultActivitySuite(t *testing.T) { suite.Run(t, new(DefaultActivitySuite)) }

func defaultActivitySetup(activity string) *SetupInput {
	return &SetupInput{
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("activity-room", 0, 0, 10, 10)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}, AppearanceRef: appearanceMerchant, DefaultActivityRef: activity},
		},
		Endings:      []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: appearanceTestCapabilities(),
	}
}

func (s *DefaultActivitySuite) encounter() *Encounter {
	enc, err := NewEncounter(defaultActivitySetup(activityThoughtful))
	s.Require().NoError(err)
	return enc
}

func (s *DefaultActivitySuite) TestCaptureMemoryReloadAndReobservation() {
	enc := s.encounter()
	seen, current, held := heldBy(s.T(), enc, "alice", "bob")
	s.True(current && held)
	s.Equal(activityThoughtful, seen.DefaultActivityRef)
	s.Equal(appearanceMerchant, seen.AppearanceRef)
	enc.sight = seesNobody{}
	_, err := enc.rebuildPercepts(enc.rosterIDs())
	s.Require().NoError(err)
	data := enc.ToData()
	for i := range data.Members {
		if data.Members[i].ID == "bob" {
			s.Equal(activityThoughtful, data.Members[i].DefaultActivityRef)
			data.Members[i].DefaultActivityRef = activityRelaxed
		}
	}
	encoded, err := json.Marshal(data)
	s.Require().NoError(err)
	var restored EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &restored))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: restored, Capabilities: appearanceTestCapabilities()})
	s.Require().NoError(err)
	memory, current, held := heldBy(s.T(), loaded, "alice", "bob")
	s.True(held)
	s.False(current)
	s.Equal(activityThoughtful, memory.DefaultActivityRef, "loading never enriches remembered activity from live data")
	s.Equal(activityRelaxed, loaded.members["bob"].DefaultActivityRef)
	_, err = loaded.rebuildPercepts(loaded.rosterIDs())
	s.Require().NoError(err)
	seen, current, _ = heldBy(s.T(), loaded, "alice", "bob")
	s.True(current)
	s.Equal(activityRelaxed, seen.DefaultActivityRef)
}

func (s *DefaultActivitySuite) TestReserveJoinAndLoadValidateDefaultActivity() {
	enc := s.encounter()
	_, err := enc.Join(&JoinInput{Member: "waiting", Kind: KindMonster, Cell: spatial.Position{X: 3, Y: 1}, DefaultActivityRef: activityThoughtful, Arrives: TriggerRound{Round: 99}})
	s.Require().NoError(err)
	data := enc.ToData()
	s.Require().Len(data.Reserve, 1)
	s.Equal(activityThoughtful, data.Reserve[0].DefaultActivityRef)
	encoded, err := json.Marshal(data)
	s.Require().NoError(err)
	var restored EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &restored))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: restored, Capabilities: appearanceTestCapabilities()})
	s.Require().NoError(err)
	s.Equal(activityThoughtful, loaded.ToData().Reserve[0].DefaultActivityRef)
	before := enc.ToData()
	_, err = enc.Join(&JoinInput{Member: "bad", Kind: KindPlayer, Cell: spatial.Position{X: 4, Y: 1}, DefaultActivityRef: "not-a-ref"})
	s.ErrorIs(err, ErrBadMember)
	s.Equal(before, enc.ToData(), "invalid join is refused before mutation")
	_, err = NewEncounter(defaultActivitySetup("bad"))
	s.ErrorIs(err, ErrBadMember)
	data.Members[0].DefaultActivityRef = "bad"
	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: appearanceTestCapabilities()})
	s.Error(err)
	data = enc.ToData()
	data.Reserve[0].DefaultActivityRef = "bad"
	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: appearanceTestCapabilities()})
	s.Error(err)
}

func (s *DefaultActivitySuite) TestStrictDefaultActivityPayload() {
	for _, state := range []LocationState{LocationKnown, LocationLost} {
		payload, err := EncodeSightTestimony(SightTestimony{State: state, Position: spatial.Position{X: 1, Y: 2}, DefaultActivityRef: activityThoughtful, Conditions: &ConditionSet{}})
		s.Require().NoError(err)
		decoded, ok := DecodeSightTestimony(payload)
		s.Require().True(ok)
		s.Equal(activityThoughtful, decoded.DefaultActivityRef)
		public, err := deliveredSightPayload(payload)
		s.Require().NoError(err)
		delivered, ok := DecodeSightTestimony(public)
		s.Require().True(ok)
		s.Nil(delivered.Conditions)
		s.Equal(activityThoughtful, delivered.DefaultActivityRef)
	}
	for _, payload := range []string{`{"state":"known","x":1,"y":2}`, `{"state":"unknown"}`} {
		decoded, ok := DecodeSightTestimony([]byte(payload))
		s.True(ok)
		s.Empty(decoded.DefaultActivityRef)
	}
	for _, value := range []string{`null`, `""`, `"bad"`, `42`} {
		_, ok := DecodeSightTestimony([]byte(`{"state":"known","x":1,"y":2,"default_activity_ref":` + value + `}`))
		s.False(ok, value)
	}
	_, err := EncodeSightTestimony(SightTestimony{State: LocationUnknown, DefaultActivityRef: activityThoughtful})
	s.Error(err)
	_, ok := DecodeSightTestimony([]byte(`{"state":"unknown","default_activity_ref":"` + activityThoughtful + `"}`))
	s.False(ok)
	_, ok = DecodeSightTestimony([]byte(`{"x":1,"y":2,"default_activity_ref":"` + activityThoughtful + `"}`))
	s.False(ok, "an activity does not turn untagged coordinates into known testimony")
}
