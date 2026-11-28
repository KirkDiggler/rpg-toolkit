// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

func rageItemGiver(t *testing.T) dnd5eEvents.RollSource {
	t.Helper()
	ref, err := core.ParseString("example:items:rage_ring")
	require.NoError(t, err)
	return dnd5eEvents.RollSource{Ref: ref, Name: "Rage ring", Label: "Rage ring", SourceID: "ring-1"}
}

func describeRageFor(t *testing.T, params string, pools map[coreResources.ResourceKey]int) (*actions.Definition, error) {
	t.Helper()
	return DescribeFromRef(&DescribeFromRefInput{
		Ref: *refs.Features.Rage(), Parameters: json.RawMessage(params), Giver: rageItemGiver(t), Pools: pools,
	})
}

func ragingBindings(t *testing.T, d *actions.Definition) conditions.RagingBindings {
	t.Helper()
	var b conditions.RagingBindings
	require.NoError(t, json.Unmarshal(d.Effect.Condition.Parameters, &b))
	return b
}

// Test 1: the word. Fails before the change (no entry); not a pin.
func TestRageIsFoundByAllFive(t *testing.T) {
	ref, ok := NamedAbility("rage")
	require.True(t, ok)
	assert.Equal(t, *refs.Features.Rage(), ref)
	assert.Equal(t, []string{"action surge", "rage", "second wind"}, NamedAbilities())
	assert.False(t, MustBeCounted(ref))

	var word AbilityWord
	for _, w := range AbilityWords(&AbilityWordsInput{}).Words {
		if w.Name == "rage" {
			word = w
		}
	}
	assert.Equal(t, "Rage", word.Label)
	assert.False(t, word.Counted)
	assert.Equal(t, []AbilityValue{{
		Word: "damage_bonus", Required: true, Min: 1, NoMax: true,
		Label: "adds {value} to the damage of each melee weapon hit made with strength",
	}}, word.Values)
}

// Test 2: described with a number.
func TestRageDescribedWithANumber(t *testing.T) {
	pools := map[coreResources.ResourceKey]int{"ring:charges": 1}
	d, err := describeRageFor(t, `{"damage_bonus": 2}`, pools)
	require.NoError(t, err)
	assert.Equal(t, *refs.Features.Rage(), d.Ref)
	assert.Equal(t, "Rage", d.Name)
	assert.Contains(t, d.Description, "add 2 to the damage of melee weapon hits made with Strength")
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, d.Cost.Slots)
	assert.Equal(t, pools, d.Cost.Pools)
	assert.Empty(t, d.Cost.GrantSlots)
	assert.Equal(t, actions.EffectSelf, d.Effect.To)
	assert.Equal(t, *refs.Conditions.Raging(), d.Effect.Condition.Ref)
	assert.JSONEq(t, `{"damage_bonus":{"fixed":2}}`, string(d.Effect.Condition.Parameters))
	two := 2
	assert.Equal(t, conditions.RagingBindings{DamageBonus: conditions.RagingDamageBonus{Fixed: &two}}, ragingBindings(t, d))
	require.NoError(t, d.Validate())

	// The same parameters build the raging status, as rage_described_test.go
	// proves for the by-level form: CreateFromRef accepts them.
	_, err = conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref: refs.Conditions.Raging().String(), MemberID: "wearer",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "wearer"},
		Config: d.Effect.Condition.Parameters,
	})
	require.NoError(t, err)

	none, err := describeRageFor(t, `{"damage_bonus": 2}`, nil)
	require.NoError(t, err, "not counted: no pools is fine")
	assert.Empty(t, none.Cost.Pools)
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, none.Cost.Slots)
	require.NoError(t, none.Validate())
}

// Test 3: described by barbarian level.
func TestRageDescribedByBarbarianLevel(t *testing.T) {
	d, err := describeRageFor(t, `{"damage_bonus": "barbarian level"}`, nil)
	require.NoError(t, err)
	assert.Equal(t, conditions.RagingBindings{DamageBonus: conditions.RagingDamageBonus{BarbarianLevel: true}}, ragingBindings(t, d))
	assert.Equal(t, newRageForTest("rage-feature").Description(), d.Description)
	require.NoError(t, d.Validate())
}

