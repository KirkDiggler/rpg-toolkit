// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// TestStrictJSONObjectRefusesTrailingContent pins the walker's own boundary:
// one JSON object and nothing after it.
func TestStrictJSONObjectRefusesTrailingContent(t *testing.T) {
	_, ok := strictJSONObject([]byte(`{"beat":"struck"}`))
	require.True(t, ok, "the well-formed object itself still parses")
	_, ok = strictJSONObject([]byte(`{"beat":"struck"} 42`))
	require.False(t, ok, "a trailing JSON value is trailing content")
	_, ok = strictJSONObject([]byte(`{"beat":"struck"} garbage`))
	require.False(t, ok, "and so is trailing garbage")
	_, ok = strictJSONObject([]byte(`{"beat":"struck"} {"beat":"moved"}`))
	require.False(t, ok, "a trailing object is still trailing content")
}

// TestActivationResultsMapEveryProviderFieldInOrder guards the persistence
// adapter rather than one currently-produced ability at a time. Resolution's
// effect values are authoritative; Session copies every field — including the
// healing's sourced roll calculation, cloned so no captured trace aliases the
// provider's graph — and preserves publication order without parsing refs or
// redoing arithmetic.
func TestActivationResultsMapEveryProviderFieldInOrder(t *testing.T) {
	three := 3
	effects := []resolution.ActivationEffect{
		{
			Kind: resolution.EffectHealingApplied, TargetID: "alice",
			Ref: "dnd5e:features:second_wind", Name: "Second Wind",
			Amount: 2, Requested: 11, Before: 28, After: 30,
			Calculation: &dnd5eEvents.RollCalculation{
				Components: []dnd5eEvents.RollComponent{
					{
						Source: dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind", SourceID: "alice"},
						Dice: &dnd5eEvents.DiceTrace{
							Notation: "1d10", DieSize: 10,
							OriginalRolls: []int{8}, FinalRolls: []int{8}, Subtotal: 8,
						},
					},
					{
						Source: dnd5eEvents.RollSource{
							Ref: refs.Classes.Fighter(), Name: "Fighter", Label: "Fighter level",
						},
						Modifier: &three,
					},
				},
				Total: 11,
			},
		},
		{
			Kind: resolution.EffectConditionApplied, Name: "Raging",
			Address: dnd5eEvents.ConditionAddress{
				MemberID: "bob", ConditionRef: "dnd5e:conditions:raging", SourceID: "alice",
			},
		},
		{
			Kind: resolution.EffectConditionRemoved, Name: "Hidden", Reason: "revealed",
			Address: dnd5eEvents.ConditionAddress{
				MemberID: "carol", ConditionRef: "dnd5e:conditions:hidden", SourceID: "alice",
			},
		},
		{
			Kind: resolution.EffectCapacityGranted, TargetID: "dave",
			Description: "30ft movement",
		},
	}

	results := activationResults(effects)
	require.Len(t, results, 4)
	require.Equal(t, encounter.ResultHealingApplied, results[0].Kind)
	require.Equal(t, encounter.MemberID("alice"), results[0].Target)
	require.Equal(t, 2, results[0].Amount)
	require.Equal(t, 11, results[0].Requested)
	require.Equal(t, 28, results[0].Before)
	require.Equal(t, 30, results[0].After)
	require.NotNil(t, results[0].Calculation)
	require.Equal(t, 11, results[0].Calculation.Total)
	require.Len(t, results[0].Calculation.Components, 2)
	require.Equal(t, "dnd5e:features:second_wind", results[0].Calculation.Components[0].Source.Ref)
	require.Equal(t, "Second Wind", results[0].Calculation.Components[0].Source.Name)
	require.Equal(t, []int{8}, results[0].Calculation.Components[0].Dice.OriginalRolls)
	require.Equal(t, 8, results[0].Calculation.Components[0].Dice.Subtotal)
	require.Equal(t, "dnd5e:classes:fighter", results[0].Calculation.Components[1].Source.Ref)
	require.Equal(t, "Fighter level", results[0].Calculation.Components[1].Source.Label)
	require.NotNil(t, results[0].Calculation.Components[1].Modifier)
	require.Equal(t, 3, *results[0].Calculation.Components[1].Modifier)

	require.Equal(t, encounter.ActivationResult{
		Kind: encounter.ResultConditionApplied,
		Address: &encounter.ConditionAddress{
			MemberID: "bob", ConditionKey: encounter.ConditionKey{ConditionRef: "dnd5e:conditions:raging", SourceID: "alice"},
		},
		Name: "Raging", Calculation: nil, // the non-healing kinds carry no calculation
	}, results[1])
	require.Equal(t, encounter.ActivationResult{
		Kind: encounter.ResultConditionRemoved,
		Address: &encounter.ConditionAddress{
			MemberID: "carol", ConditionKey: encounter.ConditionKey{ConditionRef: "dnd5e:conditions:hidden", SourceID: "alice"},
		},
		Name: "Hidden", Reason: "revealed",
	}, results[2])
	require.Equal(t, encounter.ActivationResult{
		Kind: encounter.ResultCapacityGranted, Target: "dave",
		Description: "30ft movement",
	}, results[3])

	// No aliasing: mutating the provider's captured graph after projection
	// cannot rewrite what persistence recorded.
	effects[0].Calculation.Components[0].Dice.OriginalRolls[0] = 99
	effects[0].Calculation.Components[1].Modifier = nil
	require.Equal(t, []int{8}, results[0].Calculation.Components[0].Dice.OriginalRolls)
	require.NotNil(t, results[0].Calculation.Components[1].Modifier)
	require.Equal(t, 3, *results[0].Calculation.Components[1].Modifier)
	require.NotSame(t, effects[0].Calculation, results[0].Calculation)
	require.NotSame(t, effects[0].Calculation.Components[0].Dice,
		results[0].Calculation.Components[0].Dice)
}

