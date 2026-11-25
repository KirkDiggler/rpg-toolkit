// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// looting_equipment_test.go is LOOTING A BODY GIVES ITS EQUIPMENT
// (rpg-project#850, rpg-project#1167): looting a downed creature takes the
// custom items it had equipped off the body, tells the looter alone what they
// took, and reports to the host which items to hand to whom. The law it must
// not break is loot.go's: the `looted` beat is byte for byte the same whether
// the body carried anything or nothing.
//
// The fixture is the holdings suite's tomb: RAIDER and PARTNER are players, the
// CAPTAIN is the body and the SENTRY is a second monster.

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

var (
	cloakGrant = encounter.ItemGrant{
		ID: "lieutenants-cloak", Name: "Lieutenant's cloak", Item: json.RawMessage(cloakJSON),
	}
	ringGrant = encounter.ItemGrant{
		ID: "a-ring", Name: "Ring of nothing", Item: json.RawMessage(`{"name":"Ring of nothing","slot":"ring"}`),
	}
)

func (s *HoldingsSuite) capabilities(standing encounter.Participation) encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: standing,
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		CheckResolver: findsNothing{},
		Witness:       s.witness,
		Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

// openGeared is the tomb with the captain wearing gear, holding the vault map
// when holds is true, and a sentry beside them.
func (s *HoldingsSuite) openGeared(holds bool, gear ...encounter.ItemGrant) *encounter.Encounter {
	members := s.cast(holds)
	for i := range members {
		if members[i].ID == captain {
			members[i].Equipment = gear
		}
	}
	members = append(members, encounter.MemberInput{ID: sentry, Kind: encounter.KindMonster, Position: sentryCell})
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        heirloomField(),
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention:    encounter.RetentionUnbounded,
		Capabilities: s.capabilities(s.standing),
	})
	s.Require().NoError(err)
	return enc
}

// lootCaptain downs the captain, brings the raider beside them and loots.
func (s *HoldingsSuite) lootCaptain(enc *encounter.Encounter) *encounter.LootOutput {
	s.drop(enc)
	s.walkTo(enc, raider, captainCell)
	out, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)
	return out
}

// bodyGear is what the saved record says the captain still has equipped.
func (s *HoldingsSuite) bodyGear(enc *encounter.Encounter) []encounter.ItemGrant {
	for _, m := range enc.ToData().Members {
		if m.ID == captain {
			return m.Equipment
		}
	}
	s.Require().Fail("the captain is not a member")
	return nil
}

func (s *HoldingsSuite) kinds(enc *encounter.Encounter, member encounter.MemberID) []string {
	var out []string
	for _, b := range s.beats(enc, member) {
		out = append(out, b["beat"].(string))
	}
	return out
}

func indexOf(kinds []string, kind string) int {
	for i, k := range kinds {
		if k == kind {
			return i
		}
	}
	return -1
}

func jsonOf(s *HoldingsSuite, v any) string {
	raw, err := json.Marshal(v)
	s.Require().NoError(err)
	return string(raw)
}

// Test 1: the site compiler carries the item's name beside its block.
func (s *HoldingsSuite) TestACompiledGrantCarriesTheItemsName() {
	c := cloakCompiled(s.T())
	s.Require().Len(c.Monsters, 1)
	s.Require().Len(c.Monsters[0].Equipment, 1)
	s.Require().Equal("Lieutenant's cloak", c.Monsters[0].Equipment[0].Name)
}

