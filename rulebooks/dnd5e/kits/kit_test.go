// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package kits_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

func secondWind(t *testing.T, pool string, uses int) kits.Grant {
	t.Helper()
	ref, ok := features.NamedAbility("second wind")
	require.True(t, ok)
	return kits.Grant{Ability: ref, Parameters: json.RawMessage(`{"level":1}`), Pool: pool, Uses: uses}
}

// Test 5: a clone shares nothing, and a description's Kit is a copy.
func TestAKitsCloneSharesNothing(t *testing.T) {
	k := kits.Kit{
		Pools:  map[string]kits.Pool{"charges": {Maximum: 2, RestoreOn: []coreResources.ResetType{coreResources.ResetLongRest}}},
		Grants: map[string]kits.Grant{"second-wind": secondWind(t, "charges", 1)},
	}
	clone := k.Clone()
	require.Equal(t, k, clone)

	clone.Pools["charges"].RestoreOn[0] = coreResources.ResetShortRest
	clone.Pools["other"] = kits.Pool{Maximum: 1}
	clone.Grants["second-wind"].Parameters[2] = 'X'
	clone.Grants["extra"] = kits.Grant{}

	assert.Equal(t, coreResources.ResetLongRest, k.Pools["charges"].RestoreOn[0])
	assert.NotContains(t, k.Pools, "other")
	assert.JSONEq(t, `{"level":1}`, string(k.Grants["second-wind"].Parameters))
	assert.NotContains(t, k.Grants, "extra")
}

func TestACloneKeepsANilMapNil(t *testing.T) {
	clone := kits.Kit{}.Clone()
	assert.Nil(t, clone.Pools)
	assert.Nil(t, clone.Grants)
}

func TestADescriptionsKitIsACopy(t *testing.T) {
	d := magicitems.Description{
		Pools:  map[string]magicitems.Pool{"charges": {Maximum: 2, RestoreOn: []coreResources.ResetType{coreResources.ResetLongRest}}},
		Grants: map[string]magicitems.Grant{"second-wind": secondWind(t, "charges", 1)},
	}
	k := d.Kit()
	require.Equal(t, d.Pools, k.Pools)
	require.Equal(t, d.Grants, k.Grants)

	k.Pools["charges"].RestoreOn[0] = coreResources.ResetShortRest
	delete(k.Pools, "charges")
	k.Grants["second-wind"].Parameters[2] = 'X'

	assert.Equal(t, coreResources.ResetLongRest, d.Pools["charges"].RestoreOn[0])
	assert.Contains(t, d.Pools, "charges")
	assert.JSONEq(t, `{"level":1}`, string(d.Grants["second-wind"].Parameters))
}

// A kit refuses with the holder's name at the head of every sentence, and does
// not require a grant: "an item grants nothing" is the item's own rule.
func TestAKitValidatesUnderTheHoldersName(t *testing.T) {
	holder := kits.Holder{Named: `stat block "gate-lieutenant"`, Noun: "the stat block"}
	assert.NoError(t, kits.Kit{}.Validate(holder), "a kit with no grant is the holder's rule to refuse")

	cases := []struct {
		name string
		kit  kits.Kit
		want string
	}{
		{"pool maximum", kits.Kit{Pools: map[string]kits.Pool{"charges": {Maximum: 0}}},
			`stat block "gate-lieutenant" pool "charges" maximum 0 must be at least 1`},
		{"pool name", kits.Kit{Pools: map[string]kits.Pool{"bad name": {Maximum: 1}}},
			`stat block "gate-lieutenant" pool name "bad name" must be letters, digits, '-' and '_'`},
		{"grant name", kits.Kit{Grants: map[string]kits.Grant{"bad name": secondWind(t, "", 0)}},
			`stat block "gate-lieutenant" grant name "bad name" must be letters, digits, '-' and '_'`},
		{"undeclared pool", kits.Kit{Grants: map[string]kits.Grant{"a": secondWind(t, "charges", 1)}},
			`stat block "gate-lieutenant" grant "a" names pool "charges", which the stat block does not declare`},
		{"uses with no pool", kits.Kit{Grants: map[string]kits.Grant{"a": secondWind(t, "", 2)}},
			`stat block "gate-lieutenant" grant "a" has uses 2 but names no pool to pay from`},
		{"uses below one", kits.Kit{
			Pools:  map[string]kits.Pool{"charges": {Maximum: 1}},
			Grants: map[string]kits.Grant{"a": secondWind(t, "charges", 0)}},
			`stat block "gate-lieutenant" grant "a" pays from pool "charges" but its uses 0 is below 1`},
		{"rest twice", kits.Kit{Pools: map[string]kits.Pool{"charges": {Maximum: 1,
			RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetShortRest}}}},
			`stat block "gate-lieutenant" pool "charges" restore_on names "short_rest" more than once`},
		{"not a rest", kits.Kit{Pools: map[string]kits.Pool{"charges": {Maximum: 1,
			RestoreOn: []coreResources.ResetType{"dawn"}}}},
			`stat block "gate-lieutenant" pool "charges" restore_on "dawn" must be short_rest or long_rest`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.kit.Validate(holder)
			require.Error(t, err)
			assert.Equal(t, tc.want, err.Error())
			assert.NotContains(t, err.Error(), "item", "a stat block's refusal never says item")
		})
	}
}

// Test 7: dependencies run one way.
func TestKitsDoesNotDependOnItsHolders(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)
	for _, dep := range strings.Fields(string(out)) {
		for _, holder := range []string{"/rulebooks/dnd5e/magicitems", "/rulebooks/dnd5e/character", "/rulebooks/dnd5e/monster"} {
			assert.False(t, strings.HasSuffix(dep, holder), "kits depends on %s", dep)
		}
	}
	assert.Contains(t, string(out), "rulebooks/dnd5e/features", "the kit reads the named abilities")
}

// An item's saved shape is its own: `grants` is written even when empty, and
// the keys keep the order they were always written in. This is why a
// description holds its two maps as fields and does not embed a kit, whose
// own `grants` tag omits an empty map.
func TestAnItemsSavedShapeIsItsOwnAndNotTheKits(t *testing.T) {
	raw, err := json.Marshal(magicitems.Description{
		Name: "N", Slots: []string{"cloak"},
		Pools:  map[string]magicitems.Pool{"charges": {Maximum: 1}},
		Grants: map[string]magicitems.Grant{},
	})
	require.NoError(t, err)
	assert.Equal(t,
		`{"ref":{"module":"","type":"","id":""},"name":"N","slots":["cloak"],"pools":{"charges":{"maximum":1}},"grants":{}}`,
		string(raw))
}
