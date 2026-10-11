// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

const appearanceMerchant = "dnd5e:npcs:kingdom:merchant-01"
const appearanceKing = "dnd5e:npcs:kingdom:king-01"

type AppearanceSuite struct{ suite.Suite }

func TestAppearanceSuite(t *testing.T) { suite.Run(t, new(AppearanceSuite)) }

func appearanceTestCapabilities() Capabilities {
	return Capabilities{
		Sight: everyoneSeesTheWholeMap{}, Equipment: UnobservedEquipment{},
		Sheets: zeroSheets{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Driver: passDriver{}, Actors: Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

func (s *AppearanceSuite) encounter() *Encounter {
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("appearance-room", 0, 0, 10, 10)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}, AppearanceRef: appearanceMerchant},
		},
		Endings:      []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: appearanceTestCapabilities(),
	})
	s.Require().NoError(err)
	return enc
}

func (s *AppearanceSuite) TestCaptureMemoryReloadAndReobservation() {
	enc := s.encounter()
	seen, current, held := heldBy(s.T(), enc, "alice", "bob")
	s.True(current && held)
	s.Equal(appearanceMerchant, seen.AppearanceRef)
	enc.sight = seesNobody{}
	_, err := enc.rebuildPercepts(enc.rosterIDs())
	s.Require().NoError(err)
	data := enc.ToData()
	for i := range data.Members {
		if data.Members[i].ID == "bob" {
			s.Equal(appearanceMerchant, data.Members[i].AppearanceRef)
			data.Members[i].AppearanceRef = appearanceKing
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
	s.Equal(appearanceMerchant, memory.AppearanceRef, "loading never enriches memory from changed live appearance")
	s.Equal(appearanceKing, loaded.members["bob"].AppearanceRef)
	_, err = loaded.rebuildPercepts(loaded.rosterIDs())
	s.Require().NoError(err)
	seen, current, _ = heldBy(s.T(), loaded, "alice", "bob")
	s.True(current)
	s.Equal(appearanceKing, seen.AppearanceRef)
}

func (s *AppearanceSuite) TestReserveAndJoinKeepAppearanceAndRefuseBadRefBeforeMutation() {
	enc := s.encounter()
	_, err := enc.Join(&JoinInput{Member: "waiting", Kind: KindMonster, Cell: spatial.Position{X: 3, Y: 1}, AppearanceRef: appearanceKing, Arrives: TriggerRound{Round: 99}})
	s.Require().NoError(err)
	data := enc.ToData()
	s.Require().Len(data.Reserve, 1)
	s.Equal(appearanceKing, data.Reserve[0].AppearanceRef)
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: appearanceTestCapabilities()})
	s.Require().NoError(err)
	s.Equal(appearanceKing, loaded.ToData().Reserve[0].AppearanceRef)
	before := enc.ToData()
	_, err = enc.Join(&JoinInput{Member: "bad", Kind: KindPlayer, Cell: spatial.Position{X: 4, Y: 1}, AppearanceRef: "not-a-ref"})
	s.Require().Error(err)
	s.Equal(before, enc.ToData())
	data.Members[0].AppearanceRef = "bad"
	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: appearanceTestCapabilities()})
	s.Error(err)
}

func (s *AppearanceSuite) TestStrictPayloadAndLegacyAbsence() {
	payload, err := EncodeSightTestimony(SightTestimony{State: LocationKnown, Position: spatial.Position{X: 1, Y: 2}, AppearanceRef: appearanceMerchant})
	s.Require().NoError(err)
	decoded, ok := DecodeSightTestimony(payload)
	s.True(ok)
	s.Equal(appearanceMerchant, decoded.AppearanceRef)
	decoded.Conditions = &ConditionSet{}
	private, err := EncodeSightTestimony(decoded)
	s.Require().NoError(err)
	public, err := deliveredSightPayload(private)
	s.Require().NoError(err)
	delivered, ok := DecodeSightTestimony(public)
	s.True(ok)
	s.Nil(delivered.Conditions)
	s.Equal(appearanceMerchant, delivered.AppearanceRef)
	for _, payload := range []string{`{"x":1,"y":2}`, `{"state":"known","x":1,"y":2}`, `{"state":"unknown"}`} {
		decoded, ok := DecodeSightTestimony([]byte(payload))
		s.True(ok)
		s.Empty(decoded.AppearanceRef)
	}
	for _, value := range []string{`null`, `""`, `"bad"`, `42`} {
		_, ok := DecodeSightTestimony([]byte(`{"state":"known","x":1,"y":2,"appearance_ref":` + value + `}`))
		s.False(ok, value)
	}
	_, err = EncodeSightTestimony(SightTestimony{State: LocationUnknown, AppearanceRef: appearanceMerchant})
	s.Error(err)
	_, ok = DecodeSightTestimony([]byte(`{"state":"unknown","appearance_ref":"` + appearanceMerchant + `"}`))
	s.False(ok)
}