// Test 2: the host is told, and the body is bare, saved and loaded.
func (s *HoldingsSuite) TestLootingTakesTheEquipmentOffTheBodyAndTellsTheHost() {
	enc := s.openGeared(false, cloakGrant)
	out := s.lootCaptain(enc)

	s.Require().Len(out.Items, 1)
	s.Require().Equal(raider, out.Items[0].Member)
	s.Require().Equal("lieutenants-cloak", out.Items[0].Grant.ID)
	s.Require().Equal("Lieutenant's cloak", out.Items[0].Grant.Name)
	s.Require().Equal(cloakJSON, string(out.Items[0].Grant.Item), "the block, byte for byte")

	s.Run("the grant is a copy no later write reaches", func() {
		out.Items[0].Grant.Item[0] = 'X'
		s.Require().Empty(s.bodyGear(enc))
		s.Require().Equal(cloakJSON, string(cloakGrant.Item))
	})

	s.Run("the body is bare, and stays so through ToData, JSON and Load", func() {
		s.Require().Empty(s.bodyGear(enc))
		raw, err := json.Marshal(enc.ToData())
		s.Require().NoError(err)
		var data encounter.EncounterData
		s.Require().NoError(json.Unmarshal(raw, &data))
		loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities(s.standing)})
		s.Require().NoError(err)
		s.Require().Empty(s.bodyGear(loaded))
	})
}

// Test 3: the looter alone is told; everyone sees the same looted beat.
func (s *HoldingsSuite) TestOnlyTheLooterIsToldWhatTheyTook() {
	enc := s.openGeared(false, cloakGrant)
	s.lootCaptain(enc)

	taken := s.beatsOfKind(enc, raider, encounter.BeatTaken)
	s.Require().Len(taken, 1)
	s.Require().Equal(map[string]any{
		"beat": "taken", "member": string(raider), "item": "lieutenants-cloak",
		"name": "Lieutenant's cloak", "from": string(captain),
	}, taken[0])
	for _, other := range []encounter.MemberID{partner, captain, sentry} {
		s.Require().Empty(s.beatsOfKind(enc, other, encounter.BeatTaken), "%s is not told", other)
	}
	s.Require().Equal(
		jsonOf(s, s.beatsOfKind(enc, partner, encounter.BeatLooted)),
		jsonOf(s, s.beatsOfKind(enc, raider, encounter.BeatLooted)),
		"both see the same looted beat")
}

// Test 4: THE LAW. A body with the cloak and a body with nothing give the same
// looted beat, and the bystander's whole story is the same bytes.
func (s *HoldingsSuite) TestTheLootedBeatIsTheSameWithGearOrWithout() {
	loot := func(gear ...encounter.ItemGrant) (looted, bystander string) {
		enc := s.openGeared(false, gear...)
		s.lootCaptain(enc)
		beats := s.beatsOfKind(enc, raider, encounter.BeatLooted)
		s.Require().Len(beats, 1)
		return jsonOf(s, beats), s.storyBytes(enc, partner)
	}
	geared, gearedStory := loot(cloakGrant)
	s.SetupTest()
	bare, bareStory := loot()

	s.Require().Equal(bare, geared, "the looted beat names nothing of what moved")
	s.Require().Equal(bareStory, gearedStory, "and the bystander's whole story is byte-identical")
}

// Test 5: a second loot of the same body gives nothing.
func (s *HoldingsSuite) TestASecondLootGivesNothing() {
	enc := s.openGeared(false, cloakGrant)
	s.lootCaptain(enc)
	first := s.beatsOfKind(enc, raider, encounter.BeatLooted)

	out, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
	s.Require().NoError(err)
	s.Require().Nil(out.Items)
	s.Require().Len(s.beatsOfKind(enc, raider, encounter.BeatTaken), 1, "no second taken beat")
	looted := s.beatsOfKind(enc, raider, encounter.BeatLooted)
	s.Require().Len(looted, 2)
	s.Require().Equal(jsonOf(s, first[0]), jsonOf(s, looted[1]), "the same looted beat")
}

// Test 6: two items, two entries and two beats, in item id order.
func (s *HoldingsSuite) TestTwoItemsComeInIDOrder() {
	enc := s.openGeared(false, cloakGrant, ringGrant)
	out := s.lootCaptain(enc)

	s.Require().Len(out.Items, 2)
	s.Require().Equal("a-ring", out.Items[0].Grant.ID)
	s.Require().Equal("lieutenants-cloak", out.Items[1].Grant.ID)
	taken := s.beatsOfKind(enc, raider, encounter.BeatTaken)
	s.Require().Len(taken, 2)
	s.Require().Equal("a-ring", taken[0]["item"])
	s.Require().Equal("lieutenants-cloak", taken[1]["item"])
	s.Require().Empty(s.bodyGear(enc))
}

