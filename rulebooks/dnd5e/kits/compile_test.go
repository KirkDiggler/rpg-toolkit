// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package kits_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

const (
	statBlock = `stat block "gate-lieutenant"`
	charges   = `{"max": 2, "refills": {"on": ["short rest", "long rest"]}}`
	letsPaid  = `[{"use": "second wind", "with": {"level": 1}, "cost": {"charges": 1}}]`
)

// Test 4: a kit alone compiles to what the same words give an item.
func TestAKitAloneCompilesToWhatTheSameWordsGiveAnItem(t *testing.T) {
	out, err := kits.Compile(&kits.CompileInput{
		Holder:  statBlock,
		Charges: json.RawMessage(charges),
		Lets:    json.RawMessage(letsPaid),
	})
	require.NoError(t, err)

	item, err := magicitems.Compile(&magicitems.CompileInput{
		ID:       "it",
		Authored: json.RawMessage(`{"name": "N", "slot": "cloak", "charges": ` + charges + `, "lets": ` + letsPaid + `}`),
	})
	require.NoError(t, err)

	assert.Equal(t, item.Description.Pools, out.Kit.Pools)
	assert.Equal(t, item.Description.Grants, out.Kit.Grants)
	assert.Equal(t, map[string]kits.Pool{"charges": {
		Maximum: 2, RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetLongRest},
	}}, out.Kit.Pools)
	require.Contains(t, out.Kit.Grants, "second-wind")
	assert.Equal(t, "charges", out.Kit.Grants["second-wind"].Pool)
	assert.Equal(t, 1, out.Kit.Grants["second-wind"].Uses)
	assert.NoError(t, out.Kit.Validate(statBlock))
}

func TestAKitWithNoChargesAndOneEntryWithNoCostIsOneGrantAndNoPools(t *testing.T) {
	out, err := kits.Compile(&kits.CompileInput{
		Holder: statBlock,
		Lets:   json.RawMessage(`[{"use": "second wind", "with": {"level": 1}}]`),
	})
	require.NoError(t, err)
	assert.Nil(t, out.Kit.Pools)
	require.Len(t, out.Kit.Grants, 1)
	assert.Empty(t, out.Kit.Grants["second-wind"].Pool)
	assert.Zero(t, out.Kit.Grants["second-wind"].Uses)
}

// Whether a holder must let something be used is the holder's rule: the kit
// compiles an absent or empty `lets` to no grants.
func TestAbsentOrEmptyLetsIsNoGrantsForTheHolderToJudge(t *testing.T) {
	for name, lets := range map[string]json.RawMessage{"absent": nil, "empty": json.RawMessage(`[]`)} {
		t.Run(name, func(t *testing.T) {
			out, err := kits.Compile(&kits.CompileInput{Holder: statBlock, Lets: lets})
			require.NoError(t, err)
			assert.Empty(t, out.Kit.Grants)
		})
	}
}

// Test 4, refusals: the holder's name at the head, the item's sentence after.
func TestEveryRefusalReadsUnderTheHoldersNameWithTheItemsSentence(t *testing.T) {
	cases := []struct {
		name    string
		charges string
		lets    string
		want    string
	}{
		{"unknown charges word", `{"max": 1, "regain": 1}`, ``, "`charges`: unknown word `regain`; it is one of max, refills"},
		{"unknown refills word", `{"max": 1, "refills": {"on": ["long rest"], "when": 1}}`, ``, "`refills`: unknown word `when`; it is one of on"},
		{"unknown lets word", ``, `[{"use": "second wind", "with": {"level": 1}, "when": 1}]`, "lets[0]: unknown word `when`; it is one of use, with, cost"},
		{"unknown cost word", `{"max": 2}`, `[{"use": "second wind", "with": {"level": 1}, "cost": {"gold": 1}}]`, "lets[0] `cost`: unknown word `gold`; it is one of charges"},
		{"bad rest", `{"max": 1, "refills": {"on": ["dawn"]}}`, ``, "`refills.on` has \"dawn\"; it is one of short rest, long rest"},
		{"bad use", ``, `[{"use": "fireball"}]`, "lets[0]: `use` is \"fireball\"; it is one of second wind"},
		{"max zero", `{"max": 0}`, ``, "`charges.max` is 0; it is a whole number from 1 to 1000"},
		{"max over", `{"max": 1001}`, ``, "`charges.max` is 1001; it is a whole number from 1 to 1000"},
		{"cost over max", `{"max": 2}`, `[{"use": "second wind", "with": {"level": 1}, "cost": {"charges": 3}}]`, "lets[0] cost.charges is 3; the item holds at most 2"},
		{"cost with no charges", ``, `[{"use": "second wind", "with": {"level": 1}, "cost": {"charges": 1}}]`, "lets[0] `cost` is paid in `charges`, but the item declares no `charges`"},
		{"level zero", ``, `[{"use": "second wind", "with": {"level": 0}}]`, "lets[0] `with` for \"second wind\": second wind level 0 must be at least 1"},
		{"lets not a list", ``, `{"use": "second wind"}`, "`lets` must be a list of entries"},
		{"same ability twice", ``, `[{"use": "second wind", "with": {"level": 1}}, {"use": "second wind", "with": {"level": 2}}]`, "`use` names \"second wind\" more than once; each entry uses a different ability"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := kits.Compile(&kits.CompileInput{Holder: statBlock, Charges: json.RawMessage(tc.charges), Lets: json.RawMessage(tc.lets)})
			require.Error(t, err)
			assert.Equal(t, statBlock+": "+tc.want, err.Error())

			// The same words in an item read the same after the item's name.
			authored := `{"name": "N", "slot": "cloak"`
			if tc.charges != "" {
				authored += `, "charges": ` + tc.charges
			}
			if tc.lets != "" {
				authored += `, "lets": ` + tc.lets
			} else {
				authored += `, "lets": [{"use": "second wind", "with": {"level": 1}}]`
			}
			_, itemErr := magicitems.Compile(&magicitems.CompileInput{ID: "it", Authored: json.RawMessage(authored + `}`)})
			require.Error(t, itemErr)
			assert.Equal(t, `item "it": `+tc.want, itemErr.Error())
		})
	}
}

func TestCompilingNothingIsRefused(t *testing.T) {
	_, err := kits.Compile(nil)
	assert.Error(t, err)
}
