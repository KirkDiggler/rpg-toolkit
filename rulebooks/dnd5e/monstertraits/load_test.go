// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// MonsterCompositionTestSuite covers the pair of calls that replace the
// three-call assembly: one pure load, one attach.
type MonsterCompositionTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *MonsterCompositionTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MonsterCompositionTestSuite) immunityBlob() json.RawMessage {
	raw, err := json.Marshal(ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    "gob-compose",
		DamageType: damage.Poison,
	})
	s.Require().NoError(err)

	return raw
}

// goblinData supplies a canonical generic-melee goblin fixture without
// importing the monster factory, which would create a test import cycle.
func goblinData(id string) *monster.Data {
	return &monster.Data{
		ID:               id,
		Name:             "Goblin",
		Ref:              refs.Monsters.Goblin(),
		HitPoints:        7,
		MaxHitPoints:     7,
		ArmorClass:       15,
		ProficiencyBonus: 2,
		Actions: []monster.ActionData{{
			Ref:    *refs.MonsterActions.Melee(),
			Config: json.RawMessage(`{"name":"scimitar","attack_bonus":4,"damage":[{"dice":"1d6","type":"slashing","flat_bonus":2}],"reach":1}`),
		}},
	}
}

func (s *MonsterCompositionTestSuite) goblin() *monster.Data {
	data := goblinData("gob-compose")
	data.Conditions = []json.RawMessage{s.immunityBlob()}

	return data
}

func (s *MonsterCompositionTestSuite) marshal(d *monster.Data) string {
	raw, err := json.Marshal(d)
	s.Require().NoError(err)

	return string(raw)
}

// The whole monster, loaded with no bus and written back byte for byte —
// actions and conditions included. This is what the three-call assembly could
// never promise: two of its three calls produce a monster that serializes
// without the third one's contents, and says nothing about it.
func (s *MonsterCompositionTestSuite) TestRoundTripsByteIdenticalWithNoBus() {
	data := s.goblin()

	m, err := LoadMonster(s.ctx, data)
	s.Require().NoError(err)

	s.Require().Equal(s.marshal(data), s.marshal(m.ToData()))
}

// Actions are the half monster.Load cannot do on its own.
func (s *MonsterCompositionTestSuite) TestLoadMonsterLoadsTheActions() {
	m, err := LoadMonster(s.ctx, s.goblin())
	s.Require().NoError(err)

	s.Require().NotEmpty(m.Actions())
}

// Attach turns the carried blobs into behaviour, and the monster still writes
// back the same bytes: the trait moved from unapplied to applied without being
// counted twice or re-serialized differently.
func (s *MonsterCompositionTestSuite) TestAttachAppliesTheTraitsItCarried() {
	data := s.goblin()

	m, err := LoadMonster(s.ctx, data)
	s.Require().NoError(err)
	s.Require().NoError(AttachMonster(s.ctx, m, events.NewEventBus(), nil))

	s.Require().Len(m.GetConditions(), 1)
	s.Require().Equal(s.marshal(data), s.marshal(m.ToData()))
}

// Attribution, pinned: each trait goes on through the bus scoped to its own
// ref, and the monster's own hooks stay unattributed.
func (s *MonsterCompositionTestSuite) TestAttachScopesEachTraitToItsRef() {
	bus := newRecordingBus()

	m, err := LoadMonster(s.ctx, s.goblin())
	s.Require().NoError(err)
	s.Require().NoError(AttachMonster(s.ctx, m, bus, nil))

	s.Require().Equal([]core.Ref{*refs.MonsterTraits.Immunity()}, bus.record.asked)
	s.Require().NotEmpty(bus.record.byRef[*refs.MonsterTraits.Immunity()])
	s.Require().NotEmpty(bus.record.byRef[unattributed], "the monster's own hooks are its own")
}

// The sheet keeper is part of the attach: a monster that was attached hears
// damage.
func (s *MonsterCompositionTestSuite) TestAttachPutsTheMonsterOnTheBus() {
	bus := events.NewEventBus()

	m, err := LoadMonster(s.ctx, s.goblin())
	s.Require().NoError(err)
	s.Require().NoError(AttachMonster(s.ctx, m, bus, nil))

	err = dnd5eEvents.DamageReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: m.GetID(),
		Amount:   3,
	})

	s.Require().NoError(err)
	s.Require().Equal(4, m.HP(), "a goblin has 7 HP and took 3")
	s.Require().True(m.IsDirty())
}

// A trait ref this build cannot route fails the attach rather than being
// dropped — the strictness LoadMonsterConditions already had, kept.
func (s *MonsterCompositionTestSuite) TestAttachRefusesAnUnknownTrait() {
	data := s.goblin()
	data.Conditions = append(data.Conditions,
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"nope"}}`))

	m, err := LoadMonster(s.ctx, data)
	s.Require().NoError(err)

	s.Require().Error(AttachMonster(s.ctx, m, events.NewEventBus(), nil))
}

func (s *MonsterCompositionTestSuite) TestAttachRejectsMissingArguments() {
	m, err := LoadMonster(s.ctx, s.goblin())
	s.Require().NoError(err)

	s.Require().Error(AttachMonster(s.ctx, m, nil, nil))
	s.Require().Error(AttachMonster(s.ctx, nil, events.NewEventBus(), nil))
}

func TestMonsterCompositionSuite(t *testing.T) {
	suite.Run(t, new(MonsterCompositionTestSuite))
}
