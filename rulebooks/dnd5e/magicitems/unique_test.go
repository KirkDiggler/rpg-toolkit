// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const uniqueRest = `"name": "Lieutenant's cloak", "slot": "cloak",
  "charges": {"max": 1, "refills": {"on": ["short rest", "long rest"]}},
  "lets": [{"use": "second wind", "with": {"level": 1}, "cost": {"charges": 1}}]`

func withUnique(value string) string {
	return `{` + uniqueRest + `, "unique": ` + value + `}`
}

// cloakJSONBeforeUnique was captured from cloakBlock compiled on origin/main
// (898a28ec) before the field existed.
const cloakJSONBeforeUnique = `{"ref":{"module":"site","type":"items","id":"lieutenants-cloak"},"name":"Lieutenant's cloak","slots":["cloak"],"pools":{"charges":{"maximum":1,"restore_on":["short_rest","long_rest"]}},"grants":{"second-wind":{"ability":{"module":"dnd5e","type":"features","id":"second_wind"},"parameters":{"level":1},"pool":"charges","uses":1}}}`

// Test 1
func TestCompileUniqueTrueIsUnique(t *testing.T) {
	d := mustCompile(t, "lieutenants-cloak", withUnique("true"))
	assert.True(t, d.Unique)
	require.NoError(t, d.Validate())
}

// Test 2
func TestCompileWithoutUniqueMarshalsNoUniqueKey(t *testing.T) {
	d := mustCompile(t, "lieutenants-cloak", cloakBlock)
	assert.False(t, d.Unique)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, cloakJSONBeforeUnique, string(raw))

	d = mustCompile(t, "lieutenants-cloak", withUnique("false"))
	assert.False(t, d.Unique)
	raw, err = json.Marshal(d)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "unique")
	assert.Equal(t, cloakJSONBeforeUnique, string(raw))
}

// Test 3
func TestCompileUniqueMustBeTrueOrFalse(t *testing.T) {
	for _, value := range []string{`"yes"`, `1`, `null`, `{}`} {
		out, err := compile(t, "lieutenants-cloak", withUnique(value))
		require.Error(t, err, value)
		assert.Nil(t, out)
		assert.Equal(t, "item \"lieutenants-cloak\": `unique` is true or false", err.Error(), value)
	}
}

// Test 4
func TestCompileUnknownWordListsUniqueInOrder(t *testing.T) {
	_, err := compile(t, "c", strings.Replace(cloakBlock, `"slot"`, `"colour": "red", "slot"`, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "it is one of name, slot, unique, charges, lets")
}

// Test 5
func TestCloneOfUniqueIsUniqueAndIndependent(t *testing.T) {
	d := mustCompile(t, "lieutenants-cloak", withUnique("true"))
	clone := d.Clone()
	assert.True(t, clone.Unique)
	clone.Unique = false
	assert.True(t, d.Unique)
}
