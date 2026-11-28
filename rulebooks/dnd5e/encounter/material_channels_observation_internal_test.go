// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type MaterialChannelObservationSuite struct{ suite.Suite }

func TestMaterialChannelsRememberedAppearanceDoesNotRefresh(t *testing.T) {
	suite.Run(t, new(MaterialChannelObservationSuite))
}

func (s *MaterialChannelObservationSuite) encounter() *Encounter {
	channels := materialChannelsFixture()
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("channels", 0, 0, 10, 10)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1},
				AppearanceRef: appearanceMerchant, MaterialChannels: channels},
		},
		Endings:      []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: appearanceTestCapabilities(),
	})
	s.Require().NoError(err)
	channels[0].Normal.Multiplier = 9
	return enc
}

func (s *MaterialChannelObservationSuite) TestCapturedMemoryAndReobservation() {
	enc := s.encounter()
	seen, current, held := heldBy(s.T(), enc, "alice", "bob")
	s.True(current && held)
	s.Equal(materialChannelsFixture(), seen.MaterialChannels)
	seen.MaterialChannels[0].Normal.Multiplier = 8
	again, _, _ := heldBy(s.T(), enc, "alice", "bob")
	s.Equal(materialChannelsFixture(), again.MaterialChannels)
	enc.sight = seesNobody{}
	_, err := enc.rebuildPercepts(enc.rosterIDs())
	s.Require().NoError(err)
	data := enc.ToData()
	for i := range data.Members {
		if data.Members[i].ID == "bob" {
			*data.Members[i].MaterialChannels[0].Normal.Multiplier = 2
		}
	}
	payload, err := json.Marshal(data)
	s.Require().NoError(err)
	var restored EncounterData
	s.Require().NoError(json.Unmarshal(payload, &restored))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: restored, Capabilities: appearanceTestCapabilities()})
	s.Require().NoError(err)
	memory, current, held := heldBy(s.T(), loaded, "alice", "bob")
	s.True(held)
	s.False(current)
	s.Equal(materialChannelsFixture(), memory.MaterialChannels, "load does not join memory to newer live facts")
	s.Equal(float64(2), loaded.members["bob"].MaterialChannels[0].Normal.Multiplier)
	for i := range restored.Members {
		if restored.Members[i].ID == "bob" {
			*restored.Members[i].MaterialChannels[0].Normal.Multiplier = 7
		}
	}
	_, err = loaded.rebuildPercepts(loaded.rosterIDs())
	s.Require().NoError(err)
	seen, current, _ = heldBy(s.T(), loaded, "alice", "bob")
	s.True(current)
	s.Equal(float64(2), seen.MaterialChannels[0].Normal.Multiplier)
}

func (s *MaterialChannelObservationSuite) TestReserveAndJoinValidateBeforeMutation() {
	enc := s.encounter()
	channels := materialChannelsFixture()
	_, err := enc.Join(&JoinInput{Member: "waiting", Kind: KindMonster, Cell: spatial.Position{X: 3, Y: 1},
		MaterialChannels: channels, Arrives: TriggerRound{Round: 99}})
	s.Require().NoError(err)
	channels[0].Normal.Multiplier = 6
	data := enc.ToData()
	s.Require().Len(data.Reserve, 1)
	s.Equal(float64(1), *data.Reserve[0].MaterialChannels[0].Normal.Multiplier)
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: appearanceTestCapabilities()})
	s.Require().NoError(err)
	*data.Reserve[0].MaterialChannels[0].Normal.Multiplier = 7
	s.Equal(float64(1), *loaded.ToData().Reserve[0].MaterialChannels[0].Normal.Multiplier)
	before := enc.ToData()
	_, err = enc.Join(&JoinInput{Member: "bad", Kind: KindPlayer, Cell: spatial.Position{X: 4, Y: 1},
		MaterialChannels: []MaterialChannelAdjustment{{TargetID: "empty"}}})
	s.Require().Error(err)
	s.Equal(before, enc.ToData())
}

func (s *MaterialChannelObservationSuite) TestUnknownAndMalformedTestimonyRefusesFacts() {
	_, err := EncodeSightTestimony(SightTestimony{State: LocationUnknown, MaterialChannels: materialChannelsFixture()})
	s.Error(err)
	for _, payload := range []string{
		`{"state":"unknown","material_channels":[]}`,
		`{"state":"known","x":1,"y":2,"material_channels":null}`,
		`{"state":"known","x":1,"y":2,"material_channels":[{"target_id":"body","normal":{"mode":"off","multiplier":0}}]}`,
	} {
		_, ok := DecodeSightTestimony([]byte(payload))
		s.False(ok, payload)
	}
	payload, err := EncodeSightTestimony(SightTestimony{State: LocationKnown, Position: spatial.Position{X: 1, Y: 2},
		MaterialChannels: materialChannelsFixture(), Conditions: &ConditionSet{}})
	s.Require().NoError(err)
	public, err := deliveredSightPayload(payload)
	s.Require().NoError(err)
	decoded, ok := DecodeSightTestimony(public)
	s.True(ok)
	s.Nil(decoded.Conditions)
	s.Equal(materialChannelsFixture(), decoded.MaterialChannels)
}
