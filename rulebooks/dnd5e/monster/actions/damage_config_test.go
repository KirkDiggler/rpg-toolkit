// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type DamageConfigTestSuite struct {
	suite.Suite
}

func TestDamageConfigSuite(t *testing.T) {
	suite.Run(t, new(DamageConfigTestSuite))
}

func (s *DamageConfigTestSuite) load(ref *core.Ref, config map[string]any) (monster.MonsterAction, error) {
	raw, err := json.Marshal(config)
	s.Require().NoError(err)

	return LoadAction(monster.ActionData{Ref: *ref, Config: raw})
}

func (s *DamageConfigTestSuite) written(action monster.MonsterAction) map[string]any {
	var config map[string]any
	s.Require().NoError(json.Unmarshal(action.ToData().Config, &config))
	return config
}

func (s *DamageConfigTestSuite) TestMeleeDamageConfigRoundTripsOnlyCanonicalPools() {
	action, err := s.load(refs.MonsterActions.Melee(), map[string]any{
		"name":         "bite",
		"attack_bonus": 4,
		"reach":        1,
		"damage": []damage.Damage{
			{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2},
			{Dice: "1d6", Type: damage.Acid, FlatBonus: -1},
			{Dice: "1d4", Type: damage.Poison},
		},
	})
	s.Require().NoError(err)

	written := s.written(action)
	s.Contains(written, "damage")
	s.NotContains(written, "damage_dice")
	s.NotContains(written, "damage_type")
	s.NotContains(written, "damage_bonus")

	pools, ok := written["damage"].([]any)
	s.Require().True(ok)
	s.Require().Len(pools, 3)
	s.Equal(map[string]any{"dice": "2d4", "flat_bonus": float64(2), "type": "piercing"}, pools[0])
	s.Equal(map[string]any{"dice": "1d6", "flat_bonus": float64(-1), "type": "acid"}, pools[1])
	s.Equal(map[string]any{"dice": "1d4", "type": "poison"}, pools[2])
}

func (s *DamageConfigTestSuite) TestMeleeDamageConfigRejectsMissingEmptyAndInvalidPools() {
	for _, test := range []struct {
		name   string
		damage any
	}{
		{name: "missing"},
		{name: "empty", damage: []damage.Damage{}},
		{name: "malformed dice", damage: []damage.Damage{{Dice: "1d8+2", Type: damage.Slashing}}},
		{name: "unknown type", damage: []damage.Damage{{Dice: "1d8", Type: damage.Type("purple")}}},
	} {
		s.Run(test.name, func() {
			config := map[string]any{"name": "claw", "attack_bonus": 4, "reach": 1}
			if test.damage != nil {
				config["damage"] = test.damage
			}

			_, err := s.load(refs.MonsterActions.Melee(), config)
			s.Error(err)
		})
	}
}

func (s *DamageConfigTestSuite) TestActionDamageConfigRejectsLegacyFieldsAlongsideCanonicalPools() {
	for _, test := range []struct {
		name   string
		ref    *core.Ref
		config map[string]any
	}{
		{
			name: "melee",
			ref:  refs.MonsterActions.Melee(),
			config: map[string]any{
				"name": "claw", "attack_bonus": 4, "reach": 1,
				"damage":      []damage.Damage{{Dice: "1d6", Type: damage.Slashing}},
				"damage_dice": "1d6",
			},
		},
		{
			name: "bite",
			ref:  refs.MonsterActions.Bite(),
			config: map[string]any{
				"attack_bonus": 4,
				"damage":       []damage.Damage{{Dice: "2d4", Type: damage.Piercing}},
				"knockdown_dc": 11,
			},
		},
		{
			name: "ranged",
			ref:  refs.MonsterActions.Ranged(),
			config: map[string]any{
				"name": "shortbow", "attack_bonus": 4, "range_normal": 16, "range_long": 64,
				"damage":      []damage.Damage{{Dice: "1d6", Type: damage.Piercing}},
				"damage_type": damage.Piercing,
			},
		},
	} {
		s.Run(test.name, func() {
			_, err := s.load(test.ref, test.config)
			s.Error(err)
		})
	}
}

func (s *DamageConfigTestSuite) TestActionDamageConfigRejectsTrailingJSONValues() {
	_, err := LoadAction(monster.ActionData{
		Ref:    *refs.MonsterActions.Melee(),
		Config: []byte(`{"name":"claw","attack_bonus":4,"reach":1,"damage":[{"dice":"1d6","type":"slashing"}]} {}`),
	})
	s.Error(err)
}

func (s *DamageConfigTestSuite) TestMeleeActionDefensivelyCopiesDamageAndProperties() {
	pools := []damage.Damage{{
		Dice:       "1d8",
		Type:       damage.Slashing,
		Properties: []damage.Property{damage.DoesNotCrit},
	}}
	action, err := NewMeleeAction(MeleeConfig{Name: "sword", AttackBonus: 4, Reach: 1, Damage: pools})
	s.Require().NoError(err)

	pools[0].Dice = "1d10"
	pools[0].Properties[0] = damage.AddsAttackAbilityModifier

	written := s.written(action)
	persisted := written["damage"].([]any)[0]
	s.Equal(map[string]any{
		"dice":       "1d8",
		"type":       "slashing",
		"properties": []any{"does-not-crit"},
	}, persisted)
}

func (s *DamageConfigTestSuite) TestBiteDamageConfigPreservesSaveGateWithoutKnockdownCompatibility() {
	gate := saves.NewSaveGate(abilities.STR, 11)
	action, err := s.load(refs.MonsterActions.Bite(), map[string]any{
		"attack_bonus": 4,
		"damage":       []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}},
		"save_gate":    gate,
	})
	s.Require().NoError(err)

	bite, ok := action.(*BiteAction)
	s.Require().True(ok)
	s.Equal(gate, bite.SaveGate())

	written := s.written(action)
	s.Contains(written, "damage")
	s.Contains(written, "save_gate")
	s.NotContains(written, "damage_dice")
	s.NotContains(written, "damage_type")
	s.NotContains(written, "knockdown_dc")
}

func (s *DamageConfigTestSuite) TestRangedDamageConfigPersistsCanonicalPools() {
	action, err := s.load(refs.MonsterActions.Ranged(), map[string]any{
		"name":         "shortbow",
		"attack_bonus": 4,
		"range_normal": 16,
		"range_long":   64,
		"damage":       []damage.Damage{{Dice: "1d6", Type: damage.Piercing}},
	})
	s.Require().NoError(err)

	written := s.written(action)
	s.Contains(written, "damage")
	s.NotContains(written, "damage_dice")
	s.NotContains(written, "damage_type")
}
