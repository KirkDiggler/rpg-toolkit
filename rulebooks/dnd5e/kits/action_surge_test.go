// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package kits_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	surgeUncounted = "`use` is \"action surge\", which gives its user something for its count alone; write `uses` or a `cost`"
	surgeOwnUses   = `[{"use": "action surge", "uses": {"max": 1, "refills": {"on": ["short rest", "long rest"]}}}]`
)

// Test 2 (words): the studio is offered Action Surge, with no `with` words, and
// no code in kits says so.
func TestTheStudioIsOfferedActionSurgeWithNoWithWords(t *testing.T) {
	for _, w := range kits.Words(&kits.WordsInput{}).Words {
		if w.Word != "use" {
			continue
		}
		var names []string
		for _, v := range w.Values {
			names = append(names, v.Word)
			if v.Word == "action surge" {
				assert.Equal(t, "Action Surge", v.Label)
				assert.Empty(t, v.With, "it takes no values")
			}
		}
		assert.Equal(t, []string{"action surge", "rage", "second wind"}, names)
		return
	}
	t.Fatal("no `use` word")
}

// Test 3: refused uncounted where it is written; counted two ways; its values.
func TestActionSurgeIsRefusedUncountedWhereItIsWritten(t *testing.T) {
	_, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder, Lets: json.RawMessage(`[{"use": "action surge"}]`)})
	require.Error(t, err)
	assert.Equal(t, statBlock+": lets[0]: "+surgeUncounted, err.Error())

	_, err = kits.Compile(&kits.CompileInput{
		Holder: statBlockHolder, Charges: json.RawMessage(`{"max": 2}`),
		Lets: json.RawMessage(`[{"use": "action surge", "with": {}}]`),
	})
	require.Error(t, err, "`with: {}` is accepted, and the count is still missing")
	assert.Equal(t, statBlock+": lets[0]: "+surgeUncounted, err.Error())

	t.Run("its own uses", func(t *testing.T) {
		out, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder, Lets: json.RawMessage(surgeOwnUses)})
		require.NoError(t, err)
		grant := out.Kit.Grants["action-surge"]
		assert.Equal(t, *refs.Features.ActionSurge(), grant.Ability)
		assert.Equal(t, "action-surge", grant.Pool)
		assert.Equal(t, 1, grant.Uses)
		assert.Equal(t, 1, out.Kit.Pools["action-surge"].Maximum)
		assert.NoError(t, out.Kit.Validate(statBlockHolder))
	})

	t.Run("a charge", func(t *testing.T) {
		out, err := kits.Compile(&kits.CompileInput{
			Holder: statBlockHolder, Charges: json.RawMessage(`{"max": 2}`),
			Lets: json.RawMessage(`[{"use": "action surge", "with": {}, "cost": {"charges": 1}}]`),
		})
		require.NoError(t, err)
		grant := out.Kit.Grants["action-surge"]
		assert.Equal(t, "charges", grant.Pool)
		assert.Equal(t, 1, grant.Uses)
		assert.NoError(t, out.Kit.Validate(statBlockHolder))
	})

	t.Run("it takes no values", func(t *testing.T) {
		_, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder,
			Lets: json.RawMessage(`[{"use": "action surge", "with": {"level": 1}, "uses": {"max": 1}}]`)})
		require.Error(t, err)
		assert.Equal(t, statBlock+": lets[0] `with` for \"action surge\": action surge takes no values", err.Error())
	})
}

// Test 4: refused uncounted where it is loaded.
func TestActionSurgeIsRefusedUncountedWhereItIsLoaded(t *testing.T) {
	kit := kits.Kit{Grants: map[string]kits.Grant{"action-surge": {Ability: *refs.Features.ActionSurge()}}}
	err := kit.Validate(statBlockHolder)
	require.Error(t, err)
	assert.Equal(t, statBlock+` grant "action-surge" is action surge, which must be paid from a pool`, err.Error())
}
