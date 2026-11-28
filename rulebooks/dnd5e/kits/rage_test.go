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

var itemHolder = kits.Holder{Named: `item "ring"`, Noun: "the item"}

// Test 6: content writes a number for Rage, in a stat block and in an item.
func TestRageCompilesWithANumberAndIsRefusedByLevel(t *testing.T) {
	statBlockLets := `[{"use": "rage", "with": {"damage_bonus": 2}, "uses": {"max": 1}}]`
	itemLets := `[{"use": "rage", "with": {"damage_bonus": 2}, "cost": {"charges": 1}}]`

	for name, tc := range map[string]struct {
		holder  kits.Holder
		charges string
		lets    string
		pool    string
	}{
		"stat block": {statBlockHolder, ``, statBlockLets, "rage"},
		"item":       {itemHolder, `{"max": 2}`, itemLets, "charges"},
	} {
		t.Run(name+" writes a number", func(t *testing.T) {
			in := &kits.CompileInput{Holder: tc.holder, Lets: json.RawMessage(tc.lets)}
			if tc.charges != "" {
				in.Charges = json.RawMessage(tc.charges)
			}
			out, err := kits.Compile(in)
			require.NoError(t, err)
			grant := out.Kit.Grants["rage"]
			assert.Equal(t, *refs.Features.Rage(), grant.Ability)
			assert.JSONEq(t, `{"damage_bonus":2}`, string(grant.Parameters))
			assert.Equal(t, tc.pool, grant.Pool)
			assert.NoError(t, out.Kit.Validate(tc.holder))
		})

		t.Run(name+" is refused the by-level form", func(t *testing.T) {
			lets := `[{"use": "rage", "with": {"damage_bonus": "barbarian level"}, "uses": {"max": 1}}]`
			in := &kits.CompileInput{Holder: tc.holder, Lets: json.RawMessage(lets)}
			_, err := kits.Compile(in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.holder.Named)
			assert.Contains(t, err.Error(), `"rage"`)
			assert.Contains(t, err.Error(), `rage damage_bonus is "barbarian level"; it is a whole number of at least 1`)
		})

		t.Run(name+" is refused with no with", func(t *testing.T) {
			_, err := kits.Compile(&kits.CompileInput{Holder: tc.holder, Lets: json.RawMessage(`[{"use": "rage", "uses": {"max": 1}}]`)})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "rage")
			assert.Contains(t, err.Error(), "damage_bonus")
		})
	}

	t.Run("rage needs no count", func(t *testing.T) {
		out, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder,
			Lets: json.RawMessage(`[{"use": "rage", "with": {"damage_bonus": 2}}]`)})
		require.NoError(t, err)
		assert.Equal(t, *refs.Features.Rage(), out.Kit.Grants["rage"].Ability)
	})
}
