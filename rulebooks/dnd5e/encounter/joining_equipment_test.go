// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// joining_equipment_test.go is A CREATURE THAT JOINS BRINGS WHAT IT HAS
// EQUIPPED (rpg-project#850, rpg-project#1171): the session starts a run by
// joining every placed creature, so equipment that only [MemberInput] could
// carry never reached a real run. Every scene here builds the encounter with
// the players and the sentry, and adds the captain with Join.

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// joinInput is the captain, joining with gear.
func joinInput(gear ...encounter.ItemGrant) *encounter.JoinInput {
	return &encounter.JoinInput{
		Member: captain, Kind: encounter.KindMonster, Cell: captainCell, Equipment: gear,
	}
}

// openThenJoin is the tomb without the captain, then the captain joins.
func (s *HoldingsSuite) openThenJoin(in *encounter.JoinInput) *encounter.Encounter {
	members := []encounter.MemberInput{
		{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
		{ID: partner, Kind: encounter.KindPlayer, Position: partnerCell},
		{ID: sentry, Kind: encounter.KindMonster, Position: sentryCell},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        heirloomField(),
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention:    encounter.RetentionUnbounded,
		Capabilities: s.capabilities(s.standing),
	})
	s.Require().NoError(err)
	_, err = enc.Join(in)
	s.Require().NoError(err)
	return enc
}

func (s *HoldingsSuite) savedAndLoaded(enc *encounter.Encounter) *encounter.Encounter {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities(s.standing)})
	s.Require().NoError(err)
	return loaded
}

// Test 1: a joined creature's equipment is saved, byte for byte, and loads.
func (s *HoldingsSuite) TestAJoinedCreatureKeepsItsEquipmentThroughASave() {
	enc := s.openThenJoin(joinInput(cloakGrant))
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(enc))
	s.Require().Equal(cloakJSON, string(s.bodyGear(enc)[0].Item), "the block, byte for byte")
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(s.savedAndLoaded(enc)))
}

// Test 2: a joined creature's body gives its equipment to the looter alone.
func (s *HoldingsSuite) TestLootingAJoinedCreatureGivesItsEquipment() {
	enc := s.openThenJoin(joinInput(cloakGrant))
	out := s.lootCaptain(enc)

	s.Require().Len(out.Items, 1)
	s.Require().Equal(raider, out.Items[0].Member)
	s.Require().Equal("lieutenants-cloak", out.Items[0].Grant.ID)
	s.Require().Equal(cloakJSON, string(out.Items[0].Grant.Item))
	s.Require().Len(s.beatsOfKind(enc, raider, encounter.BeatTaken), 1)
	for _, other := range []encounter.MemberID{partner, captain, sentry} {
		s.Require().Empty(s.beatsOfKind(enc, other, encounter.BeatTaken), "%s is not told", other)
	}
}

// Test 3: a creature that waits to arrive keeps its equipment while it waits,
// through a save, and when it arrives.
func (s *HoldingsSuite) TestAReservedJoinKeepsItsEquipmentUntilItArrives() {
	in := joinInput(cloakGrant)
	in.Arrives = encounter.TriggerMemberDown{Member: sentry}
	enc := s.openThenJoin(in)

	reserved := func(e *encounter.Encounter) []encounter.ItemGrant {
		data := e.ToData()
		s.Require().Len(data.Reserve, 1)
		s.Require().Equal(captain, data.Reserve[0].ID)
		return data.Reserve[0].Equipment
	}
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, reserved(enc), "while it waits")
	loaded := s.savedAndLoaded(enc)
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, reserved(loaded), "after a save")

	for _, e := range []*encounter.Encounter{enc, loaded} {
		s.standing.down = []encounter.MemberID{sentry}
		_, err := aRound(e)
		s.Require().NoError(err)
		s.Require().Empty(e.ToData().Reserve, "it has arrived")
		s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(e), "and has the cloak")

		s.standing.down = []encounter.MemberID{sentry, captain}
		out := s.lootCaptain(e)
		s.Require().Len(out.Items, 1)
		s.Require().Equal("lieutenants-cloak", out.Items[0].Grant.ID)
		s.standing.down = nil
	}
}

// Test 4: Join refuses what construction refuses, with the error a bad
// joining member already gets, and nothing joins.
func (s *HoldingsSuite) TestJoinRefusesBadEquipment() {
	cases := map[string][]encounter.ItemGrant{
		"a grant with no id":            {{Name: "x", Item: json.RawMessage(`{"a":1}`)}},
		"a block that is not an object": {{ID: "x", Item: json.RawMessage(`[1]`)}},
		"one id twice":                  {cloakGrant, cloakGrant},
	}
	for name, gear := range cases {
		s.Run(name, func() {
			enc := s.openThenJoin(&encounter.JoinInput{
				Member: "other", Kind: encounter.KindMonster, Cell: sentryCell,
			})
			before := jsonOf(s, enc.ToData())
			_, err := enc.Join(joinInput(gear...))
			s.Require().ErrorIs(err, encounter.ErrBadMember)
			s.Require().Contains(err.Error(), "join:")
			s.Require().Equal(before, jsonOf(s, enc.ToData()), "nothing joined")
		})
	}
}

// Test 5: the caller's slice is not kept, for a member and for a reserve.
func (s *HoldingsSuite) TestJoinKeepsACopyOfTheEquipment() {
	fresh := func() []encounter.ItemGrant {
		return []encounter.ItemGrant{{
			ID: "lieutenants-cloak", Name: "Lieutenant's cloak", Item: json.RawMessage(cloakJSON),
		}}
	}
	scribble := func(gear []encounter.ItemGrant) {
		gear[0].Item[1] = 'Y'
		gear[0].ID = "changed"
	}

	s.Run("a member", func() {
		gear := fresh()
		enc := s.openThenJoin(joinInput(gear...))
		scribble(gear)
		s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(enc))
	})
	s.Run("a reserve", func() {
		gear := fresh()
		in := joinInput(gear...)
		in.Arrives = encounter.TriggerMemberDown{Member: sentry}
		enc := s.openThenJoin(in)
		scribble(gear)
		s.Require().Equal([]encounter.ItemGrant{cloakGrant}, enc.ToData().Reserve[0].Equipment)
	})
}

// Test 6: a join with no equipment saves no equipment key.
func (s *HoldingsSuite) TestAJoinWithNoEquipmentSavesNoKey() {
	enc := s.openThenJoin(joinInput())
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	s.Require().NotContains(string(raw), `"equipment"`)
}