// Test 4: the check refuses.
func TestRageCheckRefuses(t *testing.T) {
	for name, raw := range map[string]json.RawMessage{
		"no parameters": nil,
		"empty object":  json.RawMessage(`{}`),
		"null":          json.RawMessage(`null`),
		"unknown":       json.RawMessage(`{"damage_bonus": 2, "level": 1}`),
		"zero":          json.RawMessage(`{"damage_bonus": 0}`),
		"negative":      json.RawMessage(`{"damage_bonus": -1}`),
		"fraction":      json.RawMessage(`{"damage_bonus": 2.5}`),
		"other string":  json.RawMessage(`{"damage_bonus": "fighter level"}`),
		"object":        json.RawMessage(`{"damage_bonus": {"fixed": 2}}`),
		"null value":    json.RawMessage(`{"damage_bonus": null}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := describeRageFor(t, string(raw), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "rage")
			assert.Contains(t, err.Error(), "damage_bonus")
			assert.Error(t, ValidateParameters(&ValidateParametersInput{Ref: *refs.Features.Rage(), Parameters: raw}))
		})
	}
}

// Test 5: the shared judgments.
func TestRageGiverAndPoolsAreJudgedAsForEveryEntry(t *testing.T) {
	_, err := DescribeFromRef(&DescribeFromRefInput{
		Ref: *refs.Features.Rage(), Parameters: json.RawMessage(`{"damage_bonus":2}`),
		Giver: dnd5eEvents.RollSource{Label: "x", SourceID: "x"},
	})
	require.Error(t, err)
	assert.Equal(t, "rage giver requires a Ref", err.Error())

	_, err = describeRageFor(t, `{"damage_bonus":2}`, map[coreResources.ResourceKey]int{"ring:charges": 0})
	require.Error(t, err)
	assert.Equal(t, `rage pool "ring:charges" amount 0 must be at least 1`, err.Error())
}

// Test 6 (part): content writes a number, never the by-level form.
func TestContentWritesANumberForRage(t *testing.T) {
	rage := *refs.Features.Rage()
	require.NoError(t, ValidateParameters(&ValidateParametersInput{Ref: rage, Parameters: json.RawMessage(`{"damage_bonus": 2}`)}))

	err := ValidateParameters(&ValidateParametersInput{Ref: rage, Parameters: json.RawMessage(`{"damage_bonus": "barbarian level"}`)})
	require.Error(t, err)
	assert.Equal(t, `rage damage_bonus is "barbarian level"; it is a whole number of at least 1`, err.Error())

	// The class feature's own description is unaffected: DescribeFromRef runs check only.
	_, err = describeRageFor(t, `{"damage_bonus": "barbarian level"}`, nil)
	require.NoError(t, err)

	// Second Wind and Action Surge refuse with the sentences they always did.
	err = ValidateParameters(&ValidateParametersInput{
		Ref: *refs.Features.SecondWind(), Parameters: json.RawMessage(`{"level": 0}`)})
	require.Error(t, err)
	assert.Equal(t, "second wind level 0 must be at least 1", err.Error())
	err = ValidateParameters(&ValidateParametersInput{
		Ref: *refs.Features.ActionSurge(), Parameters: json.RawMessage(`{"level": 1}`)})
	require.Error(t, err)
	assert.Equal(t, "action surge takes no values", err.Error())
}

// Test 7a: the barbarian's own Rage, pinned to a literal written from the code
// before the change.
func TestBarbariansRageDefinitionIsWhatItWas(t *testing.T) {
	for _, level := range []int{1, 20} {
		rage := newRageForTest("rage-feature")
		d, err := rage.Definition(newStubEntityWithRage("barbarian-1", level))
		require.NoError(t, err)

		want := &actions.Definition{
			Ref:         *refs.Features.Rage(),
			Name:        "Rage",
			Description: rage.Description(),
			Cost:        costOf(level),
			Effect: &actions.EffectProfile{
				To: actions.EffectSelf,
				Condition: &actions.ConditionApplication{
					Ref:        *refs.Conditions.Raging(),
					Parameters: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`),
				},
			},
		}
		assert.Equal(t, want, d, "level %d", level)
	}
}

// Test 7b: and it equals what the entry gives for Design 3's input.
func TestBarbariansRageIsTheEntrysDescription(t *testing.T) {
	rage := newRageForTest("rage-feature")
	owner := newStubEntityWithRage("barbarian-1", 3)
	d, err := rage.Definition(owner)
	require.NoError(t, err)

	want, err := DescribeFromRef(&DescribeFromRefInput{
		Ref:        *refs.Features.Rage(),
		Parameters: json.RawMessage(`{"damage_bonus": "barbarian level"}`),
		Giver: dnd5eEvents.RollSource{
			Ref: refs.Classes.Barbarian(), Name: "Barbarian", Label: "Barbarian", SourceID: owner.GetID(),
		},
		Pools: map[coreResources.ResourceKey]int{resources.RageCharges: 1},
	})
	require.NoError(t, err)
	want.Description = rage.Description()
	assert.Equal(t, want, d)
}

func costOf(level int) *dnd5eCombat.SpendProfile {
	cost := &dnd5eCombat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}}
	if level < 20 {
		cost.Pools = map[coreResources.ResourceKey]int{resources.RageCharges: 1}
	}
	return cost
}