func TestRollCalculationDecodesSourceQualifiedSubtractionStrictly(t *testing.T) {
	valid := json.RawMessage(`{"components":[` +
		`{"source":{"ref":"dnd5e:actions:death_save","name":"Death Saving Throw","source_id":"alice"},"dice":{"notation":"1d20","die_size":20,"original_rolls":[10],"final_rolls":[10],"subtotal":10}},` +
		`{"source":{"ref":"dnd5e:spells:bane","name":"Bane","source_id":"bard-a"},"dice":{"notation":"1d4","die_size":4,"original_rolls":[3],"final_rolls":[3],"subtotal":3},"subtract_dice":true}],"total":7}`)
	calculation, ok := decodeRollCalculation(valid)
	require.True(t, ok)
	require.Equal(t, 7, calculation.Total)
	require.True(t, calculation.Components[1].SubtractDice)
	require.Equal(t, "bard-a", calculation.Components[1].Source.SourceID)

	for name, raw := range map[string]json.RawMessage{
		"subtraction without source id": bytes.Replace(valid, []byte(`,"source_id":"bard-a"`), nil, 1),
		"missing source ref":            bytes.Replace(valid, []byte(`"ref":"dnd5e:spells:bane",`), nil, 1),
		"mismatched total":              bytes.Replace(valid, []byte(`"total":7`), []byte(`"total":8`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, accepted := decodeRollCalculation(raw)
			require.False(t, accepted)
		})
	}
}

// TestAnImposedConditionCarriesItsDetailIntoTheResult covers the cast and
// retaliation path: resolution's imposed condition hands its detail to the
// encounter result unchanged, and an imposed condition with none leaves it empty.
func TestAnImposedConditionCarriesItsDetailIntoTheResult(t *testing.T) {
	ref := refs.Conditions.Commanded()
	imposed := func(detail string) resolution.ImposedEffect {
		return resolution.ImposedEffect{
			Kind: resolution.ImposedCondition, RecipientID: "skeleton", Ref: ref,
			Address: dnd5eEvents.ConditionAddress{MemberID: "skeleton", ConditionRef: ref.String(), SourceID: "bard"},
			Detail:  detail,
		}
	}

	result, err := imposedResult(imposed("approach"), SpellRef{})
	require.NoError(t, err)
	require.Equal(t, "approach", result.Detail)

	result, err = imposedResult(imposed(""), SpellRef{})
	require.NoError(t, err)
	require.Empty(t, result.Detail)
}

// TestAnActivationEffectCarriesItsDetailIntoTheResult is the activation half:
// the effect's detail is copied onto the result unchanged.
func TestAnActivationEffectCarriesItsDetailIntoTheResult(t *testing.T) {
	results := activationResults([]resolution.ActivationEffect{{
		Kind: resolution.EffectConditionApplied, Name: "Commanded",
		Address: dnd5eEvents.ConditionAddress{MemberID: "skeleton", ConditionRef: "dnd5e:conditions:commanded"},
		Detail:  "grovel",
	}})
	require.Len(t, results, 1)
	require.Equal(t, "grovel", results[0].Detail)
}
