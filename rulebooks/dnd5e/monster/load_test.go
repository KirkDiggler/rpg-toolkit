// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type PureLoadTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *PureLoadTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// immunityBlob is a persisted trait. This package cannot route it to a loader
// — monstertraits imports this one — so it is carried as the bytes it is,
// which is exactly what the round trip below is about.
func (s *PureLoadTestSuite) immunityBlob() json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"ref":         refs.MonsterTraits.Immunity(),
		"monster_id":  "skel-load",
		"damage_type": "poison",
	})
	s.Require().NoError(err)

	return raw
}

func (s *PureLoadTestSuite) sheet() *Data {
	return &Data{
		ID:   "skel-load",
		Name: "Skeleton",
		Ref:  refs.Monsters.Skeleton(),
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 6,
			abilities.WIS: 8,
			abilities.CHA: 5,
		},
		HitPoints:        9,
		MaxHitPoints:     13,
		ArmorClass:       13,
		ProficiencyBonus: 2,
		Speed:            SpeedData{Walk: 30},
		Senses:           SensesData{Darkvision: 60, PassivePerception: 9},
		Actions:          []ActionData{},
		Proficiencies:    []ProficiencyData{{Skill: "stealth", Bonus: 4}},
		Conditions:       []json.RawMessage{s.immunityBlob()},
		Targeting:        TargetLowestHP,
	}
}

func (s *PureLoadTestSuite) marshal(d *Data) string {
	raw, err := json.Marshal(d)
	s.Require().NoError(err)

	return string(raw)
}

// Data in, the same data out, with no bus in the call — conditions included.
// Actions are the one thing this package cannot load (the action loader imports
// it); monstertraits.LoadMonster is the composition that does both, and its
// tests pin the round trip with actions in it.
func (s *PureLoadTestSuite) TestRoundTripsByteIdenticalWithNoBus() {
	data := s.sheet()

	m, err := Load(s.ctx, data)
	s.Require().NoError(err)

	s.Require().Equal(s.marshal(data), s.marshal(m.ToData()))
}

// The legacy path throws the conditions away. Its callers are expected to pass
// the same blobs to monstertraits.LoadMonsterConditions themselves, and a
// caller who forgets writes the monster back without them — the trap the
// carried blobs close.
func (s *PureLoadTestSuite) TestLegacyLoadDropsTheConditions() {
	m, err := LoadFromData(s.ctx, s.sheet(), events.NewEventBus())
	s.Require().NoError(err)

	s.Require().Empty(m.ToData().Conditions)
}

func (s *PureLoadTestSuite) TestLoadAppliesNothing() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	s.Require().Nil(m.bus, "a pure load holds no bus")
	s.Require().Empty(m.subscriptionIDs, "a pure load subscribes to nothing")
	s.Require().Empty(m.GetConditions(), "nothing is applied until a trait loader runs")
}

// The blobs are taken, not read: whoever loads them into behaviour clears them,
// so ToData cannot write the same condition twice.
func (s *PureLoadTestSuite) TestTakeUnappliedConditionsDrains() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	s.Require().Equal([]json.RawMessage{s.immunityBlob()}, m.TakeUnappliedConditions())
	s.Require().Empty(m.TakeUnappliedConditions())
	s.Require().Empty(m.ToData().Conditions)
}

// Strict as far as it can be without a loader: a blob that is not JSON fails
// the load, and the error names it.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAMalformedCondition() {
	data := s.sheet()
	data.Conditions = append(data.Conditions, json.RawMessage(`{"ref":`))

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), `{"ref":`)
}

// A blob with no ref could never be routed to a loader, so accepting it would
// only defer the failure to a place with less to say about it.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAConditionWithNoRef() {
	data := s.sheet()
	data.Conditions = append(data.Conditions, json.RawMessage(`{"monster_id":"skel-load"}`))

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "names no ref")
}

func (s *PureLoadTestSuite) TestLoadRejectsNilData() {
	_, err := Load(s.ctx, nil)

	s.Require().Error(err)
}

// Two fields of Data survive no loader: a monster has nowhere to hold features
// or inventory and ToData does not write them. Pinned so the round-trip
// guarantee above is read with its actual scope.
func (s *PureLoadTestSuite) TestKnownRoundTripGaps() {
	data := s.sheet()
	data.Features = []json.RawMessage{json.RawMessage(`{"ref":"whatever"}`)}
	data.Inventory = []InventoryItemData{{ID: "potion", Name: "Potion", Quantity: 1}}

	m, err := Load(s.ctx, data)
	s.Require().NoError(err)

	out := m.ToData()
	s.Require().Empty(out.Features, "Features has no home on a monster")
	s.Require().Empty(out.Inventory, "Inventory has no home on a monster")
}

// MonsterKeeperTestSuite drives the two things a monster sheet does about the
// world. Both fail for a keeper whose Apply subscribes nothing.
type MonsterKeeperTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
	mon *Monster
}

func (s *MonsterKeeperTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	m, err := Load(s.ctx, &Data{
		ID:           "skel-keeper",
		Name:         "Skeleton",
		HitPoints:    10,
		MaxHitPoints: 13,
		ArmorClass:   13,
	})
	s.Require().NoError(err)
	s.Require().NoError(m.SheetKeeper().Apply(s.ctx, s.bus))

	s.mon = m
}

func (s *MonsterKeeperTestSuite) TestDamageReceivedMovesHitPoints() {
	err := dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: s.mon.GetID(),
		Amount:   4,
	})

	s.Require().NoError(err)
	s.Require().Equal(6, s.mon.HP())
	s.Require().True(s.mon.IsDirty(), "a monster that took damage needs saving")
}

func (s *MonsterKeeperTestSuite) TestHealingReceivedMovesHitPoints() {
	err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.mon.GetID(),
		Amount:   2,
	})

	s.Require().NoError(err)
	s.Require().Equal(12, s.mon.HP())
	s.Require().True(s.mon.IsDirty())
}

func (s *MonsterKeeperTestSuite) TestEventsForOtherMonstersAreIgnored() {
	err := dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: "someone-else",
		Amount:   4,
	})

	s.Require().NoError(err)
	s.Require().Equal(10, s.mon.HP())
	s.Require().False(s.mon.IsDirty())
}

func (s *MonsterKeeperTestSuite) TestRemoveStopsTheMonsterListening() {
	s.Require().NoError(s.mon.SheetKeeper().Remove(s.ctx, s.bus))

	err := dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: s.mon.GetID(),
		Amount:   4,
	})

	s.Require().NoError(err)
	s.Require().Equal(10, s.mon.HP())
	s.Require().Empty(s.mon.subscriptionIDs)
}

func (s *MonsterKeeperTestSuite) TestApplyingTwiceIsRefused() {
	s.Require().Error(s.mon.SheetKeeper().Apply(s.ctx, s.bus))
}

func TestPureLoadSuite(t *testing.T) {
	suite.Run(t, new(PureLoadTestSuite))
}

func TestMonsterKeeperSuite(t *testing.T) {
	suite.Run(t, new(MonsterKeeperTestSuite))
}
