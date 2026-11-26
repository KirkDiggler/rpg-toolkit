// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// Test 6: AbilityWords and NamedAbilities agree, and what AbilityWords says an
// ability takes is what ValidateParameters accepts.
func TestAbilityWordsAgreeWithNamedAbilitiesAndTheDecode(t *testing.T) {
	words := features.AbilityWords(&features.AbilityWordsInput{}).Words
	var got []string
	for _, w := range words {
		got = append(got, w.Name)
	}
	require.Equal(t, features.NamedAbilities(), got, "same names, same order")

	for _, w := range words {
		ref, ok := features.NamedAbility(w.Name)
		require.True(t, ok, w.Name)
		assert.NotEmpty(t, w.Label, w.Name)
		validate := func(params map[string]any) error {
			raw, err := json.Marshal(params)
			require.NoError(t, err)
			return features.ValidateParameters(&features.ValidateParametersInput{Ref: ref, Parameters: raw})
		}
		atMin := map[string]any{}
		for _, v := range w.Values {
			assert.NotEmpty(t, v.Label, v.Word)
			if v.Required {
				atMin[v.Word] = v.Min
			}
		}
		require.NoError(t, validate(atMin), "every required value at its Min passes: %s", w.Name)

		for _, v := range w.Values {
			below := map[string]any{}
			for k, x := range atMin {
				below[k] = x
			}
			below[v.Word] = v.Min - 1
			err := validate(below)
			require.Error(t, err, "%s %s below its Min is refused", w.Name, v.Word)
			assert.Contains(t, err.Error(), fmt.Sprintf("must be at least %d", v.Min))

			if v.NoMax {
				above := map[string]any{v.Word: v.Min + 1_000_000}
				assert.NoError(t, validate(above), "%s %s has no upper limit", w.Name, v.Word)
			} else {
				require.NoError(t, validate(map[string]any{v.Word: v.Max}))
				require.Error(t, validate(map[string]any{v.Word: v.Max + 1}))
			}
		}
		require.Error(t, validate(map[string]any{"notlisted": 1}), "a word not listed is refused")
	}
}

func TestAbilityWordsHandsOutItsOwnCopy(t *testing.T) {
	first := features.AbilityWords(&features.AbilityWordsInput{})
	before, err := json.Marshal(first)
	require.NoError(t, err)
	first.Words[0].Values[0].Min = -7
	first.Words[0].Name = "changed"
	after, err := json.Marshal(features.AbilityWords(&features.AbilityWordsInput{}))
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))
}