// Test 7: a looter that is not a player takes no equipment.
func (s *HoldingsSuite) TestAMonsterLooterTakesNoEquipment() {
	enc := s.openGeared(false, cloakGrant)
	s.drop(enc)
	out, err := enc.Loot(&encounter.LootInput{Member: sentry, Target: captain, Range: 50})
	s.Require().NoError(err)
	s.Require().Nil(out.Items)
	for _, member := range []encounter.MemberID{raider, partner, sentry} {
		s.Require().Empty(s.beatsOfKind(enc, member, encounter.BeatTaken))
	}
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(enc), "it stays on the body")

	s.Run("and a player may still take it afterwards", func() {
		s.walkTo(enc, raider, captainCell)
		out, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().NoError(err)
		s.Require().Len(out.Items, 1)
	})
}

// Test 8: a body with intel and equipment gives both, in this order:
// looted, what the intel reveals, taken.
func (s *HoldingsSuite) TestIntelAndEquipmentBothComeAndInThisOrder() {
	enc := s.openGeared(true, cloakGrant)
	out := s.lootCaptain(enc)
	s.Require().Len(out.Items, 1)

	kinds := s.kinds(enc, raider)
	looted := indexOf(kinds, encounter.BeatLooted)
	revealed := indexOf(kinds, encounter.BeatConcealmentRevealed)
	taken := indexOf(kinds, encounter.BeatTaken)
	s.Require().GreaterOrEqual(looted, 0)
	s.Require().Greater(revealed, looted, "the intel is revealed after looted, as today")
	s.Require().Greater(taken, revealed, "and taken comes after what the intel reveals")
	doors, err := enc.DoorsFor(raider)
	s.Require().NoError(err)
	s.Require().True(doorsListed(doors, tombVault), "the intel is revealed as before")
}

// Test 9, three refusals: each leaves the equipment on the body.
func (s *HoldingsSuite) TestARefusedLootLeavesTheEquipmentOnTheBody() {
	enc := s.openGeared(false, cloakGrant)
	want := []encounter.ItemGrant{cloakGrant}

	s.Run("not down", func() {
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain, Range: 50})
		s.Require().ErrorIs(err, encounter.ErrNotDown)
		s.Require().Equal(want, s.bodyGear(enc))
	})
	s.Run("out of range", func() {
		s.drop(enc)
		_, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
		s.Require().Equal(want, s.bodyGear(enc))
	})
	s.Run("closed", func() {
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Loot(&encounter.LootInput{Member: raider, Target: captain, Range: 50})
		s.Require().ErrorIs(err, encounter.ErrClosed)
		s.Require().Equal(want, s.bodyGear(enc))
	})
}

// Test 9, the turn clock: a member whose turn it is not takes nothing.
func (s *HoldingsSuite) TestAnOffTurnLootLeavesTheEquipmentOnTheBody() {
	fight := &downList{}
	members := []encounter.MemberInput{
		{ID: raider, Kind: encounter.KindPlayer, Position: heirloomCell},
		{ID: partner, Kind: encounter.KindPlayer, Position: chaliceCell},
		{ID: captain, Kind: encounter.KindMonster, Position: captainCell, Equipment: []encounter.ItemGrant{cloakGrant}},
		{ID: sentry, Kind: encounter.KindMonster, Position: sentryCell},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: heirloomField(), Members: members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention:    encounter.RetentionUnbounded,
		Capabilities: s.capabilities(fight),
	})
	s.Require().NoError(err)
	fight.down = []encounter.MemberID{captain}
	_, err = aRound(enc)
	s.Require().NoError(err)

	now, err := enc.ClockOf(&encounter.ClockOfInput{Member: raider})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, now.Kind)
	waiting := raider
	if now.Active == raider {
		waiting = partner
	}
	_, err = enc.Loot(&encounter.LootInput{Member: waiting, Target: captain, Range: 9})
	s.Require().ErrorIs(err, encounter.ErrNotActive)
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(enc))
}
