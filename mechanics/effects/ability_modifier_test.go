// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// The ability-modifier amount names its ability. The name is opaque to this
// module: a test vocabulary reads it, as a rulebook would.
type abilityModifierSuite struct{ suite.Suite }

func TestAbilityModifierSuite(t *testing.T) { suite.Run(t, new(abilityModifierSuite)) }

// abilityFrame is a test frame: modifiers by ability name.
type abilityFrame map[string]int

func abilityInput(amount *effects.Amount) effects.ComposeInput {
	return effects.ComposeInput{
		InstanceKey: "ability", HolderID: "recipient",
		Recipe: machine.Recipe{
			Ref: core.Ref{Module: "test", Type: "effects", ID: "ability"}, Initial: "active", Bag: machine.Bag{},
			States: map[string]machine.State{"active": {Outputs: []jsontext.Value{
				output(effects.Output{Kind: queryKind, Amount: amount}),
			}}},
		},
		OutputVocabulary: effects.OutputVocabulary{
			Kinds: map[effects.KindName]effects.KindSpec{queryKind: {Execution: effects.KindQuery, Assess: row}},
			Amounts: map[effects.AmountName]effects.AmountFn{
				effects.AmountAbilityModifier: func(frame any, a effects.Amount) (effects.AmountResult, error) {
					n := frame.(abilityFrame)[a.AbilityModifier.Ability]
					return effects.AmountResult{Scalar: &n}, nil
				},
			},
		},
	}
}

// Catches a field that drops or renames the ability on the way through JSON.
func (s *abilityModifierSuite) TestTheAuthoredFormRoundTrips() {
	const authored = `{"ability_modifier":{"ability":"dex"}}`
	var amount effects.Amount
	s.Require().NoError(json.Unmarshal([]byte(authored), &amount))
	name, err := amount.Name()
	s.Require().NoError(err)
	s.Equal(effects.AmountAbilityModifier, name)
	s.Require().NotNil(amount.AbilityModifier)
	s.Equal("dex", amount.AbilityModifier.Ability)
	again, err := json.Marshal(amount)
	s.Require().NoError(err)
	s.JSONEq(authored, string(again))
}

// Catches an ability modifier that names nothing being accepted, or one that
// is set beside another amount field.
func (s *abilityModifierSuite) TestAnEmptyNameOrASecondFieldIsRefused() {
	one := 1
	for name, amount := range map[string]effects.Amount{
		"empty":        {AbilityModifier: &effects.AbilityRef{}},
		"blank":        {AbilityModifier: &effects.AbilityRef{Ability: "  "}},
		"two fields":   {Fixed: &one, AbilityModifier: &effects.AbilityRef{Ability: "dex"}},
		"empty object": {},
	} {
		s.Run(name, func() {
			_, err := amount.Name()
			s.Error(err)
		})
	}
	_, err := effects.Compose(abilityInput(&effects.Amount{AbilityModifier: &effects.AbilityRef{}}))
	s.Error(err, "composition validates the amount")
}

// Catches the vocabulary being handed something other than the authored name:
// two parts naming two abilities answer their own ability's modifier.
func (s *abilityModifierSuite) TestTheVocabularyReadsTheNamedAbility() {
	frame := abilityFrame{"dex": -1, "int": 3}
	for ability, want := range map[string]int{"dex": -1, "int": 3} {
		composed, err := effects.Compose(abilityInput(&effects.Amount{AbilityModifier: &effects.AbilityRef{Ability: ability}}))
		s.Require().NoError(err)
		answer, err := composed.CurrentOutputs()[0].Amount(frame)
		s.Require().NoError(err)
		s.Require().NotNil(answer.Scalar)
		s.Equal(want, *answer.Scalar, ability)
	}
}

// Catches a clone that shares the ability name with the original.
func (s *abilityModifierSuite) TestCloneOwnsTheAbility() {
	original := effects.Output{Kind: queryKind, Amount: &effects.Amount{AbilityModifier: &effects.AbilityRef{Ability: "dex"}}}
	clone := original.Clone()
	clone.Amount.AbilityModifier.Ability = "int"
	s.Equal("dex", original.Amount.AbilityModifier.Ability)
}
