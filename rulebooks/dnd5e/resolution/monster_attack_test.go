// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsterActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type MonsterAttackTestSuite struct {
	suite.Suite
}

func TestMonsterAttackSuite(t *testing.T) {
	suite.Run(t, new(MonsterAttackTestSuite))
}

func (s *MonsterAttackTestSuite) action(ref *core.Ref, config any) monster.ActionData {
	s.T().Helper()

	raw, err := json.Marshal(config)
	s.Require().NoError(err)

	s.Require().NotNil(ref)

	return monster.ActionData{Ref: *ref, Config: raw}
}

func (s *MonsterAttackTestSuite) TestMeleePreservesCanonicalPoolsAndIntrinsicBonuses() {
	cases := []struct {
		name      string
		flatBonus int
	}{
		{name: "positive", flatBonus: 2},
		{name: "zero", flatBonus: 0},
		{name: "negative", flatBonus: -1},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			want := []damage.Damage{
				{Dice: "1d6", Type: damage.Slashing, FlatBonus: tc.flatBonus},
				{Dice: "1d4", Type: damage.Fire, Properties: []damage.Property{damage.DoesNotCrit}},
			}
			action := s.action(refs.MonsterActions.Melee(), monsterActions.MeleeConfig{
				Name:        "enchanted scimitar",
				AttackBonus: 4,
				Damage:      want,
				Reach:       1,
			})

			profile, err := AttackFromMonsterAction(action)
			s.Require().NoError(err)
			s.Equal(refs.MonsterActions.Melee(), profile.Ref)
			s.Equal(4, profile.AttackBonus)
			s.Equal(want, profile.Damage)
			s.Empty(profile.AbilityUsed)
			s.Zero(profile.AbilityModifier)
			s.Nil(profile.Gate)
			s.Nil(profile.Imposes)
		})
	}
}

func (s *MonsterAttackTestSuite) TestBitePreservesCanonicalDamageAndGate() {
	gate := saves.NewSaveGate(abilities.STR, 11)
	want := []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}}
	action := s.action(refs.MonsterActions.Bite(), monsterActions.BiteConfig{
		AttackBonus: 4,
		Damage:      want,
		SaveGate:    gate,
	})

	profile, err := AttackFromMonsterAction(action)
	s.Require().NoError(err)
	s.Equal(want, profile.Damage)
	s.Equal(gate, profile.Gate)
	s.Require().NotNil(profile.Imposes)
	s.Equal(refs.Conditions.Prone(), profile.Imposes.atStake().Ref)
}

func (s *MonsterAttackTestSuite) TestGoblinScimitarCompilesFromGenericMeleeContent() {
	goblin := monsters.NewGoblin("goblin-1").ToData()
	s.Require().Len(goblin.Actions, 1)
	s.Equal(refs.MonsterActions.Melee(), &goblin.Actions[0].Ref)

	profile, err := AttackFromMonsterAction(goblin.Actions[0])
	s.Require().NoError(err)
	s.Equal(4, profile.AttackBonus)
	s.Equal([]damage.Damage{{Dice: "1d6", Type: damage.Slashing, FlatBonus: 2}}, profile.Damage)
	s.Empty(profile.AbilityUsed)
	s.Zero(profile.AbilityModifier)
}

func (s *MonsterAttackTestSuite) TestCanonicalDamageArrayIsRequired() {
	for _, tc := range []struct {
		name   string
		config json.RawMessage
	}{
		{name: "missing", config: json.RawMessage(`{"name":"club","attack_bonus":2,"reach":1}`)},
		{name: "empty", config: json.RawMessage(`{"name":"club","attack_bonus":2,"damage":[],"reach":1}`)},
		{name: "malformed", config: json.RawMessage(`{"name":"club","attack_bonus":2,"damage":[{"dice":"1d6+2","type":"bludgeoning"}],"reach":1}`)},
	} {
		s.Run(tc.name, func() {
			_, err := AttackFromMonsterAction(monster.ActionData{
				Ref:    *refs.MonsterActions.Melee(),
				Config: tc.config,
			})
			s.Require().ErrorIs(err, ErrBadAttack)
		})
	}
}

func (s *MonsterAttackTestSuite) TestLegacyDamageFieldsAreRejectedRatherThanTranslated() {
	_, err := AttackFromMonsterAction(monster.ActionData{
		Ref: *refs.MonsterActions.Melee(),
		Config: json.RawMessage(
			`{"name":"club","attack_bonus":2,"damage_dice":"1d6+2","damage_type":"bludgeoning","reach":1}`),
	})

	s.Require().ErrorIs(err, ErrBadAttack)
}

func (s *MonsterAttackTestSuite) TestRangedActionRemainsRefused() {
	action := s.action(refs.MonsterActions.Ranged(), monsterActions.RangedConfig{
		Name:        "shortbow",
		AttackBonus: 4,
		Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing, FlatBonus: 2}},
		RangeNormal: 16,
		RangeLong:   64,
	})

	_, err := AttackFromMonsterAction(action)
	s.Require().ErrorIs(err, ErrBadAttack)
}
